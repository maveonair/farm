import { describe, expect, it } from 'vitest'

import { pageFrom, pageParams } from '@/lib/pagination'

describe('pagination', () => {
  it.each([
    [undefined, 1],
    ['invalid', 1],
    ['0', 1],
    ['2', 2],
    [String(Number.MAX_SAFE_INTEGER + 1), 1],
  ])('parses page %s', (value, expected) => {
    expect(pageFrom(value)).toBe(expected)
  })

  it('omits page one from request parameters', () => {
    expect(pageParams(1, 25).toString()).toBe('per_page=25')
    expect(pageParams(2, 10).toString()).toBe('per_page=10&page=2')
  })
})
