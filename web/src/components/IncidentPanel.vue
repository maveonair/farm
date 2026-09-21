<script setup lang="ts">
import { RouterLink } from 'vue-router'

import type { Incident } from '@/api/client'
import { exact, instanceLabel, relative } from '@/lib/format'
import { lifecycleLabel } from '@/lib/lifecycle'

defineProps<{ incident: Incident }>()

const hints: Record<string, string> = {
  unauthorized: 'Verify the Forgejo token and its permissions.',
  forbidden: 'Verify that FARM can administer runners in this scope.',
  timeout: 'Check service connectivity and the configured timeout.',
  unavailable: 'Check Forgejo or Incus availability.',
  not_found: 'A resource disappeared during reconciliation.',
  database: 'Inspect database storage and FARM logs.',
  interrupted: 'The controller stopped before reconciliation finished.',
}
</script>

<template>
  <section class="rounded-md border border-red-200 bg-red-50 p-5 text-sm text-red-950">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="font-semibold">Reconciliation failed</p>
        <p class="mt-1 font-mono text-xs text-red-800">{{ incident.message }}</p>
      </div>
      <span class="rounded-full bg-red-100 px-2 py-1 text-xs font-medium text-red-800">{{
        lifecycleLabel(incident.code)
      }}</span>
    </div>
    <dl class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div>
        <dt class="text-xs text-red-700">Stage</dt>
        <dd class="mt-1 font-medium">{{ lifecycleLabel(incident.stage) }}</dd>
      </div>
      <div>
        <dt class="text-xs text-red-700">First seen</dt>
        <dd class="mt-1" :title="exact(incident.first_seen_at)">
          {{ relative(incident.first_seen_at) }}
        </dd>
      </div>
      <div>
        <dt class="text-xs text-red-700">Last seen</dt>
        <dd class="mt-1" :title="exact(incident.last_seen_at)">
          {{ relative(incident.last_seen_at) }}
        </dd>
      </div>
      <div>
        <dt class="text-xs text-red-700">Occurrences</dt>
        <dd class="mt-1">{{ incident.occurrences }}</dd>
      </div>
    </dl>
    <p v-if="hints[incident.code]" class="mt-4 border-t border-red-200 pt-3 text-red-800">
      {{ hints[incident.code] }}
    </p>
    <RouterLink
      v-if="incident.instance_id"
      :to="`/instances/${incident.instance_id}`"
      class="mt-3 inline-block font-medium underline"
    >
      View {{ instanceLabel({ id: incident.instance_id, name: incident.instance_name }) }}
    </RouterLink>
  </section>
</template>
