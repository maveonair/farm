<script setup lang="ts">
import type { State } from '@/api/client'
import { stateClasses } from '@/lib/lifecycle'

defineProps<{ state: State }>()
</script>

<template>
  <span
    class="inline-flex rounded-full px-2 py-1 text-xs font-medium capitalize ring-1 ring-inset"
    :class="stateClasses[state]"
  >
    {{ state }}
  </span>
</template>
