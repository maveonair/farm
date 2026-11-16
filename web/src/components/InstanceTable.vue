<script setup lang="ts">
import { RouterLink } from 'vue-router'
import type { Instance } from '@/api/client'
import { instanceLabel, relative } from '@/lib/format'
import StateBadge from './StateBadge.vue'

defineProps<{ instances: Instance[] }>()
</script>

<template>
  <div class="panel overflow-x-auto">
    <table class="data-table">
      <thead class="table-head">
        <tr>
          <th scope="col">Instance</th>
          <th scope="col">Pool</th>
          <th scope="col">State</th>
          <th scope="col" class="text-right">Runner</th>
          <th scope="col" class="text-right">Changed</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-zinc-100 dark:divide-zinc-800">
        <tr
          v-for="instance in instances"
          :key="instance.id"
          class="hover:bg-zinc-50 dark:hover:bg-zinc-800/60"
        >
          <td>
            <RouterLink
              class="font-mono text-xs font-medium text-violet-700 hover:underline dark:text-violet-300"
              :to="`/instances/${instance.id}`"
              >{{ instanceLabel({ id: instance.id, name: instance.name }) }}</RouterLink
            >
          </td>
          <td>
            <RouterLink class="font-mono text-xs hover:underline" :to="`/pools/${instance.pool}`">{{
              instance.pool
            }}</RouterLink>
          </td>
          <td><StateBadge :state="instance.state" /></td>
          <td class="text-right font-mono text-xs text-zinc-600 tabular-nums dark:text-zinc-300">
            {{ instance.runner_id || '—' }}
          </td>
          <td
            class="text-right text-xs whitespace-nowrap text-zinc-500 tabular-nums dark:text-zinc-400"
            :title="instance.state_changed_at"
          >
            {{ relative(instance.state_changed_at) }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
