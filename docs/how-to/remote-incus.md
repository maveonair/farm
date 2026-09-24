# Connect to remote Incus

Use HTTPS and a client certificate restricted to the FARM project when the
controller does not run on the Incus host or needs project-level isolation.

Create the project, network, and VM profile on the Incus server as described
in [Run your first job](../tutorial/first-job.md#prepare-incus). Trust the
client certificate in Incus with access restricted to that project.

Set `incus` in `farm.yaml`:

```yaml
incus:
  endpoint: https://incus.example.com:8443
  project: farm
  client_certificate_file: /etc/farm/incus/client.crt
  client_key_file: /etc/farm/incus/client.key
  server_certificate_file: /etc/farm/incus/server.crt
```

All three certificate files are required. The server certificate file pins
the Incus server certificate. The service account must be able to read the
client key; other unprivileged users should not.

Run `farm validate -config /etc/farm/farm.yaml`, then restart the service.
Validation checks the configuration; startup tests the connection.
