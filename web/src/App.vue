<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'

import { useSummaryQuery } from '@/queries/summary'

const route = useRoute()
const navigationOpen = ref(false)
const navigationToggle = ref<HTMLButtonElement>()

function closeNavigation() {
  navigationOpen.value = false
  navigationToggle.value?.focus()
}

watch(
  () => route.fullPath,
  () => {
    navigationOpen.value = false
  },
)

const summaryQuery = useSummaryQuery()
const summary = summaryQuery.data
const statusText = computed(() => {
  if (!summary.value) {
    return 'Disconnected'
  }
  if (summaryQuery.isRefetchError.value) {
    return `${summary.value.controller} · ${summary.value.version} · stale`
  }

  return `${summary.value.controller} · ${summary.value.version}`
})

function isSection(path: string) {
  return route.path === path || route.path.startsWith(`${path}/`)
}

function statusColor() {
  if (summaryQuery.isRefetchError.value) {
    return 'bg-amber-400'
  }

  const reconciliation = summary.value?.reconciliation
  if (!reconciliation) {
    return 'bg-zinc-400'
  }
  if (reconciliation.condition === 'degraded' || reconciliation.condition === 'stalled') {
    return 'bg-red-400'
  }
  if (reconciliation.phase === 'running' || reconciliation.condition === 'starting') {
    return 'bg-blue-400'
  }
  return 'bg-emerald-400'
}
</script>

<template>
  <div class="min-h-screen bg-zinc-50 text-zinc-900 dark:bg-canvas dark:text-zinc-200">
    <header class="sticky top-0 z-20 border-b border-white/10 bg-zinc-900 text-white dark:bg-shell">
      <div class="flex h-14 items-center gap-4 px-4 sm:px-6">
        <button
          ref="navigationToggle"
          type="button"
          class="rounded p-1 text-zinc-300 hover:text-white md:hidden"
          aria-controls="primary-navigation"
          :aria-expanded="navigationOpen"
          :aria-label="navigationOpen ? 'Close navigation' : 'Open navigation'"
          @click="navigationOpen = !navigationOpen"
        >
          <svg
            class="size-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            aria-hidden="true"
          >
            <path d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <RouterLink to="/" class="flex shrink-0 items-center gap-3 font-semibold tracking-wide">
          <img src="/farm.svg" alt="" class="size-8 mt-1" />
          <span>FARM</span>
        </RouterLink>
        <div class="ml-auto flex shrink-0 items-center gap-2 text-xs">
          <span class="size-2 rounded-full" :class="statusColor()" />
          <span class="hidden text-zinc-300 sm:inline">{{ statusText }}</span>
          <span class="text-zinc-300 sm:hidden">{{ summary?.version ?? 'Disconnected' }}</span>
        </div>
      </div>
    </header>
    <div class="md:grid md:grid-cols-[14rem_minmax(0,1fr)]">
      <aside
        class="border-b border-zinc-200 bg-white md:sticky md:top-14 md:h-[calc(100dvh-3.5rem)] md:border-r md:border-b-0 md:overflow-y-auto dark:border-zinc-800 dark:bg-shell"
        :class="navigationOpen ? 'block' : 'hidden md:block'"
        @keydown.esc="closeNavigation"
      >
        <nav
          id="primary-navigation"
          aria-label="Primary navigation"
          class="flex flex-col gap-1 p-3 text-sm text-zinc-600 md:py-6 dark:text-zinc-300"
        >
          <RouterLink to="/" class="nav-link" @click="navigationOpen = false">Overview</RouterLink>
          <RouterLink
            to="/pools"
            class="nav-link"
            :class="{ 'nav-section-active': isSection('/pools') }"
            :aria-current="isSection('/pools') ? 'page' : undefined"
            @click="navigationOpen = false"
            >Pools</RouterLink
          >
          <RouterLink
            to="/instances"
            class="nav-link"
            :class="{ 'nav-section-active': isSection('/instances') }"
            :aria-current="isSection('/instances') ? 'page' : undefined"
            @click="navigationOpen = false"
            >Instances</RouterLink
          >
          <RouterLink to="/activity" class="nav-link" @click="navigationOpen = false">
            Activity
          </RouterLink>
          <RouterLink to="/incidents" class="nav-link" @click="navigationOpen = false">
            Incidents
            <span
              v-if="summary?.reconciliation.active_failures"
              class="ml-auto rounded-full bg-red-600 px-1.5 py-0.5 text-[0.65rem] leading-none text-white"
            >
              {{ summary.reconciliation.active_failures }}
            </span>
          </RouterLink>
        </nav>
      </aside>
      <main class="mx-auto w-full min-w-0 max-w-[1600px] px-4 py-5 sm:px-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
