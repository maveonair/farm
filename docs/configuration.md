# Configuration

FARM reads one YAML file passed with `-config`. Unknown fields, duplicate image
fields, invalid values, and multiple YAML documents are rejected.

Start with [`farm.example.yaml`](../farm.example.yaml):

```sh
cp farm.example.yaml farm.yaml
./dist/farm validate -config farm.yaml
```

Validation does not connect to Forgejo or Incus and does not read token or
certificate files.

## Minimal configuration

This is the smallest practical starting point. Replace every example value and
the runner checksum:

```yaml
controller:
  id: primary
  database: ./farm.db

forgejo:
  url: https://git.example.com
  token_file: ./forgejo-token

incus:
  endpoint: unix:///var/lib/incus/unix.socket
  project: farm

runner:
  download_url: https://code.forgejo.org/forgejo/runner/releases/download/v13.0.0/forgejo-runner-13.0.0-linux-amd64
  sha256: 0000000000000000000000000000000000000000000000000000000000000000

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
      max_instances: 8
```

Create `forgejo-token`, validate the file, then run FARM:

```sh
install -m 0600 /dev/null forgejo-token
${EDITOR:-vi} forgejo-token
./dist/farm validate -config farm.yaml
./dist/farm run -config farm.yaml
```

Open `http://127.0.0.1:8080/` after the first reconciliation. The following
sections describe every available setting.

## Controller

| Field                | Meaning                                                                                |
| -------------------- | -------------------------------------------------------------------------------------- |
| `id`                 | Lowercase controller identity used to mark owned instances and runners. Keep it stable.      |
| `database`           | SQLite path used to track runner and instance lifecycle state.                               |
| `reconcile_interval` | Interval between reconciliation cycles. Defaults to `5s`.                              |
| `cleanup_timeout`    | Cleanup and shutdown deadline. Defaults to `2m`.                                       |
| `listen`             | Address serving the UI, API, health checks, and metrics. Defaults to `127.0.0.1:8080`. |

Use each database and controller ID with only one FARM process.

FARM versions its SQLite schema and refuses unsupported or unversioned
databases. Back up or remove an incompatible database before starting FARM;
the service never drops or reinterprets tables automatically.

## Logging

`logging.level` accepts `debug`, `info`, `warn`, or `error`. The default is
`info`. `logging.format` accepts `json` or `text`; the default is `json`.

Debug logs include reconciliation and capacity decisions. Info logs record
lifecycle changes without logging every polling cycle.

Tokens, runner UUIDs, private keys, cloud-init, and runner configuration are
not logged.

## Forgejo

| Field                 | Meaning                                                           |
| --------------------- | ----------------------------------------------------------------- |
| `url`                 | Absolute HTTPS URL of the Forgejo server.                         |
| `token_file`          | File containing the API token. Surrounding whitespace is removed. |
| `ca_certificate_file` | Optional PEM CA certificate appended to the system trust store.   |
| `timeout`             | Timeout for each Forgejo API request. Defaults to `15s`.          |

Repository and organization pools require Actions administration access for
their scope. User pools use the user owning the token. Global pools require a
site-administrator token.

## Incus

### Unix socket

```yaml
incus:
  endpoint: unix:///var/lib/incus/unix.socket
  project: farm
```

The socket path must be absolute. Certificate fields cannot be used with a
Unix endpoint.

### HTTPS

```yaml
incus:
  endpoint: https://incus.example.com:8443
  project: farm
  client_certificate_file: /etc/farm/incus/client.crt
  client_key_file: /etc/farm/incus/client.key
  server_certificate_file: /etc/farm/incus/server.crt
```

HTTPS requires all three certificate files. `server_certificate_file` pins the
Incus server certificate.

## Runner

| Field          | Meaning                                                 |
| -------------- | ------------------------------------------------------- |
| `download_url` | Absolute HTTPS URL of the Forgejo Runner binary.        |
| `sha256`       | Lowercase or uppercase 64-character SHA-256 hex digest. |

These fields are required when any pool uses `runner_installation: controller`.
They may be omitted when every pool uses `runner_installation: image`. If either
field is present, both are validated.

Each runner starts in `one-job` mode and waits for one matching job.

## Pools

At least one pool is required. Pool names contain lowercase letters, numbers,
and internal hyphens, with a maximum length of 40 characters.

### Scope

Repository pool:

```yaml
scope:
  type: repository
  owner: example
  repository: project
```

Organization pool:

```yaml
scope:
  type: organization
  owner: example
```

User pool:

```yaml
scope:
  type: user
```

User scope applies to every repository owned by the user whose token FARM
uses.

Global pool:

```yaml
scope:
  type: global
```

Organization scope cannot set `repository`. User and global scopes cannot set
`owner` or `repository`.

### Labels

```yaml
labels:
  - farm-ubuntu
  - amd64
```

The first label routes queued jobs to the pool. It must be the first value in
the job's `runs-on` list and must be unique across pools. Every pool label must
appear in `runs-on`; jobs may request additional labels.

```yaml
runs-on: [farm-ubuntu, amd64]
```

Labels cannot be empty, duplicated within a pool, or contain colons.

### Images

Use an alias from the configured Incus project:

```yaml
image: farm-ubuntu-24.04
```

Or pull an image from an HTTPS server:

```yaml
image:
  alias: ubuntu/24.04/cloud
  server: https://images.linuxcontainers.org
  protocol: simplestreams
```

An image may use `fingerprint` instead of `alias`. A private image server may
set `certificate_file`. Incus CLI remote names such as `images:` are not
accepted because they exist only in client configuration.

### Runner installation

By default, FARM installs the runner and its dependencies through cloud-init:

```yaml
instance:
  image: farm-debian-13
  runner_installation: controller
```

An image can provide the runner instead:

```yaml
instance:
  image: farm-nixos-current
  runner_installation: image
```

Image installation does not inject FARM cloud-init. FARM waits for the Incus
agent, writes `/etc/farm/runner.yml`, and restarts
`forgejo-runner.service`. The image must provide:

- systemd and a working Incus agent
- the `runner` user and group
- a writable `/etc/farm` owned by `runner:runner` with mode `0700`
- `forgejo-runner.service`, the runner binary, and its dependencies
- no baked `/etc/farm/runner.yml`

The service must run the runner with
`-c /etc/farm/runner.yml one-job --wait`. It should use
`ConditionPathExists=/etc/farm/runner.yml` and must not start at boot. FARM
owns runtime configuration and service activation in both modes.

### Profiles and instance config

```yaml
profiles:
  - farm-instance
config:
  limits.cpu: "4"
  limits.memory: 8GiB
```

Profiles and config are passed to Incus when FARM creates the instance. Profiles are
trusted configuration; review their devices carefully.

### Scaling

| Field              | Meaning                                                                |
| ------------------ | ---------------------------------------------------------------------- |
| `min_idle`         | Number of ready instances kept available when no jobs wait. Defaults to `0`. |
| `max_instances`    | Maximum non-finished instances, including running and cleaning instances.          |
| `max_provisioning` | Maximum concurrent instance startups. Defaults to `2`.                       |
| `bootstrap_attempt_limit` | Unsuccessful instance startup attempts before provisioning pauses. Defaults to `5`. |
| `bootstrap_retry_interval` | Delay before one recovery probe. Defaults to `15m`.            |
| `startup_timeout`  | Registration and startup deadline. Defaults to `10m`.                  |
| `idle_timeout`     | Excess ready instance idle time. Defaults to `5m`.                           |
| `max_lifetime`     | Running instance lifetime. Defaults to `6h`.                           |

FARM targets enough ready or bootstrapping capacity for waiting jobs plus
`min_idle`. `max_instances` caps total pool size; `max_provisioning` caps
in-flight startup work.

After `bootstrap_attempt_limit` consecutive unsuccessful starts, FARM stops
creating instances for the pool. Existing runners continue operating. After
`bootstrap_retry_interval`, FARM starts one probe instance. A successful start clears
the failure count and resumes normal provisioning; another failure restarts the
delay. This state persists across FARM restarts.

An attempt is consumed when FARM starts provisioning. Interrupted attempts,
including controller shutdowns and canceled concurrent startups, count toward
the limit. This conservative accounting prevents restarts from bypassing the
limit.

A paused pool does not fail `/healthz`: reconciliation is working and enforcing
the configured safety limit. The pools API and interface report the pause, and
transition logs record when the circuit opens, probes, and recovers. The
provisioning incident resolves after FARM successfully enters the paused state.

For example:

```yaml
scaling:
  max_instances: 8
```

All timeouts must be positive Go duration values such as `30s`, `5m`, or `6h`.
