import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { keepPreviousData, useQuery } from '@tanstack/vue-query'

import { api, type IncidentStatus } from '@/api/client'
import { activityKeys } from '@/queries/keys'
import { LIST_LIMIT, OVERVIEW_EVENT_LIMIT, POLL_INTERVAL_MS } from '@/queries/options'

function params(pool: string, limit: number, status: IncidentStatus | '' = '') {
  const value = new URLSearchParams({ limit: String(limit) })
  if (pool) value.set('pool', pool)
  if (status) value.set('status', status)

  return value
}

export function useEventsQuery(
  pool: MaybeRefOrGetter<string> = '',
  limit: MaybeRefOrGetter<number> = LIST_LIMIT,
) {
  const filters = computed(() => ({ pool: toValue(pool), limit: toValue(limit) }))

  return useQuery({
    queryKey: computed(() => activityKeys.events(filters.value)),
    queryFn: ({ queryKey, signal }) => {
      const query = queryKey[2]

      return api.events(params(query.pool, query.limit), signal)
    },
    placeholderData: keepPreviousData,
    refetchInterval: POLL_INTERVAL_MS,
  })
}

export function useOverviewEventsQuery() {
  return useEventsQuery('', OVERVIEW_EVENT_LIMIT)
}

export function useIncidentsQuery(
  pool: MaybeRefOrGetter<string> = '',
  status: MaybeRefOrGetter<IncidentStatus | ''> = '',
) {
  const filters = computed(() => ({
    pool: toValue(pool),
    status: toValue(status),
    limit: LIST_LIMIT,
  }))

  return useQuery({
    queryKey: computed(() => activityKeys.incidents(filters.value)),
    queryFn: ({ queryKey, signal }) => {
      const query = queryKey[2]

      return api.incidents(params(query.pool, query.limit, query.status), signal)
    },
    placeholderData: keepPreviousData,
    refetchInterval: POLL_INTERVAL_MS,
  })
}
