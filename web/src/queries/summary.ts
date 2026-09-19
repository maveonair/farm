import { useQuery } from '@tanstack/vue-query'

import { api } from '@/api/client'
import { summaryKey } from '@/queries/keys'
import { POLL_INTERVAL_MS } from '@/queries/options'

export function useSummaryQuery() {
  return useQuery({
    queryKey: summaryKey,
    queryFn: ({ signal }) => api.summary(signal),
    refetchInterval: POLL_INTERVAL_MS,
  })
}
