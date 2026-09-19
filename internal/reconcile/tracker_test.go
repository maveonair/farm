package reconcile

import (
	"testing"
	"time"
)

func TestRunningBeforeDeadlineIsHealthy(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Success(now.Add(-time.Minute))
	tracker.Start(now, now.Add(time.Minute), 1)

	snapshot := tracker.Snapshot(now.Add(30 * time.Second))
	if snapshot.Phase != PhaseRunning || snapshot.Condition != ConditionHealthy {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestExpiredProgressIsStalled(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Start(now, now.Add(time.Hour), 1)
	tracker.Progress(Progress{Pool: "ubuntu", Stage: "fetch_jobs", StartedAt: now, DeadlineAt: now.Add(time.Second)})

	if condition := tracker.Snapshot(now.Add(2 * time.Second)).Condition; condition != ConditionStalled {
		t.Fatalf("condition = %q", condition)
	}
}

func TestFailureRemainsDuringRetry(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Start(now, now.Add(time.Minute), 1)
	tracker.PoolFailure("ubuntu", "fetch_jobs", "unavailable")
	tracker.Finish(now, time.Second, false)
	tracker.Start(now.Add(time.Second), now.Add(time.Minute), 1)

	if condition := tracker.Snapshot(now.Add(2 * time.Second)).Condition; condition != ConditionDegraded {
		t.Fatalf("condition = %q", condition)
	}
	tracker.PoolSuccess("ubuntu")
	if condition := tracker.Snapshot(now.Add(2 * time.Second)).Condition; condition != ConditionStarting {
		t.Fatalf("condition after recovery = %q", condition)
	}
}
