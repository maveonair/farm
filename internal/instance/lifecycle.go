package instance

import (
	"time"

	"github.com/maveonair/farm/internal/reconcile"
)

type State string

const (
	StateBootstrapping State = "bootstrapping"
	StateReady         State = "ready"
	StateRunning       State = "running"
	StateCleaning      State = "cleaning"
	StateFinished      State = "finished"
)

type Result string

const (
	ResultSucceeded Result = "succeeded"
	ResultFailed    Result = "failed"
)

type Reason string

const (
	ReasonJobCompleted       Reason = "job_completed"
	ReasonIdleTimeout        Reason = "idle_timeout"
	ReasonMaxLifetime        Reason = "max_lifetime"
	ReasonBootstrapFailed    Reason = "bootstrap_failed"
	ReasonRunnerOffline      Reason = "runner_offline"
	ReasonInstanceMissing    Reason = "instance_missing"
	ReasonControllerRecovery Reason = "controller_recovery"
)

type Instance struct {
	ID             string
	Name           string
	Pool           string
	State          State
	Stage          reconcile.Stage
	Result         Result
	Reason         Reason
	RunnerID       int64
	Error          string
	RetryCount     int
	RetryAt        time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StateChangedAt time.Time
	StageChangedAt time.Time
	FinishedAt     time.Time
}

func CanTransition(from, to State) bool {
	if from == to {
		return true
	}
	switch from {
	case StateBootstrapping:
		return to == StateReady || to == StateRunning || to == StateCleaning
	case StateReady:
		return to == StateRunning || to == StateCleaning
	case StateRunning:
		return to == StateCleaning
	case StateCleaning:
		return to == StateFinished
	default:
		return false
	}
}
