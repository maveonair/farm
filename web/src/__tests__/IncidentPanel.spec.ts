import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import IncidentPanel from '../components/IncidentPanel.vue'

describe('IncidentPanel', () => {
  it('shows the failure cause and occurrence count', () => {
    const wrapper = mount(IncidentPanel, {
      props: {
        incident: {
          id: 1,
          pool: 'ubuntu',
          run_id: 'run',
          stage: 'fetch_jobs',
          code: 'unavailable',
          message: 'Forgejo returned HTTP 503',
          first_seen_at: '2026-09-19T08:00:00Z',
          last_seen_at: '2026-09-19T08:01:00Z',
          occurrences: 3,
        },
      },
      global: { stubs: { RouterLink: true } },
    })

    expect(wrapper.text()).toContain('fetch jobs')
    expect(wrapper.text()).toContain('Forgejo returned HTTP 503')
    expect(wrapper.text()).toContain('3')
  })
})
