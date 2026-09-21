<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import type { Pagination } from '@/api/client'

defineProps<{ pagination: Pagination }>()

const route = useRoute()
const router = useRouter()

function navigate(page: number) {
  if (page < 1) {
    return
  }

  const query = { ...route.query }
  if (page === 1) {
    delete query.page
  } else {
    query.page = String(page)
  }

  void router.push({ query })
}
</script>

<template>
  <nav
    v-if="pagination.page > 1 || pagination.has_next"
    class="mt-4 flex justify-end gap-2"
    aria-label="Pagination"
  >
    <button
      type="button"
      class="field disabled:cursor-not-allowed disabled:opacity-50"
      :disabled="pagination.page === 1"
      aria-label="Previous page"
      @click="navigate(pagination.page - 1)"
    >
      Previous
    </button>
    <button
      type="button"
      class="field disabled:cursor-not-allowed disabled:opacity-50"
      :disabled="!pagination.has_next"
      aria-label="Next page"
      @click="navigate(pagination.page + 1)"
    >
      Next
    </button>
  </nav>
</template>
