import { describe, expect, it } from 'vitest'

import { instanceLabel, scopeName } from '@/lib/format'

describe('instanceLabel', () => {
  it('prefers the external name and falls back to the ID', () => {
    expect(instanceLabel({ id: 'instance-123456789', name: 'farm-debian-eff134234' })).toBe(
      'farm-debian-eff134234',
    )
    expect(instanceLabel({ id: 'instance-123456789' })).toBe('instance-123')
    expect(instanceLabel({})).toBe('Unknown instance')
  })
})

describe('scopeName', () => {
  it.each([
    [{ type: 'user' }, 'User'],
    [{ type: 'global' }, 'Global'],
    [{ type: 'organization', owner: 'farm' }, 'Organization · farm'],
    [{ type: 'organization' }, 'Organization'],
    [{ type: 'repository', owner: 'farm', repository: 'runners' }, 'Repository · farm/runners'],
    [{ type: 'repository' }, 'Repository'],
    [{ type: 'repository', owner: 'farm' }, 'Repository'],
  ])('formats %j without repeating its type', (scope, expected) => {
    expect(scopeName(scope)).toBe(expected)
  })
})
