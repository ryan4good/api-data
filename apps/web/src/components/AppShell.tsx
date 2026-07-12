import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'

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
        <div className="sidebar-footer">
          <span className="avatar">RY</span>
          <span><strong>平台管理员</strong><small>ryan@example.com</small></span>
        </div>
      </aside>
      <main className="main-content">{children}</main>
    </div>
  )
}
