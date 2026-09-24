# Security boundaries

The operations UI and API have no authentication. `controller.listen` defaults
to loopback; use SSH forwarding or an authenticated reverse proxy for remote
access. Do not expose the listener directly to untrusted networks.

Give FARM a dedicated Incus project, network, and profiles. Profiles are
trusted configuration: host filesystems, devices, or the Incus socket passed
to runner VMs can weaken isolation. Local Unix socket access usually grants
full Incus administration. HTTPS with a project-restricted client certificate
provides stronger project isolation.

FARM injects runner credentials through the Incus agent. They are not stored
in SQLite or cloud-init configuration. Keep the Forgejo token and client key
in files readable only by the controller's service account.
