# Monitor FARM

FARM serves a read-only operations UI at `controller.listen`, which defaults
to `127.0.0.1:8080`. The same listener exposes:

- `GET /healthz`: reconciliation health
- `GET /livez`: controller progress
- `GET /metrics`: Prometheus metrics

For status codes and next steps, see [health and liveness](../reference/http.md#health-and-liveness).

For a systemd installation, follow the logs with:

```sh
sudo journalctl -u farm -f
sudo journalctl -u farm -p warning
```

## Remote access

For SSH access, keep the listener on loopback and forward the port:

```sh
ssh -L 8080:127.0.0.1:8080 user@farm-host
```

Open <http://127.0.0.1:8080/> locally.

To bind a private interface, set:

```yaml
controller:
  listen: 192.168.1.10:8080
```

Use `0.0.0.0:8080` for all IPv4 interfaces or `"[::]:8080"` for all IPv6
interfaces. This exposes the UI, API, health checks, and metrics on that address.
The listener has no authentication or TLS; anyone who can reach it can read
operational data. Never expose it directly to untrusted networks. Use an
authenticated HTTPS reverse proxy for remote access, and restrict the FARM
listener to the proxy with a loopback binding or firewall rules.
