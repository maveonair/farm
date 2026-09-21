import { mount } from '@vue/test-utils'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import App from '../App.vue'
import { api } from '../api/client'
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
})
