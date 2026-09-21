package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maveonair/farm/internal/config"
	"github.com/maveonair/farm/internal/instance"
	"github.com/maveonair/farm/internal/store"
)

func TestHealth(t *testing.T) {
	monitor := &Monitor{}
	server := New("", monitor)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}

	monitor.Success(time.Now())
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestHealthAllowsRunningReconciliation(t *testing.T) {
	monitor := &Monitor{}
	now := time.Now()
	monitor.Start(now.Add(-time.Minute), now.Add(time.Minute), 1)
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
	monitor.Start(now, now.Add(time.Minute), 1)
	monitor.PoolFailure("ubuntu", "fetch_jobs", "unavailable")
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
	monitor.Failure()
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
	monitor.Success(time.Now())
	server := New("", monitor, Options{
		Controller: "primary",
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
		Controller string         `json:"controller"`
		Waiting    int            `json:"waiting"`
		Counts     map[string]int `json:"counts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Controller != "primary" || body.Waiting != 2 || body.Counts["running"] != 1 {
		t.Fatalf("body = %#v", body)
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

type fakeReader struct {
	pools  []store.PoolData
	events []store.Event
}

func (r *fakeReader) Get(context.Context, string) (instance.Instance, error) {
	return instance.Instance{}, sql.ErrNoRows
}

func (r *fakeReader) ListInstances(context.Context, store.InstanceFilter) ([]instance.Instance, error) {
	return nil, nil
}

func (r *fakeReader) ListEvents(context.Context, store.EventFilter) ([]store.Event, error) {
	return r.events, nil
}

func (r *fakeReader) ListPoolData(context.Context) ([]store.PoolData, error) {
	return r.pools, nil
}

func (r *fakeReader) ListIncidents(context.Context, store.IncidentFilter) ([]store.PoolIncident, error) {
	return nil, nil
}

func (r *fakeReader) GetIncident(context.Context, int64) (store.PoolIncident, error) {
	return store.PoolIncident{}, sql.ErrNoRows
}
