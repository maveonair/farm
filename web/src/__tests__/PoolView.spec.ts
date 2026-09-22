import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api, type Instance, type Pool } from '@/api/client'
import PoolView from '@/views/PoolView.vue'
import { newTestQueryClient, page } from './query'

const pool: Pool = {
  name: 'ubuntu',
  scope: { type: 'organization', owner: 'farm' },
  labels: ['ubuntu-24.04', 'x64'],
  image: 'ubuntu-24.04',
  min_idle: 1,
  max_instances: 4,
  max_provisioning: 2,
  startup_timeout: '10m',
  idle_timeout: '5m',
  max_lifetime: '1h',
  capacity: {
    waiting: 0,
    needed: 0,
    bootstrapping: 0,
    ready: 1,
    running: 0,
    cleaning: 0,
    maximum: 4,
  },
  bootstrap: {
    attempts: 5,
    attempt_limit: 5,
    paused: true,
    retry_at: '2026-09-21T11:00:00Z',
  },
  runtime: {},
  observation_stale: false,
}

const instance: Instance = {
  id: 'instance-1',
  name: 'farm-ubuntu-instance-1',
  pool: 'ubuntu',
  state: 'ready',
  runner_id: 0,
  error: '',
  retry_count: 0,
  created_at: '2026-09-21T10:00:00Z',
  updated_at: '2026-09-21T10:00:00Z',
  state_changed_at: '2026-09-21T10:00:00Z',
}

describe('PoolView', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows labels in the configuration panel', async () => {
    vi.spyOn(api, 'pool').mockResolvedValue(pool)
    const getInstances = vi
      .spyOn(api, 'instances')
      .mockResolvedValue(page([instance], 1, false, 10))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/pools/:name', component: PoolView },
        { path: '/instances/:id', component: { template: '<div />' } },
      ],
    })
    await router.push('/pools/ubuntu')
    await router.isReady()

    const wrapper = mount(PoolView, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    const configuration = wrapper
      .findAll('section')
      .find((section) => section.text().startsWith('Configuration'))
    expect(configuration?.text()).toContain('Labels')
    expect(configuration?.text()).toContain('ubuntu-24.04')
    expect(configuration?.text()).toContain('x64')
    expect(wrapper.text()).toContain('Provisioning paused')
    expect(getInstances.mock.calls[0]?.[0]?.get('per_page')).toBe('10')
    expect(wrapper.text()).not.toContain('No instances recorded.')
    wrapper.unmount()
  })
})
