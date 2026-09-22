package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const sha256Length = 64

const maxNameLength = 40

const (
	defaultReconcileInterval      = 5 * time.Second
	defaultCleanupTimeout         = 2 * time.Minute
	defaultListen                 = "127.0.0.1:8080"
	defaultForgejoTimeout         = 15 * time.Second
	defaultMaxProvisioning        = 2
	defaultBootstrapAttemptLimit  = 5
	defaultBootstrapRetryInterval = 15 * time.Minute
	defaultStartupTimeout         = 10 * time.Minute
	defaultIdleTimeout            = 5 * time.Minute
	defaultMaxLifetime            = 6 * time.Hour
)

type Config struct {
	Controller Controller `yaml:"controller"`
	Logging    Logging    `yaml:"logging"`
	Forgejo    Forgejo    `yaml:"forgejo"`
	Incus      Incus      `yaml:"incus"`
	Runner     Runner     `yaml:"runner"`
	Pools      []Pool     `yaml:"pools"`
}

type Logging struct {
	Level  LogLevel  `yaml:"level"`
	Format LogFormat `yaml:"format"`
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

type Controller struct {
	ID                string   `yaml:"id"`
	Database          string   `yaml:"database"`
	ReconcileInterval Duration `yaml:"reconcile_interval"`
	CleanupTimeout    Duration `yaml:"cleanup_timeout"`
	Listen            string   `yaml:"listen"`
}

type Forgejo struct {
	URL               string   `yaml:"url"`
	TokenFile         string   `yaml:"token_file"`
	CACertificateFile string   `yaml:"ca_certificate_file"`
	Timeout           Duration `yaml:"timeout"`
}

type Incus struct {
	Endpoint       string `yaml:"endpoint"`
	Project        string `yaml:"project"`
	ClientCertFile string `yaml:"client_certificate_file"`
	ClientKeyFile  string `yaml:"client_key_file"`
	ServerCertFile string `yaml:"server_certificate_file"`
}

type Runner struct {
	DownloadURL string `yaml:"download_url"`
	SHA256      string `yaml:"sha256"`
}

type RunnerInstallMode string

const (
	RunnerInstallController RunnerInstallMode = "controller"
	RunnerInstallImage      RunnerInstallMode = "image"
)

type Pool struct {
	Name     string   `yaml:"name"`
	Scope    Scope    `yaml:"scope"`
	Labels   []string `yaml:"labels"`
	Instance Instance `yaml:"instance"`
	Scaling  Scaling  `yaml:"scaling"`
}

type Scope struct {
	Type       ScopeType `yaml:"type"`
	Owner      string    `yaml:"owner"`
	Repository string    `yaml:"repository"`
}

type ScopeType string

const (
	ScopeRepository   ScopeType = "repository"
	ScopeOrganization ScopeType = "organization"
	ScopeUser         ScopeType = "user"
	ScopeGlobal       ScopeType = "global"
)

// Instance intentionally has no type field. FARM only creates VMs.
type Instance struct {
	Image         Image             `yaml:"image"`
	RunnerInstall RunnerInstallMode `yaml:"runner_installation"`
	Profiles      []string          `yaml:"profiles"`
	Config        map[string]string `yaml:"config"`
}

type Image struct {
	Alias           string `yaml:"alias"`
	Fingerprint     string `yaml:"fingerprint"`
	Server          string `yaml:"server"`
	Protocol        string `yaml:"protocol"`
	CertificateFile string `yaml:"certificate_file"`
}

func (i *Image) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		i.Alias = node.Value
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return errors.New("image must be a string or mapping")
	}

	seen := make(map[string]struct{}, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index].Value
		value := node.Content[index+1]
		if _, found := seen[key]; found {
			return fmt.Errorf("duplicate image field %q", key)
		}
		seen[key] = struct{}{}

		var target *string
		switch key {
		case "alias":
			target = &i.Alias
		case "fingerprint":
			target = &i.Fingerprint
		case "server":
			target = &i.Server
		case "protocol":
			target = &i.Protocol
		case "certificate_file":
			target = &i.CertificateFile
		default:
			return fmt.Errorf("unknown image field %q", key)
		}
		if err := value.Decode(target); err != nil {
			return fmt.Errorf("decode image field %q: %w", key, err)
		}
	}
	return nil
}

type Scaling struct {
	MinIdle                int      `yaml:"min_idle"`
	MaxInstances           int      `yaml:"max_instances"`
	MaxProvisioning        int      `yaml:"max_provisioning"`
	BootstrapAttemptLimit  int      `yaml:"bootstrap_attempt_limit"`
	BootstrapRetryInterval Duration `yaml:"bootstrap_retry_interval"`
	StartupTimeout         Duration `yaml:"startup_timeout"`
	IdleTimeout            Duration `yaml:"idle_timeout"`
	MaxLifetime            Duration `yaml:"max_lifetime"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	value, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", node.Value, err)
	}

	d.Duration = value
	return nil
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("decode config: multiple YAML documents")
		}
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Controller.ReconcileInterval.Duration == 0 {
		c.Controller.ReconcileInterval.Duration = defaultReconcileInterval
	}
	if c.Controller.CleanupTimeout.Duration == 0 {
		c.Controller.CleanupTimeout.Duration = defaultCleanupTimeout
	}
	if c.Controller.Listen == "" {
		c.Controller.Listen = defaultListen
	}
	if c.Logging.Level == "" {
		c.Logging.Level = LogLevelInfo
	}
	if c.Logging.Format == "" {
		c.Logging.Format = LogFormatJSON
	}
	if c.Forgejo.Timeout.Duration == 0 {
		c.Forgejo.Timeout.Duration = defaultForgejoTimeout
	}
	for index := range c.Pools {
		if c.Pools[index].Instance.RunnerInstall == "" {
			c.Pools[index].Instance.RunnerInstall = RunnerInstallController
		}

		scaling := &c.Pools[index].Scaling
		if scaling.MaxProvisioning == 0 {
			scaling.MaxProvisioning = defaultMaxProvisioning
		}
		if scaling.BootstrapAttemptLimit == 0 {
			scaling.BootstrapAttemptLimit = defaultBootstrapAttemptLimit
		}
		if scaling.BootstrapRetryInterval.Duration == 0 {
			scaling.BootstrapRetryInterval.Duration = defaultBootstrapRetryInterval
		}
		if scaling.StartupTimeout.Duration == 0 {
			scaling.StartupTimeout.Duration = defaultStartupTimeout
		}
		if scaling.IdleTimeout.Duration == 0 {
			scaling.IdleTimeout.Duration = defaultIdleTimeout
		}
		if scaling.MaxLifetime.Duration == 0 {
			scaling.MaxLifetime.Duration = defaultMaxLifetime
		}
	}
}

func (c Config) Validate() error {
	if !validName(c.Controller.ID, maxNameLength) {
		return errors.New("controller.id must contain lowercase letters, numbers, or internal hyphens")
	}
	if c.Controller.Database == "" {
		return errors.New("controller.database is required")
	}
	if c.Controller.ReconcileInterval.Duration <= 0 {
		return errors.New("controller.reconcile_interval must be positive")
	}
	if c.Controller.CleanupTimeout.Duration <= 0 {
		return errors.New("controller.cleanup_timeout must be positive")
	}
	if c.Controller.Listen == "" {
		return errors.New("controller.listen is required")
	}
	if err := c.Logging.validate(); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(c.Controller.Listen)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return errors.New("controller.listen must use a loopback IP and port")
	}
	if err := validSecureURL("forgejo.url", c.Forgejo.URL); err != nil {
		return err
	}
	if c.Forgejo.TokenFile == "" {
		return errors.New("forgejo.token_file is required")
	}
	if c.Forgejo.Timeout.Duration <= 0 {
		return errors.New("forgejo.timeout must be positive")
	}
	if c.Incus.Endpoint == "" {
		return errors.New("incus.endpoint is required")
	}
	if err := c.Incus.validate(); err != nil {
		return err
	}
	if c.Incus.Project == "" {
		return errors.New("incus.project is required")
	}
	if len(c.Pools) == 0 {
		return errors.New("at least one pool is required")
	}

	names := make(map[string]struct{}, len(c.Pools))
	labels := make(map[string]string, len(c.Pools))
	needsRunner := false
	for i, pool := range c.Pools {
		if err := pool.validate(); err != nil {
			return fmt.Errorf("pool %d: %w", i, err)
		}
		if _, found := names[pool.Name]; found {
			return fmt.Errorf("pool %q: duplicate name", pool.Name)
		}
		names[pool.Name] = struct{}{}

		primary := pool.Labels[0]
		if owner, found := labels[primary]; found {
			return fmt.Errorf("pool %q: primary label %q is also used by pool %q", pool.Name, primary, owner)
		}
		labels[primary] = pool.Name
		if pool.Instance.RunnerInstall == RunnerInstallController {
			needsRunner = true
		}
	}

	if needsRunner || c.Runner.DownloadURL != "" || c.Runner.SHA256 != "" {
		if err := c.Runner.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (l Logging) validate() error {
	switch l.Level {
	case "", LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		return errors.New("logging.level must be debug, info, warn, or error")
	}
	switch l.Format {
	case "", LogFormatJSON, LogFormatText:
	default:
		return errors.New("logging.format must be json or text")
	}
	return nil
}

func (r Runner) validate() error {
	if err := validSecureURL("runner.download_url", r.DownloadURL); err != nil {
		return err
	}
	checksum, err := hex.DecodeString(r.SHA256)
	if err != nil || len(checksum) != sha256Length/2 {
		return errors.New("runner.sha256 must be a SHA-256 hex digest")
	}
	return nil
}

func (i Incus) validate() error {
	endpoint, err := url.ParseRequestURI(i.Endpoint)
	if err != nil {
		return errors.New("incus.endpoint must be an absolute HTTPS or Unix URL")
	}

	switch endpoint.Scheme {
	case "https":
		if err := validSecureURL("incus.endpoint", i.Endpoint); err != nil {
			return err
		}
		if i.ClientCertFile == "" || i.ClientKeyFile == "" || i.ServerCertFile == "" {
			return errors.New("missing Incus client certificate, key, or server certificate")
		}
	case "unix":
		if endpoint.Host != "" || endpoint.RawQuery != "" || !strings.HasPrefix(endpoint.Path, "/") {
			return errors.New("incus.endpoint Unix socket path must be absolute")
		}
		if i.ClientCertFile != "" || i.ClientKeyFile != "" || i.ServerCertFile != "" {
			return errors.New("incus certificate files cannot be used with a Unix socket")
		}
	default:
		return errors.New("incus.endpoint must use HTTPS or Unix")
	}
	return nil
}

func (p Pool) validate() error {
	if !validName(p.Name, maxNameLength) {
		return errors.New("name must contain lowercase letters, numbers, or internal hyphens")
	}
	switch p.Scope.Type {
	case ScopeRepository:
		if p.Scope.Owner == "" || p.Scope.Repository == "" {
			return errors.New("repository scope requires owner and repository")
		}
	case ScopeOrganization:
		if p.Scope.Owner == "" || p.Scope.Repository != "" {
			return errors.New("organization scope requires owner and no repository")
		}
	case ScopeUser:
		if p.Scope.Owner != "" || p.Scope.Repository != "" {
			return errors.New("user scope cannot define owner or repository")
		}
	case ScopeGlobal:
		if p.Scope.Owner != "" || p.Scope.Repository != "" {
			return errors.New("global scope cannot define owner or repository")
		}
	default:
		return errors.New("scope.type must be repository, organization, user, or global")
	}
	if len(p.Labels) == 0 {
		return errors.New("at least one label is required")
	}
	seenLabels := make(map[string]struct{}, len(p.Labels))
	for _, label := range p.Labels {
		if strings.TrimSpace(label) == "" || strings.Contains(label, ":") {
			return errors.New("labels must be non-empty and cannot contain colons")
		}
		if _, found := seenLabels[label]; found {
			return fmt.Errorf("duplicate label %q", label)
		}
		seenLabels[label] = struct{}{}
	}
	switch p.Instance.RunnerInstall {
	case RunnerInstallController, RunnerInstallImage:
	default:
		return errors.New("instance.runner_installation must be controller or image")
	}
	if p.Instance.Image.Alias == "" && p.Instance.Image.Fingerprint == "" {
		return errors.New("instance.image is required")
	}
	if p.Instance.Image.Alias != "" && p.Instance.Image.Fingerprint != "" {
		return errors.New("instance.image cannot define both alias and fingerprint")
	}
	if strings.Contains(p.Instance.Image.Alias, ":") {
		return errors.New("instance.image must be a local Incus alias or fingerprint")
	}
	if p.Instance.Image.Server != "" {
		if err := validSecureURL("instance.image.server", p.Instance.Image.Server); err != nil {
			return err
		}
		if p.Instance.Image.Protocol == "" {
			return errors.New("instance.image.protocol is required for a remote image")
		}
	} else if p.Instance.Image.Protocol != "" || p.Instance.Image.CertificateFile != "" {
		return errors.New("instance.image.server is required for remote image options")
	}
	if p.Scaling.MinIdle < 0 {
		return errors.New("scaling.min_idle cannot be negative")
	}
	if p.Scaling.MaxInstances < 1 {
		return errors.New("scaling.max_instances must be positive")
	}
	if p.Scaling.MinIdle > p.Scaling.MaxInstances {
		return errors.New("scaling.min_idle exceeds scaling.max_instances")
	}
	if p.Scaling.MaxProvisioning < 1 {
		return errors.New("scaling.max_provisioning must be positive")
	}
	if p.Scaling.BootstrapAttemptLimit < 1 {
		return errors.New("scaling.bootstrap_attempt_limit must be positive")
	}
	if p.Scaling.BootstrapRetryInterval.Duration <= 0 {
		return errors.New("scaling.bootstrap_retry_interval must be positive")
	}
	if p.Scaling.StartupTimeout.Duration <= 0 {
		return errors.New("scaling.startup_timeout must be positive")
	}
	if p.Scaling.IdleTimeout.Duration <= 0 {
		return errors.New("scaling.idle_timeout must be positive")
	}
	if p.Scaling.MaxLifetime.Duration <= 0 {
		return errors.New("scaling.max_lifetime must be positive")
	}

	return nil
}

func validName(value string, maxLength int) bool {
	if value == "" || len(value) > maxLength || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func validSecureURL(name, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute HTTPS URL", name)
	}
	return nil
}
