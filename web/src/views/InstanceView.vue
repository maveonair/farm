<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import StateBadge from '@/components/StateBadge.vue'
import ViewState from '@/components/ViewState.vue'
import { exact, relative } from '@/lib/format'
import { lifecycleLabel } from '@/lib/lifecycle'
import { useInstanceEventsQuery, useInstanceQuery } from '@/queries/instances'
import { queryError } from '@/queries/options'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const instanceQuery = useInstanceQuery(id)
const eventsQuery = useInstanceEventsQuery(id)

const instance = instanceQuery.data
const events = computed(() => eventsQuery.data.value?.items ?? [])
const loading = computed(() => instanceQuery.isPending.value || eventsQuery.isPending.value)
const error = computed(
  () =>
    queryError(instanceQuery.error.value, instanceQuery.data.value, 'Unable to load instance') ||
    queryError(eventsQuery.error.value, eventsQuery.data.value, 'Unable to load instance'),
)
</script>

<template>
  <ViewState :loading :error
    ><template #content
      ><template v-if="instance">
        <nav class="breadcrumbs" aria-label="Breadcrumb">
          <RouterLink to="/instances">Instances</RouterLink>
          <span aria-hidden="true">/</span>
          <span class="truncate" aria-current="page">{{ instance.name }}</span>
        </nav>
        <div class="page-header">
          <div class="min-w-0">
            <h1 class="page-title font-mono">{{ instance.name }}</h1>
            <p class="page-description break-all font-mono text-xs">{{ instance.id }}</p>
          </div>
          <StateBadge :state="instance.state" />
        </div>
        <div
          v-if="instance.error"
          class="notice mb-5 border-red-200 bg-red-50 text-red-800 dark:border-red-400/40 dark:bg-red-400/10 dark:text-red-200"
        >
          <p class="font-semibold">Instance error</p>
          <p class="mt-1 whitespace-pre-wrap font-mono text-xs">{{ instance.error }}</p>
        </div>
        <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
          <section class="panel overflow-hidden">
            <div class="section-header"><h2 class="font-semibold">Details</h2></div>
            <dl class="divide-y divide-zinc-100 px-4 text-sm dark:divide-zinc-800">
              <div
                v-for="item in [
                  ['Pool', instance.pool],
                  ['Stage', lifecycleLabel(instance.stage)],
                  ['Result', lifecycleLabel(instance.result)],
                  ['Reason', lifecycleLabel(instance.reason)],
                  ['Runner ID', instance.runner_id || '—'],
                  ['Created', exact(instance.created_at)],
                  ['State changed', exact(instance.state_changed_at)],
                  ['Stage changed', exact(instance.stage_changed_at)],
                  ['Finished', exact(instance.finished_at)],
                  ['Retry count', instance.retry_count],
                  ['Retry at', exact(instance.retry_at)],
                ]"
                :key="String(item[0])"
                class="grid grid-cols-2 gap-3 py-2.5"
              >
                <dt class="text-zinc-500 dark:text-zinc-400">{{ item[0] }}</dt>
                <dd class="break-words text-right capitalize tabular-nums">
                  <RouterLink
                    v-if="item[0] === 'Pool'"
                    :to="`/pools/${instance.pool}`"
                    class="text-violet-700 hover:underline dark:text-violet-300"
                    >{{ item[1] }}</RouterLink
                  ><template v-else>{{ item[1] }}</template>
                </dd>
              </div>
            </dl>
          </section>
          <section class="panel overflow-hidden">
            <div class="section-header">
              <h2 class="font-semibold">Lifecycle</h2>
            </div>
            <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
              <div v-for="event in events" :key="event.id" class="event-row flex gap-3">
                <span
                  aria-hidden="true"
                  class="mt-1.5 size-1.5 shrink-0 rounded-full bg-zinc-400"
                />
                <div class="min-w-0">
                  <p class="text-sm font-medium capitalize">{{ lifecycleLabel(event.kind) }}</p>
                  <p v-if="event.from_state" class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
                    {{ event.from_state }} → {{ event.to_state }}
                  </p>
                  <p
                    v-if="event.stage || event.result || event.reason"
                    class="mt-1 text-xs capitalize text-zinc-500 dark:text-zinc-400"
                  >
                    {{
                      [event.stage, event.result, event.reason]
                        .filter(Boolean)
                        .map((value) => lifecycleLabel(value))
                        .join(' · ')
                    }}
                  </p>
                  <p
                    v-if="event.message"
                    class="mt-2 break-words font-mono text-xs text-red-700 dark:text-red-300"
                  >
                    {{ event.message }}
                  </p>
                  <p class="mt-1 text-xs text-zinc-400" :title="exact(event.created_at)">
                    {{ relative(event.created_at) }}
                  </p>
                </div>
              </div>
              <p v-if="!events.length" class="p-6 text-sm text-zinc-500 dark:text-zinc-400">
                No lifecycle events recorded.
              </p>
            </div>
          </section>
        </div>
      </template></template
    ></ViewState
  >
</template>
