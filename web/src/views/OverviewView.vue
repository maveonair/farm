<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

import ReconcileStatus from '@/components/ReconcileStatus.vue'
import ViewState from '@/components/ViewState.vue'
import { instanceLabel, relative } from '@/lib/format'
import { useOverviewEventsQuery } from '@/queries/activity'
import { queryError } from '@/queries/options'
import { usePoolsQuery } from '@/queries/pools'
import { useSummaryQuery } from '@/queries/summary'

const summaryQuery = useSummaryQuery()
const poolsQuery = usePoolsQuery()
const eventsQuery = useOverviewEventsQuery()

const summary = summaryQuery.data
const pools = computed(() => poolsQuery.data.value ?? [])
const events = computed(() => eventsQuery.data.value?.items ?? [])
const loading = computed(
  () => summaryQuery.isPending.value || poolsQuery.isPending.value || eventsQuery.isPending.value,
)
const error = computed(
  () =>
    queryError(summaryQuery.error.value, summaryQuery.data.value, 'Unable to load overview') ||
    queryError(poolsQuery.error.value, poolsQuery.data.value, 'Unable to load overview') ||
    queryError(eventsQuery.error.value, eventsQuery.data.value, 'Unable to load overview'),
)
</script>

<template>
  <div class="page-header">
    <div>
      <h1 class="page-title">Overview</h1>
      <p class="page-description">Runner capacity and controller activity.</p>
    </div>
  </div>
  <ViewState :loading :error>
    <template #content>
      <ReconcileStatus v-if="summary" :reconciliation="summary.reconciliation" />
      <section aria-label="Runner capacity" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="item in [
            ['Waiting jobs', summary?.waiting ?? 0],
            ['Running', summary?.counts.running ?? 0],
            ['Ready', summary?.counts.ready ?? 0],
            ['Bootstrapping', summary?.counts.bootstrapping ?? 0],
          ]"
          :key="String(item[0])"
          class="metric"
        >
          <p class="metric-label">{{ item[0] }}</p>
          <p class="metric-value">{{ item[1] }}</p>
        </div>
      </section>
      <div class="mt-5 grid gap-5 xl:grid-cols-[2fr_1fr]">
        <section class="panel overflow-hidden">
          <div class="section-header">
            <h2 class="font-semibold">Pool capacity</h2>
            <RouterLink
              to="/pools"
              class="text-sm text-violet-700 hover:underline dark:text-violet-300"
              >All pools</RouterLink
            >
          </div>
          <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
            <RouterLink
              v-for="pool in pools"
              :key="pool.name"
              :to="`/pools/${pool.name}`"
              class="grid gap-3 px-4 py-3 hover:bg-zinc-50 sm:grid-cols-[minmax(0,1fr)_auto] dark:hover:bg-zinc-800/60"
            >
              <div>
                <div
                  class="break-words font-mono text-sm font-medium text-violet-700 dark:text-violet-300"
                >
                  {{ pool.name }}
                </div>
                <div class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
                  {{ pool.capacity.running }} running · {{ pool.capacity.ready }} ready ·
                  {{ pool.capacity.waiting }} waiting
                </div>
              </div>
              <div class="w-36 self-center">
                <div class="mb-1 flex justify-between text-xs text-zinc-500 dark:text-zinc-400">
                  <span>Capacity</span
                  ><span
                    >{{
                      pool.capacity.running +
                      pool.capacity.ready +
                      pool.capacity.bootstrapping +
                      pool.capacity.cleaning
                    }}/{{ pool.capacity.maximum }}</span
                  >
                </div>
                <progress
                  class="h-1.5 w-full accent-violet-500"
                  :value="
                    pool.capacity.running +
                    pool.capacity.ready +
                    pool.capacity.bootstrapping +
                    pool.capacity.cleaning
                  "
                  :max="pool.capacity.maximum"
                />
              </div>
            </RouterLink>
            <p v-if="!pools.length" class="p-6 text-sm text-zinc-500 dark:text-zinc-400">
              Waiting for the first reconciliation.
            </p>
          </div>
        </section>
        <section class="panel overflow-hidden">
          <div class="section-header">
            <h2 class="font-semibold">Recent activity</h2>
            <RouterLink
              to="/activity"
              class="text-sm text-violet-700 hover:underline dark:text-violet-300"
              >View all</RouterLink
            >
          </div>
          <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
            <div v-for="event in events" :key="event.id" class="event-row text-sm">
              <p>
                <RouterLink
                  :to="`/instances/${event.instance_id}`"
                  class="break-all font-mono text-xs font-medium text-violet-700 hover:underline dark:text-violet-300"
                  >{{
                    instanceLabel({ id: event.instance_id, name: event.instance_name })
                  }}</RouterLink
                >
                <span class="text-zinc-500 ml-1 dark:text-zinc-400">{{
                  event.kind.replace('_', ' ')
                }}</span>
              </p>
              <p class="mt-1 text-xs text-zinc-400">
                {{ event.pool }} · {{ relative(event.created_at) }}
              </p>
            </div>
            <p v-if="!events.length" class="p-6 text-sm text-zinc-500 dark:text-zinc-400">
              No lifecycle events yet.
            </p>
          </div>
        </section>
      </div>
    </template>
  </ViewState>
</template>
