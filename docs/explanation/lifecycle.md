# VM and runner lifecycle

FARM watches Forgejo for waiting jobs and creates Incus VMs to provide
capacity. Each VM registers one ephemeral runner. That runner accepts at most
one job; FARM then removes the runner registration and VM.

```mermaid
flowchart LR
    A[Waiting job] --> B[VM startup]
    B --> C[Runner ready]
    C --> D[One job]
    D --> E[Runner and VM removed]
```

With `runner_installation: controller`, FARM configures the VM through
cloud-init. With `runner_installation: image`, the image supplies the runner
service and FARM starts it after writing the runtime configuration. Both modes
need the Incus agent to deliver credentials to the VM.

FARM tracks lifecycle state in SQLite. Keep each controller ID and database
assigned to one running FARM process. Reconciliation detects and cleans up
stale FARM runners and VMs after interruptions.

See [pool matching and scaling](pools.md) for how jobs trigger new VMs.
