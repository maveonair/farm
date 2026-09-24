# HTTP endpoints

The listener defaults to `127.0.0.1:8080`. The UI and API are read-only and
have no authentication. Keep the listener private.

| Endpoint | Purpose |
| --- | --- |
| `GET /` | Operations UI |
| `GET /healthz` | Whether reconciliation is working |
| `GET /livez` | Whether the controller is progressing |
| `GET /metrics` | Prometheus metrics |
| `GET /api/v1/summary` | Controller summary |
| `GET /api/v1/pools` | Pools |
| `GET /api/v1/pools/{name}` | One pool |
| `GET /api/v1/instances` | Instances |
| `GET /api/v1/instances/{id}` | One instance |
| `GET /api/v1/instances/{id}/events` | Instance events |
| `GET /api/v1/events` | Lifecycle events |
| `GET /api/v1/incidents` | Incidents |
| `GET /api/v1/incidents/{id}` | One incident |

## Health and liveness

Check both endpoints to distinguish failed work from a stalled controller:

```sh
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/livez
```

| Condition | `/healthz` | `/livez` | What to do |
| --- | --- | --- | --- |
| Starting or reconciling normally | 200 | 200 | No action. A first successful cycle is not required. |
| A pool has an active reconciliation failure | 503 | 200 | Check the affected pool in the UI and inspect the service logs. |
| Reconciliation misses its deadline or the next cycle is overdue | 503 | 503 | Check the current stage and logs for stalled work. |

`/healthz` reports whether reconciliation is working. `/livez` reports
whether the controller is still progressing, even if a pool is failing. A
200 response from either endpoint does not prove that a workflow job can run
to completion.
