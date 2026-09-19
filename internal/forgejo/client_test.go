package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "management-token"

func TestRegister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/example/project/actions/runners" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "token "+testToken {
			t.Errorf("Authorization = %q", got)
		}

		var request struct {
			Ephemeral bool `json:"ephemeral"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode() error = %v", err)
		}
		if !request.Ephemeral {
			t.Error("runner is not ephemeral")
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":12,"uuid":"runner-uuid","token":"runner-token"}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server)
	registration, err := client.Register(context.Background(), testScope(), "farm-runner", "Managed by FARM")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if registration.ID != 12 || registration.Token != "runner-token" {
		t.Fatalf("registration = %#v", registration)
	}
}

func TestJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":42,"runs_on":["farm-ubuntu"],"status":"waiting"}]`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server)
	jobs, err := client.Jobs(context.Background(), testScope())
	if err != nil {
		t.Fatalf("Jobs() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].Status != JobWaiting {
		t.Fatalf("jobs = %#v", jobs)
	}
}

func TestRunners(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("page") != "1" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":42,"description":"Managed by FARM"}]`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server)
	runners, err := client.Runners(context.Background(), testScope())
	if err != nil {
		t.Fatalf("Runners() error = %v", err)
	}
	if len(runners) != 1 || runners[0].Description != "Managed by FARM" {
		t.Fatalf("runners = %#v", runners)
	}
}

func TestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server)
	_, err := client.Runner(context.Background(), testScope(), 12)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("Runner() error = %v", err)
	}
}

func TestScopePath(t *testing.T) {
	tests := []struct {
		name  string
		scope Scope
		want  string
	}{
		{name: "repository", scope: Scope{Kind: ScopeRepository, Owner: "example", Repository: "project"}, want: "repos/example/project/actions/runners"},
		{name: "organization", scope: Scope{Kind: ScopeOrganization, Owner: "example"}, want: "orgs/example/actions/runners"},
		{name: "user", scope: Scope{Kind: ScopeUser}, want: "user/actions/runners"},
		{name: "global", scope: Scope{Kind: ScopeGlobal}, want: "admin/actions/runners"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.scope.path(); got != test.want {
				t.Fatalf("path() = %q, want %q", got, test.want)
			}
		})
	}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	client, err := New(server.URL, testToken, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func testScope() Scope {
	return Scope{Owner: "example", Repository: "project"}
}
