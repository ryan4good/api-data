import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import type { AuthUser } from '../api/types'
import { apiClient } from '../api/client'
import { clearAuthSession, useAuthSession } from '../auth/session'

export interface NavItem {
  label: string
  to: string
  end?: boolean
  icon: string
}

interface AppShellProps {
  navItems: NavItem[]
  context?: ReactNode
  children: ReactNode
}

export function AppShell({ navItems, context, children }: AppShellProps) {
  const navigate = useNavigate()
  const user = useAuthSession()?.user

  async function logout() {
    try {
      await apiClient.logout()
    } finally {
      clearAuthSession()
      navigate('/login', { replace: true })
    }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <NavLink className="brand" to="/dashboard" aria-label="返回全局仪表盘">
          <span className="brand-mark">B</span>
          <span><strong>BizDevOps</strong><small>API Data Platform</small></span>
        </NavLink>
        {context}
        <nav className="nav-list" aria-label="主导航">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon" aria-hidden="true">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
        {user && <AuthenticatedUserView user={user} onLogout={logout} />}
      </aside>
      <main className="main-content">{children}</main>
    </div>
  )
}

interface AuthenticatedUserViewProps {
  user: AuthUser
  onLogout: () => void | Promise<void>
}

export function AuthenticatedUserView({ user, onLogout }: AuthenticatedUserViewProps) {
  const initials = user.displayName.trim().slice(0, 2).toUpperCase() || user.email.slice(0, 2).toUpperCase()
  return (
    <div className="sidebar-footer">
      <span className="avatar" aria-hidden="true">{initials}</span>
      <span className="sidebar-user"><strong>{user.displayName}</strong><small>{user.email}</small></span>
      <button className="logout-action" type="button" onClick={onLogout}>退出登录</button>
    </div>
  )
}
