import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import App from '../App.vue'
import { api, type Summary } from '../api/client'
import { newTestQueryClient } from './query'

const View = { template: '<div />' }

describe('App', () => {
  afterEach(() => vi.restoreAllMocks())

  it.each([
    ['/pools/ubuntu', '/pools'],
    ['/instances/runner-1', '/instances'],
  ])('identifies the current section for %s', async (path, link) => {
    vi.spyOn(api, 'summary').mockRejectedValue(new Error('offline'))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: View },
        { path: '/pools', component: View },
        { path: '/pools/:name', component: View },
        { path: '/instances', component: View },
        { path: '/instances/:id', component: View },
        { path: '/activity', component: View },
        { path: '/incidents', component: View },
      ],
    })
    await router.push(path)
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })

    expect(wrapper.get(`a[href="${link}"]`).attributes('aria-current')).toBe('page')
    wrapper.unmount()
  })

  it('shows the controller version', async () => {
    const summary: Summary = {
      controller: 'primary',
      version: '0.3.0',
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
    vi.spyOn(api, 'summary').mockResolvedValue(summary)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: View },
        { path: '/pools', component: View },
        { path: '/instances', component: View },
        { path: '/activity', component: View },
        { path: '/incidents', component: View },
      ],
    })
    await router.push('/')
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('primary · 0.3.0')
    wrapper.unmount()
  })
})
