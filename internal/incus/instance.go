package incus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	incus "github.com/lxc/incus/v7/client"
	"github.com/lxc/incus/v7/shared/api"
)

const (
	cloudInitKey  = "cloud-init.user-data"
	managedKey    = "user.farm.managed"
	instanceKey   = "user.farm.instance-id"
	poolKey       = "user.farm.pool"
	controllerKey = "user.farm.controller"
	agentRetry    = 2 * time.Second
)

type ConnectOptions struct {
	Endpoint       string
	Project        string
	ClientCertFile string
	ClientKeyFile  string
	ServerCertFile string
}

type Image struct {
	Alias           string
	Fingerprint     string
	Server          string
	Protocol        string
	Certificate     string
	CertificateFile string
}

type InstanceSpec struct {
	ID         string
	Name       string
	Pool       string
	Controller string
	Image      Image
	Profiles   []string
	Config     map[string]string
	CloudInit  []byte
}

type ManagedInstance struct {
	ID   string
	Name string
	Pool string
}

type Service struct {
	client incus.InstanceServer
}

func Connect(options ConnectOptions) (*Service, error) {
	endpoint, err := url.Parse(options.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Incus endpoint: %w", err)
	}
	if endpoint.Scheme == "unix" {
		if options.ClientCertFile != "" || options.ClientKeyFile != "" || options.ServerCertFile != "" {
			return nil, errors.New("incus certificate files cannot be used with a Unix socket")
		}
		client, err := incus.ConnectIncusUnix(endpoint.Path, &incus.ConnectionArgs{UserAgent: "farm"})
		if err != nil {
			return nil, fmt.Errorf("connect to Incus Unix socket: %w", err)
		}
		if options.Project != "" {
			client = client.UseProject(options.Project)
		}
		return &Service{client: client}, nil
	}
	if endpoint.Scheme != "https" {
		return nil, fmt.Errorf("unsupported Incus endpoint scheme %q", endpoint.Scheme)
	}

	clientCert, err := os.ReadFile(options.ClientCertFile)
	if err != nil {
		return nil, fmt.Errorf("read Incus client certificate: %w", err)
	}
	clientKey, err := os.ReadFile(options.ClientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read Incus client key: %w", err)
	}
	serverCert, err := os.ReadFile(options.ServerCertFile)
	if err != nil {
		return nil, fmt.Errorf("read Incus server certificate: %w", err)
	}

	client, err := incus.ConnectIncus(options.Endpoint, &incus.ConnectionArgs{
		TLSClientCert: string(clientCert),
		TLSClientKey:  string(clientKey),
		TLSServerCert: string(serverCert),
		UserAgent:     "farm",
	})
	if err != nil {
		return nil, fmt.Errorf("connect to Incus: %w", err)
	}
	if options.Project != "" {
		client = client.UseProject(options.Project)
	}

	return &Service{client: client}, nil
}

func (s *Service) Create(ctx context.Context, spec InstanceSpec) error {
	if spec.Image.CertificateFile != "" {
		certificate, err := os.ReadFile(spec.Image.CertificateFile)
		if err != nil {
			return fmt.Errorf("read image server certificate: %w", err)
		}
		spec.Image.Certificate = string(certificate)
	}
	request, err := createRequest(spec)
	if err != nil {
		return err
	}

	op, err := s.client.CreateInstance(request)
	if err != nil {
		return fmt.Errorf("create instance: %w", err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait for instance creation: %w", err)
	}

	instance, _, err := s.client.GetInstance(spec.Name)
	if err != nil {
		return fmt.Errorf("verify instance: %w", err)
	}
	if instance.Type != string(api.InstanceTypeVM) {
		return fmt.Errorf("security violation: Incus created instance type %q", instance.Type)
	}

	return nil
}

func (s *Service) Managed(ctx context.Context, controller, pool string) ([]ManagedInstance, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	instances, err := s.client.GetInstancesFull(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("list managed instances: %w", err)
	}

	var managed []ManagedInstance
	for _, instance := range instances {
		if instance.Config[managedKey] != "true" || instance.Config[controllerKey] != controller {
			continue
		}
		if pool != "" && instance.Config[poolKey] != pool {
			continue
		}
		if instance.Type != string(api.InstanceTypeVM) {
			return nil, fmt.Errorf("security violation: managed instance %q has type %q", instance.Name, instance.Type)
		}
		managed = append(managed, ManagedInstance{
			ID:   instance.Config[instanceKey],
			Name: instance.Name,
			Pool: instance.Config[poolKey],
		})
	}
	return managed, nil
}

func (s *Service) PushRunnerConfig(ctx context.Context, name string, data []byte) error {
	const temporaryPath = "/tmp/farm-runner.yml"

	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.client.CreateInstanceFile(name, temporaryPath, incus.InstanceFileArgs{
		Content:   bytes.NewReader(data),
		UID:       0,
		GID:       0,
		Mode:      0o600,
		Type:      "file",
		WriteMode: "overwrite",
	})
	if err != nil {
		return fmt.Errorf("upload runner config: %w", err)
	}

	command := []string{
		"install", "-m", "0600", "-o", "runner", "-g", "runner",
		temporaryPath, "/etc/farm/runner.yml",
	}
	if err := s.exec(ctx, name, command); err != nil {
		return fmt.Errorf("install runner config: %w", err)
	}
	if err := s.exec(ctx, name, []string{"rm", "-f", temporaryPath}); err != nil {
		return fmt.Errorf("remove temporary runner config: %w", err)
	}
	if err := s.exec(ctx, name, []string{"systemctl", "restart", "forgejo-runner.service"}); err != nil {
		return fmt.Errorf("restart runner: %w", err)
	}

	return nil
}

func (s *Service) WaitCloudInit(ctx context.Context, name string) error {
	if err := s.WaitAgent(ctx, name); err != nil {
		return err
	}

	if err := s.exec(ctx, name, []string{"cloud-init", "status", "--wait"}); err != nil {
		return fmt.Errorf("wait for cloud-init: %w", err)
	}
	return nil
}

func (s *Service) WaitAgent(ctx context.Context, name string) error {
	for {
		if err := s.exec(ctx, name, []string{"true"}); err == nil {
			return nil
		}

		timer := time.NewTimer(agentRetry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *Service) Delete(ctx context.Context, name string) error {
	instance, _, err := s.client.GetInstance(name)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get instance before deletion: %w", err)
	}
	if instance.Type != string(api.InstanceTypeVM) {
		return fmt.Errorf("security violation: refusing to delete instance type %q", instance.Type)
	}

	if instance.IsActive() {
		op, err := s.client.UpdateInstanceState(name, api.InstanceStatePut{
			Action: "stop",
			Force:  true,
		}, "")
		if err != nil {
			return fmt.Errorf("stop instance: %w", err)
		}
		if err := op.WaitContext(ctx); err != nil {
			return fmt.Errorf("wait for instance stop: %w", err)
		}
	}

	op, err := s.client.DeleteInstance(name)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait for instance deletion: %w", err)
	}
	return nil
}

func (s *Service) exec(ctx context.Context, name string, command []string) error {
	var stderr bytes.Buffer
	dataDone := make(chan bool)
	op, err := s.client.ExecInstance(name, api.InstanceExecPost{
		Command:   command,
		WaitForWS: true,
	}, &incus.InstanceExecArgs{Stderr: &stderr, DataDone: dataDone})
	if err != nil {
		return err
	}
	waitErr := op.WaitContext(ctx)
	select {
	case <-dataDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	if waitErr != nil {
		if stderr.Len() == 0 {
			return waitErr
		}
		return fmt.Errorf("%w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}

	exitCode, ok := op.Get().Metadata["return"].(float64)
	if ok && exitCode != 0 {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return fmt.Errorf("guest command exited with status %d", int(exitCode))
		}
		return fmt.Errorf("guest command exited with status %d: %s", int(exitCode), message)
	}
	return nil
}

func createRequest(spec InstanceSpec) (api.InstancesPost, error) {
	if spec.ID == "" {
		return api.InstancesPost{}, errors.New("missing instance ID")
	}
	if spec.Name == "" {
		return api.InstancesPost{}, errors.New("missing instance name")
	}
	if spec.Pool == "" {
		return api.InstancesPost{}, errors.New("pool is required")
	}
	if spec.Controller == "" {
		return api.InstancesPost{}, errors.New("controller is required")
	}
	if spec.Image.Alias == "" && spec.Image.Fingerprint == "" {
		return api.InstancesPost{}, errors.New("image alias or fingerprint is required")
	}

	config := make(map[string]string, len(spec.Config)+4)
	maps.Copy(config, spec.Config)

	if len(spec.CloudInit) != 0 {
		config[cloudInitKey] = string(spec.CloudInit)
	}
	config[managedKey] = "true"
	config[instanceKey] = spec.ID
	config[poolKey] = spec.Pool
	config[controllerKey] = spec.Controller

	return api.InstancesPost{
		Name:     spec.Name,
		Type:     api.InstanceTypeVM,
		Start:    true,
		Config:   config,
		Profiles: append([]string(nil), spec.Profiles...),
		Source: api.InstanceSource{
			Type:        "image",
			Alias:       spec.Image.Alias,
			Fingerprint: spec.Image.Fingerprint,
			Server:      spec.Image.Server,
			Protocol:    spec.Image.Protocol,
			Certificate: spec.Image.Certificate,
		},
	}, nil
}
