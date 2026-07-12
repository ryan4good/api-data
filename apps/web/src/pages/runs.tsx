import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useOutletContext, useParams } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { CreateScenarioRunInput, ScenarioDetail, ScenarioRunDetail, ScenarioSummary, SystemRole } from '../api/types'
import { Page } from '../components/Page'
import type { SystemContextState } from '../layouts/SystemLayout'
import type { PageState } from './discovery'
import { createSubmissionGuard } from './workflow-mutations'

interface Feedback { status: 'idle' | 'submitting' | 'success' | 'error'; message?: string }
type DetailState = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready'; run: ScenarioRunDetail }

const canRun = (role: SystemRole) => role === 'owner' || role === 'maintainer' || role === 'runner'

export function parseInputVariables(source: string): Record<string, unknown> {
  if (!source.trim()) return {}
  try {
    const value: unknown = JSON.parse(source)
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid')
    return value as Record<string, unknown>
  } catch {
    throw new Error('输入变量必须是 JSON 对象')
  }
}

export function RunsPageView({ role, state, scenarios = { status: 'error', message: '场景目录接口尚未就绪' }, scenarioDetails = { status: 'error', message: '场景详情尚未就绪' }, onExecute, onValidationError, feedback = { status: 'idle' } }: {
  role: SystemRole
  state: PageState<ScenarioRunDetail>
  scenarios?: PageState<ScenarioSummary>
  scenarioDetails?: PageState<ScenarioDetail>
  onExecute: (input: CreateScenarioRunInput) => void | Promise<void>
  onValidationError?: (message: string) => void
  feedback?: Feedback
}) {
  const initialScenarioId = scenarioDetails.status === 'ready' ? scenarioDetails.items[0]?.scenario.id ?? '' : ''
  const [selectedScenarioId, setSelectedScenarioId] = useState(initialScenarioId)
  const selectedDetail = scenarioDetails.status === 'ready' ? scenarioDetails.items.find((item) => item.scenario.id === selectedScenarioId) : undefined

  useEffect(() => {
    if (!selectedScenarioId && scenarioDetails.status === 'ready' && scenarioDetails.items[0]) setSelectedScenarioId(scenarioDetails.items[0].scenario.id)
  }, [scenarioDetails, selectedScenarioId])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    try {
      void onExecute({
        scenarioId: String(form.get('scenarioId') ?? '').trim(),
        scenarioVersionId: String(form.get('scenarioVersionId') ?? '').trim(),
        environmentId: String(form.get('environmentId') ?? '').trim(),
        stopAfterStepId: String(form.get('stopAfterStepId') ?? '').trim() || undefined,
        inputVariables: parseInputVariables(String(form.get('inputVariables') ?? '')),
      })
    } catch (reason) {
      onValidationError?.(reason instanceof Error ? reason.message : '输入变量必须是 JSON 对象')
    }
  }
  return <Page eyebrow="单系统工作区" title="运行记录" description="执行完整场景或仅执行到指定步骤，并查看当前系统的历史结果。">
    {canRun(role) && <form className="workflow-form" onSubmit={submit}>
      <h2>执行场景</h2>
      {scenarios.status === 'ready' && scenarios.items.length > 0 ? <label>场景<select name="scenarioId" required value={selectedScenarioId || scenarios.items[0].id} onChange={(event) => setSelectedScenarioId(event.target.value)}>{scenarios.items.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label> : <><div className="honest-state">{scenarios.status === 'loading' ? '正在加载场景目录…' : scenarios.status === 'empty' ? '场景目录为空，可使用已知 ID 调试。' : scenarios.status === 'error' ? scenarios.message : ''}</div><label>场景 ID<input name="scenarioId" required /></label></>}
      {selectedDetail ? <label>场景版本<select name="scenarioVersionId" required><option value={selectedDetail.version.id}>版本 {selectedDetail.version.versionNo}</option></select></label> : <label>场景版本 ID<input name="scenarioVersionId" required /></label>}
      <label>环境 ID<input name="environmentId" required /></label>
      {selectedDetail ? <label>执行到指定步骤<select name="stopAfterStepId"><option value="">完整执行</option>{selectedDetail.steps.map((step) => <option key={step.id} value={step.id}>{step.name}（步骤 {step.position}）</option>)}</select></label> : <label>执行到指定步骤<input name="stopAfterStepId" placeholder="stopAfterStepId（留空则完整执行）" /></label>}
      <label>输入变量<textarea name="inputVariables" rows={5} placeholder={'inputVariables，例如 {"orderId":"A1"}'} /></label>
      <button disabled={feedback.status === 'submitting'} type="submit">{feedback.status === 'submitting' ? '执行中…' : '开始执行'}</button>
    </form>}
    {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`}>{feedback.message}</div>}
    {state.status === 'loading' && <div className="workspace-state">正在加载运行记录…</div>}
    {state.status === 'error' && <div className="workspace-state error-state"><strong>{state.message}</strong></div>}
    {state.status === 'empty' && <div className="workspace-state">尚无运行记录</div>}
    {state.status === 'ready' && <div className="record-list run-list">{state.items.map((run) => <Link key={run.id} to={run.id}><article><strong>{run.scenarioId}</strong><span>{run.status} · {run.outcome} · {run.summary.executedSteps}/{run.summary.totalSteps} 步</span></article></Link>)}</div>}
  </Page>
}

function stateFor<T>(items: T[]): PageState<T> { return items.length ? { status: 'ready', items } : { status: 'empty' } }

export function RunsPage() {
  const context = useOutletContext<SystemContextState>()
  const system = context?.status === 'ready' ? context.system : undefined
  const [state, setState] = useState<PageState<ScenarioRunDetail>>({ status: 'loading' })
  const [scenarios, setScenarios] = useState<PageState<ScenarioSummary>>({ status: 'loading' })
  const [scenarioDetails, setScenarioDetails] = useState<PageState<ScenarioDetail>>({ status: 'loading' })
  const [feedback, setFeedback] = useState<Feedback>({ status: 'idle' })
  const guard = useRef(createSubmissionGuard()).current

  async function refreshRuns() {
    if (!system) return
    const items = await apiClient.listScenarioRuns(system.id)
    setState(stateFor(Array.isArray(items) ? items : []))
  }
  useEffect(() => {
    let active = true
    if (!system) return () => { active = false }
    setState({ status: 'loading' }); setScenarios({ status: 'loading' }); setScenarioDetails({ status: 'loading' })
    apiClient.listScenarioRuns(system.id).then((items) => { if (active) setState(stateFor(Array.isArray(items) ? items : [])) }).catch(() => { if (active) setState({ status: 'error', message: '运行记录加载失败' }) })
    apiClient.listScenarios(system.id).then(async (items) => {
      if (!active) return
      const list = Array.isArray(items) ? items : []
      setScenarios(stateFor(list))
      if (list.length === 0) { setScenarioDetails({ status: 'empty' }); return }
      const settled = await Promise.allSettled(list.map((item) => apiClient.getScenario(system.id, item.id)))
      if (!active) return
      const details = settled.flatMap((result) => result.status === 'fulfilled' ? [result.value] : [])
      setScenarioDetails(details.length ? { status: 'ready', items: details } : { status: 'error', message: '场景详情加载失败，可手工输入版本和步骤 ID' })
    }).catch(() => { if (active) { setScenarios({ status: 'error', message: '场景目录接口尚未就绪' }); setScenarioDetails({ status: 'error', message: '场景详情尚未就绪' }) } })
    return () => { active = false }
  }, [system?.id])

  async function execute(input: CreateScenarioRunInput) {
    if (!system || guard.pending()) return
    let variables: Record<string, unknown>
    try { variables = parseInputVariables(JSON.stringify(input.inputVariables ?? {})) } catch (reason) { setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '输入变量错误' }); return }
    setFeedback({ status: 'submitting', message: '正在执行场景…' })
    try {
      await guard.run(() => apiClient.createScenarioRun(system.id, { ...input, inputVariables: variables }))
      setFeedback({ status: 'success', message: '场景执行完成，已刷新运行记录。' })
      await refreshRuns()
    } catch (reason) {
      setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '场景执行失败' })
    }
  }
  if (!system) return <RunsPageView role="viewer" state={{ status: 'loading' }} onExecute={execute} />
  return <RunsPageView role={system.myRole} state={state} scenarios={scenarios} scenarioDetails={scenarioDetails} feedback={feedback} onExecute={execute} onValidationError={(message) => setFeedback({ status: 'error', message })} />
}

export function RunDetailView({ role, state, onRetry, feedback = { status: 'idle' } }: { role: SystemRole; state: DetailState; onRetry: (stepId: string) => void | Promise<void>; feedback?: Feedback }) {
  if (state.status === 'loading') return <Page title="运行详情" description="正在读取步骤记录。"><div className="workspace-state">正在加载运行详情…</div></Page>
  if (state.status === 'error') return <Page title="运行详情" description="运行详情不可用。"><div className="workspace-state error-state"><strong>{state.message}</strong></div></Page>
  const { run } = state
  return <Page eyebrow={`运行 ${run.id}`} title="运行详情" description={`${run.status} · ${run.outcome} · 已执行 ${run.summary.executedSteps}/${run.summary.totalSteps} 步`}>
    {run.summary.stopAfterStepId && <div className="permission-note">本次执行截止到步骤 {run.summary.stopAfterStepId}</div>}
    {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`}>{feedback.message}</div>}
    <div className="attempt-list">{run.attempts.map((attempt) => <article key={attempt.id} className="attempt-card">
      <header><div><strong>步骤 {attempt.stepId}</strong><span>第 {attempt.attemptNo} 次尝试 · {attempt.durationMs}ms</span></div><b className={attempt.status === 'passed' ? 'success' : 'error-state'}>{attempt.status}</b></header>
      {attempt.errorMessage && <p className="error-state">{attempt.errorMessage}</p>}
      <div className="assertion-list">{attempt.assertions.map((assertion) => <div key={`${attempt.id}-${assertion.key}`}><strong>{assertion.key}</strong><span>{assertion.status}</span><small>{assertion.message || `${String(assertion.actual)} / ${String(assertion.expected)}`}</small></div>)}</div>
      {canRun(role) && <button disabled={feedback.status === 'submitting'} type="button" onClick={() => void onRetry(attempt.stepId)}>重试此步骤</button>}
    </article>)}</div>
  </Page>
}

export function RunDetailPage() {
  const context = useOutletContext<SystemContextState>()
  const system = context?.status === 'ready' ? context.system : undefined
  const { runId = '' } = useParams()
  const [state, setState] = useState<DetailState>({ status: 'loading' })
  const [feedback, setFeedback] = useState<Feedback>({ status: 'idle' })
  const guard = useRef(createSubmissionGuard()).current

  async function refresh() {
    if (!system) return
    const run = await apiClient.getScenarioRun(system.id, runId)
    setState({ status: 'ready', run })
  }
  useEffect(() => {
    let active = true
    if (!system || !runId) return () => { active = false }
    setState({ status: 'loading' })
    apiClient.getScenarioRun(system.id, runId).then((run) => { if (active) setState({ status: 'ready', run }) }).catch(() => { if (active) setState({ status: 'error', message: '运行详情加载失败' }) })
    return () => { active = false }
  }, [system?.id, runId])

  async function retry(stepId: string) {
    if (!system || guard.pending()) return
    setFeedback({ status: 'submitting', message: '正在重试步骤…' })
    try {
      await guard.run(() => apiClient.retryScenarioStep(system.id, runId, stepId))
      setFeedback({ status: 'success', message: '步骤重试完成。' })
      await refresh()
    } catch (reason) {
      setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '步骤重试失败' })
    }
  }
  if (!system) return <RunDetailView role="viewer" state={{ status: 'loading' }} onRetry={retry} />
  return <RunDetailView role={system.myRole} state={state} feedback={feedback} onRetry={retry} />
}
