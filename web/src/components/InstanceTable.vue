<script setup lang="ts">
import { RouterLink } from 'vue-router'
import type { Instance } from '@/api/client'
import { relative } from '@/lib/format'
import StateBadge from './StateBadge.vue'

defineProps<{ instances: Instance[] }>()
</script>

<template>
  <div class="panel overflow-x-auto">
    <table class="w-full min-w-[720px] text-sm">
      <thead class="table-head">
        <tr>
          <th class="px-4 py-3">Instance</th>
          <th class="px-4 py-3">Pool</th>
          <th class="px-4 py-3">State</th>
          <th class="px-4 py-3">Runner</th>
          <th class="px-4 py-3">Changed</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100">
        <tr v-for="instance in instances" :key="instance.id" class="hover:bg-slate-50">
          <td class="px-4 py-3">
            <RouterLink
              class="font-medium text-violet-700 hover:underline"
              :to="`/instances/${instance.id}`"
              >{{ instance.name }}</RouterLink
            >
            <div class="mt-1 max-w-72 truncate font-mono text-xs text-slate-400">
              {{ instance.id }}
            </div>
          </td>
          <td class="px-4 py-3">
            <RouterLink class="hover:underline" :to="`/pools/${instance.pool}`">{{
              instance.pool
            }}</RouterLink>
          </td>
          <td class="px-4 py-3"><StateBadge :state="instance.state" /></td>
          <td class="px-4 py-3 font-mono text-xs text-slate-600">
            {{ instance.runner_id || '—' }}
          </td>
          <td class="px-4 py-3 text-slate-600" :title="instance.state_changed_at">
            {{ relative(instance.state_changed_at) }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
