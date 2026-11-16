<script setup lang="ts">
import type { Summary } from '@/api/client'
import { relative } from '@/lib/format'

const props = defineProps<{ reconciliation: Summary['reconciliation'] }>()

const styles = {
  starting:
    'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-400/40 dark:bg-blue-400/10 dark:text-blue-200',
  healthy:
    'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-400/40 dark:bg-blue-400/10 dark:text-blue-200',
  degraded:
    'border-red-200 bg-red-50 text-red-800 dark:border-red-400/40 dark:bg-red-400/10 dark:text-red-200',
  stalled:
    'border-red-200 bg-red-50 text-red-800 dark:border-red-400/40 dark:bg-red-400/10 dark:text-red-200',
}

function title() {
  if (props.reconciliation.condition === 'stalled') {
    return 'Reconciliation stalled'
  }
  if (props.reconciliation.condition === 'degraded') {
    return 'Reconciliation degraded'
  }
  if (props.reconciliation.condition === 'starting') {
    return 'Initial reconciliation in progress'
  }
  return 'Reconciliation in progress'
}
</script>

<template>
  <div
    v-if="
      reconciliation.phase === 'running' ||
      reconciliation.condition === 'degraded' ||
      reconciliation.condition === 'stalled'
    "
    class="notice mb-5"
    :class="styles[reconciliation.condition]"
  >
    <p class="font-semibold">{{ title() }}</p>
    <p class="mt-1">
      <template v-if="reconciliation.phase === 'running'">
        Started {{ relative(reconciliation.started_at) }} · {{ reconciliation.active_pools }} active
        pool<span v-if="reconciliation.active_pools !== 1">s</span>
        <span v-if="reconciliation.condition === 'degraded'"> · retry in progress</span>
      </template>
      <template v-else-if="reconciliation.condition === 'degraded'">
        {{ reconciliation.active_failures }} active reconciliation incident<span
          v-if="reconciliation.active_failures !== 1"
          >s</span
        >
      </template>
      <template v-else>The active operation exceeded its deadline.</template>
    </p>
  </div>
</template>
