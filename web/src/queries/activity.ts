import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { keepPreviousData, useQuery } from '@tanstack/vue-query'

import { api, type IncidentStatus } from '@/api/client'
import { pageParams } from '@/lib/pagination'
import { activityKeys } from '@/queries/keys'
import { OVERVIEW_EVENT_LIMIT, PAGE_SIZE, POLL_INTERVAL_MS } from '@/queries/options'

function activityParams(
  pool: string,
  page: number,
  perPage: number,
  status: IncidentStatus | '' = '',
) {
  const value = pageParams(page, perPage)
  if (pool) {
    value.set('pool', pool)
  }
  if (status) {
    value.set('status', status)
  }

  return value
}

export function useEventsQuery(
  pool: MaybeRefOrGetter<string> = '',
  page: MaybeRefOrGetter<number> = 1,
  perPage: MaybeRefOrGetter<number> = PAGE_SIZE,
) {
  const filters = computed(() => ({
    pool: toValue(pool),
    page: toValue(page),
    perPage: toValue(perPage),
  }))

  return useQuery({
    queryKey: computed(() => activityKeys.events(filters.value)),
    queryFn: ({ queryKey, signal }) => {
      const query = queryKey[2]

      return api.events(activityParams(query.pool, query.page, query.perPage), signal)
    },
    placeholderData: keepPreviousData,
    refetchInterval: POLL_INTERVAL_MS,
  })
}

export function useOverviewEventsQuery() {
  return useEventsQuery('', 1, OVERVIEW_EVENT_LIMIT)
}

export function useIncidentsQuery(
  pool: MaybeRefOrGetter<string> = '',
  status: MaybeRefOrGetter<IncidentStatus | ''> = '',
  page: MaybeRefOrGetter<number> = 1,
  perPage: MaybeRefOrGetter<number> = PAGE_SIZE,
) {
  const filters = computed(() => ({
    pool: toValue(pool),
    status: toValue(status),
    page: toValue(page),
    perPage: toValue(perPage),
  }))

  return useQuery({
    queryKey: computed(() => activityKeys.incidents(filters.value)),
    queryFn: ({ queryKey, signal }) => {
      const query = queryKey[2]

      return api.incidents(
        activityParams(query.pool, query.page, query.perPage, query.status),
        signal,
      )
    },
    placeholderData: keepPreviousData,
    refetchInterval: POLL_INTERVAL_MS,
  })
}
