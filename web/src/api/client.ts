export type State = 'bootstrapping' | 'ready' | 'running' | 'cleaning' | 'finished'
export type Result = 'succeeded' | 'failed'
export type Reason =
  | 'job_completed'
  | 'idle_timeout'
  | 'max_lifetime'
  | 'bootstrap_failed'
  | 'runner_offline'
  | 'instance_missing'
  | 'controller_recovery'
export type Stage =
  | 'controller'
  | 'load_instances'
  | 'collect_orphans'
  | 'fetch_jobs'
  | 'scale_down'
  | 'register_runner'
  | 'create_instance'
  | 'wait_cloud_init'
  | 'push_runner_config'
  | 'wait_runner'
  | 'observe_runner'
  | 'delete_runner'
  | 'delete_instance'
  | 'persist_state'

export interface Summary {
  controller: string
  healthy: boolean
  last_success_at: string
  reconcile_errors: number
  waiting: number
  counts: Partial<Record<State, number>>
  generated_at: string
  reconciliation: {
    phase: 'idle' | 'running'
    condition: 'starting' | 'healthy' | 'degraded' | 'stalled'
    last_outcome: 'none' | 'succeeded' | 'failed'
    started_at?: string
    finished_at?: string
    last_success_at?: string
    deadline_at?: string
    completed_pools: number
    total_pools: number
    active_failures: number
  }
}

export interface Capacity {
  waiting: number
  needed: number
  bootstrapping: number
  ready: number
  running: number
  cleaning: number
  maximum: number
}

export interface Pool {
  name: string
  scope: { type: string; owner?: string; repository?: string }
  labels: string[]
  image: string
  min_idle: number
  max_instances: number
  max_provisioning: number
  startup_timeout: string
  idle_timeout: string
  max_lifetime: string
  capacity: Capacity
  runtime: {
    state?: 'running' | 'succeeded' | 'failed'
    run_id?: string
    observed_at?: string
    started_at?: string
    finished_at?: string
    last_success_at?: string
    stage?: Stage
    stage_started_at?: string
    stage_deadline_at?: string
  }
  incident?: Incident
  observation_stale: boolean
}

export interface Incident {
  id: number
  pool: string
  run_id: string
  stage: string
  code: string
  message: string
  instance_id?: string
  instance_name?: string
  first_seen_at: string
  last_seen_at: string
  occurrences: number
  resolved_at?: string
}

export interface Instance {
  id: string
  name: string
  pool: string
  state: State
  stage?: Stage
  result?: Result
  reason?: Reason
  runner_id: number
  error: string
  retry_count: number
  retry_at?: string
  created_at: string
  updated_at: string
  state_changed_at: string
  stage_changed_at?: string
  finished_at?: string
}

export interface Event {
  id: number
  instance_id: string
  pool: string
  kind: string
  from_state?: State
  to_state?: State
  stage?: Stage
  result?: Result
  reason?: Reason
  message?: string
  created_at: string
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal, headers: { Accept: 'application/json' } })
  if (!response.ok) {
    const body = (await response.json().catch(() => ({ error: response.statusText }))) as {
      error?: string
    }
    throw new Error(body.error || `Request failed: ${response.status}`)
  }
  return response.json() as Promise<T>
}

export const api = {
  summary: (signal?: AbortSignal) => get<Summary>('/api/v1/summary', signal),
  pools: async (signal?: AbortSignal) =>
    (await get<{ pools: Pool[] }>('/api/v1/pools', signal)).pools,
  pool: (name: string, signal?: AbortSignal) =>
    get<Pool>(`/api/v1/pools/${encodeURIComponent(name)}`, signal),
  instances: async (params = new URLSearchParams(), signal?: AbortSignal) =>
    (await get<{ instances: Instance[] }>(`/api/v1/instances?${params}`, signal)).instances,
  instance: (id: string, signal?: AbortSignal) =>
    get<Instance>(`/api/v1/instances/${encodeURIComponent(id)}`, signal),
  events: async (params = new URLSearchParams(), signal?: AbortSignal) =>
    (await get<{ events: Event[] }>(`/api/v1/events?${params}`, signal)).events,
  instanceEvents: async (id: string, signal?: AbortSignal) =>
    (await get<{ events: Event[] }>(`/api/v1/instances/${encodeURIComponent(id)}/events`, signal))
      .events,
  incidents: async (params = new URLSearchParams(), signal?: AbortSignal) =>
    (await get<{ incidents: Incident[] }>(`/api/v1/incidents?${params}`, signal)).incidents,
}
