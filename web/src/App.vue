<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'

import { useSummaryQuery } from '@/queries/summary'

const route = useRoute()
const summaryQuery = useSummaryQuery()
const summary = summaryQuery.data
const statusText = computed(() => {
  if (!summary.value) return 'Disconnected'
  if (summaryQuery.isRefetchError.value) return `${summary.value.controller} · stale`

  return summary.value.controller
})

function isSection(path: string) {
  return route.path.startsWith(`${path}/`)
}

function statusColor() {
  if (summaryQuery.isRefetchError.value) return 'bg-amber-400'

  const reconciliation = summary.value?.reconciliation
  if (!reconciliation) return 'bg-slate-400'
  if (reconciliation.condition === 'degraded' || reconciliation.condition === 'stalled')
    return 'bg-red-400'
  if (reconciliation.phase === 'running' || reconciliation.condition === 'starting')
    return 'bg-blue-400'
  return 'bg-emerald-400'
}
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <header class="sticky top-0 z-20 border-b border-white/10 bg-[#171923] text-white">
      <div class="mx-auto flex h-14 max-w-[1600px] items-center gap-2 px-4 sm:gap-8 sm:px-6">
        <RouterLink to="/" class="flex shrink-0 items-center gap-3 font-semibold tracking-wide">
          <img src="/farm.svg" alt="" class="size-8 mt-1" />
          <span class="hidden sm:inline">FARM</span>
        </RouterLink>
        <nav
          class="flex h-full min-w-0 flex-1 items-center gap-1 overflow-x-auto whitespace-nowrap text-sm text-slate-300 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          <RouterLink to="/" class="nav-link">Overview</RouterLink>
          <RouterLink
            to="/pools"
            class="nav-link"
            :class="{ 'nav-section-active': isSection('/pools') }"
            :aria-current="isSection('/pools') ? 'page' : undefined"
            >Pools</RouterLink
          >
          <RouterLink
            to="/instances"
            class="nav-link"
            :class="{ 'nav-section-active': isSection('/instances') }"
            :aria-current="isSection('/instances') ? 'page' : undefined"
            >Instances</RouterLink
          >
          <RouterLink to="/activity" class="nav-link">Activity</RouterLink>
          <RouterLink to="/incidents" class="nav-link">
            Incidents
            <span
              v-if="summary?.reconciliation.active_failures"
              class="ml-1 rounded-full bg-red-600 px-1.5 py-0.5 text-[0.65rem] leading-none text-white"
            >
              {{ summary.reconciliation.active_failures }}
            </span>
          </RouterLink>
        </nav>
        <div class="ml-auto hidden shrink-0 items-center gap-2 text-xs sm:flex">
          <span class="size-2 rounded-full" :class="statusColor()" />
          <span class="text-slate-300">{{ statusText }}</span>
        </div>
      </div>
    </header>
    <main class="mx-auto max-w-[1600px] px-4 py-8 sm:px-6">
      <RouterView />
    </main>
  </div>
</template>
