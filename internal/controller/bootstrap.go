package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/maveonair/farm/internal/bootstrap"
	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/forgejo"
	incusvm "github.com/maveonair/farm/internal/incus"
	farmInstance "github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
)

func (c *Controller) provision(ctx context.Context, pool config.Pool, scope forgejo.Scope, runID string) error {
	workCtx, cancel := context.WithTimeout(ctx, pool.Scaling.StartupTimeout.Duration)
	defer cancel()
	deadline, _ := workCtx.Deadline()

	id, err := newID()
	if err != nil {
		return err
	}
	name := fmt.Sprintf("farm-%s-%s", pool.Name, id[:12])
	instance := farmInstance.Instance{ID: id, Name: name, Pool: pool.Name, State: farmInstance.StateBootstrapping}
	if err := c.store.Create(workCtx, instance); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, instance, err)
	}
	c.logger.InfoContext(
		workCtx, "instance provisioning started",
		"event", "instance_provisioning_started",
		"pool", pool.Name,
		"instance_id", id,
		"instance_name", name,
	)

	if err := c.setProgress(workCtx, pool.Name, runID, reconcile.StageRegisterRunner, deadline); err != nil {
		return err
	}
	if err := c.setInstanceStage(workCtx, instance, reconcile.StageRegisterRunner); err != nil {
		return err
	}
	registration, err := c.forge.Register(workCtx, scope, name, runnerDescription(c.id, pool.Name))
	if err != nil {
		return c.fail(ctx, instance, scope, forgejo.Registration{}, false,
			operationError(reconcile.StageRegisterRunner, classify(err), instance, err))
	}
	if err := c.store.SetRegistration(workCtx, id, registration.ID); err != nil {
		return c.fail(ctx, instance, scope, registration, false,
			operationError(reconcile.StagePersistState, reconcile.FailureDatabase, instance, err))
	}
	c.logger.DebugContext(
		workCtx, "runner registered",
		"event", "runner_registered",
		"pool", pool.Name,
		"instance_id", id,
		"instance_name", name,
		"runner_id", registration.ID,
	)

	var cloudInit []byte
	if pool.Instance.RunnerInstall != config.RunnerInstallImage {
		cloudInit, err = bootstrap.CloudInit(c.install)
		if err != nil {
			return c.fail(ctx, instance, scope, registration, false,
				operationError(reconcile.StagePushRunnerConfig, reconcile.FailureUnknown, instance, err))
		}
	}
	instanceCreated := false
	if err := c.setProgress(workCtx, pool.Name, runID, reconcile.StateCreateInstance, deadline); err != nil {
		return c.fail(ctx, instance, scope, registration, false, err)
	}
	if err := c.setInstanceStage(workCtx, instance, reconcile.StateCreateInstance); err != nil {
		return c.fail(ctx, instance, scope, registration, false, err)
	}
	err = c.instances.Create(workCtx, incusvm.InstanceSpec{
		ID:         id,
		Name:       name,
		Pool:       pool.Name,
		Controller: c.id,
		Image: incusvm.Image{
			Alias:           pool.Instance.Image.Alias,
			Fingerprint:     pool.Instance.Image.Fingerprint,
			Server:          pool.Instance.Image.Server,
			Protocol:        pool.Instance.Image.Protocol,
			CertificateFile: pool.Instance.Image.CertificateFile,
		},
		Profiles:  pool.Instance.Profiles,
		Config:    pool.Instance.Config,
		CloudInit: cloudInit,
	})
	if err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated,
			operationError(reconcile.StateCreateInstance, classify(err), instance, err))
	}
	instanceCreated = true
	c.logger.InfoContext(
		workCtx, "Instance created",
		"event", "instance_created",
		"pool", pool.Name,
		"instance_id", id,
		"instance_name", name,
	)
	waitStage := reconcile.StageWaitCloudInit
	if pool.Instance.RunnerInstall == config.RunnerInstallImage {
		waitStage = reconcile.StageWaitAgent
	}
	if err := c.setProgress(workCtx, pool.Name, runID, waitStage, deadline); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated, err)
	}
	if err := c.setInstanceStage(workCtx, instance, waitStage); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated, err)
	}
	if err := c.waitInstance(workCtx, pool.Instance.RunnerInstall, name); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated,
			operationError(waitStage, classify(err), instance, err))
	}

	runnerConfig, err := bootstrap.RunnerConfig(bootstrap.Connection{
		URL:    c.url,
		UUID:   registration.UUID,
		Token:  registration.Token,
		Labels: runnerLabels(pool.Labels),
	})
	if err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated,
			operationError(reconcile.StagePushRunnerConfig, reconcile.FailureUnknown, instance, err))
	}
	if err := c.setProgress(workCtx, pool.Name, runID, reconcile.StagePushRunnerConfig, deadline); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated, err)
	}
	if err := c.setInstanceStage(workCtx, instance, reconcile.StagePushRunnerConfig); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated, err)
	}
	if err := c.instances.PushRunnerConfig(workCtx, name, runnerConfig); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated,
			operationError(reconcile.StagePushRunnerConfig, classify(err), instance, err))
	}

	if err := c.setProgress(workCtx, pool.Name, runID, reconcile.StageWaitRunner, deadline); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated, err)
	}
	if err := c.setInstanceStage(workCtx, instance, reconcile.StageWaitRunner); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated, err)
	}
	state, err := c.waitRunner(workCtx, scope, registration.ID)
	if forgejo.IsNotFound(err) {
		instance.RunnerID = registration.ID
		return c.cleanup(ctx, instance, scope, farmInstance.ResultSucceeded, farmInstance.ReasonJobCompleted, "")
	}
	if err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated,
			operationError(reconcile.StageWaitRunner, classify(err), instance, err))
	}
	if err := c.markState(workCtx, id, state); err != nil {
		return c.fail(ctx, instance, scope, registration, instanceCreated,
			operationError(reconcile.StagePersistState, reconcile.FailureDatabase, instance, err))
	}
	c.logger.InfoContext(
		workCtx, "instance ready",
		"event", "instance_ready",
		"pool", pool.Name,
		"instance_id", id,
		"instance_name", name,
		"runner_id", registration.ID,
		"state", state,
	)
	return nil
}

func (c *Controller) waitInstance(ctx context.Context, install config.RunnerInstallMode, name string) error {
	if install == config.RunnerInstallImage {
		return c.instances.WaitAgent(ctx, name)
	}

	return c.instances.WaitCloudInit(ctx, name)
}

func runnerDescription(controller, pool string) string {
	return fmt.Sprintf("Managed by FARM controller %s pool %s", controller, pool)
}

func (c *Controller) waitRunner(ctx context.Context, scope forgejo.Scope, id int64) (farmInstance.State, error) {
	for {
		runner, err := c.forge.Runner(ctx, scope, id)
		if err != nil {
			return "", err
		}
		switch runner.Status {
		case forgejo.RunnerIdle:
			return farmInstance.StateReady, nil
		case forgejo.RunnerActive:
			return farmInstance.StateRunning, nil
		}

		timer := time.NewTimer(runnerPollPeriod)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Controller) fail(ctx context.Context, instance farmInstance.Instance, scope forgejo.Scope, registration forgejo.Registration, instanceCreated bool, cause error) error {
	cleanupCtx, cancel := c.cleanupContext(context.WithoutCancel(ctx))
	defer cancel()

	var cleanup []error
	if err := c.store.BeginCleanup(cleanupCtx, instance.ID, farmInstance.ResultFailed, farmInstance.ReasonBootstrapFailed, cause.Error()); err != nil {
		cleanup = append(cleanup, err)
	}
	if registration.ID != 0 {
		if err := c.forge.DeleteRunner(cleanupCtx, scope, registration.ID); err != nil && !forgejo.IsNotFound(err) {
			cleanup = append(cleanup, err)
		}
	}
	if instanceCreated {
		if err := c.instances.Delete(cleanupCtx, instance.Name); err != nil {
			cleanup = append(cleanup, err)
		}
	}
	if len(cleanup) == 0 {
		if err := c.store.FinishCleanup(cleanupCtx, instance.ID); err != nil {
			cleanup = append(cleanup, err)
		}
	} else if err := c.store.Retry(cleanupCtx, instance.ID, errors.Join(cleanup...).Error(), c.now().Add(retryBase)); err != nil {
		cleanup = append(cleanup, err)
	}

	return fmt.Errorf("provision instance %q: %w", instance.Name, errors.Join(append([]error{cause}, cleanup...)...))
}
