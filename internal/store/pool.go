package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/maveonair/farm/internal/reconcile"
)

const maxIncidentMessage = 4096

func (d *DB) StartPool(ctx context.Context, run PoolRun) error {
	if run.Pool == "" || run.RunID == "" {
		return errors.New("pool and run ID are required")
	}
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO pool_runtime (pool, run_id, reconcile_state, reconcile_started_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(pool) DO UPDATE SET
			run_id = excluded.run_id,
			reconcile_state = excluded.reconcile_state,
			reconcile_started_at = excluded.reconcile_started_at,
			reconcile_finished_at = NULL
	`, run.Pool, run.RunID, RuntimeRunning, run.StartedAt.UTC())
	if err != nil {
		return fmt.Errorf("start pool reconciliation: %w", err)
	}
	return nil
}

func (d *DB) ObservePool(ctx context.Context, observation PoolObservation) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE pool_runtime SET waiting_jobs = ?, observed_at = ?
		WHERE pool = ? AND run_id = ?
	`, observation.Waiting, observation.ObservedAt.UTC(), observation.Pool, observation.RunID)
	if err != nil {
		return fmt.Errorf("record pool observation: %w", err)
	}
	return requireUpdate(result)
}

func (d *DB) ReserveBootstrap(ctx context.Context, request BootstrapRequest) (BootstrapReservation, error) {
	if request.Pool == "" || request.Wanted < 1 || request.Limit < 1 {
		return BootstrapReservation{}, errors.New("pool, wanted attempts, and attempt limit are required")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return BootstrapReservation{}, fmt.Errorf("begin bootstrap reservation: %w", err)
	}
	defer tx.Rollback()

	var state BootstrapState
	if err := tx.QueryRowContext(ctx, `
		SELECT bootstrap_attempts, bootstrap_retry_at FROM pool_runtime WHERE pool = ?
	`, request.Pool).Scan(&state.Attempts, newNullTime(&state.RetryAt)); err != nil {
		return BootstrapReservation{}, fmt.Errorf("read bootstrap state: %w", err)
	}

	reservation := BootstrapReservation{State: state}
	if state.Attempts >= request.Limit {
		if !state.RetryAt.IsZero() && request.Now.Before(state.RetryAt) {
			if err := tx.Commit(); err != nil {
				return BootstrapReservation{}, fmt.Errorf("commit bootstrap reservation: %w", err)
			}
			return reservation, nil
		}
		// A missing retry time means FARM stopped during the limiting attempt.
		if state.RetryAt.IsZero() {
			reservation.State.RetryAt = request.RetryAt.UTC()
		} else {
			reservation.Count = 1
			reservation.Probe = true
			reservation.State.RetryAt = request.RetryAt.UTC()
		}
	} else {
		reservation.Count = min(request.Wanted, request.Limit-state.Attempts)
		reservation.State.Attempts += reservation.Count
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE pool_runtime SET bootstrap_attempts = ?, bootstrap_retry_at = ? WHERE pool = ?
	`, reservation.State.Attempts, nullTimeValue(reservation.State.RetryAt), request.Pool)
	if err != nil {
		return BootstrapReservation{}, fmt.Errorf("reserve bootstrap attempts: %w", err)
	}
	if err := requireUpdate(result); err != nil {
		return BootstrapReservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return BootstrapReservation{}, fmt.Errorf("commit bootstrap reservation: %w", err)
	}
	return reservation, nil
}

func (d *DB) HoldBootstrap(ctx context.Context, pool string, retryAt time.Time) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE pool_runtime SET bootstrap_retry_at = ? WHERE pool = ?
	`, retryAt.UTC(), pool)
	if err != nil {
		return fmt.Errorf("hold bootstrap attempts: %w", err)
	}
	return requireUpdate(result)
}

func (d *DB) ResetBootstrap(ctx context.Context, pool string) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE pool_runtime SET bootstrap_attempts = 0, bootstrap_retry_at = NULL WHERE pool = ?
	`, pool)
	if err != nil {
		return fmt.Errorf("reset bootstrap attempts: %w", err)
	}
	return requireUpdate(result)
}

func (d *DB) ProgressPool(ctx context.Context, progress PoolProgress) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE pool_runtime SET current_stage = ?, stage_started_at = ?, stage_deadline_at = ?
		WHERE pool = ? AND run_id = ?
	`, progress.Stage, progress.StartedAt.UTC(), progress.DeadlineAt.UTC(), progress.Pool, progress.RunID)
	if err != nil {
		return fmt.Errorf("record pool progress: %w", err)
	}
	return requireUpdate(result)
}

func (d *DB) FinishPool(ctx context.Context, result PoolResult) error {
	if result.State != RuntimeSucceeded && result.State != RuntimeFailed {
		return fmt.Errorf("invalid pool result state %q", result.State)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin pool result: %w", err)
	}
	defer tx.Rollback()

	var incidentID any
	if result.State == RuntimeFailed {
		id, err := recordIncident(ctx, tx, result)
		if err != nil {
			return err
		}
		incidentID = id
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE pool_incidents SET resolved_at = ? WHERE pool = ? AND resolved_at IS NULL
		`, result.FinishedAt.UTC(), result.Pool); err != nil {
			return fmt.Errorf("resolve pool incident: %w", err)
		}
	}

	var update sql.Result
	if result.State == RuntimeSucceeded {
		update, err = tx.ExecContext(ctx, `
			UPDATE pool_runtime SET reconcile_state = ?, reconcile_finished_at = ?,
			last_success_at = ?, active_incident_id = NULL, current_stage = '',
			stage_started_at = NULL, stage_deadline_at = NULL WHERE pool = ? AND run_id = ?
		`, result.State, result.FinishedAt.UTC(), result.FinishedAt.UTC(), result.Pool, result.RunID)
	} else {
		update, err = tx.ExecContext(ctx, `
			UPDATE pool_runtime SET reconcile_state = ?, reconcile_finished_at = ?,
			active_incident_id = ?, current_stage = '', stage_started_at = NULL,
			stage_deadline_at = NULL WHERE pool = ? AND run_id = ?
		`, result.State, result.FinishedAt.UTC(), incidentID, result.Pool, result.RunID)
	}
	if err != nil {
		return fmt.Errorf("finish pool reconciliation: %w", err)
	}
	if err := requireUpdate(update); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pool result: %w", err)
	}
	return nil
}

func recordIncident(ctx context.Context, tx *sql.Tx, result PoolResult) (int64, error) {
	failure := result.Failure
	if failure.Stage == "" {
		failure.Stage = reconcile.StageController
	}
	if failure.Code == "" {
		failure.Code = reconcile.FailureUnknown
	}
	if len(failure.Message) > maxIncidentMessage {
		failure.Message = failure.Message[:maxIncidentMessage]
	}

	var id int64
	var stage reconcile.Stage
	var code reconcile.FailureCode
	err := tx.QueryRowContext(ctx, `
		SELECT id, stage, code FROM pool_incidents WHERE pool = ? AND resolved_at IS NULL
	`, result.Pool).Scan(&id, &stage, &code)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read active pool incident: %w", err)
	}
	if err == nil && stage == failure.Stage && code == failure.Code {
		_, err := tx.ExecContext(ctx, `
			UPDATE pool_incidents SET run_id = ?, message = ?, instance_id = ?, instance_name = ?,
			last_seen_at = ?, occurrences = occurrences + 1 WHERE id = ?
		`, result.RunID, failure.Message, failure.InstanceID, failure.InstanceName, result.FinishedAt.UTC(), id)
		if err != nil {
			return 0, fmt.Errorf("update pool incident: %w", err)
		}
		return id, nil
	}
	if err == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE pool_incidents SET resolved_at = ? WHERE id = ?`, result.FinishedAt.UTC(), id); err != nil {
			return 0, fmt.Errorf("resolve replaced pool incident: %w", err)
		}
	}

	insert, err := tx.ExecContext(ctx, `
		INSERT INTO pool_incidents (
			pool, run_id, stage, code, message, instance_id, instance_name, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.Pool, result.RunID, failure.Stage, failure.Code, failure.Message, failure.InstanceID,
		failure.InstanceName, result.FinishedAt.UTC(), result.FinishedAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("create pool incident: %w", err)
	}
	id, err = insert.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read pool incident ID: %w", err)
	}
	return id, nil
}

func (d *DB) InterruptPools(ctx context.Context, at time.Time) error {
	rows, err := d.db.QueryContext(ctx, `SELECT pool, run_id FROM pool_runtime WHERE reconcile_state = ?`, RuntimeRunning)
	if err != nil {
		return fmt.Errorf("list interrupted pools: %w", err)
	}
	type run struct{ pool, id string }
	var runs []run
	for rows.Next() {
		var item run
		if err := rows.Scan(&item.pool, &item.id); err != nil {
			rows.Close()
			return fmt.Errorf("scan interrupted pool: %w", err)
		}
		runs = append(runs, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("list interrupted pools: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close interrupted pools: %w", err)
	}

	for _, run := range runs {
		if err := d.FinishPool(ctx, PoolResult{
			Pool: run.pool, RunID: run.id, State: RuntimeFailed, FinishedAt: at,
			Failure: PoolFailure{Stage: reconcile.StageController, Code: reconcile.FailureInterrupted, Message: "controller stopped during reconciliation"},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) ListPoolData(ctx context.Context) ([]PoolData, error) {
	rows, err := d.db.QueryContext(ctx, `
		WITH counts AS (
			SELECT pool,
				SUM(CASE WHEN state = 'bootstrapping' THEN 1 ELSE 0 END) bootstrapping,
				SUM(CASE WHEN state = 'ready' THEN 1 ELSE 0 END) ready,
				SUM(CASE WHEN state = 'running' THEN 1 ELSE 0 END) running,
				SUM(CASE WHEN state = 'cleaning' THEN 1 ELSE 0 END) cleaning
			FROM instances WHERE state <> 'finished' GROUP BY pool
		), pools AS (
			SELECT pool FROM pool_runtime UNION SELECT pool FROM counts
		)
		SELECT p.pool, COALESCE(r.run_id, ''), COALESCE(r.reconcile_state, ''), COALESCE(r.waiting_jobs, 0),
			r.observed_at, r.reconcile_started_at, r.reconcile_finished_at, r.last_success_at,
			COALESCE(r.current_stage, ''), r.stage_started_at, r.stage_deadline_at,
			COALESCE(r.bootstrap_attempts, 0), r.bootstrap_retry_at,
			COALESCE(c.bootstrapping, 0), COALESCE(c.ready, 0),
			COALESCE(c.running, 0), COALESCE(c.cleaning, 0),
			COALESCE(i.id, 0), COALESCE(i.run_id, ''), COALESCE(i.stage, ''), COALESCE(i.code, ''),
			COALESCE(i.message, ''), COALESCE(i.instance_id, ''), COALESCE(i.instance_name, ''),
			i.first_seen_at, i.last_seen_at, COALESCE(i.occurrences, 0), i.resolved_at
		FROM pools p
		LEFT JOIN pool_runtime r ON r.pool = p.pool
		LEFT JOIN counts c ON c.pool = p.pool
		LEFT JOIN pool_incidents i ON i.id = r.active_incident_id
		ORDER BY p.pool
	`)
	if err != nil {
		return nil, fmt.Errorf("list pool data: %w", err)
	}
	defer rows.Close()

	var result []PoolData
	for rows.Next() {
		var data PoolData
		var incident PoolIncident
		if err := rows.Scan(&data.Pool, &data.Runtime.RunID, &data.Runtime.State, &data.Runtime.Waiting,
			newNullTime(&data.Runtime.ObservedAt), newNullTime(&data.Runtime.ReconcileStartedAt),
			newNullTime(&data.Runtime.ReconcileFinishedAt), newNullTime(&data.Runtime.LastSuccessAt),
			&data.Runtime.Stage, newNullTime(&data.Runtime.StageStartedAt), newNullTime(&data.Runtime.StageDeadlineAt),
			&data.Runtime.Bootstrap.Attempts, newNullTime(&data.Runtime.Bootstrap.RetryAt),
			&data.Counts.Bootstrapping, &data.Counts.Ready, &data.Counts.Running, &data.Counts.Cleaning,
			&incident.ID, &incident.RunID, &incident.Stage, &incident.Code, &incident.Message,
			&incident.InstanceID, &incident.InstanceName, newNullTime(&incident.FirstSeenAt),
			newNullTime(&incident.LastSeenAt), &incident.Occurrences, newNullTime(&incident.ResolvedAt)); err != nil {
			return nil, fmt.Errorf("scan pool data: %w", err)
		}
		data.Runtime.Pool = data.Pool
		if incident.ID != 0 {
			incident.Pool = data.Pool
			data.Incident = &incident
		}
		result = append(result, data)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pool data: %w", err)
	}
	return result, nil
}

func (d *DB) ListIncidents(ctx context.Context, filter IncidentFilter) ([]PoolIncident, error) {
	status := ""
	if filter.Open != nil {
		if *filter.Open {
			status = "open"
		} else {
			status = "resolved"
		}
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, pool, run_id, stage, code, message, instance_id, instance_name,
		       first_seen_at, last_seen_at, occurrences, resolved_at
		FROM pool_incidents
		WHERE (? = '' OR pool = ?)
		  AND (? = '' OR (? = 'open' AND resolved_at IS NULL) OR (? = 'resolved' AND resolved_at IS NOT NULL))
		ORDER BY last_seen_at DESC, id DESC LIMIT ? OFFSET ?
	`, filter.Pool, filter.Pool, status, status, status, listLimit(filter.Limit), listOffset(filter.Offset))
	if err != nil {
		return nil, fmt.Errorf("list pool incidents: %w", err)
	}
	defer rows.Close()

	var incidents []PoolIncident
	for rows.Next() {
		var incident PoolIncident
		if err := rows.Scan(&incident.ID, &incident.Pool, &incident.RunID, &incident.Stage, &incident.Code,
			&incident.Message, &incident.InstanceID, &incident.InstanceName, &incident.FirstSeenAt,
			&incident.LastSeenAt, &incident.Occurrences, newNullTime(&incident.ResolvedAt)); err != nil {
			return nil, fmt.Errorf("scan pool incident: %w", err)
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pool incidents: %w", err)
	}
	return incidents, nil
}

func (d *DB) GetIncident(ctx context.Context, id int64) (PoolIncident, error) {
	var incident PoolIncident
	err := d.db.QueryRowContext(ctx, `
		SELECT id, pool, run_id, stage, code, message, instance_id, instance_name,
		       first_seen_at, last_seen_at, occurrences, resolved_at
		FROM pool_incidents WHERE id = ?
	`, id).Scan(&incident.ID, &incident.Pool, &incident.RunID, &incident.Stage, &incident.Code,
		&incident.Message, &incident.InstanceID, &incident.InstanceName, &incident.FirstSeenAt,
		&incident.LastSeenAt, &incident.Occurrences, newNullTime(&incident.ResolvedAt))
	if err != nil {
		return PoolIncident{}, fmt.Errorf("get pool incident: %w", err)
	}
	return incident, nil
}
