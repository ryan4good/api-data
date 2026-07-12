import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { BusinessSystem, ScenarioRunDetail } from '../api/types'
import { MetricCard, Page, PlaceholderPanel } from '../components/Page'

export interface GlobalRunRecord {
  system: BusinessSystem
  run: ScenarioRunDetail
}

export type GlobalRunsState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; records: GlobalRunRecord[]; partial: boolean; failedSystemNames: string[] }

export type GlobalRunDetailState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; system: BusinessSystem; run: ScenarioRunDetail }

interface GlobalRunsClient {
  listSystems: () => Promise<BusinessSystem[]>
  listScenarioRuns: (systemId: string) => Promise<ScenarioRunDetail[]>
}

export async function loadGlobalRuns(client: GlobalRunsClient = apiClient): Promise<GlobalRunsState> {
  let systems: BusinessSystem[]
  try {
    systems = await client.listSystems()
  } catch (reason) {
    return { status: 'error', message: reason instanceof Error ? reason.message : '授权业务系统加载失败' }
  }
  if (systems.length === 0) return { status: 'empty' }

  const settled = await Promise.allSettled(systems.map((system) => client.listScenarioRuns(system.id)))
  const records: GlobalRunRecord[] = []
  const failedSystemNames: string[] = []
  settled.forEach((result, index) => {
    const system = systems[index]
    if (result.status === 'rejected') {
      failedSystemNames.push(system.name)
      return
    }
    const runs = Array.isArray(result.value) ? result.value : []
    records.push(...runs.map((run) => ({ system, run })))
  })

  if (failedSystemNames.length === systems.length) return { status: 'error', message: '所有授权业务系统的运行记录均加载失败' }
  records.sort((left, right) => dateValue(right.run.createdAt) - dateValue(left.run.createdAt))
  if (records.length === 0 && failedSystemNames.length === 0) return { status: 'empty' }
  return { status: 'ready', records, partial: failedSystemNames.length > 0, failedSystemNames }
}

function dateValue(value?: string): number {
  const timestamp = value ? Date.parse(value) : Number.NaN
  return Number.isFinite(timestamp) ? timestamp : 0
}

function formatDate(value?: string): string {
  if (!value || !Number.isFinite(Date.parse(value))) return '—'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function formatDuration(run: ScenarioRunDetail): string {
  const durationMs = dateValue(run.finishedAt) - dateValue(run.startedAt)
  if (durationMs < 0 || !run.startedAt || !run.finishedAt) return '—'
  if (durationMs < 1000) return `${durationMs}ms`
  return `${(durationMs / 1000).toFixed(durationMs % 1000 === 0 ? 0 : 1)}s`
}

export function GlobalRunsPageView({ state }: { state: GlobalRunsState }) {
  let content
  if (state.status === 'loading') {
    content = <div className="workspace-state" role="status">正在加载全局运行记录…</div>
  } else if (state.status === 'error') {
    content = <div className="workspace-state error-state" role="alert"><strong>全局运行记录加载失败</strong><p>{state.message}</p></div>
  } else if (state.status === 'empty') {
    content = <div className="workspace-state"><strong>暂无可查看的运行记录</strong><p>当前授权业务系统尚未产生场景运行。</p></div>
  } else {
    content = <>
      {state.partial && <div className="permission-note" role="status"><strong>部分系统运行记录暂不可用</strong><span>：{state.failedSystemNames.join('、')}</span></div>}
      {state.records.length === 0
        ? <div className="workspace-state">已加载的业务系统中暂无运行记录</div>
        : <PlaceholderPanel title={`最近执行（${state.records.length}）`}>
          <div className="table-row table-head"><span>运行 / 场景</span><span>业务系统</span><span>耗时 / 时间</span><span>状态</span></div>
          {state.records.map(({ system, run }) => <Link className="table-row" key={`${system.id}-${run.id}`} to={`/runs/${encodeURIComponent(run.id)}?systemId=${encodeURIComponent(system.id)}`}>
            <span><strong>{run.scenarioId}</strong><small>{run.id}</small></span>
            <span>{system.name}</span>
            <span>{formatDuration(run)}<small>{formatDate(run.createdAt)}</small></span>
            <span>{run.status} · {run.outcome}</span>
          </Link>)}
        </PlaceholderPanel>}
    </>
  }
  return <Page eyebrow="平台全局视角" title="全局运行记录" description="跨授权业务系统查看真实场景执行状态、耗时和结果。">{content}</Page>
}

export function GlobalRunsPage() {
  const [state, setState] = useState<GlobalRunsState>({ status: 'loading' })
  useEffect(() => {
    let active = true
    void loadGlobalRuns().then((nextState) => { if (active) setState(nextState) })
    return () => { active = false }
  }, [])
  return <GlobalRunsPageView state={state} />
}

export function GlobalRunDetailView({ state }: { state: GlobalRunDetailState }) {
  if (state.status === 'loading') return <Page title="运行详情" description="正在读取跨系统运行记录。"><div className="workspace-state" role="status">正在加载运行详情…</div></Page>
  if (state.status === 'error') return <Page title="运行详情" description="运行详情不可用。" action={<Link className="secondary-action" to="/runs">返回运行列表</Link>}><div className="workspace-state error-state" role="alert"><strong>{state.message}</strong></div></Page>
  const { run, system } = state
  return <Page eyebrow="全局运行记录 / 运行详情" title="运行详情" description={`运行标识：${run.id}`} action={<Link className="secondary-action" to="/runs">返回运行列表</Link>}>
    <div className="metric-grid">
      <MetricCard label="执行状态" value={`${run.status} · ${run.outcome}`} hint={`${run.summary.executedSteps}/${run.summary.totalSteps} 步`} />
      <MetricCard label="所属系统" value={system.name} hint={system.code} />
      <MetricCard label="执行耗时" value={formatDuration(run)} hint={`环境 ${run.environmentId}`} />
      <MetricCard label="触发方式" value={run.triggerType || '—'} hint={`请求人 ${run.requestedBy || '—'}`} />
    </div>
    <PlaceholderPanel title={`步骤尝试（${run.attempts.length}）`}>
      {run.attempts.length === 0
        ? <p className="muted">接口未返回步骤尝试记录。</p>
        : <div className="attempt-list">{run.attempts.map((attempt) => <article key={attempt.id} className="attempt-card">
          <header><div><strong>步骤 {attempt.stepId}</strong><span>位置 {attempt.position} · 第 {attempt.attemptNo} 次尝试 · {attempt.durationMs}ms</span></div><b>{attempt.status}</b></header>
          {attempt.errorMessage && <p className="error-state">{attempt.errorMessage}</p>}
          {attempt.assertions.length > 0 && <div className="assertion-list">{attempt.assertions.map((assertion) => <div key={`${attempt.id}-${assertion.id ?? assertion.key}`}><strong>{assertion.key}</strong><span>{assertion.status}</span><small>{assertion.message ?? `${String(assertion.actual ?? '—')} / ${String(assertion.expected ?? '—')}`}</small></div>)}</div>}
        </article>)}</div>}
    </PlaceholderPanel>
  </Page>
}

export function GlobalRunDetailPage() {
  const { runId = '' } = useParams()
  const [searchParams] = useSearchParams()
  const systemId = searchParams.get('systemId')?.trim() ?? ''
  const [state, setState] = useState<GlobalRunDetailState>({ status: 'loading' })

  useEffect(() => {
    let active = true
    if (!runId) {
      setState({ status: 'error', message: '缺少运行标识' })
      return () => { active = false }
    }
    if (!systemId) {
      setState({ status: 'error', message: '缺少业务系统标识，无法安全定位跨系统运行记录' })
      return () => { active = false }
    }
    setState({ status: 'loading' })
    Promise.all([apiClient.listSystems(), apiClient.getScenarioRun(systemId, runId)]).then(([systems, run]) => {
      if (!active) return
      const system = systems.find((item) => item.id === systemId)
      if (!system) {
        setState({ status: 'error', message: '当前用户无权查看该业务系统' })
        return
      }
      setState({ status: 'ready', system, run })
    }).catch((reason: unknown) => {
      if (active) setState({ status: 'error', message: reason instanceof Error ? reason.message : '运行详情加载失败' })
    })
    return () => { active = false }
  }, [runId, systemId])

  return <GlobalRunDetailView state={state} />
}
