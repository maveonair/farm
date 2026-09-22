package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = `
controller:
  id: primary
  database: /var/lib/farm/farm.db
  reconcile_interval: 5s
  cleanup_timeout: 2m
  listen: 127.0.0.1:8080
logging:
  level: info
  format: json
forgejo:
  url: https://git.example.com
  token_file: /run/secrets/forgejo
  timeout: 15s
incus:
  endpoint: https://incus.example.com:8443
  project: farm
  client_certificate_file: /etc/farm/incus.crt
  client_key_file: /etc/farm/incus.key
  server_certificate_file: /etc/farm/server.crt
runner:
  download_url: https://code.forgejo.org/forgejo/runner/releases/download/v1/runner
  sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
pools:
  - name: ubuntu
    scope:
      type: repository
      owner: example
      repository: project
    labels: [farm-ubuntu]
    instance:
      image: farm-ubuntu-24.04
      profiles: [farm-instance]
    scaling:
      min_idle: 0
      max_instances: 4
      max_provisioning: 2
      startup_timeout: 5m
      idle_timeout: 5m
      max_lifetime: 6h
`

func TestLoad(t *testing.T) {
	path := writeConfig(t, validConfig)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Pools[0].Instance.Image.Alias; got != "farm-ubuntu-24.04" {
		t.Fatalf("image = %q", got)
	}
}

func TestLoadDefaultsRunnerInstallation(t *testing.T) {
	path := writeConfig(t, validConfig)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Pools[0].Instance.RunnerInstall; got != RunnerInstallController {
		t.Fatalf("runner installation = %q", got)
	}
}

func TestLoadAcceptsImageRunner(t *testing.T) {
	contents := strings.Replace(validConfig, "      image: farm-ubuntu-24.04", "      image: farm-ubuntu-24.04\n      runner_installation: image", 1)
	contents = strings.Replace(contents, `runner:
  download_url: https://code.forgejo.org/forgejo/runner/releases/download/v1/runner
  sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
`, "", 1)

	cfg, err := Load(writeConfig(t, contents))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Pools[0].Instance.RunnerInstall; got != RunnerInstallImage {
		t.Fatalf("runner installation = %q", got)
	}
}

func TestLoadRejectsUnknownRunnerInstallation(t *testing.T) {
	contents := strings.Replace(validConfig, "      image: farm-ubuntu-24.04", "      image: farm-ubuntu-24.04\n      runner_installation: external", 1)

	_, err := Load(writeConfig(t, contents))
	if err == nil || !strings.Contains(err.Error(), "runner_installation") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRequiresRunnerForMixedInstallation(t *testing.T) {
	contents := strings.Replace(validConfig, `runner:
  download_url: https://code.forgejo.org/forgejo/runner/releases/download/v1/runner
  sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
`, "", 1)
	contents = strings.Replace(contents, "pools:\n", `pools:
  - name: image
    scope:
      type: user
    labels: [farm-image]
    instance:
      image: farm-image
      runner_installation: image
    scaling:
      max_instances: 1
`, 1)

	_, err := Load(writeConfig(t, contents))
	if err == nil || !strings.Contains(err.Error(), "runner.download_url") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadValidatesUnusedRunner(t *testing.T) {
	contents := strings.Replace(validConfig, "      image: farm-ubuntu-24.04", "      image: farm-ubuntu-24.04\n      runner_installation: image", 1)
	contents = strings.Replace(contents, "https://code.forgejo.org/forgejo/runner/releases/download/v1/runner", "invalid", 1)

	_, err := Load(writeConfig(t, contents))
	if err == nil || !strings.Contains(err.Error(), "runner.download_url") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	contents := validConfig
	for _, line := range []string{
		"  reconcile_interval: 5s\n", "  cleanup_timeout: 2m\n", "  listen: 127.0.0.1:8080\n",
		"logging:\n", "  level: info\n", "  format: json\n", "  timeout: 15s\n",
		"      min_idle: 0\n", "      max_provisioning: 2\n", "      startup_timeout: 5m\n",
		"      idle_timeout: 5m\n", "      max_lifetime: 6h\n",
	} {
		contents = strings.Replace(contents, line, "", 1)
	}
	cfg, err := Load(writeConfig(t, contents))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Controller.ReconcileInterval.Duration != defaultReconcileInterval || cfg.Controller.Listen != defaultListen {
		t.Fatalf("controller defaults = %#v", cfg.Controller)
	}
	if cfg.Forgejo.Timeout.Duration != defaultForgejoTimeout {
		t.Fatalf("Forgejo timeout = %v", cfg.Forgejo.Timeout.Duration)
	}
	if cfg.Pools[0].Scaling.MaxProvisioning != defaultMaxProvisioning || cfg.Pools[0].Scaling.StartupTimeout.Duration != defaultStartupTimeout {
		t.Fatalf("scaling defaults = %#v", cfg.Pools[0].Scaling)
	}
	if cfg.Pools[0].Scaling.BootstrapAttemptLimit != defaultBootstrapAttemptLimit || cfg.Pools[0].Scaling.BootstrapRetryInterval.Duration != defaultBootstrapRetryInterval {
		t.Fatalf("bootstrap defaults = %#v", cfg.Pools[0].Scaling)
	}
}

func TestLoadRejectsNegativeBootstrapLimit(t *testing.T) {
	contents := strings.Replace(validConfig, "      max_instances: 4", "      max_instances: 4\n      bootstrap_attempt_limit: -1", 1)

	_, err := Load(writeConfig(t, contents))
	if err == nil || !strings.Contains(err.Error(), "bootstrap_attempt_limit") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadAcceptsUserScope(t *testing.T) {
	contents := strings.Replace(validConfig, `      type: repository
      owner: example
      repository: project`, `      type: user`, 1)
	path := writeConfig(t, contents)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Pools[0].Scope.Type; got != ScopeUser {
		t.Fatalf("scope type = %q", got)
	}
}

func TestLoadRejectsUserScopeFields(t *testing.T) {
	tests := []struct {
		name  string
		field string
	}{
		{name: "owner", field: "      owner: example\n"},
		{name: "repository", field: "      repository: project\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contents := strings.Replace(validConfig, `      type: repository
      owner: example
      repository: project
`, "      type: user\n"+test.field, 1)
			path := writeConfig(t, contents)

			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), "user scope") {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestLoadRejectsLoggingLevel(t *testing.T) {
	contents := strings.Replace(validConfig, "  level: info", "  level: verbose", 1)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "logging.level") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsLoggingFormat(t *testing.T) {
	contents := strings.Replace(validConfig, "  format: json", "  format: binary", 1)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "logging.format") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadAcceptsIncusUnixSocket(t *testing.T) {
	contents := strings.Replace(validConfig, `  endpoint: https://incus.example.com:8443
  project: farm
  client_certificate_file: /etc/farm/incus.crt
  client_key_file: /etc/farm/incus.key
  server_certificate_file: /etc/farm/server.crt`, `  endpoint: unix:///var/lib/incus/unix.socket
  project: farm`, 1)
	path := writeConfig(t, contents)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Incus.Endpoint != "unix:///var/lib/incus/unix.socket" {
		t.Fatalf("endpoint = %q", cfg.Incus.Endpoint)
	}
}

func TestLoadRejectsIncusUnixCertificates(t *testing.T) {
	contents := strings.Replace(validConfig, "https://incus.example.com:8443", "unix:///var/lib/incus/unix.socket", 1)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsIncusEndpointScheme(t *testing.T) {
	contents := strings.Replace(validConfig, "https://incus.example.com:8443", "http://incus.example.com:8443", 1)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "HTTPS or Unix") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsInstanceType(t *testing.T) {
	contents := strings.Replace(validConfig, "    instance:\n", "    instance:\n      type: container\n", 1)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "field type not found") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsDuplicateLabel(t *testing.T) {
	contents := validConfig + `
  - name: ubuntu-large
    scope:
      type: repository
      owner: example
      repository: project
    labels: [farm-ubuntu]
    instance:
      image: farm-ubuntu-24.04
    scaling:
      max_instances: 2
      max_provisioning: 1
      startup_timeout: 5m
      idle_timeout: 5m
      max_lifetime: 6h
`
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "primary label") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsInsecureForgejo(t *testing.T) {
	contents := strings.Replace(validConfig, "https://git.example.com", "http://git.example.com", 1)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsUnknownImageField(t *testing.T) {
	contents := strings.Replace(
		validConfig,
		"      image: farm-ubuntu-24.04",
		"      image:\n        alias: farm-ubuntu-24.04\n        protcol: simplestreams",
		1,
	)
	path := writeConfig(t, contents)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "protcol") {
		t.Fatalf("Load() error = %v", err)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "farm.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
