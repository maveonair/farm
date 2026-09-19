package controller

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/maveonair/farm/internal/bootstrap"
	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/forgejo"
	incusvm "github.com/maveonair/farm/internal/incus"
	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
	"github.com/maveonair/farm/internal/store"
)

func TestReconcilePoolProvisionsVM(t *testing.T) {
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
	}
	vms := &fakeVM{}
	repository := &fakeRepository{}
	controller := New(forge, vms, repository, Options{
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
	if vms.created.Name == "" {
		t.Fatal("VM was not created")
	}
	if bytes.Contains(vms.created.CloudInit, []byte("runner-token")) {
		t.Fatal("cloud-init contains runner token")
	}
	if !bytes.Contains(vms.runnerConfig, []byte("runner-token")) {
		t.Fatal("runner config does not contain token")
	}
	if repository.instance.State != instance.StateReady {
		t.Fatalf("state = %q", repository.instance.State)
	}
}

func TestReconcilePoolCompletesFastWorkflow(t *testing.T) {
	forge := &fakeForge{
		jobs:      []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
		runnerErr: &forgejo.HTTPError{StatusCode: http.StatusNotFound, Body: "missing"},
	}
	vms := &fakeVM{}
	repository := &fakeRepository{}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !vms.deleted || !forge.deleted {
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
	vms := &fakeVM{}
	repository := &fakeRepository{}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if vms.created.Name != "" {
		t.Fatalf("created VM %q", vms.created.Name)
	}
}

func TestReconcilePoolRecordsForgejoFailure(t *testing.T) {
	forge := &fakeForge{jobsErr: &forgejo.HTTPError{StatusCode: http.StatusServiceUnavailable, Body: "offline"}}
	repository := &fakeRepository{}
	controller := newTestController(forge, &fakeVM{}, repository)

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
	controller := newTestController(&fakeForge{}, &fakeVM{}, repository)

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

func TestReconcilePoolDeletesCompletedVM(t *testing.T) {
	forge := &fakeForge{
		runnerErr: &forgejo.HTTPError{StatusCode: http.StatusNotFound, Body: "missing"},
	}
	vms := &fakeVM{managed: []incusvm.ManagedInstance{{
		ID: "instance-id", Name: "farm-ubuntu-instance", Pool: "ubuntu", Type: "virtual-machine",
	}}}
	repository := &fakeRepository{instance: instance.Instance{
		ID:             "instance-id",
		Name:           "farm-ubuntu-instance",
		Pool:           "ubuntu",
		State:          instance.StateRunning,
		RunnerID:       42,
		StateChangedAt: time.Now(),
	}}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !vms.deleted {
		t.Fatal("VM was not deleted")
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
	vms := &fakeVM{managed: []incusvm.ManagedInstance{{
		ID: "instance-id", Name: "farm-ubuntu-instance", Pool: "ubuntu", Type: "virtual-machine",
	}}}
	repository := &fakeRepository{instance: instance.Instance{
		ID: "instance-id", Name: "farm-ubuntu-instance", Pool: "ubuntu",
		State: instance.StateReady, RunnerID: 42, StateChangedAt: time.Now(),
	}}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if repository.instance.Result != instance.ResultSucceeded || repository.instance.Reason != instance.ReasonJobCompleted {
		t.Fatalf("outcome = %q, reason = %q", repository.instance.Result, repository.instance.Reason)
	}
}

func TestReconcilePoolRecyclesInterruptedVM(t *testing.T) {
	forge := &fakeForge{runnerStatus: forgejo.RunnerOffline}
	vms := &fakeVM{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:       "instance-id",
		Name:     "farm-ubuntu-instance",
		Pool:     "ubuntu",
		State:    instance.StateBootstrapping,
		RunnerID: 42,
	}}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !vms.deleted || !forge.deleted {
		t.Fatal("interrupted resources were not deleted")
	}
}

func TestReconcilePoolAdoptsStartedRunner(t *testing.T) {
	forge := &fakeForge{runnerStatus: forgejo.RunnerIdle}
	vms := &fakeVM{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:       "instance-id",
		Name:     "farm-ubuntu-instance",
		Pool:     "ubuntu",
		State:    instance.StateBootstrapping,
		RunnerID: 42,
	}}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if vms.deleted {
		t.Fatal("running VM was deleted")
	}
	if repository.instance.State != instance.StateReady {
		t.Fatalf("state = %q", repository.instance.State)
	}
}

func TestReconcilePoolRetriesDeletion(t *testing.T) {
	deleteErr := errors.New("Incus unavailable")
	forge := &fakeForge{}
	vms := &fakeVM{deleteErr: deleteErr}
	repository := &fakeRepository{instance: instance.Instance{
		ID:    "instance-id",
		Name:  "farm-ubuntu-instance",
		Pool:  "ubuntu",
		State: instance.StateCleaning,
	}}
	controller := newTestController(forge, vms, repository)

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

func TestReconcilePoolScalesDownIdleVM(t *testing.T) {
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	forge := &fakeForge{runnerStatus: forgejo.RunnerIdle}
	vms := &fakeVM{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:             "instance-id",
		Name:           "farm-ubuntu-instance",
		Pool:           "ubuntu",
		State:          instance.StateReady,
		RunnerID:       42,
		StateChangedAt: now.Add(-2 * time.Minute),
	}}
	controller := New(forge, vms, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
		Now:      func() time.Time { return now },
	})
	pool := testPool()
	pool.Scaling.IdleTimeout.Duration = time.Minute

	if err := controller.ReconcilePool(context.Background(), pool); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !vms.deleted {
		t.Fatal("idle VM was not deleted")
	}
}

func TestReconcilePoolHonorsRetryAt(t *testing.T) {
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	forge := &fakeForge{}
	vms := &fakeVM{}
	repository := &fakeRepository{instance: instance.Instance{
		ID:      "instance-id",
		Name:    "farm-ubuntu-instance",
		Pool:    "ubuntu",
		State:   instance.StateCleaning,
		RetryAt: now.Add(time.Minute),
	}}
	controller := New(forge, vms, repository, Options{
		ID:       "primary",
		ForgeURL: "https://git.example.com",
		Now:      func() time.Time { return now },
	})

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if vms.deleted {
		t.Fatal("VM deletion ignored retry time")
	}
}

func TestEphemeralVMLifecycle(t *testing.T) {
	forge := &fakeForge{
		jobs:         []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
		runnerStatus: forgejo.RunnerIdle,
	}
	vms := &fakeVM{}
	repository := &fakeRepository{}
	controller := New(forge, vms, repository, Options{
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
	if repository.instance.State != instance.StateFinished || !vms.deleted {
		t.Fatalf("completed instance = %#v", repository.instance)
	}
}

func TestFailureCleanupUsesLiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	createErr := errors.New("create failed")
	forge := &fakeForge{
		jobs: []forgejo.Job{{Status: forgejo.JobWaiting, RunsOn: []string{"farm-ubuntu"}}},
	}
	vms := &fakeVM{createErr: createErr, cancelOnCreate: cancel}
	repository := &fakeRepository{}
	controller := newTestController(forge, vms, repository)

	err := controller.ReconcilePool(ctx, testPool())
	if !errors.Is(err, createErr) {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if forge.deleteCtxErr != nil {
		t.Fatalf("cleanup context error = %v", forge.deleteCtxErr)
	}
}

func TestReconcilePoolDeletesOrphanedVM(t *testing.T) {
	forge := &fakeForge{}
	vms := &fakeVM{managed: []incusvm.ManagedInstance{{
		ID:   "orphan-id",
		Name: "farm-orphan",
		Pool: "ubuntu",
		Type: "virtual-machine",
	}}}
	repository := &fakeRepository{}
	controller := newTestController(forge, vms, repository)

	if err := controller.ReconcilePool(context.Background(), testPool()); err != nil {
		t.Fatalf("ReconcilePool() error = %v", err)
	}
	if !vms.deleted {
		t.Fatal("orphaned VM was not deleted")
	}
}

func TestReconcilePoolRejectsManagedContainer(t *testing.T) {
	forge := &fakeForge{}
	vms := &fakeVM{managed: []incusvm.ManagedInstance{{
		ID:   "container-id",
		Name: "farm-container",
		Pool: "ubuntu",
		Type: "container",
	}}}
	repository := &fakeRepository{}
	controller := newTestController(forge, vms, repository)

	err := controller.ReconcilePool(context.Background(), testPool())
	if err == nil {
		t.Fatal("ReconcilePool() error = nil")
	}
	if vms.deleted {
		t.Fatal("managed container was deleted")
	}
}

func TestReconcilePoolDeletesOrphanedRunner(t *testing.T) {
	forge := &fakeForge{runners: []forgejo.Runner{{
		ID:          99,
		Description: runnerDescription("primary", "ubuntu"),
	}}}
	vms := &fakeVM{}
	repository := &fakeRepository{}
	controller := newTestController(forge, vms, repository)

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

func newTestController(forge *fakeForge, vms *fakeVM, repository *fakeRepository) *Controller {
	return New(forge, vms, repository, Options{
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
			Profiles: []string{"farm-vm"},
		},
		Scaling: config.Scaling{
			MaxInstances:    2,
			MaxProvisioning: 1,
			StartupTimeout:  config.Duration{Duration: time.Minute},
			IdleTimeout:     config.Duration{Duration: time.Minute},
			MaxLifetime:     config.Duration{Duration: time.Hour},
		},
	}
}

type fakeForge struct {
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
	f.deleted = true
	f.deleteCtxErr = ctx.Err()
	return nil
}

type fakeVM struct {
	created        incusvm.InstanceSpec
	managed        []incusvm.ManagedInstance
	runnerConfig   []byte
	deleted        bool
	deleteErr      error
	createErr      error
	cancelOnCreate func()
	deleteCtxErr   error
}

func (v *fakeVM) Managed(context.Context, string, string) ([]incusvm.ManagedInstance, error) {
	return v.managed, nil
}

func (v *fakeVM) Create(_ context.Context, spec incusvm.InstanceSpec) error {
	if v.cancelOnCreate != nil {
		v.cancelOnCreate()
	}
	if v.createErr != nil {
		return v.createErr
	}
	v.created = spec
	v.managed = append(v.managed, incusvm.ManagedInstance{
		ID: spec.ID, Name: spec.Name, Pool: spec.Pool, Type: "virtual-machine",
	})
	return nil
}

func (v *fakeVM) WaitCloudInit(context.Context, string) error {
	return nil
}

func (v *fakeVM) PushRunnerConfig(_ context.Context, _ string, data []byte) error {
	v.runnerConfig = append([]byte(nil), data...)
	return nil
}

func (v *fakeVM) Delete(ctx context.Context, _ string) error {
	v.deleted = true
	v.deleteCtxErr = ctx.Err()
	v.managed = nil
	return v.deleteErr
}

type fakeRepository struct {
	instance    instance.Instance
	observation store.PoolObservation
	result      store.PoolResult
	progress    []store.PoolProgress
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

func (r *fakeRepository) FinishPool(_ context.Context, result store.PoolResult) error {
	r.result = result
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
