import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import StateBadge from '../components/StateBadge.vue'

describe('StateBadge', () => {
  it('renders the instance state', () => {
    const wrapper = mount(StateBadge, { props: { state: 'running' } })

    expect(wrapper.text()).toBe('running')
  })
})
