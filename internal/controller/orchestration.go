package controller

import (
	"context"
	"errors"

	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/forgejo"
	farmInstance "github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
)

func (c *Controller) provisionMany(ctx context.Context, pool config.Pool, scope forgejo.Scope, runID string, count int) error {
	if count == 0 {
		return nil
	}

	provisionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, count)
	for range count {
		go func() {
			errs <- c.provision(provisionCtx, pool, scope, runID)
		}()
	}

	var result []error
	for range count {
		if err := <-errs; err != nil {
			result = append(result, err)
			cancel()
		}
	}
	return errors.Join(result...)
}

func (c *Controller) reconcileInstances(ctx context.Context, pool config.Pool, scope forgejo.Scope) error {
	instances, err := c.store.ListPool(ctx, pool.Name)
	if err != nil {
		return operationError(reconcile.StageLoadInstances, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}
	if err := c.collectOrphans(ctx, pool, scope, instances); err != nil {
		return operationError(reconcile.StageCollectOrphans, classify(err), farmInstance.Instance{}, err)
	}
	instances, err = c.store.ListPool(ctx, pool.Name)
	if err != nil {
		return operationError(reconcile.StageLoadInstances, reconcile.FailureDatabase, farmInstance.Instance{}, err)
	}

	for _, instance := range instances {
		if instance.State == farmInstance.StateRunning && c.now().Sub(instance.StateChangedAt) >= pool.Scaling.MaxLifetime.Duration {
			c.logger.InfoContext(ctx, "instance reached maximum lifetime",
				"event", "instance_retiring",
				"pool", pool.Name,
				"instance_id", instance.ID,
				"instance_name", instance.Name,
				"reason", "max_lifetime",
			)
			if err := c.cleanup(ctx, instance, scope, farmInstance.ResultSucceeded, farmInstance.ReasonMaxLifetime, ""); err != nil {
				return err
			}
			continue
		}
		switch instance.State {
		case farmInstance.StateCleaning:
			if !instance.RetryAt.IsZero() && c.now().Before(instance.RetryAt) {
				continue
			}
			if err := c.cleanup(ctx, instance, scope, instance.Result, instance.Reason, instance.Error); err != nil {
				return err
			}
		case farmInstance.StateBootstrapping:
			if instance.RunnerID == 0 {
				if err := c.cleanup(ctx, instance, scope, farmInstance.ResultFailed, farmInstance.ReasonControllerRecovery, "controller restarted during bootstrap"); err != nil {
					return err
				}
				continue
			}
			if err := c.observe(ctx, instance, scope); err != nil {
				return err
			}
		case farmInstance.StateReady, farmInstance.StateRunning:
			if err := c.observe(ctx, instance, scope); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Controller) scaleDown(ctx context.Context, pool config.Pool, scope forgejo.Scope, instances []farmInstance.Instance, waiting int) error {
	desired := pool.Scaling.MinIdle + waiting
	var ready []farmInstance.Instance
	for _, instance := range instances {
		if instance.State == farmInstance.StateReady {
			ready = append(ready, instance)
		}
	}
	excess := len(ready) - desired
	for _, instance := range ready {
		if excess <= 0 {
			break
		}
		if c.now().Sub(instance.StateChangedAt) < pool.Scaling.IdleTimeout.Duration {
			continue
		}
		c.logger.InfoContext(ctx, "idle instance selected for scale down",
			"event", "instance_retiring",
			"pool", pool.Name,
			"instance_id", instance.ID,
			"instance_name", instance.Name,
			"reason", "idle_timeout",
		)
		if err := c.cleanup(ctx, instance, scope, farmInstance.ResultSucceeded, farmInstance.ReasonIdleTimeout, ""); err != nil {
			return err
		}
		excess--
	}
	return nil
}
