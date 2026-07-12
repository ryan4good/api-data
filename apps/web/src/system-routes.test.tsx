import { matchRoutes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { routes } from './routes'

describe('two-level system module routes', () => {
  it.each([
    ['/systems/system-a/code-sources', 'code-sources'],
    ['/systems/system-a/settings/environments', 'settings/environments'],
    ['/systems/system-a/settings/members', 'settings/members'],
  ])('matches %s as an independent second-level page', (path, routePath) => {
    expect(matchRoutes(routes, path)?.at(-1)?.route.path).toBe(routePath)
  })

  it('keeps the old settings route as a compatibility redirect page', () => {
    expect(matchRoutes(routes, '/systems/system-a/settings')?.at(-1)?.route.path).toBe('settings')
  })
})
