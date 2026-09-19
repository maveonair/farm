import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { keepPreviousData, useQuery } from '@tanstack/vue-query'

import { api } from '@/api/client'
import { instanceKeys } from '@/queries/keys'
import { LIST_LIMIT, POLL_INTERVAL_MS } from '@/queries/options'

export function useInstancesQuery(
  pool: MaybeRefOrGetter<string>,
  state: MaybeRefOrGetter<string> = '',
) {
  const filters = computed(() => ({
    pool: toValue(pool),
    state: toValue(state),
    limit: LIST_LIMIT,
  }))

  return useQuery({
    queryKey: computed(() => instanceKeys.list(filters.value)),
    queryFn: ({ queryKey, signal }) => {
      const query = queryKey[2]
      const params = new URLSearchParams({ limit: String(query.limit) })
      if (query.pool) params.set('pool', query.pool)
      if (query.state) params.set('state', query.state)

      return api.instances(params, signal)
    },
    placeholderData: keepPreviousData,
    refetchInterval: POLL_INTERVAL_MS,
  })
}

export function useInstanceQuery(id: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: computed(() => instanceKeys.detail(toValue(id))),
    queryFn: ({ queryKey, signal }) => api.instance(queryKey[1], signal),
    enabled: computed(() => Boolean(toValue(id))),
    refetchInterval: POLL_INTERVAL_MS,
  })
}

export function useInstanceEventsQuery(id: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: computed(() => instanceKeys.events(toValue(id))),
    queryFn: ({ queryKey, signal }) => api.instanceEvents(queryKey[1], signal),
    enabled: computed(() => Boolean(toValue(id))),
    refetchInterval: POLL_INTERVAL_MS,
  })
}
