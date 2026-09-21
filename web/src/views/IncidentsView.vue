<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { IncidentStatus } from '@/api/client'
import PaginationControls from '@/components/PaginationControls.vue'
import ViewState from '@/components/ViewState.vue'
import { exact, instanceLabel, relative } from '@/lib/format'
import { lifecycleLabel } from '@/lib/lifecycle'
import { pageFrom } from '@/lib/pagination'
import { useIncidentsQuery } from '@/queries/activity'
import { queryError } from '@/queries/options'
import { usePoolOptionsQuery } from '@/queries/pools'

type IncidentFilter = IncidentStatus | 'all'

const route = useRoute()
const router = useRouter()
const pool = computed({
  get: () => (typeof route.query.pool === 'string' ? route.query.pool : ''),
  set: (value: string) => {
    const query = { ...route.query }
    delete query.page
    if (value) {
      query.pool = value
    } else {
      delete query.pool
    }

    void router.replace({ query })
  },
})
const status = computed<IncidentFilter>({
  get: () => {
    if (route.query.status === 'resolved' || route.query.status === 'all') {
      return route.query.status
    }

    return 'open'
  },
  set: (value) => {
    const query = { ...route.query }
    delete query.page
    if (value === 'open') {
      delete query.status
    } else {
      query.status = value
    }

    void router.replace({ query })
  },
})

const poolsQuery = usePoolOptionsQuery()
const queryStatus = computed(() => (status.value === 'all' ? '' : status.value))
const page = computed(() => pageFrom(route.query.page))
const incidentsQuery = useIncidentsQuery(pool, queryStatus, page)

const pools = computed(() => poolsQuery.data.value ?? [])
const incidents = computed(() => incidentsQuery.data.value?.items ?? [])
const pagination = computed(() => incidentsQuery.data.value?.pagination)
const error = computed(() =>
  queryError(incidentsQuery.error.value, incidentsQuery.data.value, 'Unable to load incidents'),
)
</script>

<template>
  <div class="mb-7 flex flex-wrap items-end justify-between gap-4">
    <h1 class="page-title">Incidents</h1>
    <div class="flex gap-2">
      <select v-model="pool" class="field" aria-label="Pool">
        <option value="">All pools</option>
        <option v-for="item in pools" :key="item.name" :value="item.name">{{ item.name }}</option>
      </select>
      <select v-model="status" class="field" aria-label="Incident status">
        <option value="open">Open</option>
        <option value="resolved">Resolved</option>
        <option value="all">All incidents</option>
      </select>
    </div>
  </div>

  <ViewState
    :loading="incidentsQuery.isPending.value"
    :error
    :empty="!incidents.length && page === 1"
  >
    <template #content>
      <section class="panel overflow-hidden">
        <div class="divide-y divide-slate-100">
          <div
            v-for="incident in incidents"
            :key="incident.id"
            class="grid gap-3 px-5 py-4 sm:grid-cols-[10rem_1fr_auto]"
          >
            <div>
              <RouterLink
                :to="`/pools/${incident.pool}`"
                class="text-sm font-medium hover:underline"
                >{{ incident.pool }}</RouterLink
              >
              <p class="mt-1 text-xs text-slate-400" :title="exact(incident.last_seen_at)">
                {{ relative(incident.last_seen_at) }}
              </p>
            </div>
            <div>
              <p class="text-sm font-medium">
                <span class="capitalize">{{ lifecycleLabel(incident.stage) }}</span> ·
                <span class="capitalize">{{ lifecycleLabel(incident.code) }}</span>
              </p>
              <p class="mt-1 text-xs text-slate-500">{{ incident.message }}</p>
              <RouterLink
                v-if="incident.instance_id"
                :to="`/instances/${incident.instance_id}`"
                class="mt-2 inline-block font-mono text-xs text-violet-700 hover:underline"
              >
                {{ instanceLabel({ id: incident.instance_id, name: incident.instance_name }) }}
              </RouterLink>
            </div>
            <div class="text-right text-xs text-slate-500">
              <span
                class="rounded-full px-2 py-1 font-medium"
                :class="
                  incident.resolved_at ? 'bg-slate-100 text-slate-600' : 'bg-red-100 text-red-700'
                "
              >
                {{ incident.resolved_at ? 'Resolved' : 'Open' }}
              </span>
              <p class="mt-2">
                {{ incident.occurrences }}
                {{ incident.occurrences === 1 ? 'occurrence' : 'occurrences' }}
              </p>
            </div>
          </div>
        </div>
      </section>
      <PaginationControls v-if="pagination" :pagination />
    </template>
    <span v-if="status === 'open'">No open incidents.</span>
    <span v-else-if="status === 'resolved'">No resolved incidents.</span>
    <span v-else>No incidents recorded.</span>
  </ViewState>
</template>
