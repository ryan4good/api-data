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

export interface NavGroup {
  label: string
  items: NavItem[]
}

interface AppShellProps {
  navItems?: NavItem[]
  navGroups?: NavGroup[]
  context?: ReactNode
  children: ReactNode
}

export function AppShell({ navItems = [], navGroups, context, children }: AppShellProps) {
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
        <NavigationMenu groups={navGroups ?? [{ label: '', items: navItems }]} />
        {user && <AuthenticatedUserView user={user} onLogout={logout} />}
      </aside>
      <main className="main-content">{children}</main>
    </div>
  )
}

export function NavigationMenu({ groups }: { groups: NavGroup[] }) {
  return (
    <nav className="nav-list" aria-label="主导航">
      {groups.map((group) => group.items.length > 0 && <section className="nav-group" key={group.label || 'default'}>
        {group.label && <div className="nav-group-title">{group.label}</div>}
        <div className="nav-sublist">
          {group.items.map((item) => <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => `nav-item nav-subitem${isActive ? ' active' : ''}`}>
            <span className="nav-icon" aria-hidden="true">{item.icon}</span>
            <span>{item.label}</span>
          </NavLink>)}
        </div>
      </section>)}
    </nav>
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
