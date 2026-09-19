<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import ViewState from '@/components/ViewState.vue'
import { exact, relative } from '@/lib/format'
import { lifecycleLabel } from '@/lib/lifecycle'
import { useEventsQuery, useIncidentsQuery } from '@/queries/activity'
import { queryError } from '@/queries/options'
import { usePoolOptionsQuery } from '@/queries/pools'

const route = useRoute()
const router = useRouter()
const selected = computed({
  get: () => (typeof route.query.pool === 'string' ? route.query.pool : ''),
  set: (value: string) => {
    const query = { ...route.query }
    if (value) query.pool = value
    else delete query.pool

    void router.replace({ query })
  },
})

const poolsQuery = usePoolOptionsQuery()
const eventsQuery = useEventsQuery(selected)
const incidentsQuery = useIncidentsQuery(selected)

const pools = computed(() => poolsQuery.data.value ?? [])
const events = computed(() => eventsQuery.data.value ?? [])
const incidents = computed(() => incidentsQuery.data.value ?? [])
const loading = computed(() => eventsQuery.isPending.value || incidentsQuery.isPending.value)
const error = computed(
  () =>
    queryError(eventsQuery.error.value, eventsQuery.data.value, 'Unable to load activity') ||
    queryError(incidentsQuery.error.value, incidentsQuery.data.value, 'Unable to load activity'),
)
</script>

<template>
  <div class="mb-7 flex items-end justify-between">
    <div>
      <p class="eyebrow">History</p>
      <h1 class="page-title mt-1">Activity</h1>
    </div>
    <select v-model="selected" class="field">
      <option value="">All pools</option>
      <option v-for="pool in pools" :key="pool.name">{{ pool.name }}</option>
    </select>
  </div>
  <ViewState :loading :error :empty="!events.length && !incidents.length"
    ><template #content
      ><section v-if="incidents.length" class="panel mb-6 overflow-hidden">
        <div class="border-b border-slate-200 px-5 py-4 font-semibold">
          Reconciliation incidents
        </div>
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
              <p class="mt-1 text-xs text-slate-400">{{ relative(incident.last_seen_at) }}</p>
            </div>
            <div>
              <p class="text-sm">
                <span class="capitalize">{{ lifecycleLabel(incident.stage) }}</span> ·
                <span class="text-red-700">{{ incident.code }}</span>
              </p>
              <p class="mt-1 text-xs text-slate-500">{{ incident.message }}</p>
            </div>
            <span class="text-xs text-slate-500"
              >{{ incident.occurrences }}×<span v-if="incident.resolved_at"> · resolved</span></span
            >
          </div>
        </div>
      </section>
      <div v-if="events.length" class="panel divide-y divide-slate-100">
        <div
          v-for="event in events"
          :key="event.id"
          class="grid gap-3 px-5 py-4 sm:grid-cols-[10rem_1fr_auto]"
        >
          <div>
            <RouterLink :to="`/pools/${event.pool}`" class="text-sm font-medium hover:underline">{{
              event.pool
            }}</RouterLink>
            <p class="mt-1 text-xs text-slate-400" :title="exact(event.created_at)">
              {{ relative(event.created_at) }}
            </p>
          </div>
          <div>
            <p class="text-sm capitalize">
              {{ event.kind.replace('_', ' ') }}
              <span v-if="event.from_state" class="text-slate-500"
                >· {{ event.from_state }} → {{ event.to_state }}</span
              >
            </p>
            <p
              v-if="event.stage || event.result || event.reason"
              class="mt-1 text-xs capitalize text-slate-500"
            >
              {{
                [event.stage, event.result, event.reason]
                  .filter(Boolean)
                  .map((value) => lifecycleLabel(value))
                  .join(' · ')
              }}
            </p>
            <p v-if="event.message" class="mt-1 truncate text-xs text-red-700">
              {{ event.message }}
            </p>
          </div>
          <RouterLink
            :to="`/instances/${event.instance_id}`"
            class="font-mono text-xs text-violet-700 hover:underline"
            >{{ event.instance_id.slice(0, 12) }}</RouterLink
          >
        </div>
      </div></template
    ><span>No activity recorded.</span></ViewState
  >
</template>
