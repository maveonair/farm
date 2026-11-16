<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import ViewState from '@/components/ViewState.vue'
import { relative, scopeName } from '@/lib/format'
import { queryError } from '@/queries/options'
import { usePoolsQuery } from '@/queries/pools'

const poolsQuery = usePoolsQuery()
const pools = computed(() => poolsQuery.data.value ?? [])
const loading = poolsQuery.isPending
const error = computed(() =>
  queryError(poolsQuery.error.value, poolsQuery.data.value, 'Unable to load pools'),
)
</script>

<template>
  <div class="page-header">
    <div>
      <h1 class="page-title">Pools</h1>
      <p class="page-description">Runner groups, capacity, and provisioning status.</p>
    </div>
  </div>
  <ViewState :loading :error :empty="!pools.length">
    <template #content>
      <div class="grid gap-3 xl:grid-cols-2">
        <RouterLink
          v-for="pool in pools"
          :key="pool.name"
          :to="`/pools/${pool.name}`"
          class="panel p-4 transition-colors hover:border-violet-300 dark:hover:border-violet-400"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0">
              <h2
                class="break-words font-mono text-sm font-semibold text-violet-700 dark:text-violet-300"
              >
                {{ pool.name }}
              </h2>
              <p class="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
                Scope: {{ scopeName(pool.scope) }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <span
                v-if="pool.bootstrap.paused"
                class="badge bg-amber-50 text-amber-800 dark:bg-amber-400/10 dark:text-amber-200"
                >Provisioning paused</span
              >
              <span
                v-if="pool.runtime.state"
                class="badge capitalize"
                :class="
                  pool.runtime.state === 'failed'
                    ? 'bg-red-50 text-red-700 dark:bg-red-400/10 dark:text-red-300'
                    : pool.runtime.state === 'running'
                      ? 'bg-blue-50 text-blue-700 dark:bg-blue-400/10 dark:text-blue-300'
                      : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300'
                "
                >{{ pool.runtime.state }}</span
              ><span class="badge bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                >{{ pool.capacity.waiting }} waiting</span
              >
            </div>
          </div>
          <div
            class="mt-4 grid grid-cols-2 gap-3 border-y border-zinc-100 py-3 sm:grid-cols-4 dark:border-zinc-800"
          >
            <div
              v-for="item in [
                ['Running', pool.capacity.running],
                ['Ready', pool.capacity.ready],
                ['Bootstrapping', pool.capacity.bootstrapping],
                ['Limit', pool.capacity.maximum],
              ]"
              :key="String(item[0])"
            >
              <p class="text-xl font-semibold tabular-nums">{{ item[1] }}</p>
              <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">{{ item[0] }}</p>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <span
              v-for="label in pool.labels"
              :key="label"
              class="badge break-all bg-zinc-100 font-mono text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
              >{{ label }}</span
            >
          </div>
          <p class="mt-4 text-xs text-zinc-400">
            Queue observed {{ relative(pool.runtime.observed_at) }}
          </p>
        </RouterLink>
      </div>
    </template>
  </ViewState>
</template>
