import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'

import { api } from '@/api/client'
import { poolKeys } from '@/queries/keys'
import { POLL_INTERVAL_MS } from '@/queries/options'

export function usePoolsQuery() {
  return useQuery({
    queryKey: poolKeys.all,
    queryFn: ({ signal }) => api.pools(signal),
    refetchInterval: POLL_INTERVAL_MS,
  })
}

export function usePoolOptionsQuery() {
  return useQuery({
    queryKey: poolKeys.all,
    queryFn: ({ signal }) => api.pools(signal),
  })
}

export function usePoolQuery(name: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: computed(() => poolKeys.detail(toValue(name))),
    queryFn: ({ queryKey, signal }) => api.pool(queryKey[1], signal),
    enabled: computed(() => Boolean(toValue(name))),
    refetchInterval: POLL_INTERVAL_MS,
  })
}
