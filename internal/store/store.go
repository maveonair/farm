package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
	_ "modernc.org/sqlite"
)

type InstanceFilter struct {
	Pool  string
	State instance.State
	Limit int
}

type EventKind string

const (
	EventCreated        EventKind = "created"
	EventRegistered     EventKind = "registered"
	EventStageChanged   EventKind = "stage_changed"
	EventStateChanged   EventKind = "state_changed"
	EventRetryScheduled EventKind = "retry_scheduled"
)

type Event struct {
	ID           int64
	InstanceID   string
	InstanceName string
	Pool         string
	Kind         EventKind
	FromState    instance.State
	ToState      instance.State
	Stage        reconcile.Stage
	Result       instance.Result
	Reason       instance.Reason
	Message      string
	CreatedAt    time.Time
}

type EventFilter struct {
	InstanceID string
	Pool       string
	Limit      int
}

type RuntimeState string

const (
	RuntimeRunning   RuntimeState = "running"
	RuntimeSucceeded RuntimeState = "succeeded"
	RuntimeFailed    RuntimeState = "failed"
)

type PoolRun struct {
	Pool      string
	RunID     string
	StartedAt time.Time
}

type PoolObservation struct {
	Pool       string
	RunID      string
	Waiting    int
	ObservedAt time.Time
}

type PoolProgress struct {
	Pool       string
	RunID      string
	Stage      reconcile.Stage
	StartedAt  time.Time
	DeadlineAt time.Time
}

type PoolFailure struct {
	Stage        reconcile.Stage
	Code         reconcile.FailureCode
	Message      string
	InstanceID   string
	InstanceName string
}

type PoolResult struct {
	Pool       string
	RunID      string
	State      RuntimeState
	FinishedAt time.Time
	Failure    PoolFailure
}

type PoolRuntime struct {
	Pool                string
	RunID               string
	State               RuntimeState
	Waiting             int
	ObservedAt          time.Time
	ReconcileStartedAt  time.Time
	ReconcileFinishedAt time.Time
	LastSuccessAt       time.Time
	Stage               reconcile.Stage
	StageStartedAt      time.Time
	StageDeadlineAt     time.Time
}

type PoolCounts struct {
	Bootstrapping int
	Ready         int
	Running       int
	Cleaning      int
}

type PoolIncident struct {
	ID           int64
	Pool         string
	RunID        string
	Stage        reconcile.Stage
	Code         reconcile.FailureCode
	Message      string
	InstanceID   string
	InstanceName string
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	Occurrences  int
	ResolvedAt   time.Time
}

type PoolData struct {
	Pool     string
	Runtime  PoolRuntime
	Counts   PoolCounts
	Incident *PoolIncident
}

type IncidentFilter struct {
	Pool  string
	Open  *bool
	Limit int
}

type DB struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &DB{db: db}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (d *DB) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func newNullTime(target *time.Time) sql.Scanner {
	return nullTime{target: target}
}

type nullTime struct {
	target *time.Time
}

func (n nullTime) Scan(value any) error {
	if value == nil {
		*n.target = time.Time{}
		return nil
	}
	var scanned sql.NullTime
	if err := scanned.Scan(value); err != nil {
		return err
	}
	if scanned.Valid {
		*n.target = scanned.Time
	}
	return nil
}

func requireUpdate(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if rows != 1 {
		return sql.ErrNoRows
	}
	return nil
}
