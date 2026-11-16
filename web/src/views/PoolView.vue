<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import IncidentPanel from '@/components/IncidentPanel.vue'
import InstanceTable from '@/components/InstanceTable.vue'
import PaginationControls from '@/components/PaginationControls.vue'
import ViewState from '@/components/ViewState.vue'
import { exact, relative, scopeName } from '@/lib/format'
import { lifecycleLabel } from '@/lib/lifecycle'
import { pageFrom } from '@/lib/pagination'
import { useInstancesQuery } from '@/queries/instances'
import { POOL_PAGE_SIZE, queryError } from '@/queries/options'
import { usePoolQuery } from '@/queries/pools'

const route = useRoute()
const name = computed(() => String(route.params.name ?? ''))
const page = computed(() => pageFrom(route.query.page))
const poolQuery = usePoolQuery(name)
const instancesQuery = useInstancesQuery(name, '', page, POOL_PAGE_SIZE)

const pool = poolQuery.data
const instances = computed(() => instancesQuery.data.value?.items ?? [])
const pagination = computed(() => instancesQuery.data.value?.pagination)
const loading = computed(() => poolQuery.isPending.value || instancesQuery.isPending.value)
const error = computed(
  () =>
    queryError(poolQuery.error.value, poolQuery.data.value, 'Unable to load pool') ||
    queryError(instancesQuery.error.value, instancesQuery.data.value, 'Unable to load pool'),
)
</script>

<template>
  <ViewState :loading :error>
    <template #content>
      <template v-if="pool">
        <nav class="breadcrumbs" aria-label="Breadcrumb">
          <RouterLink to="/pools">Pools</RouterLink>
          <span aria-hidden="true">/</span>
          <span class="truncate" aria-current="page">{{ pool.name }}</span>
        </nav>
        <div class="page-header">
          <div class="min-w-0">
            <h1 class="page-title font-mono">{{ pool.name }}</h1>
            <p class="page-description">Scope: {{ scopeName(pool.scope) }}</p>
          </div>
        </div>
        <IncidentPanel v-if="pool.incident" :incident="pool.incident" class="mb-6" />
        <div
          v-if="pool.bootstrap.paused"
          class="notice mb-5 border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-400/40 dark:bg-amber-400/10 dark:text-amber-200"
        >
          <strong>Provisioning paused.</strong> {{ pool.bootstrap.attempts }} unsuccessful starts
          reached the configured limit. The next probe is {{ relative(pool.bootstrap.retry_at) }}.
        </div>
        <div
          v-if="pool.observation_stale"
          class="notice mb-5 border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-400/40 dark:bg-amber-400/10 dark:text-amber-200"
        >
          <strong>Queue information may be stale.</strong> Forgejo was last observed
          {{ relative(pool.runtime.observed_at) }}.
        </div>
        <section aria-label="Pool capacity" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <div
            v-for="item in [
              ['Waiting', pool.capacity.waiting],
              ['Running', pool.capacity.running],
              ['Ready', pool.capacity.ready],
              ['Bootstrapping', pool.capacity.bootstrapping],
              ['Maximum', pool.capacity.maximum],
            ]"
            :key="String(item[0])"
            class="metric"
          >
            <p class="metric-label">{{ item[0] }}</p>
            <p class="metric-value">{{ item[1] }}</p>
          </div>
        </section>
        <section class="panel mt-5 overflow-hidden">
          <div class="section-header"><h2 class="font-semibold">Configuration</h2></div>
          <dl class="grid gap-4 p-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Image</dt>
              <dd class="mt-1 break-all font-mono text-xs">{{ pool.image }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Labels</dt>
              <dd v-if="pool.labels.length" class="mt-1 flex flex-wrap gap-2">
                <span
                  v-for="label in pool.labels"
                  :key="label"
                  class="badge break-all bg-zinc-100 font-mono text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                  >{{ label }}</span
                >
              </dd>
              <dd v-else class="mt-1">None</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Minimum idle</dt>
              <dd class="mt-1">{{ pool.min_idle }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Max provisioning</dt>
              <dd class="mt-1">{{ pool.max_provisioning }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Bootstrap attempt limit</dt>
              <dd class="mt-1">{{ pool.bootstrap.attempt_limit }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Startup timeout</dt>
              <dd class="mt-1">{{ pool.startup_timeout }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Idle timeout</dt>
              <dd class="mt-1">{{ pool.idle_timeout }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Max lifetime</dt>
              <dd class="mt-1">{{ pool.max_lifetime }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Reconciliation</dt>
              <dd class="mt-1 capitalize">{{ pool.runtime.state || 'not started' }}</dd>
            </div>
            <div v-if="pool.runtime.stage">
              <dt class="text-zinc-500 dark:text-zinc-400">Current stage</dt>
              <dd class="mt-1 capitalize">{{ lifecycleLabel(pool.runtime.stage) }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Queue observed</dt>
              <dd class="mt-1">{{ relative(pool.runtime.observed_at) }}</dd>
            </div>
            <div>
              <dt class="text-zinc-500 dark:text-zinc-400">Last success</dt>
              <dd class="mt-1">{{ exact(pool.runtime.last_success_at) }}</dd>
            </div>
          </dl>
        </section>
        <div class="mb-3 mt-5 flex items-center justify-between">
          <h2 class="text-sm font-semibold">Instances</h2>
          <span class="text-xs text-zinc-500 tabular-nums dark:text-zinc-400"
            >Showing {{ instances.length }}</span
          >
        </div>
        <InstanceTable v-if="instances.length" :instances />
        <div v-else class="panel p-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
          No instances recorded.
        </div>
        <PaginationControls v-if="pagination" :pagination />
      </template>
    </template>
  </ViewState>
</template>
