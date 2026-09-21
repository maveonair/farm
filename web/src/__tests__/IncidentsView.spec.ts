import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api, type Incident } from '@/api/client'
import IncidentsView from '@/views/IncidentsView.vue'
import { newTestQueryClient } from './query'

const incidents: Record<string, Incident[]> = {
  open: [
    {
      id: 1,
      pool: 'debian',
      run_id: 'run-1',
      stage: 'create_instance',
      code: 'unavailable',
      message: 'Incus is unavailable',
      instance_id: 'instance-123456789',
      instance_name: 'farm-debian-instance-1234',
      first_seen_at: '2026-09-21T09:00:00Z',
      last_seen_at: '2026-09-21T10:00:00Z',
      occurrences: 2,
    },
  ],
  resolved: [
    {
      id: 2,
      pool: 'debian',
      run_id: 'run-2',
      stage: 'fetch_jobs',
      code: 'unauthorized',
      message: 'Forgejo authentication failed',
      first_seen_at: '2026-09-20T09:00:00Z',
      last_seen_at: '2026-09-20T10:00:00Z',
      occurrences: 1,
      resolved_at: '2026-09-20T10:05:00Z',
    },
  ],
}

describe('IncidentsView', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows open incidents and lets the user inspect resolved incidents', async () => {
    vi.spyOn(api, 'pools').mockResolvedValue([])
    const getIncidents = vi.spyOn(api, 'incidents').mockImplementation(async (params) => {
      return incidents[params?.get('status') || 'all'] ?? []
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/incidents', component: IncidentsView },
        { path: '/instances/:id', component: { template: '<div />' } },
        { path: '/pools/:name', component: { template: '<div />' } },
      ],
    })
    await router.push('/incidents')
    await router.isReady()

    const wrapper = mount(IncidentsView, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    expect(getIncidents.mock.calls[0]?.[0]?.get('status')).toBe('open')
    expect(wrapper.text()).toContain('Incus is unavailable')
    expect(wrapper.text()).toContain('farm-debian-instance-1234')

    await wrapper.get('select[aria-label="Incident status"]').setValue('resolved')
    await flushPromises()

    expect(router.currentRoute.value.query).toEqual({ status: 'resolved' })
    expect(wrapper.text()).toContain('Forgejo authentication failed')
    expect(wrapper.text()).not.toContain('Incus is unavailable')
    wrapper.unmount()
  })
})
