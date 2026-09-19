package reconcile

type Stage string

const (
	StageController       Stage = "controller"
	StageLoadInstances    Stage = "load_instances"
	StageCollectOrphans   Stage = "collect_orphans"
	StageFetchJobs        Stage = "fetch_jobs"
	StageScaleDown        Stage = "scale_down"
	StageRegisterRunner   Stage = "register_runner"
	StateCreateInstance   Stage = "create_instance"
	StageWaitCloudInit    Stage = "wait_cloud_init"
	StagePushRunnerConfig Stage = "push_runner_config"
	StageWaitRunner       Stage = "wait_runner"
	StageObserveRunner    Stage = "observe_runner"
	StageDeleteRunner     Stage = "delete_runner"
	StageDeleteInstance   Stage = "delete_instance"
	StagePersistState     Stage = "persist_state"
)

type FailureCode string

const (
	FailureTimeout      FailureCode = "timeout"
	FailureCanceled     FailureCode = "canceled"
	FailureUnauthorized FailureCode = "unauthorized"
	FailureForbidden    FailureCode = "forbidden"
	FailureNotFound     FailureCode = "not_found"
	FailureUnavailable  FailureCode = "unavailable"
	FailureConflict     FailureCode = "conflict"
	FailureDatabase     FailureCode = "database"
	FailureInterrupted  FailureCode = "interrupted"
	FailureUnknown      FailureCode = "unknown"
)
