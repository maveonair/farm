# Setup

This guide installs FARM from source as a systemd service with a local Incus
server. It assumes a Linux host with systemd and standard user-management
tools. Commands may differ across distributions.

For every setting, see the [configuration guide](configuration.md).

## 1. Prepare Forgejo

Create a Forgejo token with Actions administration access for each configured
repository or organization. User pools apply to the token owner's repositories.
Global pools require a site-administrator token. FARM reads the token from a
file.

## 2. Prepare Incus

Create a dedicated project, managed network, and instance profile. Replace `default`
with the storage pool FARM should use:

```sh
incus project create farm
incus network create farmbr0 --project farm
incus profile create farm-instance --project farm
incus profile device add farm-instance root disk path=/ pool=default --project farm
incus profile device add farm-instance eth0 nic network=farmbr0 name=eth0 \
  --project farm
```

The profile must not expose host filesystems, the Incus socket, or host
devices.

For controller installation, the instance image must:

- be usable as an Incus instance
- run systemd and cloud-init
- include the Incus agent
- reach Forgejo and the runner download URL over HTTPS

FARM installs `ca-certificates`, `curl`, `git`, and `nodejs` through cloud-init.
Prepared images can instead use `runner_installation: image`; see
[Image-provided runner](#image-provided-runner).

Copy an instance image into the project:

```sh
incus image copy images:ubuntu/24.04/cloud local: --vm \
  --alias farm-ubuntu-24.04 --project farm
```

The command assumes that the standard `images:` remote exists. FARM can also
pull an image directly from a simplestreams server; see the
[image configuration](configuration.md#images).

## 3. Build and install FARM

Install Go 1.27.1 or newer, Node.js `^22.18.0` or `>=24.12.0`, and pnpm 12.4.2.
Build from the repository:

```sh
make build
```

Create the service account and directories:

```sh
sudo useradd --system --create-home --home-dir /var/lib/farm \
  --shell /usr/sbin/nologin farm
sudo install -d -o root -g farm -m 0750 /etc/farm
sudo install -d -o farm -g farm -m 0750 /var/lib/farm
```

Install FARM and its systemd unit:

```sh
sudo install -o root -g root -m 0755 dist/farm /usr/local/bin/farm
sudo install -o root -g farm -m 0640 farm.example.yaml /etc/farm/farm.yaml
sudo install -o root -g root -m 0644 packaging/farm.service \
  /etc/systemd/system/farm.service
```

Create the token file, then enter only the Forgejo token:

```sh
sudo install -o root -g farm -m 0640 /dev/null /etc/farm/forgejo-token
sudoedit /etc/farm/forgejo-token
```

## 4. Configure the runner

Download the release selected in `runner.download_url` and calculate its
digest. Match the binary architecture to the instance image.

```sh
curl --fail --location --output forgejo-runner \
  https://code.forgejo.org/forgejo/runner/releases/download/v13.0.0/forgejo-runner-13.0.0-linux-amd64
sha256sum forgejo-runner
rm forgejo-runner
```

Set the digest in `runner.sha256`. FARM verifies it inside every new instance.

Skip these fields when every pool uses an image-provided runner.

### Image-provided runner

Set `runner_installation: image` on a pool whose image already provides the
runner:

```yaml
instance:
  image: farm-nixos
  runner_installation: image
```

FARM does not inject cloud-init for this mode. It waits for the Incus agent,
writes `/etc/farm/runner.yml`, and restarts `forgejo-runner.service`.

A NixOS image can define the required service as follows:

```nix
{pkgs, ...}:
{
  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  environment.systemPackages = with pkgs; [
    forgejo-runner
  ];

  users.groups.runner = {};

  users.users.runner = {
    isSystemUser = true;
    group = "runner";
    home = "/var/lib/forgejo-runner";
    createHome = true;
  };

  systemd.tmpfiles.rules = [
    "d /etc/farm 0700 runner runner -"
  ];

  systemd.services.forgejo-runner = {
    description = "Forgejo Actions Runner";
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];

    unitConfig.ConditionPathExists = "/etc/farm/runner.yml";

    serviceConfig = {
      Type = "simple";
      User = "runner";
      Group = "runner";
      WorkingDirectory = "/var/lib/forgejo-runner";
      ExecStart = "${pkgs.forgejo-runner}/bin/forgejo-runner -c /etc/farm/runner.yml one-job --wait";
      Restart = "on-failure";
      RestartSec = 5;
    };

    path = with pkgs; [
      bash
      coreutils
      git
      nix
    ];

    environment.NIX_PATH = "nixpkgs=${pkgs.path}";
  };
}
```

Do not add `wantedBy` or bake `runner.yml` into the image. FARM supplies the
ephemeral credentials before starting the service.

## 5. Configure FARM

Edit `/etc/farm/farm.yaml`. Replace:

- the Forgejo URL and token path
- the runner URL and SHA-256 digest
- the Incus endpoint and project
- each pool's scope, labels, image, and profiles

### Local Incus

Use the Unix socket when FARM and Incus share a host:

```yaml
incus:
  endpoint: unix:///var/lib/incus/unix.socket
  project: farm
```

Grant the service account socket access. System installations commonly use the
`incus-admin` group:

```sh
sudo usermod -aG incus-admin farm
```

The group name is distribution-dependent. Socket access is generally
root-equivalent.

### Remote Incus

For stronger project isolation, connect over HTTPS with a client certificate
restricted to the FARM project:

```yaml
incus:
  endpoint: https://incus.example.com:8443
  project: farm
  client_certificate_file: /etc/farm/incus/client.crt
  client_key_file: /etc/farm/incus/client.key
  server_certificate_file: /etc/farm/incus/server.crt
```

The client certificate must be trusted by Incus. Its private key must be
readable by the service account and no other unprivileged users.

## 6. Start FARM

Validate the configuration, then start the service:

```sh
sudo -u farm /usr/local/bin/farm validate -config /etc/farm/farm.yaml
sudo systemctl daemon-reload
sudo systemctl enable --now farm
```

Validation checks the file structure and values. Starting FARM also checks
file access and connections to Forgejo and Incus.

Inspect the first reconciliation:

```sh
sudo systemctl status farm
sudo journalctl -u farm -f
curl --fail http://127.0.0.1:8080/healthz
```

Open `http://127.0.0.1:8080/` to view pools, instances, activity, and incidents.
The interface is read-only.

For a remote host, keep FARM on loopback and forward the port:

```sh
ssh -L 8080:127.0.0.1:8080 user@farm-host
```

Then open `http://127.0.0.1:8080/` locally.

## 7. Run a test job

Add a workflow to the repository configured by the example pool:

```yaml
name: FARM test
on: [push]

jobs:
  test:
    runs-on: [farm-ubuntu]
    steps:
      - run: uname -a
```

Push the workflow and watch FARM create the instance:

```sh
sudo journalctl -u farm -f
incus list --project farm
```

The interface should show the instance moving through bootstrapping, ready or
running, cleaning, and finished. After the job completes, confirm that its
runner registration and instance are removed. A short workflow may complete before
every intermediate state is observed.

## Operations

FARM exposes loopback-only endpoints configured by `controller.listen`:

- `GET /healthz`
- `GET /livez`
- `GET /metrics`

It writes structured logs to stderr. With the supplied systemd unit:

```sh
sudo journalctl -u farm -f
sudo journalctl -u farm -p warning
```

## Troubleshooting

### No instance is created

Check that the job scope matches the pool and that its first `runs-on` value is
the pool's first label. Confirm the token can administer Actions runners in
that scope.

### Incus returns permission denied

Confirm the `farm` user can open the configured socket. Restart FARM after
changing group membership. For HTTPS, check certificate trust and project
restriction.

### Instance startup times out

Confirm the image has systemd and a working Incus agent. Controller-installed
runners also require cloud-init. Check instance network, DNS, Forgejo access, and,
when applicable, runner download access.

### Runner installation fails

Confirm that `runner.download_url` matches the instance architecture and that
`runner.sha256` is the digest of that exact file.

For image-installed runners, confirm the image contract above and inspect
`forgejo-runner.service` inside the instance.

### Health returns 503

`healthz` returns 503 for an active reconciliation incident or stalled
operation. `livez` returns 503 only when the controller loop is stalled. Check
the web interface or service log for the failing pool, stage, and cause.
