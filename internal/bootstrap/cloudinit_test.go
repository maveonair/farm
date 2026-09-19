package bootstrap

import (
	"bytes"
	"testing"
)

func TestCloudInitOmitsCredentials(t *testing.T) {
	data, err := CloudInit(Install{
		DownloadURL: "https://code.forgejo.org/forgejo/runner/releases/download/v1/runner",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatalf("CloudInit() error = %v", err)
	}

	for _, secret := range [][]byte{[]byte("uuid"), []byte("token")} {
		if bytes.Contains(data, secret) {
			t.Fatalf("cloud-init contains %q", secret)
		}
	}
	if !bytes.HasPrefix(data, []byte("#cloud-config\n")) {
		t.Fatal("cloud-init header is missing")
	}
}

func TestRunnerConfig(t *testing.T) {
	data, err := RunnerConfig(Connection{
		URL:    "https://git.example.com",
		UUID:   "runner-uuid",
		Token:  "runner-token",
		Labels: []string{"farm-ubuntu:host"},
	})
	if err != nil {
		t.Fatalf("RunnerConfig() error = %v", err)
	}

	for _, want := range [][]byte{[]byte("runner-uuid"), []byte("runner-token"), []byte("farm-ubuntu:host")} {
		if !bytes.Contains(data, want) {
			t.Fatalf("runner config does not contain %q", want)
		}
	}
}

func TestSystemdUnitRunsOneJob(t *testing.T) {
	unit := systemdUnit()
	want := "ExecStart=/usr/local/bin/forgejo-runner -c /etc/farm/runner.yml one-job --wait"

	if !bytes.Contains([]byte(unit), []byte(want)) {
		t.Fatalf("systemd unit does not contain %q", want)
	}
	if bytes.Contains([]byte(unit), []byte(" daemon ")) {
		t.Fatal("systemd unit uses daemon mode")
	}
}
