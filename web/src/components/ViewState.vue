<script setup lang="ts">
defineProps<{ loading?: boolean; error?: string; empty?: boolean }>()
</script>

<template>
  <div v-if="loading" class="panel p-10 text-center text-sm text-slate-500">Loading…</div>
  <div
    v-else-if="error"
    class="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-800"
  >
    {{ error }}
  </div>
  <div v-else-if="empty" class="panel p-10 text-center text-sm text-slate-500">
    <slot>No data yet.</slot>
  </div>
  <slot v-else name="content" />
</template>
