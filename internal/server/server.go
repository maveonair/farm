package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/maveonair/farm/internal/reconcile"
)

type Monitor = reconcile.Tracker

type Server struct {
	http    *http.Server
	monitor *Monitor
}

func New(address string, monitor *Monitor, options ...Options) *Server {
	server := &Server{monitor: monitor}
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", server.live)
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /metrics", server.metrics)
	server.routes(mux, opts)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "endpoint not found")
	})
	mux.Handle("GET /", uiHandler())
	server.http = &http.Server{
		Addr:              address,
		Handler:           secureHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server
}

func (s *Server) Run() error {
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve operations API: %w", err)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown operations API: %w", err)
	}
	return nil
}

func (s *Server) Close() error {
	if err := s.http.Close(); err != nil {
		return fmt.Errorf("close operations API: %w", err)
	}
	return nil
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	condition := s.monitor.Snapshot(time.Now()).Condition
	if condition == reconcile.ConditionDegraded || condition == reconcile.ConditionStalled {
		http.Error(w, "reconciliation unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	if s.monitor.Snapshot(time.Now()).Condition == reconcile.ConditionStalled {
		http.Error(w, "controller stalled", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var metrics strings.Builder
	lastSuccess, errorsTotal, errorsBy, failedPools := s.monitor.MetricData()
	metrics.WriteString("# TYPE farm_reconcile_success_timestamp_seconds gauge\n")
	metrics.WriteString("farm_reconcile_success_timestamp_seconds ")
	metrics.WriteString(strconv.FormatInt(lastSuccess, 10))
	metrics.WriteString("\n# TYPE farm_reconcile_errors_total counter\n")
	metrics.WriteString("farm_reconcile_errors_total ")
	metrics.WriteString(strconv.FormatUint(errorsTotal, 10))
	metrics.WriteByte('\n')
	for key, count := range errorsBy {
		parts := strings.SplitN(key, "\x00", 2)
		metrics.WriteString("farm_reconcile_errors_by_cause_total{stage=\"")
		metrics.WriteString(parts[0])
		metrics.WriteString("\",code=\"")
		metrics.WriteString(parts[1])
		metrics.WriteString("\"} ")
		metrics.WriteString(strconv.FormatUint(count, 10))
		metrics.WriteByte('\n')
	}
	for _, pool := range failedPools {
		metrics.WriteString("farm_pool_reconcile_failed{pool=\"")
		metrics.WriteString(pool)
		metrics.WriteString("\"} 1\n")
	}
	if _, err := w.Write([]byte(metrics.String())); err != nil {
		return
	}
}
