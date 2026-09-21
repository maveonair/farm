import { flushPromises, mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/api/client'
import ActivityView from '@/views/ActivityView.vue'
import InstancesView from '@/views/InstancesView.vue'
import { newTestQueryClient } from './query'

function routerFor(component: object) {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/view', component }],
  })
}

describe('route filters', () => {
  afterEach(() => vi.restoreAllMocks())

  it('reads and updates instance filters in the URL', async () => {
    vi.spyOn(api, 'pools').mockResolvedValue([])
    const getInstances = vi.spyOn(api, 'instances').mockResolvedValue([])
    const router = routerFor(InstancesView)
    await router.push('/view?pool=ubuntu&state=running')
    await router.isReady()

    const wrapper = mount(InstancesView, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    expect(getInstances.mock.calls[0]?.[0]?.get('pool')).toBe('ubuntu')
    expect(getInstances.mock.calls[0]?.[0]?.get('state')).toBe('running')

    await wrapper.findAll('select')[1]!.setValue('ready')
    await flushPromises()

    expect(router.currentRoute.value.query).toEqual({ pool: 'ubuntu', state: 'ready' })
    wrapper.unmount()
  })

  it('reads and updates the activity pool in the URL', async () => {
    vi.spyOn(api, 'pools').mockResolvedValue([])
    const getEvents = vi.spyOn(api, 'events').mockResolvedValue([])
    const router = routerFor(ActivityView)
    await router.push('/view?pool=ubuntu')
    await router.isReady()

    const wrapper = mount(ActivityView, {
      global: {
        plugins: [router, [VueQueryPlugin, { queryClient: newTestQueryClient() }]],
      },
    })
    await flushPromises()

    expect(getEvents.mock.calls[0]?.[0]?.get('pool')).toBe('ubuntu')

    await wrapper.get('select').setValue('')
    await flushPromises()

    expect(router.currentRoute.value.query).toEqual({})
    wrapper.unmount()
  })
})
