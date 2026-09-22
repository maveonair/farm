package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/maveonair/farm/internal/bootstrap"
	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/controller"
	"github.com/maveonair/farm/internal/forgejo"
	"github.com/maveonair/farm/internal/incus"
	"github.com/maveonair/farm/internal/reconcile"
	"github.com/maveonair/farm/internal/server"
	"github.com/maveonair/farm/internal/store"
)

func Validate(path string) error {
	_, err := config.Load(path)
	return err
}

func Run(ctx context.Context, path, version string) (runErr error) {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	logger, err := newLogger(cfg.Logging, os.Stderr)
	if err != nil {
		return err
	}
	logger = logger.With("service", "farm")
	appLogger := logger.With("controller", cfg.Controller.ID)

	token, err := readToken(cfg.Forgejo.TokenFile)
	if err != nil {
		return err
	}
	httpClient, err := forgeHTTPClient(cfg.Forgejo)
	if err != nil {
		return err
	}
	forge, err := forgejo.New(cfg.Forgejo.URL, token, httpClient)
	if err != nil {
		return err
	}
	instanceBackend, err := incus.Connect(incus.ConnectOptions{
		Endpoint:       cfg.Incus.Endpoint,
		Project:        cfg.Incus.Project,
		ClientCertFile: cfg.Incus.ClientCertFile,
		ClientKeyFile:  cfg.Incus.ClientKeyFile,
		ServerCertFile: cfg.Incus.ServerCertFile,
	})
	if err != nil {
		return err
	}
	repository, err := store.Open(ctx, cfg.Controller.Database)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, repository.Close())
	}()
	if err := repository.InterruptPools(ctx, time.Now().UTC()); err != nil {
		return err
	}

	monitor := &server.Monitor{}
	poolNames := make([]string, 0, len(cfg.Pools))
	for _, pool := range cfg.Pools {
		poolNames = append(poolNames, pool.Name)
	}
	monitor.Configure(poolNames, cfg.Controller.ReconcileInterval.Duration)
	manager := controller.New(forge, instanceBackend, repository, controller.Options{
		ID:       cfg.Controller.ID,
		ForgeURL: cfg.Forgejo.URL,
		Install: bootstrap.Install{
			DownloadURL: cfg.Runner.DownloadURL,
			SHA256:      cfg.Runner.SHA256,
		},
		CleanupTimeout: cfg.Controller.CleanupTimeout.Duration,
		RequestTimeout: cfg.Forgejo.Timeout.Duration,
		Progress: func(progress store.PoolProgress) {
			monitor.Progress(reconcile.Progress{
				Pool: progress.Pool, Stage: string(progress.Stage),
				StartedAt: progress.StartedAt, DeadlineAt: progress.DeadlineAt,
			})
		},
		Logger: logger,
	})
	operations := server.New(
		cfg.Controller.Listen,
		monitor,
		server.Options{
			Controller:        cfg.Controller.ID,
			Version:           version,
			ReconcileInterval: cfg.Controller.ReconcileInterval.Duration,
			Pools:             cfg.Pools,
			Store:             repository,
			Logger:            appLogger,
		},
	)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- operations.Run()
	}()
	appLogger.InfoContext(ctx, "service started",
		"event", "service_started",
		"incus_project", cfg.Incus.Project,
		"pools", len(cfg.Pools),
	)

	workerCtx, stopWorkers := context.WithCancel(ctx)
	var workers sync.WaitGroup
	complete := func(result poolResult) {
		finished := time.Now()
		if result.err != nil {
			failure := controller.DescribeError(result.err)
			appLogger.ErrorContext(ctx, "pool reconciliation failed",
				"event", "reconcile_failed",
				"pool", result.pool,
				"stage", failure.Stage,
				"code", failure.Code,
				"instance_id", failure.InstanceID,
				"instance_name", failure.InstanceName,
				"duration", result.duration,
				"error", result.err,
			)
			monitor.PoolFailure(result.pool, string(failure.Stage), string(failure.Code), finished)
			return
		}

		monitor.PoolSuccess(result.pool, finished)
		appLogger.DebugContext(ctx, "pool reconciled",
			"event", "reconcile_completed",
			"pool", result.pool,
			"duration", result.duration,
		)
	}
	for _, pool := range cfg.Pools {
		workers.Add(1)
		go func() {
			defer workers.Done()
			runPool(workerCtx, pool, cfg.Controller.ReconcileInterval.Duration, func(ctx context.Context, pool config.Pool) error {
				started := time.Now()
				deadline := started.Add(poolOperationTimeout(cfg, pool))
				monitor.PoolStart(pool.Name, started, deadline)
				reconcileCtx, cancel := context.WithDeadline(ctx, deadline)
				defer cancel()
				return manager.ReconcilePool(reconcileCtx, pool)
			}, complete)
		}()
	}

	select {
	case <-ctx.Done():
		appLogger.Info("service stopping", "event", "service_stopping")
		stopWorkers()
		workers.Wait()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Controller.CleanupTimeout.Duration)
		defer cancel()
		shutdownErr := operations.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			shutdownErr = errors.Join(shutdownErr, operations.Close())
		}
		shutdownErr = errors.Join(shutdownErr, <-serverErr)
		if shutdownErr == nil {
			appLogger.Info("service stopped", "event", "service_stopped")
		}
		return shutdownErr
	case err := <-serverErr:
		stopWorkers()
		workers.Wait()
		return err
	}
}

type poolResult struct {
	pool     string
	duration time.Duration
	err      error
}

func runPool(ctx context.Context, pool config.Pool, interval time.Duration, reconcile func(context.Context, config.Pool) error, complete func(poolResult)) {
	for {
		if ctx.Err() != nil {
			return
		}
		started := time.Now()
		err := reconcile(ctx, pool)
		complete(poolResult{pool: pool.Name, duration: time.Since(started), err: err})

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func poolOperationTimeout(cfg config.Config, pool config.Pool) time.Duration {
	return pool.Scaling.StartupTimeout.Duration +
		time.Duration(pool.Scaling.MaxInstances)*cfg.Controller.CleanupTimeout.Duration +
		time.Duration(pool.Scaling.MaxInstances+3)*cfg.Forgejo.Timeout.Duration
}

func forgeHTTPClient(cfg config.Forgejo) (*http.Client, error) {
	client := &http.Client{Timeout: cfg.Timeout.Duration}
	if cfg.CACertificateFile == "" {
		return client, nil
	}

	certificate, err := os.ReadFile(cfg.CACertificateFile)
	if err != nil {
		return nil, fmt.Errorf("read Forgejo CA certificate: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system CA certificates: %w", err)
	}
	if !roots.AppendCertsFromPEM(certificate) {
		return nil, errors.New("forgejo CA certificate contains no certificates")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
	}
	client.Transport = transport
	return client, nil
}

func readToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read Forgejo token: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("forgejo token file %q is empty", path)
	}
	return token, nil
}
