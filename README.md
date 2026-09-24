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

## Get started

See [Run your first job](https://maveonair.github.io/farm/tutorial/first-job/) in the FARM documentation for installation instructions and first steps.

- [Documentation](https://maveonair.github.io/farm/)
- [Configuration](https://maveonair.github.io/farm/reference/configuration/)

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
