import { describe, expect, it } from 'vitest'
import { matchRoutes } from 'react-router-dom'
import { routes } from './routes'

describe('global run routes', () => {
  it('keeps login public and application pages under the authentication boundary', () => {
    expect(matchRoutes(routes, '/login')?.map((match) => match.route.id)).toContain('login')
    expect(matchRoutes(routes, '/dashboard')?.map((match) => match.route.id)).toContain('authenticated')
  })

  it('matches the global run list', () => {
    const matches = matchRoutes(routes, '/runs')
    expect(matches?.at(-1)?.route.path).toBe('runs')
  })

  it('matches a run detail without entering a system workspace', () => {
    const matches = matchRoutes(routes, '/runs/run-20260711')
    expect(matches?.at(-1)?.route.path).toBe('runs/:runId')
  })

  it('matches a run detail inside its business system', () => {
    const matches = matchRoutes(routes, '/systems/system-a/runs/run-1')
    expect(matches?.at(-1)?.route.path).toBe('runs/:runId')
  })
})
