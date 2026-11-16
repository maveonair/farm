import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api, type Instance } from '@/api/client'
import InstanceView from '@/views/InstanceView.vue'
import { newTestQueryClient, page } from './query'

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

describe('InstanceView', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows a breadcrumb, instance identity, and pool link', async () => {
    vi.spyOn(api, 'instance').mockResolvedValue(instance)
    vi.spyOn(api, 'instanceEvents').mockResolvedValue(page([]))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/instances/:id', component: InstanceView },
        { path: '/instances', component: { template: '<div />' } },
        { path: '/pools/:name', component: { template: '<div />' } },
      ],
    })
    await router.push('/instances/instance-1')
    await router.isReady()

    const wrapper = mount(InstanceView, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    const breadcrumb = wrapper.get('nav[aria-label="Breadcrumb"]')
    expect(breadcrumb.get('a').attributes('href')).toBe('/instances')
    expect(breadcrumb.get('[aria-current="page"]').text()).toBe(instance.name)
    expect(wrapper.get('h1').text()).toBe(instance.name)
    expect(wrapper.get('.page-description').text()).toBe(instance.id)
    expect(wrapper.get('a[href="/pools/ubuntu"]').text()).toBe('ubuntu')
    expect(wrapper.text()).toContain('No lifecycle events recorded.')
    wrapper.unmount()
  })
})
