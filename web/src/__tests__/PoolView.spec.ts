import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api, type Pool } from '@/api/client'
import PoolView from '@/views/PoolView.vue'
import { newTestQueryClient } from './query'

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
  runtime: {},
  observation_stale: false,
}

describe('PoolView', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows labels in the configuration panel', async () => {
    vi.spyOn(api, 'pool').mockResolvedValue(pool)
    vi.spyOn(api, 'instances').mockResolvedValue([])
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/pools/:name', component: PoolView }],
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
    wrapper.unmount()
  })
})
