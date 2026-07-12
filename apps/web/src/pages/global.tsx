import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { BusinessSystem, SystemRole } from '../api/types'
import { Page, PlaceholderPanel } from '../components/Page'

export function SystemsPage() {
  const [state, setState] = useState<SystemsPageState>({ status: 'loading' })

  useEffect(() => {
    let active = true
    apiClient.listSystems().then((systems) => {
      if (!active) return
      setState(systems.length === 0 ? { status: 'empty' } : { status: 'ready', systems })
    }).catch((error: unknown) => {
      if (!active) return
      setState({ status: 'error', message: error instanceof Error ? error.message : '加载业务系统失败' })
    })
    return () => { active = false }
  }, [])

  return <SystemsPageView {...state} />
}

export type SystemsPageState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; systems: BusinessSystem[] }

const roleLabels: Record<SystemRole, string> = {
  owner: '所有者',
  maintainer: '维护者',
  reviewer: '核验者',
  runner: '执行者',
  viewer: '只读成员',
}

export function SystemsPageView(state: SystemsPageState) {
  let content
  if (state.status === 'loading') {
    content = <div className="system-list-state" role="status">正在加载授权业务系统…</div>
  } else if (state.status === 'error') {
    content = <div className="system-list-state error-state" role="alert"><strong>业务系统加载失败</strong><p>{state.message}</p></div>
  } else if (state.status === 'empty') {
    content = <div className="system-list-state"><strong>暂无授权业务系统</strong><p>请联系系统所有者添加成员权限。</p></div>
  } else {
    content = (
      <div className="card-grid">
        {state.systems.map((system) => (
          <Link key={system.id} to={`/systems/${system.id}/overview`} className="system-card">
            <span className="system-icon">{system.name.slice(0, 1)}</span>
            <div>
              <h2>{system.name}</h2>
              <p>{system.code}</p>
              <small>{system.description || (system.status === 'active' ? '运行中' : '已归档')}</small>
            </div>
            <span className="role-badge">{roleLabels[system.myRole]}</span>
            <b aria-hidden="true">→</b>
          </Link>
        ))}
      </div>
    )
  }

  return (
    <Page eyebrow="平台全局视角" title="业务系统" description="统一管理代码仓库、环境连接和数据访问边界。">
      {content}
    </Page>
  )
}

export function GlobalSettingsPage() {
  return <Page eyebrow="平台全局视角" title="平台设置" description="管理平台成员、全局角色、扫描插件和审计策略。"><PlaceholderPanel title="基础设置"><p className="muted">平台级设置表单将在权限模块接入后开放。</p></PlaceholderPanel></Page>
}
