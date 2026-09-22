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

type Progress struct {
	Pool       string
	Stage      string
	StartedAt  time.Time
	DeadlineAt time.Time
}

type Snapshot struct {
	Phase          Phase
	Condition      Condition
	StartedAt      time.Time
	FinishedAt     time.Time
	LastSuccessAt  time.Time
	DeadlineAt     time.Time
	NextExpectedAt time.Time
	ActivePools    int
	TotalPools     int
	ActiveFailures int
	Errors         uint64
	Progress       []Progress
}

type poolState struct {
	active         bool
	attempted      bool
	failed         bool
	startedAt      time.Time
	finishedAt     time.Time
	lastSuccessAt  time.Time
	deadlineAt     time.Time
	nextExpectedAt time.Time
}

type Tracker struct {
	mu          sync.Mutex
	interval    time.Duration
	pools       map[string]poolState
	progress    map[string]Progress
	errors      uint64
	errorsBy    map[string]uint64
	lastSuccess time.Time
}

func (t *Tracker) Configure(pools []string, interval time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.interval = interval
	t.pools = make(map[string]poolState, len(pools))
	for _, pool := range pools {
		t.pools[pool] = poolState{}
	}
	t.progress = make(map[string]Progress, len(pools))
	if t.errorsBy == nil {
		t.errorsBy = make(map[string]uint64)
	}
}

func (t *Tracker) PoolStart(pool string, startedAt, deadlineAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.pools[pool]
	state.active = true
	state.startedAt = startedAt
	state.deadlineAt = deadlineAt
	state.nextExpectedAt = time.Time{}
	t.pools[pool] = state
}

func (t *Tracker) Progress(value Progress) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.progress == nil {
		t.progress = make(map[string]Progress)
	}
	t.progress[value.Pool] = value
}

func (t *Tracker) PoolFailure(pool, stage, code string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.errors++
	if t.errorsBy == nil {
		t.errorsBy = make(map[string]uint64)
	}
	t.errorsBy[stage+"\x00"+code]++

	state := t.pools[pool]
	state.active = false
	state.attempted = true
	state.failed = true
	state.finishedAt = at
	state.deadlineAt = time.Time{}
	state.nextExpectedAt = at.Add(t.interval)
	t.pools[pool] = state
	delete(t.progress, pool)
}

func (t *Tracker) PoolSuccess(pool string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.pools[pool]
	state.active = false
	state.attempted = true
	state.failed = false
	state.finishedAt = at
	state.lastSuccessAt = at
	state.deadlineAt = time.Time{}
	state.nextExpectedAt = at.Add(t.interval)
	t.pools[pool] = state
	delete(t.progress, pool)
	if t.allSucceeded() {
		t.lastSuccess = at
	}
}

func (t *Tracker) Snapshot(now time.Time) Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	snapshot := Snapshot{TotalPools: len(t.pools), Errors: t.errors, LastSuccessAt: t.lastSuccess}
	allAttempted := len(t.pools) > 0
	for _, state := range t.pools {
		if state.active {
			snapshot.ActivePools++
			setEarlier(&snapshot.StartedAt, state.startedAt)
			setEarlier(&snapshot.DeadlineAt, state.deadlineAt)
		} else if state.attempted {
			setEarlier(&snapshot.NextExpectedAt, state.nextExpectedAt)
		}
		if state.finishedAt.After(snapshot.FinishedAt) {
			snapshot.FinishedAt = state.finishedAt
		}
		if !state.attempted {
			allAttempted = false
		}
		if state.failed {
			snapshot.ActiveFailures++
		}
	}
	for _, progress := range t.progress {
		snapshot.Progress = append(snapshot.Progress, progress)
		setEarlier(&snapshot.DeadlineAt, progress.DeadlineAt)
	}

	if snapshot.ActivePools > 0 {
		snapshot.Phase = PhaseRunning
	} else {
		snapshot.Phase = PhaseIdle
	}
	snapshot.Condition = t.condition(now, allAttempted)
	return snapshot
}

func (t *Tracker) condition(now time.Time, allAttempted bool) Condition {
	grace := max(2*t.interval, minimumScheduleGrace)
	for _, state := range t.pools {
		if state.active && !state.deadlineAt.IsZero() && now.After(state.deadlineAt) {
			return ConditionStalled
		}
		if !state.active && state.attempted && !state.nextExpectedAt.IsZero() && now.After(state.nextExpectedAt.Add(grace)) {
			return ConditionStalled
		}
	}
	for _, progress := range t.progress {
		if !progress.DeadlineAt.IsZero() && now.After(progress.DeadlineAt) {
			return ConditionStalled
		}
	}
	for _, state := range t.pools {
		if state.failed {
			return ConditionDegraded
		}
	}
	if !allAttempted && len(t.pools) > 0 {
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
	failed := make([]string, 0, len(t.pools))
	for pool, state := range t.pools {
		if state.failed {
			failed = append(failed, pool)
		}
	}
	var lastSuccessUnix int64
	if !t.lastSuccess.IsZero() {
		lastSuccessUnix = t.lastSuccess.Unix()
	}
	return lastSuccessUnix, t.errors, errorsBy, failed
}

func (t *Tracker) allSucceeded() bool {
	if len(t.pools) == 0 {
		return false
	}
	for _, state := range t.pools {
		if state.failed || state.lastSuccessAt.IsZero() {
			return false
		}
	}
	return true
}

func setEarlier(current *time.Time, candidate time.Time) {
	if candidate.IsZero() {
		return
	}
	if current.IsZero() || candidate.Before(*current) {
		*current = candidate
	}
}
