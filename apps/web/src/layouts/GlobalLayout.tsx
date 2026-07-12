import { Outlet } from 'react-router-dom'
import { AppShell } from '../components/AppShell'
import { toGlobalNavItems } from '../navigation'

export function GlobalLayout() {
  return <AppShell navItems={toGlobalNavItems()}><Outlet /></AppShell>
}
