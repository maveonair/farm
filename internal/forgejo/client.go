package forgejo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxResponseSize = 4 << 20

const runnerPageSize = 100

type Client struct {
	baseURL *url.URL
	token   string
	http    *http.Client
}

type Scope struct {
	Kind       ScopeKind
	Owner      string
	Repository string
}

type ScopeKind string

const (
	ScopeRepository   ScopeKind = "repository"
	ScopeOrganization ScopeKind = "organization"
	ScopeUser         ScopeKind = "user"
	ScopeGlobal       ScopeKind = "global"
)

type JobStatus string

const (
	JobWaiting JobStatus = "waiting"
	JobRunning JobStatus = "running"
)

type Job struct {
	ID      int64     `json:"id"`
	Handle  string    `json:"handle"`
	RunsOn  []string  `json:"runs_on"`
	Status  JobStatus `json:"status"`
	TaskID  int64     `json:"task_id"`
	RepoID  int64     `json:"repo_id"`
	OwnerID int64     `json:"owner_id"`
}

type Registration struct {
	ID    int64  `json:"id"`
	UUID  string `json:"uuid"`
	Token string `json:"token"`
}

type RunnerStatus string

const (
	RunnerOffline RunnerStatus = "offline"
	RunnerIdle    RunnerStatus = "idle"
	RunnerActive  RunnerStatus = "active"
)

type Runner struct {
	ID          int64        `json:"id"`
	UUID        string       `json:"uuid"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Labels      []string     `json:"labels"`
	Ephemeral   bool         `json:"ephemeral"`
	Status      RunnerStatus `json:"status"`
}

func (c *Client) Runners(ctx context.Context, scope Scope) ([]Runner, error) {
	var runners []Runner
	for page := 1; ; page++ {
		path := fmt.Sprintf("%s?page=%d&limit=%d", scope.path(), page, runnerPageSize)
		var batch []Runner
		if err := c.do(ctx, http.MethodGet, path, nil, &batch); err != nil {
			return nil, fmt.Errorf("list runners: %w", err)
		}
		runners = append(runners, batch...)
		if len(batch) < runnerPageSize {
			return runners, nil
		}
	}
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("forgejo returned HTTP %d: %s", e.StatusCode, e.Body)
}

func IsNotFound(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

func New(rawURL, token string, httpClient *http.Client) (*Client, error) {
	baseURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse Forgejo URL: %w", err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, errors.New("forgejo URL must be absolute")
	}
	if token == "" {
		return nil, errors.New("forgejo token is required")
	}
	if httpClient == nil {
		return nil, errors.New("missing HTTP client")
	}

	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/"
	return &Client{baseURL: baseURL, token: token, http: httpClient}, nil
}

func (c *Client) Jobs(ctx context.Context, scope Scope) ([]Job, error) {
	var jobs []Job
	if err := c.do(ctx, http.MethodGet, scope.path()+"/jobs", nil, &jobs); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	return jobs, nil
}

func (c *Client) Register(ctx context.Context, scope Scope, name, description string) (Registration, error) {
	request := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Ephemeral   bool   `json:"ephemeral"`
	}{
		Name:        name,
		Description: description,
		Ephemeral:   true,
	}

	var registration Registration
	if err := c.do(ctx, http.MethodPost, scope.path(), request, &registration); err != nil {
		return Registration{}, fmt.Errorf("register runner: %w", err)
	}
	return registration, nil
}

func (c *Client) Runner(ctx context.Context, scope Scope, id int64) (Runner, error) {
	var runner Runner
	path := scope.path() + "/" + strconv.FormatInt(id, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, &runner); err != nil {
		return Runner{}, fmt.Errorf("get runner: %w", err)
	}
	return runner, nil
}

func (c *Client) DeleteRunner(ctx context.Context, scope Scope, id int64) error {
	path := scope.path() + "/" + strconv.FormatInt(id, 10)
	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete runner: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	endpoint, err := c.baseURL.Parse("api/v1/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return fmt.Errorf("build request URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "token "+c.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		// ReadAll reports response failures. Close only releases transport resources.
		_ = response.Body.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if output == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (s Scope) path() string {
	switch s.Kind {
	case ScopeOrganization:
		return "orgs/" + url.PathEscape(s.Owner) + "/actions/runners"
	case ScopeUser:
		return "user/actions/runners"
	case ScopeGlobal:
		return "admin/actions/runners"
	default:
		return "repos/" + url.PathEscape(s.Owner) + "/" + url.PathEscape(s.Repository) + "/actions/runners"
	}
}
