import { QueryClient } from '@tanstack/vue-query'

import { QUERY_CACHE_MS, QUERY_STALE_MS } from '@/queries/options'

export function newQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: QUERY_STALE_MS,
        gcTime: QUERY_CACHE_MS,
        retry: 1,
        refetchOnWindowFocus: true,
        refetchIntervalInBackground: false,
      },
    },
  })
}
