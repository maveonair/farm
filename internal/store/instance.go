package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
)

func (d *DB) Create(ctx context.Context, record instance.Instance) error {
	if record.ID == "" || record.Name == "" || record.Pool == "" {
		return errors.New("instance ID, name, and pool are required")
	}
	if record.State == "" {
		record.State = instance.StateBootstrapping
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin instance creation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO instances (id, name, pool, state, stage, result, reason, runner_id, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.Name, record.Pool, record.State, record.Stage, record.Result, record.Reason,
		record.RunnerID, record.Error); err != nil {
		return fmt.Errorf("create instance: %w", err)
	}
	if err := insertEvent(ctx, tx, record, EventCreated, "", record.State, record.Error); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit instance creation: %w", err)
	}
	return nil
}

func (d *DB) Get(ctx context.Context, id string) (instance.Instance, error) {
	record, err := scanInstance(d.db.QueryRowContext(ctx, instanceSelect+` WHERE id = ?`, id))
	if err != nil {
		return instance.Instance{}, fmt.Errorf("get instance: %w", err)
	}
	return record, nil
}

func (d *DB) ListPool(ctx context.Context, pool string) ([]instance.Instance, error) {
	rows, err := d.db.QueryContext(ctx, instanceSelect+`
		WHERE pool = ? AND state <> ? ORDER BY created_at
	`, pool, instance.StateFinished)
	if err != nil {
		return nil, fmt.Errorf("list pool instances: %w", err)
	}
	return scanInstances(rows, "list pool instances")
}

func (d *DB) ListInstances(ctx context.Context, filter InstanceFilter) ([]instance.Instance, error) {
	rows, err := d.db.QueryContext(ctx, instanceSelect+`
		WHERE (? = '' OR pool = ?) AND (? = '' OR state = ?)
		ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?
	`, filter.Pool, filter.Pool, filter.State, filter.State, listLimit(filter.Limit), listOffset(filter.Offset))
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	return scanInstances(rows, "list instances")
}

func (d *DB) ListEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, instance_id, instance_name, pool, kind, from_state, to_state,
		       stage, result, reason, message, created_at
		FROM instance_events
		WHERE (? = '' OR instance_id = ?) AND (? = '' OR pool = ?)
		ORDER BY id DESC LIMIT ? OFFSET ?
	`, filter.InstanceID, filter.InstanceID, filter.Pool, filter.Pool,
		listLimit(filter.Limit), listOffset(filter.Offset))
	if err != nil {
		return nil, fmt.Errorf("list instance events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.InstanceID, &event.InstanceName, &event.Pool,
			&event.Kind, &event.FromState, &event.ToState, &event.Stage, &event.Result,
			&event.Reason, &event.Message, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan instance event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list instance events: %w", err)
	}
	return events, nil
}

func (d *DB) SetRegistration(ctx context.Context, id string, runnerID int64) error {
	return d.mutate(ctx, id, EventRegistered, func(current *instance.Instance) error {
		current.RunnerID = runnerID
		current.Stage = reconcile.StageRegisterRunner
		current.RetryCount = 0
		current.RetryAt = time.Time{}
		current.Error = ""
		return nil
	})
}

func (d *DB) SetStage(ctx context.Context, id string, stage reconcile.Stage) error {
	return d.mutate(ctx, id, EventStageChanged, func(current *instance.Instance) error {
		if current.State != instance.StateBootstrapping && current.State != instance.StateCleaning {
			return fmt.Errorf("set stage for instance in state %q", current.State)
		}
		current.Stage = stage
		return nil
	})
}

func (d *DB) MarkReady(ctx context.Context, id string) error {
	return d.setOperationalState(ctx, id, instance.StateReady)
}

func (d *DB) MarkRunning(ctx context.Context, id string) error {
	return d.setOperationalState(ctx, id, instance.StateRunning)
}

func (d *DB) setOperationalState(ctx context.Context, id string, state instance.State) error {
	return d.mutate(ctx, id, EventStateChanged, func(current *instance.Instance) error {
		if !instance.CanTransition(current.State, state) {
			return fmt.Errorf("invalid instance transition %q to %q", current.State, state)
		}
		current.State = state
		current.Stage = ""
		current.RetryCount = 0
		current.RetryAt = time.Time{}
		current.Error = ""
		return nil
	})
}

func (d *DB) BeginCleanup(ctx context.Context, id string, result instance.Result, reason instance.Reason, message string) error {
	return d.mutate(ctx, id, EventStateChanged, func(current *instance.Instance) error {
		if !instance.CanTransition(current.State, instance.StateCleaning) {
			return fmt.Errorf("invalid instance transition %q to %q", current.State, instance.StateCleaning)
		}
		current.State = instance.StateCleaning
		current.Stage = reconcile.StageDeleteRunner
		if current.Result == "" {
			current.Result = result
		}
		if current.Reason == "" {
			current.Reason = reason
		}
		if current.Error == "" {
			current.Error = message
		}
		return nil
	})
}

func (d *DB) FinishCleanup(ctx context.Context, id string) error {
	return d.mutate(ctx, id, EventStateChanged, func(current *instance.Instance) error {
		if !instance.CanTransition(current.State, instance.StateFinished) {
			return fmt.Errorf("invalid instance transition %q to %q", current.State, instance.StateFinished)
		}
		current.State = instance.StateFinished
		current.Stage = ""
		current.RetryAt = time.Time{}
		return nil
	})
}

func (d *DB) Retry(ctx context.Context, id, message string, retryAt time.Time) error {
	return d.mutate(ctx, id, EventRetryScheduled, func(current *instance.Instance) error {
		current.RetryCount++
		current.RetryAt = retryAt.UTC()
		current.Error = mergeError(current.Error, message)
		return nil
	})
}

func mergeError(current, next string) string {
	if current == "" {
		return next
	}
	if next == "" || strings.Contains(current, next) {
		return current
	}

	return current + "\ncleanup: " + next
}

func (d *DB) mutate(ctx context.Context, id string, kind EventKind, change func(*instance.Instance) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin instance update: %w", err)
	}
	defer tx.Rollback()

	before, err := scanInstance(tx.QueryRowContext(ctx, instanceSelect+` WHERE id = ?`, id))
	if err != nil {
		return fmt.Errorf("get instance for update: %w", err)
	}
	after := before
	if err := change(&after); err != nil {
		return err
	}
	if before == after {
		return tx.Commit()
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE instances SET state = ?, stage = ?, result = ?, reason = ?, runner_id = ?,
		error = ?, retry_count = ?, retry_at = ?,
		state_changed_at = CASE WHEN state <> ? THEN CURRENT_TIMESTAMP ELSE state_changed_at END,
		stage_changed_at = CASE WHEN stage <> ? THEN CURRENT_TIMESTAMP ELSE stage_changed_at END,
		finished_at = CASE WHEN ? = 'finished' THEN CURRENT_TIMESTAMP ELSE finished_at END,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, after.State, after.Stage, after.Result, after.Reason, after.RunnerID, after.Error,
		after.RetryCount, nullTimeValue(after.RetryAt), after.State, after.Stage, after.State, id)
	if err != nil {
		return fmt.Errorf("update instance: %w", err)
	}
	if err := requireUpdate(result); err != nil {
		return err
	}
	if err := insertEvent(ctx, tx, after, kind, before.State, after.State, after.Error); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit instance update: %w", err)
	}
	return nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, record instance.Instance, kind EventKind, from, to instance.State, message string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO instance_events (
			instance_id, instance_name, pool, kind, from_state, to_state, stage, result, reason, message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.Name, record.Pool, kind, from, to, record.Stage, record.Result, record.Reason, message)
	if err != nil {
		return fmt.Errorf("record instance event: %w", err)
	}
	return nil
}

const instanceSelect = `
	SELECT id, name, pool, state, stage, result, reason, runner_id, error, retry_count,
	       retry_at, created_at, updated_at, state_changed_at, stage_changed_at, finished_at
	FROM instances`

func scanInstance(row scanner) (instance.Instance, error) {
	var record instance.Instance
	err := row.Scan(&record.ID, &record.Name, &record.Pool, &record.State, &record.Stage,
		&record.Result, &record.Reason, &record.RunnerID, &record.Error, &record.RetryCount,
		newNullTime(&record.RetryAt), &record.CreatedAt, &record.UpdatedAt,
		&record.StateChangedAt, newNullTime(&record.StageChangedAt), newNullTime(&record.FinishedAt))
	return record, err
}

func scanInstances(rows *sql.Rows, operation string) ([]instance.Instance, error) {
	defer rows.Close()
	var records []instance.Instance
	for rows.Next() {
		record, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	return records, nil
}

func nullTimeValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}
