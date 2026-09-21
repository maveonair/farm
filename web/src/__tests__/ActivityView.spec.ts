import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/api/client'
import ActivityView from '@/views/ActivityView.vue'
import { newTestQueryClient, page } from './query'

describe('ActivityView', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows instance lifecycle activity', async () => {
    vi.spyOn(api, 'pools').mockResolvedValue([])
    vi.spyOn(api, 'events').mockResolvedValue(
      page([
        {
          id: 1,
          instance_id: 'instance-123456789',
          instance_name: 'farm-ubuntu-instance-1234',
          pool: 'ubuntu',
          kind: 'state_changed',
          from_state: 'ready',
          to_state: 'running',
          created_at: '2026-09-21T10:00:00Z',
        },
      ]),
    )
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/activity', component: ActivityView },
        { path: '/instances/:id', component: { template: '<div />' } },
        { path: '/pools/:name', component: { template: '<div />' } },
      ],
    })
    await router.push('/activity')
    await router.isReady()

    const wrapper = mount(ActivityView, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    expect(wrapper.get('a[href="/instances/instance-123456789"]').text()).toBe(
      'farm-ubuntu-instance-1234',
    )
    expect(wrapper.text()).toContain('changed from ready to running')
    wrapper.unmount()
  })
})
