<script setup lang="ts">
defineProps<{ loading?: boolean; error?: string; empty?: boolean }>()
</script>

<template>
  <div v-if="loading" class="panel p-10 text-center text-sm text-zinc-500 dark:text-zinc-400">
    Loading…
  </div>
  <div
    v-else-if="error"
    class="notice border-red-200 bg-red-50 text-red-800 dark:border-red-400/40 dark:bg-red-400/10 dark:text-red-200"
  >
    {{ error }}
  </div>
  <div v-else-if="empty" class="panel p-10 text-center text-sm text-zinc-500 dark:text-zinc-400">
    <slot>No data yet.</slot>
  </div>
  <slot v-else name="content" />
</template>
