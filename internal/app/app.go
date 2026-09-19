package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/maveonair/farm/internal/bootstrap"
	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/controller"
	"github.com/maveonair/farm/internal/forgejo"
	incusvm "github.com/maveonair/farm/internal/incus"
	"github.com/maveonair/farm/internal/reconcile"
	"github.com/maveonair/farm/internal/server"
	"github.com/maveonair/farm/internal/store"
)

func Validate(path string) error {
	_, err := config.Load(path)
	return err
}

func Run(ctx context.Context, path string) (runErr error) {
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
	slog.SetDefault(appLogger)

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
	vms, err := incusvm.Connect(incusvm.ConnectOptions{
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
	manager := controller.New(forge, vms, repository, controller.Options{
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
			ReconcileInterval: cfg.Controller.ReconcileInterval.Duration,
			Pools:             cfg.Pools,
			Store:             repository,
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

	reconcile := func() {
		started := time.Now()
		monitor.Start(started, started.Add(maxOperationTimeout(cfg)), len(cfg.Pools))
		succeeded := true
		type result struct {
			pool     string
			duration time.Duration
			err      error
		}
		results := make(chan result, len(cfg.Pools))
		for _, pool := range cfg.Pools {
			go func() {
				poolStarted := time.Now()
				err := manager.ReconcilePool(ctx, pool)
				results <- result{
					pool:     pool.Name,
					duration: time.Since(poolStarted),
					err:      err,
				}
			}()
		}
		for range cfg.Pools {
			result := <-results
			if result.err != nil {
				failure := controller.DescribeError(result.err)
				appLogger.ErrorContext(ctx, "pool reconciliation failed",
					"event", "reconcile_failed",
					"pool", result.pool,
					"stage", failure.Stage,
					"code", failure.Code,
					"instance_id", failure.InstanceID,
					"duration", result.duration,
					"error", result.err,
				)
				monitor.PoolFailure(result.pool, string(failure.Stage), string(failure.Code))
				succeeded = false
				continue
			}
			monitor.PoolSuccess(result.pool)
			appLogger.DebugContext(ctx, "pool reconciled",
				"event", "reconcile_completed",
				"pool", result.pool,
				"duration", result.duration,
			)
		}
		monitor.Finish(time.Now(), cfg.Controller.ReconcileInterval.Duration, succeeded)
		appLogger.DebugContext(ctx, "reconciliation cycle completed",
			"event", "reconcile_cycle_completed",
			"duration", time.Since(started),
			"success", succeeded,
		)
	}
	reconcile()

	timer := time.NewTimer(cfg.Controller.ReconcileInterval.Duration)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			appLogger.Info("service stopping", "event", "service_stopping")
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
			return err
		case <-timer.C:
			reconcile()
			timer.Reset(cfg.Controller.ReconcileInterval.Duration)
		}
	}
}

func maxOperationTimeout(cfg config.Config) time.Duration {
	maximum := cfg.Forgejo.Timeout.Duration
	for _, pool := range cfg.Pools {
		budget := pool.Scaling.StartupTimeout.Duration +
			time.Duration(pool.Scaling.MaxInstances)*cfg.Controller.CleanupTimeout.Duration +
			time.Duration(pool.Scaling.MaxInstances+3)*cfg.Forgejo.Timeout.Duration
		if budget > maximum {
			maximum = budget
		}
	}
	return maximum
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
