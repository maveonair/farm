package store

import (
	"context"
	"errors"
	"fmt"
)

const schemaVersion = 2

func (d *DB) initialize(ctx context.Context) error {
	for _, pragma := range []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
	} {
		if _, err := d.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure database: %w", err)
		}
	}

	var version int
	if err := d.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read database schema version: %w", err)
	}
	if version == schemaVersion {
		return nil
	}
	if version == 1 {
		return d.migrateV1(ctx)
	}
	if version != 0 {
		return fmt.Errorf("unsupported database schema version %d", version)
	}

	var tables int
	if err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name IN ('instances', 'instance_events', 'pool_status', 'pool_runtime', 'pool_incidents')
	`).Scan(&tables); err != nil {
		return fmt.Errorf("inspect database schema: %w", err)
	}
	if tables != 0 {
		return errors.New("unsupported unversioned database schema; recreate the FARM database")
	}

	if _, err := d.db.ExecContext(ctx, schema()); err != nil {
		return fmt.Errorf("create database schema: %w", err)
	}
	return nil
}

func (d *DB) migrateV1(ctx context.Context) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()

	for _, statement := range []string{
		`ALTER TABLE pool_runtime ADD COLUMN bootstrap_attempts INTEGER NOT NULL DEFAULT 0 CHECK (bootstrap_attempts >= 0)`,
		`ALTER TABLE pool_runtime ADD COLUMN bootstrap_retry_at DATETIME`,
		fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion),
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

func schema() string {
	return fmt.Sprintf(`
CREATE TABLE instances (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	pool TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('bootstrapping','ready','running','cleaning','finished')),
	stage TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '' CHECK (result IN ('','succeeded','failed')),
	reason TEXT NOT NULL DEFAULT '',
	runner_id INTEGER NOT NULL DEFAULT 0,
	error TEXT NOT NULL DEFAULT '',
	retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
	retry_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	state_changed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	stage_changed_at DATETIME,
	finished_at DATETIME
);
CREATE INDEX instances_pool_state ON instances(pool, state);
CREATE INDEX instances_created ON instances(created_at DESC, id);

CREATE TABLE instance_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	instance_id TEXT NOT NULL,
	instance_name TEXT NOT NULL,
	pool TEXT NOT NULL,
	kind TEXT NOT NULL,
	from_state TEXT NOT NULL DEFAULT '',
	to_state TEXT NOT NULL DEFAULT '',
	stage TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	message TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX instance_events_instance ON instance_events(instance_id, id DESC);
CREATE INDEX instance_events_pool ON instance_events(pool, id DESC);

CREATE TABLE pool_incidents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	pool TEXT NOT NULL,
	run_id TEXT NOT NULL,
	stage TEXT NOT NULL,
	code TEXT NOT NULL,
	message TEXT NOT NULL,
	instance_id TEXT NOT NULL DEFAULT '',
	instance_name TEXT NOT NULL DEFAULT '',
	first_seen_at DATETIME NOT NULL,
	last_seen_at DATETIME NOT NULL,
	occurrences INTEGER NOT NULL DEFAULT 1 CHECK (occurrences > 0),
	resolved_at DATETIME
);
CREATE UNIQUE INDEX pool_incidents_active ON pool_incidents(pool) WHERE resolved_at IS NULL;
CREATE INDEX pool_incidents_history ON pool_incidents(pool, last_seen_at DESC);

CREATE TABLE pool_runtime (
	pool TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	reconcile_state TEXT NOT NULL CHECK (reconcile_state IN ('running','succeeded','failed')),
	waiting_jobs INTEGER NOT NULL DEFAULT 0 CHECK (waiting_jobs >= 0),
	observed_at DATETIME,
	reconcile_started_at DATETIME NOT NULL,
	reconcile_finished_at DATETIME,
	last_success_at DATETIME,
	current_stage TEXT NOT NULL DEFAULT '',
	stage_started_at DATETIME,
	stage_deadline_at DATETIME,
	bootstrap_attempts INTEGER NOT NULL DEFAULT 0 CHECK (bootstrap_attempts >= 0),
	bootstrap_retry_at DATETIME,
	active_incident_id INTEGER,
	FOREIGN KEY (active_incident_id) REFERENCES pool_incidents(id)
);

PRAGMA user_version = %d;
`, schemaVersion)
}
