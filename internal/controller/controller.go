package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/maveonair/farm/internal/bootstrap"
	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/forgejo"
	incusvm "github.com/maveonair/farm/internal/incus"
	farmInstance "github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
	"github.com/maveonair/farm/internal/scheduler"
	"github.com/maveonair/farm/internal/store"
)

const (
	idBytes          = 16
	runnerPollPeriod = 2 * time.Second
	retryBase        = 5 * time.Second
	retryMax         = 5 * time.Minute
)

type forge interface {
	Jobs(context.Context, forgejo.Scope) ([]forgejo.Job, error)
	Runners(context.Context, forgejo.Scope) ([]forgejo.Runner, error)
	Register(context.Context, forgejo.Scope, string, string) (forgejo.Registration, error)
	Runner(context.Context, forgejo.Scope, int64) (forgejo.Runner, error)
	DeleteRunner(context.Context, forgejo.Scope, int64) error
}

type instances interface {
	Create(context.Context, incusvm.InstanceSpec) error
	Managed(context.Context, string, string) ([]incusvm.ManagedInstance, error)
	WaitAgent(context.Context, string) error
	WaitCloudInit(context.Context, string) error
	PushRunnerConfig(context.Context, string, []byte) error
	Delete(context.Context, string) error
}

type repository interface {
	Create(context.Context, farmInstance.Instance) error
	ListPool(context.Context, string) ([]farmInstance.Instance, error)
	StartPool(context.Context, store.PoolRun) error
	ObservePool(context.Context, store.PoolObservation) error
	ProgressPool(context.Context, store.PoolProgress) error
	FinishPool(context.Context, store.PoolResult) error
	SetRegistration(context.Context, string, int64) error
	SetStage(context.Context, string, reconcile.Stage) error
	MarkReady(context.Context, string) error
	MarkRunning(context.Context, string) error
	BeginCleanup(context.Context, string, farmInstance.Result, farmInstance.Reason, string) error
	FinishCleanup(context.Context, string) error
	Retry(context.Context, string, string, time.Time) error
	ReserveBootstrap(context.Context, store.BootstrapRequest) (store.BootstrapReservation, error)
	HoldBootstrap(context.Context, string, time.Time) error
	ResetBootstrap(context.Context, string) error
}

type Controller struct {
	forge          forge
	instances      instances
	store          repository
	url            string
	install        bootstrap.Install
	id             string
	now            func() time.Time
	cleanupTimeout time.Duration
	requestTimeout time.Duration
	progress       func(store.PoolProgress)
	logger         *slog.Logger
}

type Options struct {
	ID             string
	ForgeURL       string
	Install        bootstrap.Install
	CleanupTimeout time.Duration
	RequestTimeout time.Duration
	Progress       func(store.PoolProgress)
	Now            func() time.Time
	Logger         *slog.Logger
}

type reconcileError struct {
	stage    reconcile.Stage
	code     reconcile.FailureCode
	instance farmInstance.Instance
	err      error
}

func (e *reconcileError) Error() string {
	return e.err.Error()
}

func (e *reconcileError) Unwrap() error {
	return e.err
}

func operationError(stage reconcile.Stage, code reconcile.FailureCode, instance farmInstance.Instance, err error) error {
	if err == nil {
		return nil
	}
	var existing *reconcileError
	if errors.As(err, &existing) {
		return err
	}
	return &reconcileError{stage: stage, code: code, instance: instance, err: err}
}

func failureFrom(err error) store.PoolFailure {
	failure := store.PoolFailure{
		Stage: reconcile.StageController, Code: classify(err), Message: err.Error(),
	}
	var operation *reconcileError
	if !errors.As(err, &operation) {
		return failure
	}
	failure.Stage = operation.stage
	failure.Code = operation.code
	failure.InstanceID = operation.instance.ID
	failure.InstanceName = operation.instance.Name
	return failure
}

func DescribeError(err error) store.PoolFailure {
	return failureFrom(err)
}

func classify(err error) reconcile.FailureCode {
	if errors.Is(err, context.DeadlineExceeded) {
		return reconcile.FailureTimeout
	}
	if errors.Is(err, context.Canceled) {
		return reconcile.FailureCanceled
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return reconcile.FailureTimeout
		}
		return reconcile.FailureUnavailable
	}
	var httpErr *forgejo.HTTPError
	if !errors.As(err, &httpErr) {
		return reconcile.FailureUnknown
	}
	switch httpErr.StatusCode {
	case http.StatusUnauthorized:
		return reconcile.FailureUnauthorized
	case http.StatusForbidden:
		return reconcile.FailureForbidden
	case http.StatusNotFound:
		return reconcile.FailureNotFound
	case http.StatusConflict:
		return reconcile.FailureConflict
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return reconcile.FailureUnavailable
	default:
		return reconcile.FailureUnknown
	}
}

func New(forge forge, vms instances, repository repository, options Options) *Controller {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Controller{
		forge:          forge,
		instances:      vms,
		store:          repository,
		url:            options.ForgeURL,
		install:        options.Install,
		id:             options.ID,
		now:            now,
		cleanupTimeout: options.CleanupTimeout,
		requestTimeout: options.RequestTimeout,
		progress:       options.Progress,
		logger:         logger.With("controller", options.ID),
	}
}

func (c *Controller) ReconcilePool(ctx context.Context, pool config.Pool) (reconcileErr error) {
	runID, err := newID()
	if err != nil {
		return err
	}
	if err := c.store.StartPool(ctx, store.PoolRun{Pool: pool.Name, RunID: runID, StartedAt: c.now()}); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}
	if err := c.setProgress(ctx, pool.Name, runID, reconcile.StageLoadInstances, c.now().Add(c.maintenanceBudget(pool))); err != nil {
		return err
	}
	defer func() {
		result := store.PoolResult{
			Pool: pool.Name, RunID: runID, State: store.RuntimeSucceeded, FinishedAt: c.now(),
		}
		if reconcileErr != nil {
			result.State = store.RuntimeFailed
			result.Failure = failureFrom(reconcileErr)
		}
		if err := c.store.FinishPool(context.WithoutCancel(ctx), result); err != nil {
			reconcileErr = errors.Join(reconcileErr,
				operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err))
		}
	}()

	scope := scopeFor(pool)
	if err := c.reconcileInstances(ctx, pool, scope); err != nil {
		return operationError(reconcile.StageObserveRunner, reconcile.FailureUnknown, farmInstance.Instance{}, err)
	}

	if err := c.setProgress(ctx, pool.Name, runID, reconcile.StageFetchJobs, c.now().Add(c.requestTimeout)); err != nil {
		return err
	}
	jobs, err := c.forge.Jobs(ctx, scope)
	if err != nil {
		return operationError(reconcile.StageFetchJobs, classify(err), farmInstance.Instance{}, err)
	}

	waiting := 0
	for _, job := range jobs {
		if job.Status == forgejo.JobWaiting && matches(pool.Labels, job.RunsOn) {
			waiting++
		}
	}
	if err := c.store.ObservePool(ctx, store.PoolObservation{
		Pool: pool.Name, RunID: runID, Waiting: waiting, ObservedAt: c.now(),
	}); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}

	instances, err := c.store.ListPool(ctx, pool.Name)
	if err != nil {
		return operationError(reconcile.StageLoadInstances, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}
	if err := c.setProgress(ctx, pool.Name, runID, reconcile.StageScaleDown, c.now().Add(c.maintenanceBudget(pool))); err != nil {
		return err
	}
	if err := c.scaleDown(ctx, pool, scope, instances, waiting); err != nil {
		return operationError(reconcile.StageScaleDown, classify(err), farmInstance.Instance{}, err)
	}
	instances, err = c.store.ListPool(ctx, pool.Name)
	if err != nil {
		return operationError(reconcile.StageLoadInstances, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}
	needed := scheduler.Needed(scheduler.Capacity{
		Waiting:         waiting,
		MinIdle:         pool.Scaling.MinIdle,
		MaxInstances:    pool.Scaling.MaxInstances,
		MaxProvisioning: pool.Scaling.MaxProvisioning,
		Instances:       instances,
	})
	c.logger.DebugContext(
		ctx, "capacity calculated",
		"event", "capacity_calculated",
		"pool", pool.Name,
		"waiting_jobs", waiting,
		"instances", len(instances),
		"needed", needed,
	)

	return c.provisionNeeded(ctx, pool, scope, runID, needed)
}

func (c *Controller) provisionNeeded(ctx context.Context, pool config.Pool, scope forgejo.Scope, runID string, needed int) error {
	if needed == 0 {
		return nil
	}

	now := c.now()
	reservation, err := c.store.ReserveBootstrap(ctx, store.BootstrapRequest{
		Pool: pool.Name, Wanted: needed, Limit: pool.Scaling.BootstrapAttemptLimit,
		Now: now, RetryAt: now.Add(pool.Scaling.BootstrapRetryInterval.Duration),
	})
	if err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}
	if reservation.Count == 0 {
		return nil
	}
	if reservation.Probe {
		c.logger.InfoContext(ctx, "bootstrap recovery probe started",
			"event", "bootstrap_probe_started",
			"pool", pool.Name,
			"bootstrap_attempts", reservation.State.Attempts,
			"bootstrap_attempt_limit", pool.Scaling.BootstrapAttemptLimit,
		)
	}

	result := c.provisionMany(ctx, pool, scope, runID, reservation.Count)
	if result.succeeded > 0 {
		if err := c.store.ResetBootstrap(ctx, pool.Name); err != nil {
			return errors.Join(result.err,
				operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err))
		}
		if reservation.Probe {
			c.logger.InfoContext(ctx, "bootstrap circuit recovered",
				"event", "bootstrap_circuit_recovered",
				"pool", pool.Name,
			)
		}
		return result.err
	}
	if result.err == nil || reservation.State.Attempts < pool.Scaling.BootstrapAttemptLimit {
		return result.err
	}

	retryAt := c.now().Add(pool.Scaling.BootstrapRetryInterval.Duration)
	if err := c.store.HoldBootstrap(ctx, pool.Name, retryAt); err != nil {
		return errors.Join(result.err,
			operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err))
	}
	event := "bootstrap_circuit_opened"
	message := "bootstrap circuit opened"
	if reservation.Probe {
		event = "bootstrap_probe_failed"
		message = "bootstrap recovery probe failed"
	}
	c.logger.WarnContext(ctx, message,
		"event", event,
		"pool", pool.Name,
		"bootstrap_attempts", reservation.State.Attempts,
		"bootstrap_attempt_limit", pool.Scaling.BootstrapAttemptLimit,
		"bootstrap_retry_at", retryAt,
	)
	return result.err
}

func (c *Controller) maintenanceBudget(pool config.Pool) time.Duration {
	cleanup := time.Duration(pool.Scaling.MaxInstances) * c.cleanupTimeout
	requests := time.Duration(pool.Scaling.MaxInstances+3) * c.requestTimeout
	return cleanup + requests
}

func (c *Controller) setProgress(ctx context.Context, pool, runID string, stage reconcile.Stage, deadline time.Time) error {
	progress := store.PoolProgress{
		Pool: pool, RunID: runID, Stage: stage, StartedAt: c.now(), DeadlineAt: deadline,
	}
	if err := c.store.ProgressPool(ctx, progress); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}
	if c.progress != nil {
		c.progress(progress)
	}
	return nil
}

func (c *Controller) setInstanceStage(ctx context.Context, record farmInstance.Instance, stage reconcile.Stage) error {
	if err := c.store.SetStage(ctx, record.ID, stage); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, record, err)
	}
	return nil
}

func (c *Controller) markState(ctx context.Context, id string, state farmInstance.State) error {
	if state == farmInstance.StateReady {
		return c.store.MarkReady(ctx, id)
	}
	return c.store.MarkRunning(ctx, id)
}

func scopeFor(pool config.Pool) forgejo.Scope {
	kind := forgejo.ScopeRepository
	switch pool.Scope.Type {
	case config.ScopeOrganization:
		kind = forgejo.ScopeOrganization
	case config.ScopeUser:
		kind = forgejo.ScopeUser
	case config.ScopeGlobal:
		kind = forgejo.ScopeGlobal
	}
	return forgejo.Scope{Kind: kind, Owner: pool.Scope.Owner, Repository: pool.Scope.Repository}
}

func matches(poolLabels, jobLabels []string) bool {
	if len(poolLabels) == 0 || len(jobLabels) == 0 || poolLabels[0] != jobLabels[0] {
		return false
	}
	jobSet := make(map[string]struct{}, len(jobLabels))
	for _, label := range jobLabels {
		jobSet[label] = struct{}{}
	}
	for _, label := range poolLabels {
		if _, found := jobSet[label]; !found {
			return false
		}
	}
	return true
}

func runnerLabels(labels []string) []string {
	result := make([]string, len(labels))
	for i, label := range labels {
		result[i] = label + ":host"
	}
	return result
}

func newID() (string, error) {
	value := make([]byte, idBytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate instance ID: %w", err)
	}
	return strings.ToLower(hex.EncodeToString(value)), nil
}
