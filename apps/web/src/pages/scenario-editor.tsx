import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useOutletContext, useParams } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { ApiClient } from '../api/client'
import type { BusinessSystem, ScenarioDetail, ScenarioStatus, ScenarioStepType, SystemRole, UpdateScenarioInput } from '../api/types'
import { Page } from '../components/Page'
import type { SystemContextState } from '../layouts/SystemLayout'
import { createSubmissionGuard } from './workflow-mutations'

export interface ScenarioStepDraft {
  key: string
  name: string
  type: ScenarioStepType
  operationId: string
  dependsOn: string
  requestConfig: string
}

export interface ScenarioDraft {
  name: string
  description: string
  status: string
  steps: ScenarioStepDraft[]
}

export type ScenarioEditorState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'no-scenario' }
  | { status: 'ready'; detail: ScenarioDetail }

export interface EditorFeedback { status: 'idle' | 'saving' | 'success' | 'error'; message?: string }

const editableRoles: SystemRole[] = ['owner', 'maintainer']
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

function canEdit(role: SystemRole): boolean {
  return editableRoles.includes(role)
}

export function detailToScenarioDraft(detail: ScenarioDetail): ScenarioDraft {
  return {
    name: detail.scenario.name,
    description: detail.scenario.description ?? '',
    status: detail.scenario.status,
    steps: detail.steps.map((step) => ({
      key: step.key,
      name: step.name,
      type: step.type as ScenarioStepType,
      operationId: step.operationId ?? '',
      dependsOn: step.dependsOn.join(', '),
      requestConfig: JSON.stringify(step.requestConfig ?? {}, null, 2),
    })),
  }
}

function dependencyKeys(value: string): string[] {
  return value.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean)
}

function requestConfigObject(value: string, stepIndex: number): Record<string, unknown> | undefined {
  if (!value.trim()) return undefined
  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    throw new Error(`步骤 ${stepIndex + 1} 的请求配置不是有效 JSON`)
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(`步骤 ${stepIndex + 1} 的请求配置必须是 JSON 对象`)
  return parsed as Record<string, unknown>
}

export function validateScenarioDraft(draft: ScenarioDraft): UpdateScenarioInput {
  const name = draft.name.trim()
  if (!name) throw new Error('场景名称不能为空')
  if (name.length > 255) throw new Error('场景名称不能超过 255 个字符')
  if (!(['draft', 'active', 'archived'] as string[]).includes(draft.status)) throw new Error('场景状态无效')

  const keys = new Set<string>()
  for (const [index, step] of draft.steps.entries()) {
    const key = step.key.trim()
    if (!key) throw new Error(`步骤 ${index + 1} 的 Key 不能为空`)
    if (keys.has(key)) throw new Error('步骤 Key 不能重复')
    keys.add(key)
    if (!step.name.trim()) throw new Error(`步骤 ${index + 1} 的名称不能为空`)
    if (!(['http', 'script', 'delay'] as string[]).includes(step.type)) throw new Error(`步骤 ${index + 1} 的类型无效`)
    if (step.operationId.trim() && !uuidPattern.test(step.operationId.trim())) throw new Error(`步骤 ${index + 1} 的 Operation ID 必须是 UUID`)
  }

  const graph = new Map<string, string[]>()
  const steps = draft.steps.map((step, index) => {
    const key = step.key.trim()
    const dependsOn = dependencyKeys(step.dependsOn)
    const uniqueDependencies = new Set(dependsOn)
    if (uniqueDependencies.size !== dependsOn.length) throw new Error(`步骤 ${index + 1} 存在重复依赖`)
    for (const dependency of dependsOn) {
      if (!keys.has(dependency)) throw new Error(`步骤 ${index + 1} 依赖了不存在的步骤 ${dependency}`)
      if (dependency === key) throw new Error(`步骤 ${index + 1} 不能依赖自身`)
    }
    graph.set(key, dependsOn)
    const requestConfig = requestConfigObject(step.requestConfig, index)
    return {
      key,
      name: step.name.trim(),
      type: step.type,
      ...(step.operationId.trim() ? { operationId: step.operationId.trim() } : {}),
      dependsOn,
      ...(requestConfig ? { requestConfig } : {}),
    }
  })

  const visiting = new Set<string>()
  const visited = new Set<string>()
  function visit(key: string): void {
    if (visiting.has(key)) throw new Error('步骤依赖存在循环依赖')
    if (visited.has(key)) return
    visiting.add(key)
    graph.get(key)?.forEach(visit)
    visiting.delete(key)
    visited.add(key)
  }
  keys.forEach(visit)

  return { name, description: draft.description.trim(), status: draft.status as ScenarioStatus, steps }
}

type ScenarioUpdateClient = Pick<ApiClient, 'updateScenario'>

export async function saveScenarioRevision(role: SystemRole, client: ScenarioUpdateClient, systemId: string, scenarioId: string, input: UpdateScenarioInput): Promise<ScenarioDetail> {
  if (!canEdit(role)) throw new Error('当前角色没有场景编辑权限')
  return client.updateScenario(systemId, scenarioId, input)
}

function ReadOnlyScenario({ detail }: { detail: ScenarioDetail }) {
  return <div className="record-list">
    <div className="permission-note">当前角色只读查看，保存和步骤变更操作未开放。</div>
    <article><strong>{detail.scenario.name}</strong><span>{detail.scenario.status} · 当前版本 {detail.version.versionNo}</span><p>{detail.scenario.description || '未填写场景描述'}</p></article>
    {detail.steps.map((step) => <article key={step.id}><strong>{step.position}. {step.name}</strong><span>{step.key} · {step.type}</span><p>Operation ID：{step.operationId || '—'} · 依赖：{step.dependsOn.join('、') || '无'}</p><pre>{JSON.stringify(step.requestConfig ?? {}, null, 2)}</pre></article>)}
  </div>
}

export function ScenarioEditorView({ system, scenarioId, state, feedback = { status: 'idle' }, onSave, onValidationError }: {
  system: BusinessSystem
  scenarioId: string
  state: ScenarioEditorState
  feedback?: EditorFeedback
  onSave: (input: UpdateScenarioInput) => void | Promise<void>
  onValidationError?: (message: string) => void
}) {
  if (state.status === 'loading') return <Page title="场景编排" description="正在读取当前场景版本。"><div className="workspace-state" role="status">正在加载场景详情…</div></Page>
  if (state.status === 'error') return <Page title="场景编排" description="场景详情不可用。"><div className="workspace-state error-state" role="alert"><strong>{state.message}</strong></div></Page>
  if (state.status === 'no-scenario' || !scenarioId) return <Page eyebrow={`${system.name} / 工作空间`} title="场景编排" description="场景必须来自已核验候选或已应用的导入。"><div className="workspace-state"><strong>当前没有直接创建空白场景的接口</strong><p>请先发布已接受的场景候选，或应用场景导入，然后从场景列表进入具体版本编辑。</p><div className="workspace-actions"><Link className="secondary-action" to={`/systems/${system.id}/review`}>前往候选核验</Link><Link className="secondary-action" to={`/systems/${system.id}/discovery`}>查看场景发现</Link></div></div></Page>

  const detail = state.detail
  if (!canEdit(system.myRole)) return <Page eyebrow={`${system.name} / 工作空间`} title="场景编排" description={`场景 ${scenarioId}`}><ReadOnlyScenario detail={detail} /></Page>
  return <EditableScenario detail={detail} feedback={feedback} onSave={onSave} onValidationError={onValidationError} />
}

function EditableScenario({ detail, feedback, onSave, onValidationError }: {
  detail: ScenarioDetail
  feedback: EditorFeedback
  onSave: (input: UpdateScenarioInput) => void | Promise<void>
  onValidationError?: (message: string) => void
}) {
  const [draft, setDraft] = useState<ScenarioDraft>(() => detailToScenarioDraft(detail))
  useEffect(() => { setDraft(detailToScenarioDraft(detail)) }, [detail])

  function updateStep(index: number, patch: Partial<ScenarioStepDraft>) {
    setDraft((current) => ({ ...current, steps: current.steps.map((step, stepIndex) => stepIndex === index ? { ...step, ...patch } : step) }))
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      void onSave(validateScenarioDraft(draft))
    } catch (reason) {
      onValidationError?.(reason instanceof Error ? reason.message : '场景内容校验失败')
    }
  }
  return <Page eyebrow={`场景 ${detail.scenario.key || detail.scenario.id}`} title="场景编排" description={`当前版本 ${detail.version.versionNo}；保存会创建不可变的新版本。`}>
    {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`} role={feedback.status === 'error' ? 'alert' : 'status'}>{feedback.message}</div>}
    <form className="workflow-form" onSubmit={submit}>
      <label>场景名称<input required maxLength={255} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
      <label>场景描述<textarea rows={3} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label>
      <label>场景状态<select value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value })}><option value="draft">draft</option><option value="active">active</option><option value="archived">archived</option></select></label>
      <div className="workspace-actions"><button type="button" onClick={() => setDraft((current) => ({ ...current, steps: [...current.steps, { key: '', name: '', type: 'http', operationId: '', dependsOn: '', requestConfig: '{}' }] }))}>新增步骤</button></div>
      <div className="attempt-list">{draft.steps.map((step, index) => <fieldset className="attempt-card" key={index}>
        <legend>步骤 {index + 1}</legend>
        <label>步骤 Key<input required value={step.key} onChange={(event) => updateStep(index, { key: event.target.value })} /></label>
        <label>步骤名称<input required value={step.name} onChange={(event) => updateStep(index, { name: event.target.value })} /></label>
        <label>步骤类型<select value={step.type} onChange={(event) => updateStep(index, { type: event.target.value as ScenarioStepType })}><option value="http">http</option><option value="script">script</option><option value="delay">delay</option></select></label>
        <label>Operation ID<input value={step.operationId} placeholder="可选 UUID" onChange={(event) => updateStep(index, { operationId: event.target.value })} /></label>
        <label>依赖步骤 Key<input value={step.dependsOn} placeholder="多个 Key 用逗号分隔" onChange={(event) => updateStep(index, { dependsOn: event.target.value })} /></label>
        <label>Request Config JSON<textarea rows={6} value={step.requestConfig} onChange={(event) => updateStep(index, { requestConfig: event.target.value })} /></label>
        <button type="button" className="secondary-action" onClick={() => setDraft((current) => ({ ...current, steps: current.steps.filter((_, stepIndex) => stepIndex !== index) }))}>删除步骤</button>
      </fieldset>)}</div>
      <button type="submit" disabled={feedback.status === 'saving'}>{feedback.status === 'saving' ? '正在保存…' : '保存为新版本'}</button>
    </form>
  </Page>
}

export function ScenarioEditorPage() {
  const context = useOutletContext<SystemContextState>()
  const { scenarioId = '' } = useParams()
  const system = context?.status === 'ready' ? context.system : undefined
  const [state, setState] = useState<ScenarioEditorState>(scenarioId ? { status: 'loading' } : { status: 'no-scenario' })
  const [feedback, setFeedback] = useState<EditorFeedback>({ status: 'idle' })
  const guard = useRef(createSubmissionGuard()).current

  useEffect(() => {
    let active = true
    if (!system) return () => { active = false }
    if (!scenarioId) {
      setState({ status: 'no-scenario' })
      return () => { active = false }
    }
    setState({ status: 'loading' })
    apiClient.getScenario(system.id, scenarioId).then((detail) => { if (active) setState({ status: 'ready', detail }) }).catch((reason: unknown) => {
      if (active) setState({ status: 'error', message: reason instanceof Error ? reason.message : '场景详情加载失败' })
    })
    return () => { active = false }
  }, [system?.id, scenarioId])

  async function save(input: UpdateScenarioInput) {
    if (!system || !scenarioId || guard.pending()) return
    setFeedback({ status: 'saving', message: '正在保存新版本…' })
    try {
      const detail = await guard.run(() => saveScenarioRevision(system.myRole, apiClient, system.id, scenarioId, input))
      if (!detail) return
      setState({ status: 'ready', detail })
      setFeedback({ status: 'success', message: `已生成版本 ${detail.version.versionNo}` })
    } catch (reason) {
      setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '场景保存失败' })
    }
  }

  if (!system) return <Page title="场景编排" description="正在加载业务系统…"><div className="workspace-state">正在加载业务系统…</div></Page>
  return <ScenarioEditorView system={system} scenarioId={scenarioId} state={state} feedback={feedback} onSave={save} onValidationError={(message) => setFeedback({ status: 'error', message })} />
}
