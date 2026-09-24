# Pools and scaling

A pool defines which Forgejo jobs it serves, which VM it creates, and how
many instances it can run. Pools can serve a repository, organization, user,
or the whole Forgejo instance, subject to the token's permissions.

The pool's first label routes waiting jobs. It must be unique across pools
and first in the job's `runs-on` list; the job must request every pool label.

FARM targets enough ready or bootstrapping VMs for waiting jobs plus
`min_idle`. `max_instances` limits the pool's non-finished VMs, including
those running or cleaning up. `max_provisioning` limits simultaneous starts.

After `bootstrap_attempt_limit` unsuccessful starts, FARM pauses new VMs in
that pool. Existing runners continue. After `bootstrap_retry_interval`, it
starts one probe VM. A successful probe resumes provisioning; a failed probe
starts another delay. The pause persists across FARM restarts and does not
make `/healthz` fail: reconciliation is still enforcing the configured limit.

See [scaling fields](../reference/configuration.md#scaling) for defaults and
limits.
