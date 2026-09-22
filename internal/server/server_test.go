package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/store"
)

func TestWriteJSONLogsWriteFailure(t *testing.T) {
	var logs bytes.Buffer
	server := &Server{logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))}

	server.writeJSON(context.Background(), failingWriter{}, http.StatusOK, map[string]string{"status": "ok"})

	if !strings.Contains(logs.String(), "write HTTP response") {
		t.Fatalf("log = %q", logs.String())
	}
}

type failingWriter struct{}

func (failingWriter) Header() http.Header {
	return make(http.Header)
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("client disconnected")
}

func (failingWriter) WriteHeader(int) {}

func TestHealth(t *testing.T) {
	monitor := &Monitor{}
	server := New("", monitor)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}

	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestHealthAllowsRunningReconciliation(t *testing.T) {
	monitor := &Monitor{}
	now := time.Now()
	monitor.Configure([]string{"ubuntu"}, time.Minute)
	monitor.PoolStart("ubuntu", now.Add(-time.Minute), now.Add(time.Minute))
	server := New("", monitor)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestHealthRejectsDegradedReconciliation(t *testing.T) {
	monitor := &Monitor{}
	now := time.Now()
	monitor.Configure([]string{"ubuntu"}, time.Minute)
	monitor.PoolStart("ubuntu", now, now.Add(time.Minute))
	monitor.PoolFailure("ubuntu", "fetch_jobs", "unavailable", now)
	server := New("", monitor)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestMetrics(t *testing.T) {
	monitor := &Monitor{}
	now := time.Now()
	monitor.Configure([]string{"ubuntu"}, time.Minute)
	monitor.PoolStart("ubuntu", now, now.Add(time.Minute))
	monitor.PoolFailure("ubuntu", "fetch_jobs", "unavailable", now)
	server := New("", monitor)

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestSummaryAPI(t *testing.T) {
	monitor := &Monitor{}
	now := time.Now()
	monitor.Configure([]string{"ubuntu"}, time.Minute)
	monitor.PoolStart("ubuntu", now, now.Add(time.Minute))
	monitor.PoolSuccess("ubuntu", now)
	server := New("", monitor, Options{
		Controller: "primary",
		Version:    "0.3.0",
		Pools:      []config.Pool{{Name: "ubuntu"}},
		Store: &fakeReader{pools: []store.PoolData{{
			Pool: "ubuntu", Runtime: store.PoolRuntime{Waiting: 2},
			Counts: store.PoolCounts{Ready: 1, Running: 1},
		}}},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Controller     string                     `json:"controller"`
		Version        string                     `json:"version"`
		Waiting        int                        `json:"waiting"`
		Counts         map[string]int             `json:"counts"`
		Reconciliation map[string]json.RawMessage `json:"reconciliation"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Controller != "primary" || body.Version != "0.3.0" || body.Waiting != 2 || body.Counts["running"] != 1 {
		t.Fatalf("body = %#v", body)
	}
	if _, found := body.Reconciliation["active_pools"]; !found {
		t.Fatal("reconciliation.active_pools is missing")
	}
	for _, field := range []string{"completed_pools", "last_outcome"} {
		if _, found := body.Reconciliation[field]; found {
			t.Fatalf("reconciliation.%s is present", field)
		}
	}
}

func TestPoolAPIUsesLiveCounts(t *testing.T) {
	server := New("", &Monitor{}, Options{
		Pools: []config.Pool{{Name: "ubuntu"}},
		Store: &fakeReader{pools: []store.PoolData{{
			Pool: "ubuntu", Counts: store.PoolCounts{Bootstrapping: 1},
		}}},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/pools/ubuntu", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Capacity struct {
			Bootstrapping int `json:"bootstrapping"`
		} `json:"capacity"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Capacity.Bootstrapping != 1 {
		t.Fatalf("bootstrapping = %d", body.Capacity.Bootstrapping)
	}
}

func TestPoolAPIReportsBootstrapPause(t *testing.T) {
	retryAt := time.Now().Add(time.Hour)
	server := New("", &Monitor{}, Options{
		Pools: []config.Pool{{Name: "ubuntu", Scaling: config.Scaling{BootstrapAttemptLimit: 5}}},
		Store: &fakeReader{pools: []store.PoolData{{
			Pool: "ubuntu", Runtime: store.PoolRuntime{
				Bootstrap: store.BootstrapState{Attempts: 5, RetryAt: retryAt},
			},
		}}},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/pools/ubuntu", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var body struct {
		Bootstrap struct {
			Attempts int  `json:"attempts"`
			Paused   bool `json:"paused"`
		} `json:"bootstrap"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Bootstrap.Attempts != 5 || !body.Bootstrap.Paused {
		t.Fatalf("bootstrap = %#v", body.Bootstrap)
	}
}

func TestEventsAPIIncludesInstanceName(t *testing.T) {
	server := New("", &Monitor{}, Options{
		Store: &fakeReader{events: []store.Event{{
			ID: 1, InstanceID: "instance-id", InstanceName: "farm-debian-instance", Pool: "debian",
		}}},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Events []struct {
			InstanceName string `json:"instance_name"`
		} `json:"events"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].InstanceName != "farm-debian-instance" {
		t.Fatalf("events = %#v", body.Events)
	}
}

func TestInstancesAPIFinalPage(t *testing.T) {
	repository := &fakeReader{instances: make([]instance.Instance, defaultPageSize)}
	server := New("", &Monitor{}, Options{Store: repository})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var body struct {
		Instances  []instanceResponse `json:"instances"`
		Pagination paginationResponse `json:"pagination"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Instances) != defaultPageSize || body.Pagination.PerPage != defaultPageSize || body.Pagination.HasNext {
		t.Fatalf("body = %#v", body)
	}
}

func TestInstancesAPIRejectsInvalidPagination(t *testing.T) {
	queries := []string{"page=invalid", "page=0", "page=-1", "per_page=0", "per_page=" + strconv.Itoa(maxPageSize+1)}
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			server := New("", &Monitor{}, Options{Store: &fakeReader{}})

			request := httptest.NewRequest(http.MethodGet, "/api/v1/instances?"+query, nil)
			response := httptest.NewRecorder()
			server.http.Handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestInstancesAPIPaginates(t *testing.T) {
	repository := &fakeReader{instances: make([]instance.Instance, defaultPageSize+1)}
	server := New("", &Monitor{}, Options{Store: repository})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/instances?page=2", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Instances  []instanceResponse `json:"instances"`
		Pagination paginationResponse `json:"pagination"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Instances) != defaultPageSize || !body.Pagination.HasNext || body.Pagination.Page != 2 {
		t.Fatalf("body = %#v", body)
	}
	if repository.instanceFilter.Offset != defaultPageSize {
		t.Fatalf("offset = %d", repository.instanceFilter.Offset)
	}
}

func TestInstancesAPIUsesRequestedPageSize(t *testing.T) {
	for _, perPage := range []int{10, maxPageSize} {
		t.Run(strconv.Itoa(perPage), func(t *testing.T) {
			repository := &fakeReader{instances: make([]instance.Instance, perPage+1)}
			server := New("", &Monitor{}, Options{Store: repository})

			path := "/api/v1/instances?page=2&per_page=" + strconv.Itoa(perPage)
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			server.http.Handler.ServeHTTP(response, request)
			var body struct {
				Instances  []instanceResponse `json:"instances"`
				Pagination struct {
					Page    int  `json:"page"`
					PerPage int  `json:"per_page"`
					HasNext bool `json:"has_next"`
				} `json:"pagination"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(body.Instances) != perPage || body.Pagination.PerPage != perPage || !body.Pagination.HasNext {
				t.Fatalf("body = %#v", body)
			}
			if repository.instanceFilter.Limit != perPage+1 || repository.instanceFilter.Offset != perPage {
				t.Fatalf("filter = %#v", repository.instanceFilter)
			}
		})
	}
}

func TestEventsAPIPaginates(t *testing.T) {
	repository := &fakeReader{events: make([]store.Event, defaultPageSize+1)}
	server := New("", &Monitor{}, Options{Store: repository})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?page=3", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var body struct {
		Events     []eventResponse    `json:"events"`
		Pagination paginationResponse `json:"pagination"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != defaultPageSize || !body.Pagination.HasNext || repository.eventFilter.Offset != 2*defaultPageSize {
		t.Fatalf("body = %#v, offset = %d", body, repository.eventFilter.Offset)
	}
}

func TestIncidentsAPIPaginates(t *testing.T) {
	repository := &fakeReader{incidents: make([]store.PoolIncident, defaultPageSize+1)}
	server := New("", &Monitor{}, Options{Store: repository})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?page=2", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var body struct {
		Incidents  []incidentResponse `json:"incidents"`
		Pagination paginationResponse `json:"pagination"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Incidents) != defaultPageSize || !body.Pagination.HasNext || repository.incidentFilter.Offset != defaultPageSize {
		t.Fatalf("body = %#v, offset = %d", body, repository.incidentFilter.Offset)
	}
}

type fakeReader struct {
	pools          []store.PoolData
	events         []store.Event
	instances      []instance.Instance
	incidents      []store.PoolIncident
	instanceFilter store.InstanceFilter
	eventFilter    store.EventFilter
	incidentFilter store.IncidentFilter
}

func (r *fakeReader) Get(context.Context, string) (instance.Instance, error) {
	return instance.Instance{}, sql.ErrNoRows
}

func (r *fakeReader) ListInstances(_ context.Context, filter store.InstanceFilter) ([]instance.Instance, error) {
	r.instanceFilter = filter
	return r.instances, nil
}

func (r *fakeReader) ListEvents(_ context.Context, filter store.EventFilter) ([]store.Event, error) {
	r.eventFilter = filter
	return r.events, nil
}

func (r *fakeReader) ListPoolData(context.Context) ([]store.PoolData, error) {
	return r.pools, nil
}

func (r *fakeReader) ListIncidents(_ context.Context, filter store.IncidentFilter) ([]store.PoolIncident, error) {
	r.incidentFilter = filter
	return r.incidents, nil
}

func (r *fakeReader) GetIncident(context.Context, int64) (store.PoolIncident, error) {
	return store.PoolIncident{}, sql.ErrNoRows
}
