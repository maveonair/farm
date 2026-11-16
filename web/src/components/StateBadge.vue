<script setup lang="ts">
import type { State } from '@/api/client'
import { stateClasses } from '@/lib/lifecycle'

defineProps<{ state: State }>()
</script>

<template>
  <span class="badge capitalize ring-1 ring-inset" :class="stateClasses[state]">
    {{ state }}
  </span>
</template>
