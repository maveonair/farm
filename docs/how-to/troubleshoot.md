# Troubleshoot startup and jobs

Check the [operations UI and logs](monitor.md) for the failing pool, stage,
and cause. With systemd, use `sudo journalctl -u farm -f`.

## No VM is created

Check that the job scope matches the pool. The pool's first label must be the
first value in the job's `runs-on` list. Confirm that the Forgejo token can
administer Actions runners in that scope. Check whether the pool has paused
provisioning after repeated startup failures.

## Incus returns permission denied

Confirm the FARM user can open the configured socket. Restart FARM after
changing group membership. For HTTPS, check certificate trust and project
restriction.

## VM startup times out

Confirm the image has systemd and a working Incus agent. Controller-installed
runners also need cloud-init. Check VM network, DNS, Forgejo access, and runner
download access.

## Runner installation fails

Check that `runner.download_url` matches the VM architecture and that
`runner.sha256` is the digest of that exact binary. For an image-provided
runner, check the [image contract](image-runner.md) and inspect
`forgejo-runner.service` inside the VM.

## Health returns 503

Compare `/healthz` and `/livez` to tell a pool failure from a stalled
controller. See [health and liveness](../reference/http.md#health-and-liveness)
for the status codes and next steps.
