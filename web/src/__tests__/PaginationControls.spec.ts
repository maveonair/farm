import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'

import PaginationControls from '@/components/PaginationControls.vue'

const View = { template: '<div />' }

describe('PaginationControls', () => {
  it('navigates without keeping page one in the URL', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/view', component: View }],
    })
    await router.push('/view?pool=ubuntu&page=2')
    await router.isReady()

    const wrapper = mount(PaginationControls, {
      props: { pagination: { page: 2, per_page: 25, has_next: true } },
      global: { plugins: [router] },
    })

    await wrapper.get('button[aria-label="Previous page"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ pool: 'ubuntu' })

    await router.push('/view?pool=ubuntu&page=2')
    await wrapper.get('button[aria-label="Next page"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ pool: 'ubuntu', page: '3' })
  })

  it('hides controls when neither direction is available', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/view', component: View }],
    })
    await router.push('/view')
    await router.isReady()

    const wrapper = mount(PaginationControls, {
      props: { pagination: { page: 1, per_page: 25, has_next: false } },
      global: { plugins: [router] },
    })

    expect(wrapper.find('nav').exists()).toBe(false)
  })
})
