import { QueryClient } from '@tanstack/vue-query'
import type { Page } from '@/api/client'

export function page<T>(items: T[], current = 1, hasNext = false, perPage = 25): Page<T> {
  return {
    items,
    pagination: { page: current, per_page: perPage, has_next: hasNext },
  }
}

export function newTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: Infinity,
      },
    },
  })
}
