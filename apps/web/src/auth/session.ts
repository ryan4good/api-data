import { useSyncExternalStore } from 'react'
import type { AuthUser, LoginResult } from '../api/types'

export interface AuthSessionSnapshot {
  user: AuthUser
  expiresAt?: number
}

let currentSession: AuthSessionSnapshot | undefined
const listeners = new Set<() => void>()

function emitChange() {
  listeners.forEach((listener) => listener())
}

export function establishAuthSession(result: LoginResult, now = Date.now()): void {
  currentSession = {
    user: result.user,
    expiresAt: now + result.expiresIn * 1_000,
  }
  emitChange()
}

export function setAuthenticatedUser(user: AuthUser): void {
  currentSession = { ...currentSession, user }
  emitChange()
}

export function getAuthSessionSnapshot(): AuthSessionSnapshot | undefined {
  return currentSession
}

export function clearAuthSession(): void {
  currentSession = undefined
  emitChange()
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useAuthSession(): AuthSessionSnapshot | undefined {
  return useSyncExternalStore(subscribe, getAuthSessionSnapshot, getAuthSessionSnapshot)
}
