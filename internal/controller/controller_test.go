package controller

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/maveonair/farm/internal/bootstrap"
	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/forgejo"
	"github.com/maveonair/farm/internal/incus"
	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
	"github.com/maveonair/farm/internal/store"
)

func TestReconcilePoolProvisionsInstance(t *testing.T) {
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
	}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{}
	controller := New(forge, instanceBackend, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
		Install: bootstrap.Install{
			DownloadURL: "https://downloads.example.com/forgejo-runner",
			SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if instanceBackend.created.Name == "" {
		t.Fatal("instance was not created")
	}
	if bytes.Contains(instanceBackend.created.CloudInit, []byte("runner-token")) {
		t.Fatal("cloud-init contains runner token")
	}
	if !bytes.Contains(instanceBackend.runnerConfig, []byte("runner-token")) {
		t.Fatal("runner config does not contain token")
	}
	if repository.instance.State != instance.StateReady {
		t.Fatalf("state = %q", repository.instance.State)
	}
}

func TestReconcilePoolUsesImageRunner(t *testing.T) {
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
	}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)
	pool := testPool()
	pool.Instance.RunnerInstall = config.RunnerInstallImage

	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if len(instanceBackend.created.CloudInit) != 0 {
		t.Fatal("image runner received cloud-init")
	}
	if instanceBackend.waitAgent != 1 || instanceBackend.waitCloudInit != 0 {
		t.Fatalf("readiness waits: agent=%d cloud-init=%d", instanceBackend.waitAgent, instanceBackend.waitCloudInit)
	}
	if !bytes.Contains(instanceBackend.runnerConfig, []byte("runner-token")) {
		t.Fatal("runner config does not contain token")
	}
}

func TestImageRunnerAgentFailureCleansUp(t *testing.T) {
	agentErr := errors.New("agent unavailable")
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
	}
	instanceBackend := &fakeInstances{waitAgentErr: agentErr}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)
	pool := testPool()
	pool.Instance.RunnerInstall = config.RunnerInstallImage

	err := controller.ReconcilePool(context.Background(), pool)
	if !errors.Is(err, agentErr) {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !instanceBackend.deleted || !forge.deleted {
		t.Fatal("failed image runner resources were not deleted")
	}
	if repository.result.Failure.Stage != reconcile.StageWaitAgent {
		t.Fatalf("failure stage = %q", repository.result.Failure.Stage)
	}
}

func TestReconcilePoolCompletesFastWorkflow(t *testing.T) {
	forge := &fakeForge{
		jobs:      []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
		runnerErr: &forgejo.HTTPError{StatusCode: http.StatusNotFound, Body: "missing"},
	}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !instanceBackend.deleted || !forge.deleted {
		t.Fatal("completed runner resources were not deleted")
	}
	if repository.instance.State != instance.StateFinished {
		t.Fatalf("state = %q", repository.instance.State)
	}
	if repository.instance.Result != instance.ResultSucceeded || repository.instance.Reason != instance.ReasonJobCompleted {
		t.Fatalf("outcome = %q, reason = %q", repository.instance.Result, repository.instance.Reason)
	}
}

func TestReconcilePoolIgnoresUnmatchedJob(t *testing.T) {
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"other"}}},
	}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if instanceBackend.created.Name != "" {
		t.Fatalf("created instance %q", instanceBackend.created.Name)
	}
}

func TestReconcilePoolRecordsForgejoFailure(t *testing.T) {
	forge := &fakeForge{jobsErr: &forgejo.HTTPError{StatusCode: http.StatusServiceUnavailable, Body: "offline"}}
	repository := &fakeRepository{}
	controller := newTestController(forge, &fakeInstances{}, repository)

	err := controller.ReconcilePool(context.Background(), testPool())
	if err == nil {
		t.Fatal("ReconcilePool() error = nil")
	}
	if repository.result.State != store.RuntimeFailed {
		t.Fatalf("state = %q", repository.result.State)
	}
	if repository.result.Failure.Stage != reconcile.StageFetchJobs || repository.result.Failure.Code != reconcile.FailureUnavailable {
		t.Fatalf("failure = %#v", repository.result.Failure)
	}
}

func TestReconcilePoolReportsProgress(t *testing.T) {
	repository := &fakeRepository{}
	controller := newTestController(&fakeForge{}, &fakeInstances{}, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	found := false
	for _, progress := range repository.progress {
		if progress.Stage == reconcile.StageFetchJobs {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("progress = %#v", repository.progress)
	}
}

func TestReconcilePoolBoundsFinalWrite(t *testing.T) {
	repository := &fakeRepository{}
	controller := New(&fakeForge{}, &fakeInstances{}, repository, Options{
		ID:             "primary",
		ForgeURL:       "https://git.example.com",
		CleanupTimeout: time.Minute,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := controller.ReconcilePool(ctx, testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !repository.finishHasDeadline {
		t.Fatal("FinishPool context has no deadline")
	}
	if repository.finishContextErr != nil {
		t.Fatalf("FinishPool context error = %v", repository.finishContextErr)
	}
}

func TestReconcilePoolDeletesCompletedInstance(t *testing.T) {
	forge := &fakeForge{
		runnerErr: &forgejo.HTTPError{StatusCode: http.StatusNotFound, Body: "missing"},
	}
	instanceBackend := &fakeInstances{managed: []incus.ManagedInstance{{
		ID: "instance-id", Name: "farm-ubuntu-instance", Pool: "ubuntu",
	}}}
	repository := &fakeRepository{instance: instance.Instance{
		ID:             "instance-id",
		Name:           "farm-ubuntu-instance",
		Pool:           "ubuntu",
		State:          instance.StateRunning,
		RunnerID:       42,
		StateChangedAt: time.Now(),
	}}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !instanceBackend.deleted {
		t.Fatal("instance was not deleted")
	}
	if repository.instance.State != instance.StateFinished {
		t.Fatalf("state = %q", repository.instance.State)
	}
	if repository.instance.Result != instance.ResultSucceeded || repository.instance.Reason != instance.ReasonJobCompleted {
		t.Fatalf("outcome = %q, reason = %q", repository.instance.Result, repository.instance.Reason)
	}
}

func TestReconcilePoolCompletesMissingReadyRunner(t *testing.T) {
	forge := &fakeForge{runnerErr: &forgejo.HTTPError{StatusCode: http.StatusNotFound, Body: "missing"}}
	instanceBackend := &fakeInstances{managed: []incus.ManagedInstance{{
		ID: "instance-id", Name: "farm-ubuntu-instance", Pool: "ubuntu",
	}}}
	repository := &fakeRepository{instance: instance.Instance{
		ID: "instance-id", Name: "farm-ubuntu-instance", Pool: "ubuntu",
		State: instance.StateReady, RunnerID: 42, StateChangedAt: time.Now(),
	}}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if repository.instance.Result != instance.ResultSucceeded || repository.instance.Reason != instance.ReasonJobCompleted {
		t.Fatalf("outcome = %q, reason = %q", repository.instance.Result, repository.instance.Reason)
	}
}

func TestReconcilePoolRecyclesInterruptedInstance(t *testing.T) {
	forge := &fakeForge{runnerStatus: forgejo.RunnerOffline}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:       "instance-id",
		Name:     "farm-ubuntu-instance",
		Pool:     "ubuntu",
		State:    instance.StateBootstrapping,
		RunnerID: 42,
	}}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !instanceBackend.deleted || !forge.deleted {
		t.Fatal("interrupted resources were not deleted")
	}
}

func TestReconcilePoolAdoptsStartedRunner(t *testing.T) {
	forge := &fakeForge{runnerStatus: forgejo.RunnerIdle}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:       "instance-id",
		Name:     "farm-ubuntu-instance",
		Pool:     "ubuntu",
		State:    instance.StateBootstrapping,
		RunnerID: 42,
	}}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if instanceBackend.deleted {
		t.Fatal("running instance was deleted")
	}
	if repository.instance.State != instance.StateReady {
		t.Fatalf("state = %q", repository.instance.State)
	}
}

func TestReconcilePoolRetriesDeletion(t *testing.T) {
	deleteErr := errors.New("Incus unavailable")
	forge := &fakeForge{}
	instanceBackend := &fakeInstances{deleteErr: deleteErr}
	repository := &fakeRepository{instance: instance.Instance{
		ID:    "instance-id",
		Name:  "farm-ubuntu-instance",
		Pool:  "ubuntu",
		State: instance.StateCleaning,
	}}
	controller := newTestController(forge, instanceBackend, repository)

	err := controller.ReconcilePool(context.Background(), testPool())
	if !errors.Is(err, deleteErr) {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if repository.instance.State != instance.StateCleaning {
		t.Fatalf("state = %q", repository.instance.State)
	}
	if repository.instance.RetryCount != 1 || repository.instance.RetryAt.IsZero() {
		t.Fatalf("retry = %d at %v", repository.instance.RetryCount, repository.instance.RetryAt)
	}
}

func TestReconcilePoolScalesDownIdleInstance(t *testing.T) {
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	forge := &fakeForge{runnerStatus: forgejo.RunnerIdle}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:             "instance-id",
		Name:           "farm-ubuntu-instance",
		Pool:           "ubuntu",
		State:          instance.StateReady,
		RunnerID:       42,
		StateChangedAt: now.Add(-2 * time.Minute),
	}}
	controller := New(forge, instanceBackend, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
		Now:      func() time.Time { return now },
	})
	pool := testPool()
	pool.Scaling.IdleTimeout.Duration = time.Minute

	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !instanceBackend.deleted {
		t.Fatal("idle instance was not deleted")
	}
}

func TestReconcilePoolHonorsRetryAt(t *testing.T) {
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	forge := &fakeForge{}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:      "instance-id",
		Name:    "farm-ubuntu-instance",
		Pool:    "ubuntu",
		State:   instance.StateCleaning,
		RetryAt: now.Add(time.Minute),
	}}
	controller := New(forge, instanceBackend, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
		Now:      func() time.Time { return now },
	})

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if instanceBackend.deleted {
		t.Fatal("instance deletion ignored retry time")
	}
}

func TestBootstrapFailuresPauseAndProbe(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	createErr := errors.New("image not found")
	forge := &fakeForge{jobs: []forgejo.Job{
		{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}},
		{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}},
		{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}},
	}}
	instanceBackend := &fakeInstances{createErr: createErr}
	repository, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "farm.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	controller := New(forge, instanceBackend, repository, Options{
		ID: "primary", ForgeURL: "https://git.example.com", Now: func() time.Time { return now },
	})
	pool := testPool()
	pool.Scaling.MaxInstances = 3
	pool.Scaling.MaxProvisioning = 3
	pool.Scaling.BootstrapAttemptLimit = 2
	pool.Scaling.BootstrapRetryInterval.Duration = time.Hour

	if err := controller.ReconcilePool(context.Background(), pool); !errors.Is(err, createErr) {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("paused ReconcilePool() error = %v", err)
	}
	if calls := instanceBackend.createCount(); calls != pool.Scaling.BootstrapAttemptLimit {
		t.Fatalf("create calls while paused = %d", calls)
	}

	now = now.Add(pool.Scaling.BootstrapRetryInterval.Duration)
	if err := controller.ReconcilePool(context.Background(), pool); !errors.Is(err, createErr) {
		t.Fatalf("probe ReconcilePool() error = %v", err)
	}
	if calls := instanceBackend.createCount(); calls != pool.Scaling.BootstrapAttemptLimit+1 {
		t.Fatalf("create calls after probe = %d", calls)
	}

	now = now.Add(pool.Scaling.BootstrapRetryInterval.Duration)
	instanceBackend.createErr = nil
	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("recovery ReconcilePool() error = %v", err)
	}
	data, err := repository.ListPoolData(context.Background())
	if err != nil {
		t.Fatalf("ListPoolData() error = %v", err)
	}
	if len(data) != 1 || data[0].Runtime.Bootstrap.Attempts != 0 || !data[0].Runtime.Bootstrap.RetryAt.IsZero() {
		t.Fatalf("pool data after recovery = %#v", data)
	}
}

func TestEphemeralInstanceLifecycle(t *testing.T) {
	forge := &fakeForge{
		jobs:         []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
		runnerStatus: forgejo.RunnerIdle,
	}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{}
	controller := New(forge, instanceBackend, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
		Install: bootstrap.Install{
			DownloadURL: "https://downloads.example.com/forgejo-runner",
			SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})
	pool := testPool()

	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("provision: %v", err)
	}
	if repository.instance.State != instance.StateReady {
		t.Fatalf("provisioned state = %q", repository.instance.State)
	}

	forge.jobs = nil
	forge.runnerStatus = forgejo.RunnerActive
	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if repository.instance.State != instance.StateRunning {
		t.Fatalf("active state = %q", repository.instance.State)
	}

	forge.runnerErr = &forgejo.HTTPError{StatusCode: http.StatusNotFound, Body: "missing"}
	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if repository.instance.State != instance.StateFinished || !instanceBackend.deleted {
		t.Fatalf("completed instance = %#v", repository.instance)
	}
}

func TestFailureCleanupUsesLiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	createErr := errors.New("create failed")
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
	}
	instanceBackend := &fakeInstances{createErr: createErr, cancelOnCreate: cancel}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)

	err := controller.ReconcilePool(ctx, testPool())
	if !errors.Is(err, createErr) {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if forge.deleteCtxErr != nil {
		t.Fatalf("cleanup context error = %v", forge.deleteCtxErr)
	}
}

func TestReconcilePoolDeletesOrphanedInstance(t *testing.T) {
	forge := &fakeForge{}
	instanceBackend := &fakeInstances{managed: []incus.ManagedInstance{{
		ID:   "orphan-id",
		Name: "farm-orphan",
		Pool: "ubuntu",
	}}}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !instanceBackend.deleted {
		t.Fatal("orphaned instance was not deleted")
	}
}

func TestReconcilePoolDeletesOrphanedRunner(t *testing.T) {
	forge := &fakeForge{runners: []forgejo.Runner{{
		ID:          99,
		Description: runnerDescription("primary", "ubuntu"),
	}}}
	instanceBackend := &fakeInstances{}
	repository := &fakeRepository{}
	controller := newTestController(forge, instanceBackend, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !forge.deleted {
		t.Fatal("orphaned runner was not deleted")
	}
}

func TestScopeForUser(t *testing.T) {
	scope := scopeFor(config.Pool{Scope: config.Scope{Type: config.ScopeUser}})

	if scope.Kind != forgejo.ScopeUser {
		t.Fatalf("scope kind = %q", scope.Kind)
	}
}

func newTestController(forge *fakeForge, instanceBackend *fakeInstances, repository *fakeRepository) *Controller {
	return New(forge, instanceBackend, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
	})
}

func testPool() config.Pool {
	return config.Pool{
		Name:   "ubuntu",
		Scope:  config.Scope{Type: config.ScopeRepository, Owner: "example", Repository: "project"},
		Labels: []string{"farm-ubuntu"},
		Instance: config.Instance{
			Image:    config.Image{Alias: "farm-ubuntu-24.04"},
			Profiles: []string{"farm-instance"},
		},
		Scaling: config.Scaling{
			MaxInstances:           2,
			MaxProvisioning:        1,
			BootstrapAttemptLimit:  5,
			BootstrapRetryInterval: config.Duration{Duration: 15 * time.Minute},
			StartupTimeout:         config.Duration{Duration: time.Minute},
			IdleTimeout:            config.Duration{Duration: time.Minute},
			MaxLifetime:            config.Duration{Duration: time.Hour},
		},
	}
}

type fakeForge struct {
	mu           sync.Mutex
	jobs         []forgejo.Job
	jobsErr      error
	runners      []forgejo.Runner
	runnerErr    error
	runnerStatus forgejo.RunnerStatus
	deleted      bool
	deleteCtxErr error
}

func (f *fakeForge) Runners(context.Context, forgejo.Scope) ([]forgejo.Runner, error) {
	return f.runners, nil
}

func (f *fakeForge) Jobs(context.Context, forgejo.Scope) ([]forgejo.Job, error) {
	return f.jobs, f.jobsErr
}

func (f *fakeForge) Register(context.Context, forgejo.Scope, string, string) (forgejo.Registration, error) {
	return forgejo.Registration{ID: 42, UUID: "runner-uuid", Token: "runner-token"}, nil
}

func (f *fakeForge) Runner(context.Context, forgejo.Scope, int64) (forgejo.Runner, error) {
	if f.runnerErr != nil {
		return forgejo.Runner{}, f.runnerErr
	}
	status := f.runnerStatus
	if status == "" {
		status = forgejo.RunnerIdle
	}
	return forgejo.Runner{Status: status}, nil
}

func (f *fakeForge) DeleteRunner(ctx context.Context, _ forgejo.Scope, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleted = true
	f.deleteCtxErr = ctx.Err()
	return nil
}

type fakeInstances struct {
	mu             sync.Mutex
	created        incus.InstanceSpec
	managed        []incus.ManagedInstance
	runnerConfig   []byte
	deleted        bool
	deleteErr      error
	createErr      error
	cancelOnCreate func()
	deleteCtxErr   error
	waitAgent      int
	waitAgentErr   error
	waitCloudInit  int
	createCalls    int
}

func (v *fakeInstances) Managed(context.Context, string, string) ([]incus.ManagedInstance, error) {
	return v.managed, nil
}

func (v *fakeInstances) Create(_ context.Context, spec incus.InstanceSpec) error {
	v.mu.Lock()
	v.createCalls++
	cancel := v.cancelOnCreate
	createErr := v.createErr
	v.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if createErr != nil {
		return createErr
	}
	v.created = spec
	v.managed = append(v.managed, incus.ManagedInstance{
		ID: spec.ID, Name: spec.Name, Pool: spec.Pool,
	})
	return nil
}

func (v *fakeInstances) createCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.createCalls
}

func (v *fakeInstances) WaitCloudInit(context.Context, string) error {
	v.waitCloudInit++
	return nil
}

func (v *fakeInstances) WaitAgent(context.Context, string) error {
	v.waitAgent++
	return v.waitAgentErr
}

func (v *fakeInstances) PushRunnerConfig(_ context.Context, _ string, data []byte) error {
	v.runnerConfig = append([]byte(nil), data...)
	return nil
}

func (v *fakeInstances) Delete(ctx context.Context, _ string) error {
	v.deleted = true
	v.deleteCtxErr = ctx.Err()
	v.managed = nil
	return v.deleteErr
}

type fakeRepository struct {
	instance          instance.Instance
	observation       store.PoolObservation
	result            store.PoolResult
	progress          []store.PoolProgress
	bootstrap         store.BootstrapState
	finishHasDeadline bool
	finishContextErr  error
}

func (r *fakeRepository) Create(_ context.Context, instance instance.Instance) error {
	r.instance = instance
	return nil
}

func (r *fakeRepository) ListPool(context.Context, string) ([]instance.Instance, error) {
	if r.instance.ID == "" || r.instance.State == instance.StateFinished {
		return nil, nil
	}
	return []instance.Instance{r.instance}, nil
}

func (r *fakeRepository) StartPool(context.Context, store.PoolRun) error {
	return nil
}

func (r *fakeRepository) ObservePool(_ context.Context, observation store.PoolObservation) error {
	r.observation = observation
	return nil
}

func (r *fakeRepository) ProgressPool(_ context.Context, progress store.PoolProgress) error {
	r.progress = append(r.progress, progress)
	return nil
}

func (r *fakeRepository) FinishPool(ctx context.Context, result store.PoolResult) error {
	r.result = result
	_, r.finishHasDeadline = ctx.Deadline()
	r.finishContextErr = ctx.Err()
	return nil
}

func (r *fakeRepository) SetRegistration(_ context.Context, _ string, id int64) error {
	r.instance.RunnerID = id
	r.instance.State = instance.StateBootstrapping
	return nil
}

func (r *fakeRepository) SetStage(_ context.Context, _ string, stage reconcile.Stage) error {
	r.instance.Stage = stage
	return nil
}

func (r *fakeRepository) MarkReady(context.Context, string) error {
	r.instance.State = instance.StateReady
	r.instance.StateChangedAt = time.Now()
	return nil
}

func (r *fakeRepository) MarkRunning(context.Context, string) error {
	r.instance.State = instance.StateRunning
	r.instance.StateChangedAt = time.Now()
	return nil
}

func (r *fakeRepository) BeginCleanup(_ context.Context, _ string, result instance.Result, reason instance.Reason, message string) error {
	r.instance.State = instance.StateCleaning
	r.instance.Result = result
	r.instance.Reason = reason
	r.instance.Error = message
	return nil
}

func (r *fakeRepository) FinishCleanup(context.Context, string) error {
	r.instance.State = instance.StateFinished
	return nil
}

func (r *fakeRepository) Retry(_ context.Context, _ string, message string, retryAt time.Time) error {
	r.instance.RetryCount++
	r.instance.RetryAt = retryAt
	r.instance.Error = message
	return nil
}

func (r *fakeRepository) ReserveBootstrap(_ context.Context, request store.BootstrapRequest) (store.BootstrapReservation, error) {
	if r.bootstrap.Attempts >= request.Limit {
		if !r.bootstrap.RetryAt.IsZero() && request.Now.Before(r.bootstrap.RetryAt) {
			return store.BootstrapReservation{State: r.bootstrap}, nil
		}
		if r.bootstrap.RetryAt.IsZero() {
			r.bootstrap.RetryAt = request.RetryAt
			return store.BootstrapReservation{State: r.bootstrap}, nil
		}
		r.bootstrap.RetryAt = request.RetryAt
		return store.BootstrapReservation{Count: 1, Probe: true, State: r.bootstrap}, nil
	}

	count := min(request.Wanted, request.Limit-r.bootstrap.Attempts)
	r.bootstrap.Attempts += count
	return store.BootstrapReservation{Count: count, State: r.bootstrap}, nil
}

func (r *fakeRepository) HoldBootstrap(_ context.Context, _ string, retryAt time.Time) error {
	r.bootstrap.RetryAt = retryAt
	return nil
}

func (r *fakeRepository) ResetBootstrap(context.Context, string) error {
	r.bootstrap = store.BootstrapState{}
	return nil
}
