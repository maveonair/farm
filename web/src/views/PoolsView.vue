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
  <div class="mb-7">
    <h1 class="page-title mt-1">Pools</h1>
  </div>
  <ViewState :loading :error :empty="!pools.length">
    <template #content>
      <div class="grid gap-4 lg:grid-cols-2">
        <RouterLink
          v-for="pool in pools"
          :key="pool.name"
          :to="`/pools/${pool.name}`"
          class="panel p-5 transition hover:border-violet-300 hover:shadow-md"
        >
          <div class="flex items-start justify-between">
            <div>
              <h2 class="text-lg font-semibold">{{ pool.name }}</h2>
              <p class="mt-1 text-sm text-slate-500">
                {{ pool.scope.type }} · {{ scopeName(pool.scope) }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <span
                v-if="pool.runtime.state"
                class="rounded-full px-2 py-1 text-xs capitalize"
                :class="
                  pool.runtime.state === 'failed'
                    ? 'bg-red-50 text-red-700'
                    : pool.runtime.state === 'running'
                      ? 'bg-blue-50 text-blue-700'
                      : 'bg-emerald-50 text-emerald-700'
                "
                >{{ pool.runtime.state }}</span
              ><span class="rounded-full bg-slate-100 px-2 py-1 text-xs"
                >{{ pool.capacity.waiting }} waiting</span
              >
            </div>
          </div>
          <div class="mt-5 grid grid-cols-4 gap-3 text-center">
            <div
              v-for="item in [
                ['Running', pool.capacity.running],
                ['Ready', pool.capacity.ready],
                ['Bootstrapping', pool.capacity.bootstrapping],
                ['Limit', pool.capacity.maximum],
              ]"
              :key="String(item[0])"
            >
              <p class="text-xl font-semibold">{{ item[1] }}</p>
              <p class="mt-1 text-xs text-slate-500">{{ item[0] }}</p>
            </div>
          </div>
          <div class="mt-5 flex flex-wrap gap-2">
            <span
              v-for="label in pool.labels"
              :key="label"
              class="rounded bg-violet-50 px-2 py-1 font-mono text-xs text-violet-700"
              >{{ label }}</span
            >
          </div>
          <p class="mt-4 text-xs text-slate-400">
            Queue observed {{ relative(pool.runtime.observed_at) }}
          </p>
        </RouterLink>
      </div>
    </template>
  </ViewState>
</template>
