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
          last_outcome: 'succeeded',
          started_at: new Date().toISOString(),
          completed_pools: 1,
          total_pools: 3,
          active_failures: 0,
        },
      },
    })

    expect(wrapper.text()).toContain('Reconciliation in progress')
    expect(wrapper.classes()).toContain('bg-blue-50')
    expect(wrapper.text()).not.toContain('errors')
  })
})
