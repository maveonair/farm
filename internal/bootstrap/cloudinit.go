package bootstrap

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	runnerPath  = "/usr/local/bin/forgejo-runner"
	configPath  = "/etc/farm/runner.yml"
	serviceName = "forgejo-runner.service"
)

type Install struct {
	DownloadURL string
	SHA256      string
}

type Connection struct {
	URL    string
	UUID   string
	Token  string
	Labels []string
}

type cloudConfig struct {
	Packages   []string    `yaml:"packages"`
	WriteFiles []cloudFile `yaml:"write_files"`
	RunCmd     []string    `yaml:"runcmd"`
}

type cloudFile struct {
	Path        string `yaml:"path"`
	Owner       string `yaml:"owner,omitempty"`
	Permissions string `yaml:"permissions"`
	Content     string `yaml:"content"`
}

func CloudInit(spec Install) ([]byte, error) {
	config := cloudConfig{
		Packages: []string{"ca-certificates", "curl", "git", "nodejs"},
		WriteFiles: []cloudFile{
			{
				Path:        "/usr/local/sbin/farm-install-runner",
				Owner:       "root:root",
				Permissions: "0755",
				Content:     installScript(spec),
			},
			{
				Path:        "/etc/systemd/system/" + serviceName,
				Owner:       "root:root",
				Permissions: "0644",
				Content:     systemdUnit(),
			},
		},
		RunCmd: []string{
			"id -u runner >/dev/null 2>&1 || useradd --system --create-home --shell /bin/bash runner",
			"install -d -m 0700 -o runner -g runner /etc/farm",
			"/usr/local/sbin/farm-install-runner",
			"systemctl daemon-reload",
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud-init: %w", err)
	}

	return append([]byte("#cloud-config\n"), data...), nil
}

func RunnerConfig(connection Connection) ([]byte, error) {
	config := struct {
		Runner struct {
			Labels []string `yaml:"labels"`
		} `yaml:"runner"`
		Server struct {
			Connections map[string]struct {
				URL   string `yaml:"url"`
				UUID  string `yaml:"uuid"`
				Token string `yaml:"token"`
			} `yaml:"connections"`
		} `yaml:"server"`
	}{}
	config.Runner.Labels = append([]string(nil), connection.Labels...)
	config.Server.Connections = map[string]struct {
		URL   string `yaml:"url"`
		UUID  string `yaml:"uuid"`
		Token string `yaml:"token"`
	}{
		"farm": {
			URL:   connection.URL,
			UUID:  connection.UUID,
			Token: connection.Token,
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal runner config: %w", err)
	}
	return data, nil
}

func installScript(spec Install) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

curl --fail --location --silent --show-error --output "$tmp" %s
printf '%%s  %%s\n' %s "$tmp" | sha256sum --check --status
install -m 0755 "$tmp" %s
`, shellQuote(spec.DownloadURL), shellQuote(spec.SHA256), runnerPath)
}

func systemdUnit() string {
	return `[Unit]
Description=Forgejo Actions Runner
After=network-online.target
Wants=network-online.target
ConditionPathExists=` + configPath + `

[Service]
Type=simple
User=runner
Group=runner
WorkingDirectory=/home/runner
ExecStart=` + runnerPath + ` -c ` + configPath + ` one-job --wait
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
