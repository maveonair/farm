import { defineComponent, h, nextTick, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api, type Summary } from '@/api/client'
import { useInstancesQuery } from '@/queries/instances'
import { useOverviewEventsQuery } from '@/queries/activity'
import { POLL_INTERVAL_MS } from '@/queries/options'
import { useSummaryQuery } from '@/queries/summary'
import { newTestQueryClient, page } from './query'

const summary: Summary = {
  controller: 'farm',
  version: 'dev',
  healthy: true,
  last_success_at: '',
  reconcile_errors: 0,
  waiting: 0,
  counts: {},
  generated_at: '',
  reconciliation: {
    phase: 'idle',
    condition: 'healthy',
    last_outcome: 'succeeded',
    completed_pools: 1,
    total_pools: 1,
    active_failures: 0,
  },
}

function mountQuery(component: ReturnType<typeof defineComponent>) {
  return mount(component, {
    global: {
      plugins: [[VueQueryPlugin, { queryClient: newTestQueryClient() }]],
    },
  })
}

describe('queries', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('deduplicates summary consumers and polls', async () => {
    vi.useFakeTimers()
    const getSummary = vi.spyOn(api, 'summary').mockResolvedValue(summary)
    const component = defineComponent({
      setup() {
        useSummaryQuery()
        useSummaryQuery()

        return () => h('div')
      },
    })
    const wrapper = mountQuery(component)
    await flushPromises()

    expect(getSummary).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await flushPromises()

    expect(getSummary).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('keeps cached data when a poll fails', async () => {
    vi.useFakeTimers()
    vi.spyOn(api, 'summary')
      .mockResolvedValueOnce(summary)
      .mockRejectedValueOnce(new Error('offline'))
    const component = defineComponent({
      setup() {
        const query = useSummaryQuery()

        return () => h('div', `${query.data.value?.controller} ${query.isRefetchError.value}`)
      },
    })
    const wrapper = mountQuery(component)
    await flushPromises()

    expect(wrapper.text()).toBe('farm false')

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await flushPromises()

    expect(wrapper.text()).toBe('farm true')
    wrapper.unmount()
  })

  it('cancels the obsolete filtered request', async () => {
    const requests: Array<{ params: string; signal?: AbortSignal }> = []
    vi.spyOn(api, 'instances').mockImplementation((params, signal) => {
      requests.push({ params: params?.toString() ?? '', signal })

      return new Promise((_, reject) => {
        signal?.addEventListener('abort', () => reject(signal.reason), { once: true })
      })
    })
    const pool = ref('first')
    const component = defineComponent({
      setup() {
        useInstancesQuery(pool)

        return () => h('div')
      },
    })
    const wrapper = mountQuery(component)
    await flushPromises()

    pool.value = 'second'
    await nextTick()
    await flushPromises()

    expect(requests).toHaveLength(2)
    expect(requests[0]?.params).toContain('pool=first')
    expect(requests[0]?.signal?.aborted).toBe(true)
    expect(requests[1]?.params).toContain('pool=second')
    wrapper.unmount()
  })

  it('requests eight overview events', async () => {
    const getEvents = vi.spyOn(api, 'events').mockResolvedValue(page([]))
    const component = defineComponent({
      setup() {
        useOverviewEventsQuery()

        return () => h('div')
      },
    })
    const wrapper = mountQuery(component)
    await flushPromises()

    expect(getEvents.mock.calls[0]?.[0]?.get('per_page')).toBe('8')
    wrapper.unmount()
  })
})
