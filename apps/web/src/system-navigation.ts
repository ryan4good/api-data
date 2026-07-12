import type { NavGroup } from './components/AppShell'

export type SystemModuleSegment = 'overview' | 'code-sources' | 'scan' | 'apis' | 'discovery' | 'review' | 'editor' | 'runs' | 'settings/environments' | 'settings/members'

function modulePath(systemId: string, segment: SystemModuleSegment): string {
  return `/systems/${encodeURIComponent(systemId)}/${segment}`
}

export function toSystemNavGroups(systemId: string): NavGroup[] {
  const item = (label: string, segment: SystemModuleSegment, icon: string, end = false) => ({ label, icon, end, to: modulePath(systemId, segment) })
  return [
    { label: '工作台', items: [item('系统概览', 'overview', '◫', true)] },
    { label: '资产管理', items: [item('代码源', 'code-sources', '⌘'), item('代码扫描', 'scan', '⌕'), item('API 资产', 'apis', '⇄')] },
    { label: '场景管理', items: [item('场景发现', 'discovery', '✦'), item('待核验场景', 'review', '✓'), item('场景编排', 'editor', '⌘')] },
    { label: '执行中心', items: [item('运行记录', 'runs', '▶')] },
    { label: '系统管理', items: [item('环境与密钥', 'settings/environments', '♢'), item('成员与权限', 'settings/members', '♙')] },
  ]
}
