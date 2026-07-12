import { type FormEvent, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { ApiError, apiClient } from '../api/client'
import { establishAuthSession } from '../auth/session'

function requestedDestination(state: unknown): string {
  if (!state || typeof state !== 'object' || !('from' in state)) return '/dashboard'
  const from = (state as { from?: unknown }).from
  return typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') ? from : '/dashboard'
}

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const destination = requestedDestination(location.state)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      const result = await apiClient.login({ email: email.trim(), password })
      establishAuthSession(result)
      navigate(destination, { replace: true })
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.message : '登录失败，请稍后重试')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="login-title">
        <div className="login-brand"><span className="brand-mark">B</span><strong>BizDevOps</strong></div>
        <p className="eyebrow">API Data Platform</p>
        <h1 id="login-title">登录 BizDevOps</h1>
        <p className="login-description">使用平台账号访问已授权的业务系统和运行数据。</p>
        <form onSubmit={submit}>
          <label htmlFor="login-email">邮箱</label>
          <input id="login-email" name="email" type="email" autoComplete="username" required value={email} onChange={(event) => setEmail(event.target.value)} />
          <label htmlFor="login-password">密码</label>
          <input id="login-password" name="password" type="password" autoComplete="current-password" required value={password} onChange={(event) => setPassword(event.target.value)} />
          {error && <p className="login-error" role="alert">{error}</p>}
          <button type="submit" disabled={submitting}>{submitting ? '正在登录…' : '进入平台'}</button>
        </form>
      </section>
    </main>
  )
}
