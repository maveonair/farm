package reconcile

import (
	"testing"
	"time"
)

func TestRunningBeforeDeadlineIsHealthy(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Configure([]string{"ubuntu"}, time.Minute)
	tracker.PoolSuccess("ubuntu", now.Add(-time.Minute))
	tracker.PoolStart("ubuntu", now, now.Add(time.Minute))

	snapshot := tracker.Snapshot(now.Add(30 * time.Second))
	if snapshot.Phase != PhaseRunning || snapshot.Condition != ConditionHealthy {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestExpiredProgressIsStalled(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Configure([]string{"ubuntu"}, time.Minute)
	tracker.PoolStart("ubuntu", now, now.Add(time.Hour))
	tracker.Progress(Progress{Pool: "ubuntu", Stage: "fetch_jobs", StartedAt: now, DeadlineAt: now.Add(time.Second)})

	if condition := tracker.Snapshot(now.Add(2 * time.Second)).Condition; condition != ConditionStalled {
		t.Fatalf("condition = %q", condition)
	}
}

func TestFailureRemainsDuringRetry(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Configure([]string{"ubuntu"}, time.Second)
	tracker.PoolStart("ubuntu", now, now.Add(time.Minute))
	tracker.PoolFailure("ubuntu", "fetch_jobs", "unavailable", now)
	tracker.PoolStart("ubuntu", now.Add(time.Second), now.Add(time.Minute))

	if condition := tracker.Snapshot(now.Add(2 * time.Second)).Condition; condition != ConditionDegraded {
		t.Fatalf("condition = %q", condition)
	}
	tracker.PoolSuccess("ubuntu", now.Add(2*time.Second))
	if condition := tracker.Snapshot(now.Add(2 * time.Second)).Condition; condition != ConditionHealthy {
		t.Fatalf("condition after recovery = %q", condition)
	}
}

func TestPoolSchedulesAreIndependent(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Configure([]string{"slow", "fast"}, time.Minute)
	tracker.PoolStart("slow", now, now.Add(time.Hour))
	tracker.PoolStart("fast", now, now.Add(time.Minute))
	tracker.PoolSuccess("fast", now.Add(time.Second))

	snapshot := tracker.Snapshot(now.Add(2 * time.Second))
	if snapshot.ActivePools != 1 || snapshot.TotalPools != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestMissedPoolScheduleIsStalled(t *testing.T) {
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	tracker := &Tracker{}
	tracker.Configure([]string{"ubuntu"}, time.Second)
	tracker.PoolStart("ubuntu", now, now.Add(time.Minute))
	tracker.PoolSuccess("ubuntu", now)

	if condition := tracker.Snapshot(now.Add(time.Minute)).Condition; condition != ConditionStalled {
		t.Fatalf("condition = %q", condition)
	}
}
