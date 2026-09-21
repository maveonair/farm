import { describe, expect, it } from 'vitest'

import { instanceLabel } from '@/lib/format'

describe('instanceLabel', () => {
  it('prefers the external name and falls back to the ID', () => {
    expect(instanceLabel({ id: 'instance-123456789', name: 'farm-debian-eff134234' })).toBe(
      'farm-debian-eff134234',
    )
    expect(instanceLabel({ id: 'instance-123456789' })).toBe('instance-123')
    expect(instanceLabel({})).toBe('Unknown instance')
  })
})
