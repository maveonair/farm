package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	farmInstance "github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
)

func TestInstanceLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	err := db.Create(ctx, farmInstance.Instance{
		ID:   "instance-id",
		Name: "farm-ubuntu-01",
		Pool: "ubuntu",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := db.SetRegistration(ctx, "instance-id", 42); err != nil {
		t.Fatalf("SetRegistration() error = %v", err)
	}
	if err := db.MarkReady(ctx, "instance-id"); err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}

	instance, err := db.Get(ctx, "instance-id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if instance.State != farmInstance.StateReady || instance.RunnerID != 42 {
		t.Fatalf("instance = %#v", instance)
	}
}

func TestListPoolExcludesTerminal(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for _, instance := range []farmInstance.Instance{
		{ID: "ready", Name: "ready", Pool: "ubuntu", State: farmInstance.StateReady},
		{ID: "failed", Name: "failed", Pool: "ubuntu", State: farmInstance.StateFinished},
		{ID: "other", Name: "other", Pool: "debian", State: farmInstance.StateReady},
	} {
		if err := db.Create(ctx, instance); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	instances, err := db.ListPool(ctx, "ubuntu")
	if err != nil {
		t.Fatalf("ListPool() error = %v", err)
	}
	if len(instances) != 1 || instances[0].ID != "ready" {
		t.Fatalf("instances = %#v", instances)
	}
}

func TestRetry(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Create(ctx, farmInstance.Instance{ID: "id", Name: "name", Pool: "pool"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	retryAt := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	if err := db.Retry(ctx, "id", "unavailable", retryAt); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	instance, err := db.Get(ctx, "id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if instance.RetryCount != 1 || !instance.RetryAt.Equal(retryAt) {
		t.Fatalf("retry = %d at %v", instance.RetryCount, instance.RetryAt)
	}
}

func TestRetryPreservesFailure(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := db.Create(ctx, farmInstance.Instance{ID: "id", Name: "name", Pool: "pool"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := db.BeginCleanup(ctx, "id", farmInstance.ResultFailed, farmInstance.ReasonBootstrapFailed, "cloud-init failed"); err != nil {
		t.Fatalf("BeginCleanup() error = %v", err)
	}
	if err := db.Retry(ctx, "id", "delete VM failed", time.Now()); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	record, err := db.Get(ctx, "id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.Contains(record.Error, "cloud-init failed") || !strings.Contains(record.Error, "delete VM failed") {
		t.Fatalf("error = %q", record.Error)
	}
}

func TestInstanceEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Create(ctx, farmInstance.Instance{ID: "id", Name: "name", Pool: "pool"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := db.MarkReady(ctx, "id"); err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}
	if err := db.Retry(ctx, "id", "unavailable", time.Now()); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	events, err := db.ListEvents(ctx, EventFilter{InstanceID: "id"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Kind != "retry_scheduled" || events[1].ToState != farmInstance.StateReady || events[2].Kind != "created" {
		t.Fatalf("events = %#v", events)
	}
}

func TestPoolRuntime(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.September, 19, 8, 0, 0, 0, time.UTC)
	if err := db.StartPool(ctx, PoolRun{Pool: "ubuntu", RunID: "run", StartedAt: now}); err != nil {
		t.Fatalf("StartPool() error = %v", err)
	}
	if err := db.ObservePool(ctx, PoolObservation{Pool: "ubuntu", RunID: "run", Waiting: 2, ObservedAt: now}); err != nil {
		t.Fatalf("ObservePool() error = %v", err)
	}
	if err := db.FinishPool(ctx, PoolResult{Pool: "ubuntu", RunID: "run", State: RuntimeSucceeded, FinishedAt: now}); err != nil {
		t.Fatalf("FinishPool() error = %v", err)
	}

	data, err := db.ListPoolData(ctx)
	if err != nil {
		t.Fatalf("ListPoolData() error = %v", err)
	}
	if len(data) != 1 || data[0].Runtime.Waiting != 2 || !data[0].Runtime.LastSuccessAt.Equal(now) {
		t.Fatalf("data = %#v", data)
	}
}

func TestPoolStatusUsesLiveInstanceCounts(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.StartPool(ctx, PoolRun{Pool: "ubuntu", RunID: "run", StartedAt: now}); err != nil {
		t.Fatalf("StartPool() error = %v", err)
	}
	if err := db.Create(ctx, farmInstance.Instance{
		ID: "starting", Name: "farm-ubuntu-starting", Pool: "ubuntu", State: farmInstance.StateBootstrapping,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	data, err := db.ListPoolData(ctx)
	if err != nil {
		t.Fatalf("ListPoolData() error = %v", err)
	}
	if len(data) != 1 || data[0].Counts.Bootstrapping != 1 {
		t.Fatalf("data = %#v", data)
	}
}

func TestPoolIncidentLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.September, 19, 9, 0, 0, 0, time.UTC)

	for index := range 2 {
		runID := fmt.Sprintf("run-%d", index)
		if err := db.StartPool(ctx, PoolRun{Pool: "ubuntu", RunID: runID, StartedAt: now}); err != nil {
			t.Fatalf("StartPool() error = %v", err)
		}
		if err := db.FinishPool(ctx, PoolResult{
			Pool: "ubuntu", RunID: runID, State: RuntimeFailed, FinishedAt: now.Add(time.Duration(index) * time.Minute),
			Failure: PoolFailure{Stage: reconcile.StageFetchJobs, Code: reconcile.FailureUnavailable, Message: "Forgejo unavailable"},
		}); err != nil {
			t.Fatalf("FinishPool() error = %v", err)
		}
	}

	data, err := db.ListPoolData(ctx)
	if err != nil {
		t.Fatalf("ListPoolData() error = %v", err)
	}
	if data[0].Incident == nil || data[0].Incident.Occurrences != 2 {
		t.Fatalf("incident = %#v", data[0].Incident)
	}

	if err := db.StartPool(ctx, PoolRun{Pool: "ubuntu", RunID: "recovered", StartedAt: now}); err != nil {
		t.Fatalf("StartPool() error = %v", err)
	}
	if err := db.FinishPool(ctx, PoolResult{Pool: "ubuntu", RunID: "recovered", State: RuntimeSucceeded, FinishedAt: now}); err != nil {
		t.Fatalf("FinishPool() error = %v", err)
	}
	open := true
	incidents, err := db.ListIncidents(ctx, IncidentFilter{Open: &open})
	if err != nil {
		t.Fatalf("ListIncidents() error = %v", err)
	}
	if len(incidents) != 0 {
		t.Fatalf("open incidents = %#v", incidents)
	}
}

func TestOpenRejectsUnversionedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "farm.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE instances (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	_, err = Open(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "recreate") {
		t.Fatalf("Open() error = %v", err)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "farm.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return db
}
