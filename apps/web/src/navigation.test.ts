import { describe, expect, it } from 'vitest'
import { globalNavigation, systemNavigation, systemPath } from './navigation'

describe('navigation contract', () => {
  it('exposes the complete global workspace navigation', () => {
    expect(globalNavigation.map((item) => item.segment)).toEqual([
      'dashboard',
      'systems',
      'runs',
      'settings',
    ])
  })

  it('exposes every required system workspace module', () => {
    expect(systemNavigation.map((item) => item.segment)).toEqual([
      'overview',
      'scan',
      'apis',
      'discovery',
      'review',
      'editor',
      'runs',
      'settings',
    ])
  })

  it('encodes system ids when building a workspace route', () => {
    expect(systemPath('order center/华东', 'apis')).toBe(
      '/systems/order%20center%2F%E5%8D%8E%E4%B8%9C/apis',
    )
  })
})
