package controller

import (
	"context"
	"errors"

	"github.com/maveonair/farm/internal/forgejo"
	farmInstance "github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
)

func (c *Controller) cleanup(ctx context.Context, instance farmInstance.Instance, scope forgejo.Scope, result farmInstance.Result, reason farmInstance.Reason, message string) error {
	if err := c.store.BeginCleanup(ctx, instance.ID, result, reason, message); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, instance, err)
	}
	if instance.State != farmInstance.StateCleaning {
		c.logger.InfoContext(
			ctx, "instance deletion started",
			"event", "instance_deletion_started",
			"pool", instance.Pool,
			"instance_id", instance.ID,
			"instance_name", instance.Name,
			"runner_id", instance.RunnerID,
		)
	}
	deleteCtx, cancel := c.cleanupContext(ctx)
	defer cancel()

	if instance.RunnerID != 0 {
		err := c.forge.DeleteRunner(deleteCtx, scope, instance.RunnerID)
		if err != nil && !forgejo.IsNotFound(err) {
			return c.retryDelete(ctx, instance, operationError(reconcile.StageDeleteRunner, classify(err), instance, err))
		}
	}
	if err := c.setInstanceStage(ctx, instance, reconcile.StageDeleteInstance); err != nil {
		return err
	}
	if err := c.instances.Delete(deleteCtx, instance.Name); err != nil {
		return c.retryDelete(ctx, instance, operationError(reconcile.StageDeleteInstance, classify(err), instance, err))
	}
	if err := c.store.FinishCleanup(ctx, instance.ID); err != nil {
		return operationError(reconcile.StagePersistState, reconcile.FailureDatabase, instance, err)
	}
	c.logger.InfoContext(
		ctx, "instance deleted",
		"event", "instance_deleted",
		"pool", instance.Pool,
		"instance_id", instance.ID,
		"instance_name", instance.Name,
		"runner_id", instance.RunnerID,
	)
	return nil
}

func (c *Controller) cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.cleanupTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.cleanupTimeout)
}

func (c *Controller) retryDelete(ctx context.Context, instance farmInstance.Instance, cause error) error {
	delay := retryBase
	for range instance.RetryCount {
		delay *= 2
		if delay >= retryMax {
			delay = retryMax
			break
		}
	}
	if err := c.store.Retry(ctx, instance.ID, cause.Error(), c.now().Add(delay)); err != nil {
		return errors.Join(cause, err)
	}
	c.logger.WarnContext(
		ctx, "instance deletion retry scheduled",
		"event", "instance_deletion_retry_scheduled",
		"pool", instance.Pool,
		"instance_id", instance.ID,
		"instance_name", instance.Name,
		"retry_count", instance.RetryCount+1,
		"retry_in", delay,
		"error", cause,
	)
	return cause
}
