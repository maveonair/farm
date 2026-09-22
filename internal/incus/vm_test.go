package incus

import (
	"context"
	"testing"

	incus "github.com/lxc/incus/v7/client"
	"github.com/lxc/incus/v7/shared/api"
)

func TestExecReturnsGuestExitError(t *testing.T) {
	service := &Service{client: &execClient{
		op: &execOperation{operation: api.Operation{
			Metadata: map[string]any{"return": float64(7)},
		}},
	}}

	err := service.exec(context.Background(), "runner", []string{"false"})
	if err == nil {
		t.Fatal("exec() error = nil")
	}
}

func TestConnectRejectsUnixCertificates(t *testing.T) {
	_, err := Connect(ConnectOptions{
		Endpoint:       "unix:///var/lib/incus/unix.socket",
		ClientCertFile: "/tmp/client.crt",
	})
	if err == nil {
		t.Fatal("Connect() error = nil")
	}
}

type execClient struct {
	incus.InstanceServer
	op       incus.Operation
	commands [][]string
}

func (c *execClient) ExecInstance(_ string, post api.InstanceExecPost, args *incus.InstanceExecArgs) (incus.Operation, error) {
	c.commands = append(c.commands, append([]string(nil), post.Command...))
	if args.DataDone != nil {
		close(args.DataDone)
	}
	return c.op, nil
}

func (c *execClient) CreateInstanceFile(string, string, incus.InstanceFileArgs) error {
	return nil
}

type execOperation struct {
	incus.Operation
	operation api.Operation
}

func (o *execOperation) WaitContext(context.Context) error {
	return nil
}

func (o *execOperation) Get() api.Operation {
	return o.operation
}

func TestCreateRequestIsVM(t *testing.T) {
	request, err := createRequest(InstanceSpec{
		ID:         "instance-id",
		Name:       "farm-runner-01",
		Pool:       "ubuntu",
		Controller: "primary",
		Image:      Image{Alias: "farm-ubuntu-24.04"},
		Config: map[string]string{
			"limits.cpu": "4",
		},
		CloudInit: []byte("#cloud-config\n"),
	})
	if err != nil {
		t.Fatalf("createRequest() error = %v", err)
	}
	if request.Type != api.InstanceTypeVM {
		t.Fatalf("instance type = %q", request.Type)
	}
	if request.Config[managedKey] != "true" {
		t.Fatal("managed marker is missing")
	}
	if request.Config[controllerKey] != "primary" {
		t.Fatal("controller marker is missing")
	}
	if request.Config[cloudInitKey] != "#cloud-config\n" {
		t.Fatal("cloud-init is missing")
	}
}

func TestCreateRequestCopiesConfig(t *testing.T) {
	input := map[string]string{"limits.cpu": "4"}
	request, err := createRequest(InstanceSpec{
		ID:         "instance-id",
		Name:       "farm-runner-01",
		Pool:       "ubuntu",
		Controller: "primary",
		Image:      Image{Alias: "farm-ubuntu-24.04"},
		Config:     input,
	})
	if err != nil {
		t.Fatalf("createRequest() error = %v", err)
	}

	request.Config["limits.cpu"] = "8"
	if input["limits.cpu"] != "4" {
		t.Fatal("createRequest modified input config")
	}
}

func TestCreateRequestOmitsEmptyCloudInit(t *testing.T) {
	request, err := createRequest(InstanceSpec{
		ID:         "instance-id",
		Name:       "farm-runner-01",
		Pool:       "ubuntu",
		Controller: "primary",
		Image:      Image{Alias: "nixos-runner"},
	})
	if err != nil {
		t.Fatalf("createRequest() error = %v", err)
	}
	if _, found := request.Config[cloudInitKey]; found {
		t.Fatal("empty cloud-init was added")
	}
}

func TestPushRunnerConfigRestartsService(t *testing.T) {
	client := &execClient{op: &execOperation{operation: api.Operation{
		Metadata: map[string]any{"return": float64(0)},
	}}}
	service := &Service{client: client}

	if err := service.PushRunnerConfig(context.Background(), "runner", []byte("runner: {}\n")); err != nil {
		t.Fatalf("PushRunnerConfig() error = %v", err)
	}
	want := []string{"systemctl", "restart", "forgejo-runner.service"}
	got := client.commands[len(client.commands)-1]
	if len(got) != len(want) {
		t.Fatalf("service command = %q", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("service command = %q", got)
		}
	}
}

func TestWaitCloudInitWaitsForAgent(t *testing.T) {
	client := &execClient{op: &execOperation{operation: api.Operation{
		Metadata: map[string]any{"return": float64(0)},
	}}}
	service := &Service{client: client}

	if err := service.WaitCloudInit(context.Background(), "runner"); err != nil {
		t.Fatalf("WaitCloudInit() error = %v", err)
	}
	if len(client.commands) != 2 {
		t.Fatalf("commands = %q", client.commands)
	}
	if got := client.commands[0]; len(got) != 1 || got[0] != "true" {
		t.Fatalf("agent command = %q", got)
	}
	if got := client.commands[1]; len(got) != 3 || got[0] != "cloud-init" {
		t.Fatalf("cloud-init command = %q", got)
	}
}
