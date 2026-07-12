import { useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { apiClient } from '../api/client'
import { setAuthenticatedUser } from './session'

type VerificationState = 'checking' | 'authenticated' | 'unauthenticated' | 'error'

export function AuthenticatedRoute() {
  const location = useLocation()
  const [state, setState] = useState<VerificationState>('checking')

  useEffect(() => {
    if (state !== 'checking') return
    let active = true
    apiClient.getCurrentUser()
      .then((user) => {
        if (!active) return
        setAuthenticatedUser(user)
        setState('authenticated')
      })
      .catch((error: unknown) => {
        if (!active) return
        const status = typeof error === 'object' && error && 'status' in error ? error.status : undefined
        setState(status === 401 ? 'unauthenticated' : 'error')
      })
    return () => { active = false }
  }, [state])

  if (state === 'unauthenticated') {
    return <Navigate to="/login" replace state={{ from: `${location.pathname}${location.search}` }} />
  }
  if (state === 'error') {
    return (
      <div className="auth-state" role="alert">
        <strong>无法验证登录状态</strong>
        <p>请检查网络连接后重试。</p>
        <button type="button" onClick={() => setState('checking')}>重新验证</button>
      </div>
    )
  }
  if (state === 'checking') return <div className="auth-state">正在验证登录状态…</div>
  return <Outlet />
}
