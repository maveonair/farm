package reconcile

import (
	"sync"
	"time"
)

const minimumScheduleGrace = 30 * time.Second

type Phase string

const (
	PhaseIdle    Phase = "idle"
	PhaseRunning Phase = "running"
)

type Condition string

const (
	ConditionStarting Condition = "starting"
	ConditionHealthy  Condition = "healthy"
	ConditionDegraded Condition = "degraded"
	ConditionStalled  Condition = "stalled"
)

type Outcome string

const (
	OutcomeNone      Outcome = "none"
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
)

type Progress struct {
	Pool       string
	Stage      string
	StartedAt  time.Time
	DeadlineAt time.Time
}

type Snapshot struct {
	Phase          Phase
	Condition      Condition
	LastOutcome    Outcome
	StartedAt      time.Time
	FinishedAt     time.Time
	LastSuccessAt  time.Time
	DeadlineAt     time.Time
	NextExpectedAt time.Time
	CompletedPools int
	TotalPools     int
	ActiveFailures int
	Errors         uint64
	Progress       []Progress
}

type Tracker struct {
	mu             sync.Mutex
	phase          Phase
	lastOutcome    Outcome
	startedAt      time.Time
	finishedAt     time.Time
	lastSuccessAt  time.Time
	deadlineAt     time.Time
	nextExpectedAt time.Time
	totalPools     int
	done           map[string]bool
	failed         map[string]bool
	progress       map[string]Progress
	errors         uint64
	errorsBy       map[string]uint64
}

func (t *Tracker) Start(startedAt, deadlineAt time.Time, pools int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.phase = PhaseRunning
	t.startedAt = startedAt
	t.deadlineAt = deadlineAt
	t.totalPools = pools
	t.nextExpectedAt = time.Time{}
	t.done = make(map[string]bool, pools)
	if t.failed == nil {
		t.failed = make(map[string]bool)
	}
	if t.progress == nil {
		t.progress = make(map[string]Progress)
	}
}

func (t *Tracker) Progress(value Progress) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.progress == nil {
		t.progress = make(map[string]Progress)
	}
	t.progress[value.Pool] = value
}

func (t *Tracker) PoolFailure(pool, stage, code string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errors++
	if t.errorsBy == nil {
		t.errorsBy = make(map[string]uint64)
	}
	if t.failed == nil {
		t.failed = make(map[string]bool)
	}
	t.errorsBy[stage+"\x00"+code]++
	t.failed[pool] = true
	t.markDone(pool)
}

func (t *Tracker) PoolSuccess(pool string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failed, pool)
	t.markDone(pool)
}

func (t *Tracker) markDone(pool string) {
	if t.done == nil {
		t.done = make(map[string]bool)
	}
	t.done[pool] = true
	delete(t.progress, pool)
}

func (t *Tracker) Finish(at time.Time, interval time.Duration, succeeded bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.phase = PhaseIdle
	t.finishedAt = at
	grace := max(2*interval, minimumScheduleGrace)
	t.nextExpectedAt = at.Add(interval + grace)
	if succeeded {
		t.lastOutcome = OutcomeSucceeded
		t.lastSuccessAt = at
		return
	}
	t.lastOutcome = OutcomeFailed
}

// Success preserves the small monitor API used by callers without cycle details.
func (t *Tracker) Success(at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastSuccessAt = at
	t.lastOutcome = OutcomeSucceeded
}

func (t *Tracker) Failure() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errors++
}

func (t *Tracker) Snapshot(now time.Time) Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	snapshot := Snapshot{
		Phase: t.phase, LastOutcome: t.lastOutcome, StartedAt: t.startedAt,
		FinishedAt: t.finishedAt, LastSuccessAt: t.lastSuccessAt, DeadlineAt: t.deadlineAt,
		NextExpectedAt: t.nextExpectedAt, CompletedPools: len(t.done), TotalPools: t.totalPools,
		ActiveFailures: len(t.failed), Errors: t.errors,
	}
	if snapshot.Phase == "" {
		snapshot.Phase = PhaseIdle
	}
	if snapshot.LastOutcome == "" {
		snapshot.LastOutcome = OutcomeNone
	}
	for _, progress := range t.progress {
		snapshot.Progress = append(snapshot.Progress, progress)
		if !progress.DeadlineAt.IsZero() && (snapshot.DeadlineAt.IsZero() || progress.DeadlineAt.Before(snapshot.DeadlineAt)) {
			snapshot.DeadlineAt = progress.DeadlineAt
		}
	}
	snapshot.Condition = t.condition(now, snapshot.DeadlineAt)
	return snapshot
}

func (t *Tracker) condition(now, deadline time.Time) Condition {
	if t.phase == PhaseRunning && !deadline.IsZero() && now.After(deadline) {
		return ConditionStalled
	}
	if t.phase == PhaseIdle && !t.nextExpectedAt.IsZero() && now.After(t.nextExpectedAt) {
		return ConditionStalled
	}
	if len(t.failed) > 0 {
		return ConditionDegraded
	}
	if t.lastSuccessAt.IsZero() {
		return ConditionStarting
	}
	return ConditionHealthy
}

func (t *Tracker) MetricData() (int64, uint64, map[string]uint64, []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	errorsBy := make(map[string]uint64, len(t.errorsBy))
	for key, value := range t.errorsBy {
		errorsBy[key] = value
	}
	failed := make([]string, 0, len(t.failed))
	for pool := range t.failed {
		failed = append(failed, pool)
	}
	var lastSuccess int64
	if !t.lastSuccessAt.IsZero() {
		lastSuccess = t.lastSuccessAt.Unix()
	}
	return lastSuccess, t.errors, errorsBy, failed
}
