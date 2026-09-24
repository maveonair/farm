# Run your first job

This tutorial runs FARM against a local Incus server and a Forgejo repository.
You will create a VM, run one workflow job, and confirm that FARM removes the
VM and runner afterward. Use a Linux host with Incus installed.

You need:

- Forgejo 15 LTS or newer, with Actions enabled and a token that can administer
  Actions runners for the repository.
- An Incus storage pool and a host that can run VMs.
- Go 1.27.1 or newer, Node.js `^22.18.0` or `>=24.12.0`, and pnpm 12.4.2.
- A VM image with systemd, cloud-init, the Incus agent, and HTTPS access to
  Forgejo and the runner download URL.

## Prepare Incus

Create a project, network, and VM profile. Replace `default` with your storage
pool name:

```sh
incus project create farm
incus network create farmbr0 --project farm
incus profile create farm-vm --project farm
incus profile device add farm-vm root disk path=/ pool=default --project farm
incus profile device add farm-vm eth0 nic network=farmbr0 name=eth0 \
  --project farm
incus image copy images:ubuntu/24.04/cloud local: --vm \
  --alias farm-ubuntu-24.04 --project farm
```

The image command assumes the standard `images:` remote exists. The profile
must not expose host filesystems, devices, or the Incus socket.

## Build and configure FARM

From the repository root:

```sh
make build
cp farm.example.yaml farm.yaml
install -m 0600 /dev/null forgejo-token
${EDITOR:-vi} forgejo-token
```

Put only the Forgejo token in `forgejo-token`. Edit `farm.yaml`:

- Set `controller.database` to `./farm.db` and `forgejo.token_file` to
  `./forgejo-token`.
- Set `forgejo.url` to your Forgejo HTTPS URL.
- Set the pool's `scope.owner` and `scope.repository` to your repository.
- Keep `incus.project: farm`, `instance.image: farm-ubuntu-24.04`, and
  `instance.profiles: [farm-vm]`.

Download a runner binary for the VM's architecture and compute its digest:

```sh
curl --fail --location --output forgejo-runner \
  https://code.forgejo.org/forgejo/runner/releases/download/v13.0.0/forgejo-runner-13.0.0-linux-amd64
sha256sum forgejo-runner
rm forgejo-runner
```

Set `runner.sha256` to that digest. `runner.download_url` must point to the
same binary. FARM verifies the digest inside every new VM.

## Start FARM

Your user needs access to the Incus socket. This usually grants full Incus
administration; use a dedicated host or [remote Incus with a restricted client
certificate](../how-to/remote-incus.md) when appropriate.
On installations with an `incus-admin` group, add your user to that group and
start a new login session before running FARM.

```sh
./dist/farm validate -config farm.yaml
./dist/farm run -config farm.yaml
```

Validation checks configuration values. Startup also checks access to files,
Forgejo, and Incus. Keep the process running while you submit a job. Open
<http://127.0.0.1:8080/> to see pool and instance status.

## Run a workflow

Add this workflow to the configured repository and push it:

```yaml
name: FARM test
on: [push]

jobs:
  test:
    runs-on: [farm-ubuntu]
    steps:
      - run: uname -a
```

The first `runs-on` label must match the pool's first label. Watch the UI or
run `incus list --project farm`. FARM should create a VM, run the job, and
remove the VM and runner registration. A short job may finish before you see
every intermediate state.

Next: [install FARM as a service](../how-to/install-systemd.md) or
[adjust pool settings](../reference/configuration.md#pools).
