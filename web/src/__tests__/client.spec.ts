import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/api/client'

describe('API client', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('passes the cancellation signal to fetch', async () => {
    const signal = new AbortController().signal
    const fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ controller: 'farm' }), {
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetch)

    await api.summary(signal)

    expect(fetch).toHaveBeenCalledWith('/api/v1/summary', {
      signal,
      headers: { Accept: 'application/json' },
    })
  })

  it('preserves cancellation errors', async () => {
    const canceled = new DOMException('canceled', 'AbortError')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(canceled))

    await expect(api.summary(new AbortController().signal)).rejects.toBe(canceled)
  })
})
