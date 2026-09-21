import type { IncidentStatus } from '@/api/client'

export interface InstanceFilters {
  pool: string
  state: string
  page: number
  perPage: number
}

export interface ActivityFilters {
  pool: string
  page: number
  perPage: number
}

export interface IncidentFilters extends ActivityFilters {
  status: IncidentStatus | ''
}

export const summaryKey = ['summary'] as const

export const poolKeys = {
  all: ['pools'] as const,
  detail: (name: string) => ['pools', name] as const,
}

export const instanceKeys = {
  all: ['instances'] as const,
  list: (filters: InstanceFilters) => ['instances', 'list', filters] as const,
  detail: (id: string) => ['instances', id] as const,
  events: (id: string) => ['instances', id, 'events'] as const,
}

export const activityKeys = {
  events: (filters: ActivityFilters) => ['activity', 'events', filters] as const,
  incidents: (filters: IncidentFilters) => ['activity', 'incidents', filters] as const,
}
