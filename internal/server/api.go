package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/reconcile"
	"github.com/maveonair/farm/internal/scheduler"
	"github.com/maveonair/farm/internal/store"
)

type reader interface {
	Get(context.Context, string) (instance.Instance, error)
	ListInstances(context.Context, store.InstanceFilter) ([]instance.Instance, error)
	ListEvents(context.Context, store.EventFilter) ([]store.Event, error)
	ListPoolData(context.Context) ([]store.PoolData, error)
	ListIncidents(context.Context, store.IncidentFilter) ([]store.PoolIncident, error)
	GetIncident(context.Context, int64) (store.PoolIncident, error)
}

type Options struct {
	Controller        string
	Version           string
	ReconcileInterval time.Duration
	Pools             []config.Pool
	Store             reader
}

type instanceResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Pool           string          `json:"pool"`
	State          instance.State  `json:"state"`
	Stage          reconcile.Stage `json:"stage,omitempty"`
	Result         instance.Result `json:"result,omitempty"`
	Reason         instance.Reason `json:"reason,omitempty"`
	RunnerID       int64           `json:"runner_id"`
	Error          string          `json:"error"`
	RetryCount     int             `json:"retry_count"`
	RetryAt        string          `json:"retry_at,omitempty"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
	StateChangedAt string          `json:"state_changed_at"`
	StageChangedAt string          `json:"stage_changed_at,omitempty"`
	FinishedAt     string          `json:"finished_at,omitempty"`
}

type eventResponse struct {
	ID           int64           `json:"id"`
	InstanceID   string          `json:"instance_id"`
	InstanceName string          `json:"instance_name"`
	Pool         string          `json:"pool"`
	Kind         store.EventKind `json:"kind"`
	FromState    instance.State  `json:"from_state,omitempty"`
	ToState      instance.State  `json:"to_state,omitempty"`
	Stage        reconcile.Stage `json:"stage,omitempty"`
	Result       instance.Result `json:"result,omitempty"`
	Reason       instance.Reason `json:"reason,omitempty"`
	Message      string          `json:"message,omitempty"`
	CreatedAt    string          `json:"created_at"`
}

type poolResponse struct {
	Name             string              `json:"name"`
	Scope            scopeResponse       `json:"scope"`
	Labels           []string            `json:"labels"`
	Image            string              `json:"image"`
	MinIdle          int                 `json:"min_idle"`
	MaxInstances     int                 `json:"max_instances"`
	MaxProvisioning  int                 `json:"max_provisioning"`
	StartupTimeout   string              `json:"startup_timeout"`
	IdleTimeout      string              `json:"idle_timeout"`
	MaxLifetime      string              `json:"max_lifetime"`
	Capacity         capacityResponse    `json:"capacity"`
	Bootstrap        bootstrapResponse   `json:"bootstrap"`
	Runtime          poolRuntimeResponse `json:"runtime"`
	Incident         *incidentResponse   `json:"incident,omitempty"`
	ObservationStale bool                `json:"observation_stale"`
}

type bootstrapResponse struct {
	Attempts     int    `json:"attempts"`
	AttemptLimit int    `json:"attempt_limit"`
	Paused       bool   `json:"paused"`
	RetryAt      string `json:"retry_at,omitempty"`
}

type scopeResponse struct {
	Type       config.ScopeType `json:"type"`
	Owner      string           `json:"owner,omitempty"`
	Repository string           `json:"repository,omitempty"`
}

type capacityResponse struct {
	Waiting       int `json:"waiting"`
	Needed        int `json:"needed"`
	Bootstrapping int `json:"bootstrapping"`
	Ready         int `json:"ready"`
	Running       int `json:"running"`
	Cleaning      int `json:"cleaning"`
	Maximum       int `json:"maximum"`
}

type poolRuntimeResponse struct {
	State           store.RuntimeState `json:"state,omitempty"`
	RunID           string             `json:"run_id,omitempty"`
	ObservedAt      string             `json:"observed_at,omitempty"`
	StartedAt       string             `json:"started_at,omitempty"`
	FinishedAt      string             `json:"finished_at,omitempty"`
	LastSuccessAt   string             `json:"last_success_at,omitempty"`
	Stage           reconcile.Stage    `json:"stage,omitempty"`
	StageStartedAt  string             `json:"stage_started_at,omitempty"`
	StageDeadlineAt string             `json:"stage_deadline_at,omitempty"`
}

type incidentResponse struct {
	ID           int64                 `json:"id"`
	Pool         string                `json:"pool"`
	RunID        string                `json:"run_id"`
	Stage        reconcile.Stage       `json:"stage"`
	Code         reconcile.FailureCode `json:"code"`
	Message      string                `json:"message"`
	InstanceID   string                `json:"instance_id,omitempty"`
	InstanceName string                `json:"instance_name,omitempty"`
	FirstSeenAt  string                `json:"first_seen_at"`
	LastSeenAt   string                `json:"last_seen_at"`
	Occurrences  int                   `json:"occurrences"`
	ResolvedAt   string                `json:"resolved_at,omitempty"`
}

type reconciliationResponse struct {
	Phase          reconcile.Phase     `json:"phase"`
	Condition      reconcile.Condition `json:"condition"`
	LastOutcome    reconcile.Outcome   `json:"last_outcome"`
	StartedAt      string              `json:"started_at,omitempty"`
	FinishedAt     string              `json:"finished_at,omitempty"`
	LastSuccessAt  string              `json:"last_success_at,omitempty"`
	DeadlineAt     string              `json:"deadline_at,omitempty"`
	CompletedPools int                 `json:"completed_pools"`
	TotalPools     int                 `json:"total_pools"`
	ActiveFailures int                 `json:"active_failures"`
}

func (s *Server) routes(mux *http.ServeMux, options Options) {
	if options.Store == nil {
		return
	}

	mux.HandleFunc("GET /api/v1/summary", s.summary(options))
	mux.HandleFunc("GET /api/v1/pools", s.pools(options))
	mux.HandleFunc("GET /api/v1/pools/{name}", s.pool(options))
	mux.HandleFunc("GET /api/v1/instances", s.instances(options.Store))
	mux.HandleFunc("GET /api/v1/instances/{id}", s.instance(options.Store))
	mux.HandleFunc("GET /api/v1/instances/{id}/events", s.instanceEvents(options.Store))
	mux.HandleFunc("GET /api/v1/events", s.events(options.Store))
	mux.HandleFunc("GET /api/v1/incidents", s.incidents(options.Store))
	mux.HandleFunc("GET /api/v1/incidents/{id}", s.incident(options.Store))
}

func (s *Server) summary(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pools, err := options.Store.ListPoolData(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read summary")
			return
		}

		counts := make(map[instance.State]int)
		waiting := 0
		configured := configuredPools(options.Pools)
		for _, pool := range pools {
			if _, found := configured[pool.Pool]; !found {
				continue
			}
			waiting += pool.Runtime.Waiting
			counts[instance.StateBootstrapping] += pool.Counts.Bootstrapping
			counts[instance.StateReady] += pool.Counts.Ready
			counts[instance.StateRunning] += pool.Counts.Running
			counts[instance.StateCleaning] += pool.Counts.Cleaning
		}

		now := time.Now()
		snapshot := s.monitor.Snapshot(now)
		healthy := snapshot.Condition == reconcile.ConditionStarting || snapshot.Condition == reconcile.ConditionHealthy
		writeJSON(w, http.StatusOK, map[string]any{
			"controller":       options.Controller,
			"version":          options.Version,
			"healthy":          healthy,
			"last_success_at":  formatTime(snapshot.LastSuccessAt),
			"reconcile_errors": snapshot.Errors,
			"reconciliation": reconciliationResponse{
				Phase: snapshot.Phase, Condition: snapshot.Condition, LastOutcome: snapshot.LastOutcome,
				StartedAt: formatTime(snapshot.StartedAt), FinishedAt: formatTime(snapshot.FinishedAt),
				LastSuccessAt: formatTime(snapshot.LastSuccessAt), DeadlineAt: formatTime(snapshot.DeadlineAt),
				CompletedPools: snapshot.CompletedPools, TotalPools: snapshot.TotalPools,
				ActiveFailures: snapshot.ActiveFailures,
			},
			"waiting":      waiting,
			"counts":       counts,
			"generated_at": now.UTC().Format(time.RFC3339),
		})
	}
}

func (s *Server) pools(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := options.Store.ListPoolData(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read pools")
			return
		}
		byName := make(map[string]store.PoolData, len(data))
		for _, item := range data {
			byName[item.Pool] = item
		}

		pools := make([]poolResponse, 0, len(options.Pools))
		for _, pool := range options.Pools {
			pools = append(pools, newPoolResponse(pool, byName[pool.Name], staleAfter(options.ReconcileInterval)))
		}
		writeJSON(w, http.StatusOK, map[string]any{"pools": pools})
	}
}

func (s *Server) pool(options Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		for _, pool := range options.Pools {
			if pool.Name != name {
				continue
			}
			pools, err := options.Store.ListPoolData(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "read pool")
				return
			}
			var data store.PoolData
			for _, candidate := range pools {
				if candidate.Pool == name {
					data = candidate
					break
				}
			}
			writeJSON(w, http.StatusOK, newPoolResponse(pool, data, staleAfter(options.ReconcileInterval)))
			return
		}
		writeError(w, http.StatusNotFound, "pool not found")
	}
}

func (s *Server) instances(repository reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		state := instance.State(r.URL.Query().Get("state"))
		if state != "" && !validState(state) {
			writeError(w, http.StatusBadRequest, "invalid state")
			return
		}
		instances, err := repository.ListInstances(r.Context(), store.InstanceFilter{
			Pool: r.URL.Query().Get("pool"), State: state, Limit: page.limit(), Offset: page.offset,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read instances")
			return
		}
		instances, pagination := finishPage(instances, page)
		responses := make([]instanceResponse, 0, len(instances))
		for _, instance := range instances {
			responses = append(responses, newInstanceResponse(instance))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"instances":  responses,
			"pagination": pagination,
		})
	}
}

func (s *Server) instance(repository reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instance, err := repository.Get(r.Context(), r.PathValue("id"))
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read instance")
			return
		}
		writeJSON(w, http.StatusOK, newInstanceResponse(instance))
	}
}

func (s *Server) events(repository reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeEvents(w, r, repository, store.EventFilter{Pool: r.URL.Query().Get("pool")})
	}
}

func (s *Server) instanceEvents(repository reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeEvents(w, r, repository, store.EventFilter{InstanceID: r.PathValue("id")})
	}
}

func (s *Server) incidents(repository reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var open *bool
		status := r.URL.Query().Get("status")
		switch status {
		case "":
		case "open":
			value := true
			open = &value
		case "resolved":
			value := false
			open = &value
		default:
			writeError(w, http.StatusBadRequest, "invalid incident status")
			return
		}
		incidents, err := repository.ListIncidents(r.Context(), store.IncidentFilter{
			Pool: r.URL.Query().Get("pool"), Open: open, Limit: page.limit(), Offset: page.offset,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read incidents")
			return
		}
		incidents, pagination := finishPage(incidents, page)
		responses := make([]incidentResponse, 0, len(incidents))
		for _, incident := range incidents {
			responses = append(responses, newIncidentResponse(incident))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"incidents":  responses,
			"pagination": pagination,
		})
	}
}

func (s *Server) incident(repository reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid incident ID")
			return
		}
		incident, err := repository.GetIncident(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read incident")
			return
		}
		writeJSON(w, http.StatusOK, newIncidentResponse(incident))
	}
}

func writeEvents(w http.ResponseWriter, r *http.Request, repository reader, filter store.EventFilter) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.Limit = page.limit()
	filter.Offset = page.offset
	events, err := repository.ListEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read events")
		return
	}
	events, pagination := finishPage(events, page)
	responses := make([]eventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, eventResponse{
			ID: event.ID, InstanceID: event.InstanceID, InstanceName: event.InstanceName,
			Pool: event.Pool, Kind: event.Kind,
			FromState: event.FromState, ToState: event.ToState, Stage: event.Stage,
			Result: event.Result, Reason: event.Reason, Message: event.Message,
			CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":     responses,
		"pagination": pagination,
	})
}

func newPoolResponse(pool config.Pool, data store.PoolData, staleAfter time.Duration) poolResponse {
	image := pool.Instance.Image.Alias
	if image == "" {
		image = pool.Instance.Image.Fingerprint
	}
	response := poolResponse{
		Name:   pool.Name,
		Scope:  scopeResponse{Type: pool.Scope.Type, Owner: pool.Scope.Owner, Repository: pool.Scope.Repository},
		Labels: pool.Labels, Image: image,
		MinIdle: pool.Scaling.MinIdle, MaxInstances: pool.Scaling.MaxInstances,
		MaxProvisioning: pool.Scaling.MaxProvisioning,
		StartupTimeout:  pool.Scaling.StartupTimeout.String(), IdleTimeout: pool.Scaling.IdleTimeout.String(),
		MaxLifetime: pool.Scaling.MaxLifetime.String(), Capacity: capacityResponse{
			Waiting: data.Runtime.Waiting, Needed: estimatedNeeded(pool, data.Runtime.Waiting, data.Counts),
			Bootstrapping: data.Counts.Bootstrapping, Ready: data.Counts.Ready,
			Running: data.Counts.Running, Cleaning: data.Counts.Cleaning,
			Maximum: pool.Scaling.MaxInstances,
		},
		Runtime: poolRuntimeResponse{
			State: data.Runtime.State, RunID: data.Runtime.RunID,
			ObservedAt: formatTime(data.Runtime.ObservedAt), StartedAt: formatTime(data.Runtime.ReconcileStartedAt),
			FinishedAt: formatTime(data.Runtime.ReconcileFinishedAt), LastSuccessAt: formatTime(data.Runtime.LastSuccessAt),
			Stage: data.Runtime.Stage, StageStartedAt: formatTime(data.Runtime.StageStartedAt),
			StageDeadlineAt: formatTime(data.Runtime.StageDeadlineAt),
		},
		Bootstrap: bootstrapResponse{
			Attempts:     data.Runtime.Bootstrap.Attempts,
			AttemptLimit: pool.Scaling.BootstrapAttemptLimit,
			Paused: data.Runtime.Bootstrap.Attempts >= pool.Scaling.BootstrapAttemptLimit &&
				!data.Runtime.Bootstrap.RetryAt.IsZero() && time.Now().Before(data.Runtime.Bootstrap.RetryAt),
			RetryAt: formatTime(data.Runtime.Bootstrap.RetryAt),
		},
		ObservationStale: !data.Runtime.ObservedAt.IsZero() && time.Since(data.Runtime.ObservedAt) > staleAfter,
	}
	if data.Incident != nil {
		incident := newIncidentResponse(*data.Incident)
		response.Incident = &incident
	}
	return response
}

func staleAfter(interval time.Duration) time.Duration {
	const minimum = 30 * time.Second
	age := 3 * interval
	if age < minimum {
		return minimum
	}
	return age
}

func estimatedNeeded(pool config.Pool, waiting int, counts store.PoolCounts) int {
	return scheduler.NeededFor(scheduler.Demand{
		Waiting: waiting, MinIdle: pool.Scaling.MinIdle,
		MaxInstances: pool.Scaling.MaxInstances, MaxProvisioning: pool.Scaling.MaxProvisioning,
		Ready: counts.Ready, Provisioning: counts.Bootstrapping,
		Instances: counts.Bootstrapping + counts.Ready + counts.Running + counts.Cleaning,
	})
}

func newIncidentResponse(incident store.PoolIncident) incidentResponse {
	return incidentResponse{
		ID: incident.ID, Pool: incident.Pool, RunID: incident.RunID, Stage: incident.Stage,
		Code: incident.Code, Message: incident.Message, InstanceID: incident.InstanceID,
		InstanceName: incident.InstanceName, FirstSeenAt: formatTime(incident.FirstSeenAt),
		LastSeenAt: formatTime(incident.LastSeenAt), Occurrences: incident.Occurrences,
		ResolvedAt: formatTime(incident.ResolvedAt),
	}
}

func configuredPools(pools []config.Pool) map[string]struct{} {
	configured := make(map[string]struct{}, len(pools))
	for _, pool := range pools {
		configured[pool.Name] = struct{}{}
	}
	return configured
}

func newInstanceResponse(instance instance.Instance) instanceResponse {
	return instanceResponse{
		ID: instance.ID, Name: instance.Name, Pool: instance.Pool, State: instance.State,
		Stage: instance.Stage, Result: instance.Result, Reason: instance.Reason,
		RunnerID: instance.RunnerID, Error: instance.Error, RetryCount: instance.RetryCount,
		RetryAt: formatTime(instance.RetryAt), CreatedAt: formatTime(instance.CreatedAt),
		UpdatedAt: formatTime(instance.UpdatedAt), StateChangedAt: formatTime(instance.StateChangedAt),
		StageChangedAt: formatTime(instance.StageChangedAt), FinishedAt: formatTime(instance.FinishedAt),
	}
}

func validState(state instance.State) bool {
	switch state {
	case instance.StateBootstrapping, instance.StateReady, instance.StateRunning, instance.StateCleaning, instance.StateFinished:
		return true
	default:
		return false
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
