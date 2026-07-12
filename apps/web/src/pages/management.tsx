import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { ManagementOverview, ManagementRisk, ManagementSystemOverview } from '../api/types'
import { MetricCard, Page } from '../components/Page'

export type ManagementState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; overview: T }

function metric(value?: number): string { return typeof value === 'number' ? value.toLocaleString('zh-CN') : '—' }
function runText(runs?: ManagementSystemOverview['runs24h']): string {
  return `成功 ${metric(runs?.succeeded)} · 失败 ${metric(runs?.failed)} · 进行中 ${metric(runs?.running)}`
}
function dateTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function Risks({ risks = [] }: { risks?: ManagementRisk[] }) {
  if (risks.length === 0) return <div className="management-empty">暂无后端返回的风险提示</div>
  return <div className="risk-list">{risks.map((risk) => <article key={risk.id}><span className={`risk-level ${risk.level}`}>{risk.level}</span><div><strong>{risk.title}</strong>{risk.systemName && <small>{risk.systemName}</small>}{risk.description && <p>{risk.description}</p>}</div></article>)}</div>
}

export function ManagementOverviewView({ state, selectedSystemId = '', onSystemFilter }: {
  state: ManagementState<ManagementOverview>
  selectedSystemId?: string
  onSystemFilter?: (systemId: string) => void
}) {
  if (state.status === 'loading') return <Page title="管理工作台" description="仅读取后端授权聚合。"><div className="workspace-state">正在加载授权管理总览…</div></Page>
  if (state.status === 'error') return <Page title="管理工作台" description="授权聚合暂不可用。"><div className="workspace-state error-state"><strong>{state.message}</strong></div></Page>
  if (state.status === 'empty') return <Page title="管理工作台" description="当前账户没有可展示的系统聚合。"><div className="workspace-state">暂无授权业务系统数据</div></Page>

  const { overview } = state
  const visibleSystems = selectedSystemId ? overview.systems.filter((system) => system.systemId === selectedSystemId) : overview.systems
  const platformScope = overview.accessScope === 'platform' || overview.accessScope === 'platform_admin'
  return <Page eyebrow="管理视角" title="管理工作台" description="统计值由后端按当前身份授权范围聚合，前端不跨系统重算。">
    <div className="management-toolbar"><span className="scope-badge">{platformScope ? '平台管理员全局范围' : '仅授权系统'}</span><label>业务系统筛选<select value={selectedSystemId} onChange={(event) => onSystemFilter?.(event.target.value)}><option value="">全部授权系统</option>{overview.systems.map((system) => <option key={system.systemId} value={system.systemId}>{system.name}</option>)}</select></label></div>
    {overview.partial && <div className="partial-warning">部分指标暂不可用：{overview.unavailableMetrics?.join('、') || '后端未返回完整聚合'}</div>}
    <div className="metric-grid management-metrics">
      <MetricCard label="有权业务系统" value={metric(overview.systemCount)} hint="由授权聚合返回" />
      <MetricCard label="API 资产" value={metric(overview.apiAssetCount)} hint="不在前端汇总" />
      <MetricCard label="P0 候选" value={metric(overview.p0CandidateCount)} hint="当前授权范围" />
      <MetricCard label="场景" value={metric(overview.scenarioCount)} hint="当前授权范围" />
    </div>
    <section className="management-run-summary"><h2>近 24 小时运行</h2><strong>{runText(overview.runs24h)}</strong></section>
    <section className="management-section"><h2>业务系统概况</h2>{visibleSystems.length === 0 ? <div className="management-empty">筛选范围内没有系统</div> : <div className="management-system-list">{visibleSystems.map((system) => <Link key={system.systemId} to={`/systems/${system.systemId}/overview`}><article><div><strong>{system.name}</strong><small>{system.code}{system.myRole ? ` · ${system.myRole}` : ''}</small></div><span>API {metric(system.apiAssetCount)}</span><span>P0 {metric(system.p0CandidateCount)}</span><span>场景 {metric(system.scenarioCount)}</span><span>{runText(system.runs24h)}</span><b>进入系统 →</b></article></Link>)}</div>}</section>
    <section className="management-section"><h2>风险提示</h2><Risks risks={overview.risks} /></section>
  </Page>
}

export function ManagementDashboardPage() {
  const [state, setState] = useState<ManagementState<ManagementOverview>>({ status: 'loading' })
  const [selectedSystemId, setSelectedSystemId] = useState('')
  useEffect(() => {
    let active = true
    apiClient.getManagementOverview().then((overview) => {
      if (!active) return
      setState(!overview.systems || overview.systems.length === 0 ? { status: 'empty' } : { status: 'ready', overview })
    }).catch((reason: unknown) => { if (active) setState({ status: 'error', message: reason instanceof Error ? reason.message : '管理总览加载失败' }) })
    return () => { active = false }
  }, [])
  return <ManagementOverviewView state={state} selectedSystemId={selectedSystemId} onSystemFilter={setSelectedSystemId} />
}

export function SystemManagementOverviewView({ state }: { state: ManagementState<ManagementSystemOverview> }) {
  if (state.status === 'loading') return <section className="system-management-summary"><div className="muted">正在加载系统管理摘要…</div></section>
  if (state.status === 'error') return <section className="system-management-summary error-state"><strong>{state.message}</strong></section>
  if (state.status === 'empty') return <section className="system-management-summary"><div className="muted">当前系统暂无管理聚合数据</div></section>
  const { overview } = state
  return <section className="system-management-summary"><header><div><h2>{overview.name}管理摘要</h2><small>由系统级授权读模型返回</small></div><Link className="secondary-action" to={`/systems/${overview.systemId}/runs`}>查看运行记录</Link></header>{overview.partial && <div className="partial-warning">部分指标暂不可用</div>}<div className="system-summary-grid"><span>API 资产<strong>{metric(overview.apiAssetCount)}</strong></span><span>P0 候选<strong>{metric(overview.p0CandidateCount)}</strong></span><span>场景<strong>{metric(overview.scenarioCount)}</strong></span><span>风险<strong>{metric(overview.riskCount)}</strong></span><span>成员<strong>{metric(overview.memberCount)}</strong></span><span>环境<strong>{metric(overview.environmentCount)}</strong></span><span>代码源<strong>{metric(overview.codeSourceCount)}</strong></span><span>最近运行<strong>{dateTime(overview.lastRunAt)}</strong></span></div><p>{runText(overview.runs24h)}</p></section>
}

export function SystemManagementOverview({ systemId }: { systemId: string }) {
  const [state, setState] = useState<ManagementState<ManagementSystemOverview>>({ status: 'loading' })
  useEffect(() => {
    let active = true
    setState({ status: 'loading' })
    apiClient.getManagementSystemOverview(systemId).then((overview) => { if (active) setState(overview ? { status: 'ready', overview } : { status: 'empty' }) }).catch((reason: unknown) => { if (active) setState({ status: 'error', message: reason instanceof Error ? reason.message : '系统管理摘要加载失败' }) })
    return () => { active = false }
  }, [systemId])
  return <SystemManagementOverviewView state={state} />
}
