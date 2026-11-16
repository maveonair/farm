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

  async function mountNavigation() {
    vi.spyOn(api, 'summary').mockRejectedValue(new Error('offline'))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: ['/', '/pools', '/instances', '/activity', '/incidents'].map((path) => ({
        path,
        component: View,
      })),
    })
    await router.push('/')
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })

    return { wrapper, router }
  }

  it('lists the five sections in the sidebar', async () => {
    const { wrapper } = await mountNavigation()
    const navigation = wrapper.get('aside nav')

    expect(navigation.attributes('aria-label')).toBe('Primary navigation')
    expect(navigation.findAll('a').map((link) => link.text())).toEqual([
      'Overview',
      'Pools',
      'Instances',
      'Activity',
      'Incidents',
    ])
    expect(navigation.get('a[href="/"]').attributes('aria-current')).toBe('page')
    wrapper.unmount()
  })

  it('toggles mobile navigation and closes it on Escape', async () => {
    const { wrapper } = await mountNavigation()
    const toggle = wrapper.get('button[aria-controls="primary-navigation"]')

    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('aside').classes()).toContain('hidden')

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('aside').classes()).not.toContain('hidden')

    await wrapper.get('nav').trigger('keydown', { key: 'Escape' })
    expect(toggle.attributes('aria-expanded')).toBe('false')
    wrapper.unmount()
  })

  it('closes mobile navigation after navigation, including the current page', async () => {
    const { wrapper, router } = await mountNavigation()
    const toggle = wrapper.get('button[aria-controls="primary-navigation"]')

    await toggle.trigger('click')
    await wrapper.get('nav a[href="/"]').trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('false')

    await toggle.trigger('click')
    await router.push('/pools')
    await flushPromises()
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('nav a[href="/pools"]').attributes('aria-current')).toBe('page')
    wrapper.unmount()
  })

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
        active_pools: 0,
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
