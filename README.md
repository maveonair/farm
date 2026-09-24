# FARM

FARM runs Forgejo Actions jobs in disposable Incus virtual machines. Each VM
registers one ephemeral Forgejo Runner, accepts at most one job, then is
deleted with its runner registration.

```text
queued job → FARM → Incus VM → one-job runner → cleanup
```

FARM provides:

- repository, organization, user, and global runner pools
- queue-based scaling with optional idle capacity
- startup, concurrency, idle, and lifetime limits
- cleanup of stale FARM runners and VMs
- health and Prometheus metrics endpoints
- an embedded, read-only operations interface

FARM manages virtual machines only. It never creates or manages Incus
containers.

[Documentation](https://maveonair.github.io/farm/) ·
[First job](https://maveonair.github.io/farm/tutorial/first-job/) ·
[Configuration](https://maveonair.github.io/farm/reference/configuration/)

## Quick start

You need:

- Forgejo 15 LTS or newer and an Actions administration token
- Incus with a dedicated project, VM profile, and managed network
- A VM image with the Incus agent and either cloud-init or a prepared runner
- Go 1.27.1 or newer
- Node.js `^22.18.0` or `>=24.12.0`, and pnpm 12.4.2

The [first-job tutorial](https://maveonair.github.io/farm/tutorial/first-job/) covers
the Forgejo and Incus preparation.

Build FARM and copy the example configuration:

```sh
make build
cp farm.example.yaml farm.yaml
```

For a local test, change `controller.database` and `forgejo.token_file` in
`farm.yaml` to writable paths such as `./farm.db` and `./forgejo-token`. Then
configure:

- the Forgejo URL and token
- the runner download URL and SHA-256 digest for controller-installed runners
- the Incus endpoint and project
- the pool scope, label, image, and profile

Create the token file without placing the token in the YAML file:

```sh
install -m 0600 /dev/null forgejo-token
${EDITOR:-vi} forgejo-token
```

Validate and start FARM:

```sh
./dist/farm validate -config farm.yaml
./dist/farm run -config farm.yaml
```

Open <http://127.0.0.1:8080/>. The read-only interface shows pool capacity,
instances, lifecycle activity, and reconciliation incidents.

Run a workflow using the pool's first label:

```yaml
jobs:
  build:
    runs-on: [farm-ubuntu]
    steps:
      - run: uname -a
```

FARM creates a VM, runs at most one job, and removes the VM and ephemeral runner
afterward.

See the [systemd installation guide](https://maveonair.github.io/farm/how-to/install-systemd/)
and [configuration reference](https://maveonair.github.io/farm/reference/configuration/).

The interface is served from `controller.listen`, which defaults to loopback.
Use SSH forwarding or an authenticated reverse proxy for remote access.

## Security

The operations UI and API do not provide authentication. Keep
`controller.listen` on loopback and never expose it directly to untrusted
networks. Use SSH forwarding or an authenticated reverse proxy for remote
access.

Use a dedicated Incus project, network, and profiles. Do not expose the Incus
socket, host filesystems, or host devices to runner VMs. Profiles are trusted
configuration and can weaken VM isolation.

Unix socket access usually grants full Incus administration and is effectively
root-equivalent. For stronger project isolation, use HTTPS with a restricted
client certificate.

Runner credentials are injected through the Incus agent. They are not stored
in SQLite or cloud-init configuration.

## Contributing

Read the [contribution guidelines](CONTRIBUTING.md) before opening an issue or
pull request.

## License

Licensed under the [Apache License 2.0](LICENSE).
