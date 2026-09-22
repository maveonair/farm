package controller

import (
	"context"
	"fmt"

	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/forgejo"
	farmInstance "github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
)

func (c *Controller) collectOrphans(ctx context.Context, pool config.Pool, scope forgejo.Scope, instances []farmInstance.Instance) error {
	instanceNames := make(map[string]string, len(instances))
	runnerIDs := make(map[int64]struct{}, len(instances))
	for _, instance := range instances {
		instanceNames[instance.ID] = instance.Name
		if instance.RunnerID != 0 {
			runnerIDs[instance.RunnerID] = struct{}{}
		}
	}

	managed, err := c.instances.Managed(ctx, c.id, pool.Name)
	if err != nil {
		return err
	}
	managedIDs := make(map[string]struct{}, len(managed))
	for _, instance := range managed {
		if name, found := instanceNames[instance.ID]; found {
			if name != instance.Name {
				return fmt.Errorf("managed instance %q reuses instance ID for %q", instance.Name, name)
			}
			managedIDs[instance.ID] = struct{}{}
			continue
		}
		deleteCtx, cancel := c.cleanupContext(ctx)
		err := c.instances.Delete(deleteCtx, instance.Name)
		cancel()
		if err != nil {
			return fmt.Errorf("delete orphaned instance %q: %w", instance.Name, err)
		}
		c.logger.InfoContext(
			ctx, "orphaned Instance deleted",
			"event", "orphan_instance_deleted",
			"pool", pool.Name,
			"instance_id", instance.ID,
			"instance_name", instance.Name,
		)
	}
	for _, instance := range instances {
		if instance.State != farmInstance.StateReady && instance.State != farmInstance.StateRunning {
			continue
		}
		if _, found := managedIDs[instance.ID]; found {
			continue
		}
		if err := c.cleanup(ctx, instance, scope, farmInstance.ResultFailed, farmInstance.ReasonInstanceMissing, "managed instance is missing"); err != nil {
			return fmt.Errorf("delete runner for missing instance %q: %w", instance.Name, err)
		}
	}

	runners, err := c.forge.Runners(ctx, scope)
	if err != nil {
		return err
	}
	description := runnerDescription(c.id, pool.Name)
	for _, runner := range runners {
		if runner.Description != description {
			continue
		}
		if _, found := runnerIDs[runner.ID]; found {
			continue
		}
		if err := c.forge.DeleteRunner(ctx, scope, runner.ID); err != nil && !forgejo.IsNotFound(err) {
			return fmt.Errorf("delete orphaned runner %d: %w", runner.ID, err)
		}
		c.logger.InfoContext(
			ctx, "orphaned runner deleted",
			"event", "orphan_runner_deleted",
			"pool", pool.Name,
			"runner_id", runner.ID,
		)
	}
	return nil
}

func (c *Controller) observe(ctx context.Context, instance farmInstance.Instance, scope forgejo.Scope) error {
	runner, err := c.forge.Runner(ctx, scope, instance.RunnerID)
	if forgejo.IsNotFound(err) {
		return c.cleanup(ctx, instance, scope, farmInstance.ResultSucceeded, farmInstance.ReasonJobCompleted, "")
	}
	if err != nil {
		return operationError(reconcile.StageObserveRunner, classify(err), instance, err)
	}

	switch runner.Status {
	case forgejo.RunnerIdle:
		if instance.State == farmInstance.StateRunning {
			return c.cleanup(ctx, instance, scope, farmInstance.ResultSucceeded, farmInstance.ReasonJobCompleted, "")
		}
		return c.observeState(ctx, instance, farmInstance.StateReady)
	case forgejo.RunnerActive:
		return c.observeState(ctx, instance, farmInstance.StateRunning)
	case forgejo.RunnerOffline:
		return c.cleanup(ctx, instance, scope, farmInstance.ResultFailed, farmInstance.ReasonRunnerOffline, "runner is offline")
	default:
		return fmt.Errorf("runner %d returned unknown status %q", instance.RunnerID, runner.Status)
	}
}

func (c *Controller) observeState(ctx context.Context, instance farmInstance.Instance, state farmInstance.State) error {
	var err error
	if state == farmInstance.StateReady {
		err = c.store.MarkReady(ctx, instance.ID)
	} else {
		err = c.store.MarkRunning(ctx, instance.ID)
	}
	if err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, instance, err)
	}
	if instance.State == state {
		return nil
	}
	c.logger.InfoContext(
		ctx, "instance state changed",
		"event", "instance_state_changed",
		"pool", instance.Pool,
		"instance_id", instance.ID,
		"instance_name", instance.Name,
		"runner_id", instance.RunnerID,
		"from", instance.State,
		"to", state,
	)
	return nil
}
