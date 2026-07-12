import { useEffect, useState } from 'react'
import { Link, Outlet, useParams } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { BusinessSystem, SystemRole } from '../api/types'
import { AppShell } from '../components/AppShell'
import { toSystemNavItems } from '../navigation'

export function SystemLayout() {
  const { systemId = '' } = useParams()
  const [contextState, setContextState] = useState<SystemContextState>({ status: 'loading' })

  useEffect(() => {
    let active = true
    setContextState({ status: 'loading' })
    apiClient.getSystem(systemId).then((system) => {
      if (active) setContextState({ status: 'ready', system })
    }).catch(() => {
      if (active) setContextState({ status: 'error' })
    })
    return () => { active = false }
  }, [systemId])

  return (
    <AppShell
      navItems={toSystemNavItems(systemId)}
      context={<SystemContextView {...contextState} />}
    >
      <Outlet context={contextState} />
    </AppShell>
  )
}

export type SystemContextState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; system: BusinessSystem }

const roleLabels: Record<SystemRole, string> = {
  owner: '所有者',
  maintainer: '维护者',
  reviewer: '核验者',
  runner: '执行者',
  viewer: '只读成员',
}

export function SystemContextView(state: SystemContextState) {
  return (
    <div className="system-context">
      <Link to="/systems">← 所有业务系统</Link>
      <small>当前工作空间</small>
      {state.status === 'loading' && <strong>正在加载工作空间…</strong>}
      {state.status === 'error' && <strong>工作空间不可用</strong>}
      {state.status === 'ready' && (
        <>
          <strong>{state.system.name}</strong>
          <small>{state.system.code} · {roleLabels[state.system.myRole]}</small>
          <span className="status"><i />{state.system.status === 'active' ? '运行正常' : '已归档'}</span>
        </>
      )}
    </div>
  )
}
