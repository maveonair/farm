export const POLL_INTERVAL_MS = 5_000
export const QUERY_STALE_MS = 4_000
export const QUERY_CACHE_MS = 5 * 60_000

export const PAGE_SIZE = 25
export const POOL_PAGE_SIZE = 10
export const OVERVIEW_EVENT_LIMIT = 8

export function queryError(error: Error | null, data: unknown, fallback: string): string {
  if (data !== undefined || !error) {
    return ''
  }

  return error.message || fallback
}
