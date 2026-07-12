import type { NavItem } from './components/AppShell'

export type GlobalSegment = 'dashboard' | 'systems' | 'runs' | 'settings'
export type SystemSegment = 'overview' | 'scan' | 'apis' | 'discovery' | 'review' | 'editor' | 'runs' | 'settings'

export interface NavigationDefinition {
  label: string
  segment: string
  icon: string
}

export const globalNavigation: ReadonlyArray<NavigationDefinition & { segment: GlobalSegment }> = [
  { label: '全局仪表盘', segment: 'dashboard', icon: '◫' },
  { label: '业务系统', segment: 'systems', icon: '◇' },
  { label: '全局运行记录', segment: 'runs', icon: '▶' },
  { label: '平台设置', segment: 'settings', icon: '⚙' },
]

export const systemNavigation: ReadonlyArray<NavigationDefinition & { segment: SystemSegment }> = [
  { label: '系统概览', segment: 'overview', icon: '◫' },
  { label: '代码扫描', segment: 'scan', icon: '⌕' },
  { label: 'API 资产', segment: 'apis', icon: '⇄' },
  { label: '场景发现', segment: 'discovery', icon: '✦' },
  { label: '待核验场景', segment: 'review', icon: '✓' },
  { label: '场景编排', segment: 'editor', icon: '⌘' },
  { label: '运行记录', segment: 'runs', icon: '▶' },
  { label: '系统设置', segment: 'settings', icon: '⚙' },
]

export function systemPath(systemId: string, segment: SystemSegment): string {
  return `/systems/${encodeURIComponent(systemId)}/${segment}`
}

export function toGlobalNavItems(): NavItem[] {
  return globalNavigation.map(({ segment, ...item }) => ({ ...item, to: `/${segment}` }))
}

export function toSystemNavItems(systemId: string): NavItem[] {
  return systemNavigation.map(({ segment, ...item }) => ({ ...item, to: systemPath(systemId, segment) }))
}
