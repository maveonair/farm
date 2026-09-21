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
        <div class="mb-7 flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="eyebrow">Instance</p>
            <h1 class="page-title mt-1">{{ instance.name }}</h1>
          </div>
          <StateBadge :state="instance.state" />
        </div>
        <div
          v-if="instance.error"
          class="mb-6 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-800"
        >
          <p class="font-semibold">Instance error</p>
          <p class="mt-1 whitespace-pre-wrap font-mono text-xs">{{ instance.error }}</p>
        </div>
        <div class="grid gap-6 lg:grid-cols-[1fr_1.4fr]">
          <section class="panel p-5">
            <h2 class="font-semibold">Details</h2>
            <dl class="mt-4 divide-y divide-slate-100 text-sm">
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
                class="grid grid-cols-2 gap-4 py-3"
              >
                <dt class="text-slate-500">{{ item[0] }}</dt>
                <dd class="text-right capitalize">
                  <RouterLink
                    v-if="item[0] === 'Pool'"
                    :to="`/pools/${instance.pool}`"
                    class="text-violet-700 hover:underline"
                    >{{ item[1] }}</RouterLink
                  ><template v-else>{{ item[1] }}</template>
                </dd>
              </div>
            </dl>
          </section>
          <section class="panel overflow-hidden">
            <div class="border-b border-slate-200 px-5 py-4">
              <h2 class="font-semibold">Lifecycle</h2>
            </div>
            <div class="divide-y divide-slate-100">
              <div v-for="event in events" :key="event.id" class="flex gap-4 px-5 py-4">
                <span class="mt-1 size-2 shrink-0 rounded-full bg-violet-500" />
                <div>
                  <p class="text-sm font-medium capitalize">{{ lifecycleLabel(event.kind) }}</p>
                  <p v-if="event.from_state" class="mt-1 text-xs text-slate-500">
                    {{ event.from_state }} → {{ event.to_state }}
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
                  <p v-if="event.message" class="mt-2 text-xs text-red-700">{{ event.message }}</p>
                  <p class="mt-1 text-xs text-slate-400" :title="exact(event.created_at)">
                    {{ relative(event.created_at) }}
                  </p>
                </div>
              </div>
              <p v-if="!events.length" class="p-6 text-sm text-slate-500">
                No lifecycle events recorded.
              </p>
            </div>
          </section>
        </div>
      </template></template
    ></ViewState
  >
</template>
