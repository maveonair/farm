<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { Event } from '@/api/client'
import PaginationControls from '@/components/PaginationControls.vue'
import ViewState from '@/components/ViewState.vue'
import { exact, instanceLabel, relative } from '@/lib/format'
import { lifecycleLabel } from '@/lib/lifecycle'
import { pageFrom } from '@/lib/pagination'
import { useEventsQuery } from '@/queries/activity'
import { queryError } from '@/queries/options'
import { usePoolOptionsQuery } from '@/queries/pools'

const route = useRoute()
const router = useRouter()
const selected = computed({
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

const poolsQuery = usePoolOptionsQuery()
const page = computed(() => pageFrom(route.query.page))
const eventsQuery = useEventsQuery(selected, page)

const pools = computed(() => poolsQuery.data.value ?? [])
const events = computed(() => eventsQuery.data.value?.items ?? [])
const pagination = computed(() => eventsQuery.data.value?.pagination)
const error = computed(() =>
  queryError(eventsQuery.error.value, eventsQuery.data.value, 'Unable to load activity'),
)

function eventSummary(event: Event) {
  switch (event.kind) {
    case 'created':
      return 'was created'
    case 'registered':
      return 'registered its runner'
    case 'stage_changed':
      return `entered the ${lifecycleLabel(event.stage)} stage`
    case 'state_changed':
      return `changed from ${lifecycleLabel(event.from_state)} to ${lifecycleLabel(event.to_state)}`
    case 'retry_scheduled':
      return 'scheduled a retry'
    default:
      return lifecycleLabel(event.kind)
  }
}
</script>

<template>
  <div class="mb-7 flex flex-wrap items-end justify-between gap-4">
    <h1 class="page-title">Activity</h1>
    <select v-model="selected" class="field" aria-label="Pool">
      <option value="">All pools</option>
      <option v-for="pool in pools" :key="pool.name">{{ pool.name }}</option>
    </select>
  </div>

  <ViewState :loading="eventsQuery.isPending.value" :error :empty="!events.length && page === 1">
    <template #content>
      <div class="panel divide-y divide-slate-100">
        <div v-for="event in events" :key="event.id" class="px-5 py-4">
          <p class="text-sm">
            <RouterLink
              :to="`/instances/${event.instance_id}`"
              class="font-mono font-medium text-violet-700 hover:underline"
              >{{ instanceLabel({ id: event.instance_id, name: event.instance_name }) }}</RouterLink
            >
            <span class="ml-1">{{ eventSummary(event) }}</span>
          </p>
          <p class="mt-1 text-xs text-slate-500">
            <RouterLink :to="`/pools/${event.pool}`" class="hover:underline">{{
              event.pool
            }}</RouterLink>
            <span> · </span>
            <span :title="exact(event.created_at)">{{ relative(event.created_at) }}</span>
            <template v-if="event.result || event.reason">
              <span> · </span>
              <span class="capitalize">{{
                [event.result, event.reason]
                  .filter(Boolean)
                  .map((value) => lifecycleLabel(value))
                  .join(' · ')
              }}</span>
            </template>
          </p>
          <p v-if="event.message" class="mt-2 truncate text-xs text-red-700">
            {{ event.message }}
          </p>
        </div>
      </div>
      <PaginationControls v-if="pagination" :pagination />
    </template>
    <span>No lifecycle activity.</span>
  </ViewState>
</template>
