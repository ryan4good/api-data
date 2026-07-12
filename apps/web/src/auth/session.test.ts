import { describe, expect, it } from 'vitest'
import { clearAuthSession, establishAuthSession, getAuthSessionSnapshot, setAuthenticatedUser } from './session'

const loginResult = {
  accessToken: 'access-token',
  tokenType: 'Bearer',
  expiresIn: 3600,
  user: {
    id: 'user-1',
    email: 'admin@example.com',
    displayName: '平台管理员',
    platformRole: 'admin' as const,
  },
}

describe('authentication session', () => {
  it('keeps only user metadata in memory and never retains the returned access token', () => {
    establishAuthSession(loginResult, 1_000)

    expect(getAuthSessionSnapshot()).toEqual({ user: loginResult.user, expiresAt: 3_601_000 })
    expect(getAuthSessionSnapshot()).not.toHaveProperty('accessToken')
  })

  it('accepts a user restored by auth/me and clears it on logout', () => {
    setAuthenticatedUser(loginResult.user)
    expect(getAuthSessionSnapshot()?.user).toEqual(loginResult.user)
    clearAuthSession()
    expect(getAuthSessionSnapshot()).toBeUndefined()
  })
})
