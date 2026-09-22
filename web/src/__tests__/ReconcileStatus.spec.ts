import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import ReconcileStatus from '../components/ReconcileStatus.vue'

describe('ReconcileStatus', () => {
  it('renders running reconciliation as information', () => {
    const wrapper = mount(ReconcileStatus, {
      props: {
        reconciliation: {
          phase: 'running',
          condition: 'healthy',
          started_at: new Date().toISOString(),
          active_pools: 2,
          total_pools: 3,
          active_failures: 0,
        },
      },
    })

    expect(wrapper.text()).toContain('Reconciliation in progress')
    expect(wrapper.text()).toContain('2 active pools')
    expect(wrapper.classes()).toContain('bg-blue-50')
    expect(wrapper.text()).not.toContain('errors')
  })
})
