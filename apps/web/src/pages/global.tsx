import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { BusinessSystem, SystemRole } from '../api/types'
import { MetricCard, Page, PlaceholderPanel } from '../components/Page'

export function DashboardPage() {
  return (
    <Page eyebrow="平台全局视角" title="晚上好，Ryan" description="查看所有业务系统的 API 资产、场景与执行健康度。" action={<button>＋ 新建业务系统</button>}>
      <div className="metric-grid">
        <MetricCard label="业务系统" value="12" hint="10 个运行正常" />
        <MetricCard label="API 资产" value="2,486" hint="本周新增 86" />
        <MetricCard label="已发布场景" value="374" hint="29 个待核验" />
        <MetricCard label="今日执行成功率" value="98.6%" hint="共执行 1,204 次" />
      </div>
      <PlaceholderPanel title="最近业务系统">
        <div className="table-row table-head"><span>系统</span><span>API</span><span>场景</span><span>状态</span></div>
        <Link className="table-row" to="/systems/order-center/overview"><strong>订单中心</strong><span>326</span><span>48</span><span className="success">运行正常</span></Link>
        <div className="table-row"><strong>会员中心</strong><span>184</span><span>31</span><span className="success">运行正常</span></div>
      </PlaceholderPanel>
    </Page>
  )
}

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
    <Page eyebrow="平台全局视角" title="业务系统" description="统一管理代码仓库、环境连接和数据访问边界。" action={<button>＋ 新建业务系统</button>}>
      {content}
    </Page>
  )
}

export function GlobalSettingsPage() {
  return <Page eyebrow="平台全局视角" title="平台设置" description="管理平台成员、全局角色、扫描插件和审计策略。"><PlaceholderPanel title="基础设置"><p className="muted">平台级设置表单将在权限模块接入后开放。</p></PlaceholderPanel></Page>
}

export function GlobalRunsPage() {
  return (
    <Page eyebrow="平台全局视角" title="全局运行记录" description="跨业务系统查看场景执行状态、耗时和失败分布。">
      <PlaceholderPanel title="最近执行">
        <div className="table-row table-head"><span>运行 / 场景</span><span>业务系统</span><span>耗时</span><span>状态</span></div>
        <Link className="table-row" to="/runs/run-20260711"><strong>创建订单主流程</strong><span>订单中心</span><span>1.8s</span><span className="success">执行成功</span></Link>
        <div className="table-row"><strong>会员登录校验</strong><span>会员中心</span><span>0.7s</span><span className="success">执行成功</span></div>
      </PlaceholderPanel>
    </Page>
  )
}

export function GlobalRunDetailPage() {
  const { runId = '' } = useParams()
  return (
    <Page eyebrow="全局运行记录 / 运行详情" title="运行详情" description={`运行标识：${runId}`} action={<Link className="secondary-action" to="/runs">返回运行列表</Link>}>
      <div className="metric-grid">
        <MetricCard label="执行状态" value="成功" hint="所有断言已通过" />
        <MetricCard label="所属系统" value="订单中心" hint="生产验证环境" />
        <MetricCard label="执行耗时" value="1.8s" hint="共 4 个步骤" />
        <MetricCard label="触发方式" value="手动" hint="平台管理员触发" />
      </div>
      <PlaceholderPanel title="步骤明细"><p className="muted">请求、响应、变量提取与断言日志将在执行器接口接入后展示。</p></PlaceholderPanel>
    </Page>
  )
}
