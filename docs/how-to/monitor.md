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

For remote access, keep the listener on loopback and forward the port:

```sh
ssh -L 8080:127.0.0.1:8080 user@farm-host
```

Open <http://127.0.0.1:8080/> locally. The UI and API have no authentication;
use an authenticated reverse proxy if you need direct remote access.
