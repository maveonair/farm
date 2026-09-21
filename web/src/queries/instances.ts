import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { keepPreviousData, useQuery } from '@tanstack/vue-query'

import { api } from '@/api/client'
import { pageParams } from '@/lib/pagination'
import { instanceKeys } from '@/queries/keys'
import { PAGE_SIZE, POLL_INTERVAL_MS } from '@/queries/options'

export function useInstancesQuery(
  pool: MaybeRefOrGetter<string>,
  state: MaybeRefOrGetter<string> = '',
  page: MaybeRefOrGetter<number> = 1,
  perPage: MaybeRefOrGetter<number> = PAGE_SIZE,
) {
  const filters = computed(() => ({
    pool: toValue(pool),
    state: toValue(state),
    page: toValue(page),
    perPage: toValue(perPage),
  }))

  return useQuery({
    queryKey: computed(() => instanceKeys.list(filters.value)),
    queryFn: ({ queryKey, signal }) => {
      const query = queryKey[2]
      const params = pageParams(query.page, query.perPage)
      if (query.pool) {
        params.set('pool', query.pool)
      }
      if (query.state) {
        params.set('state', query.state)
      }

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
    queryFn: ({ queryKey, signal }) =>
      api.instanceEvents(queryKey[1], pageParams(1, PAGE_SIZE), signal),
    enabled: computed(() => Boolean(toValue(id))),
    refetchInterval: POLL_INTERVAL_MS,
  })
}
