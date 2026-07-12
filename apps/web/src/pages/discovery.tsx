import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useOutletContext } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { CodeSource, CreateDiscoveryInput, DiscoveryCandidate, DiscoveryRecord, SystemRole } from '../api/types'
import { Page } from '../components/Page'
import type { SystemContextState } from '../layouts/SystemLayout'
import { createSubmissionGuard } from './workflow-mutations'

export type PageState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: T[] }

interface Feedback { status: 'idle' | 'submitting' | 'success' | 'error'; message?: string }

export type DiscoveryCodeSourceState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: CodeSource[] }

const canDiscover = (role: SystemRole) => role === 'owner' || role === 'maintainer'
const canReview = (role: SystemRole) => canDiscover(role) || role === 'reviewer'

export function discoveryNeedsCodeSource(type: CreateDiscoveryInput['type']): boolean {
  return type === 'code' || type === 'mixed'
}

export function createDiscoveryInput(form: FormData): CreateDiscoveryInput {
  const type = String(form.get('type')) as CreateDiscoveryInput['type']
  const codeSourceId = String(form.get('codeSourceId') ?? '').trim() || undefined
  if (discoveryNeedsCodeSource(type) && !codeSourceId) throw new Error('请选择代码源')
  return {
    type,
    name: String(form.get('name') ?? '').trim(),
    codeSourceId: discoveryNeedsCodeSource(type) ? codeSourceId : undefined,
    prompt: String(form.get('prompt') ?? '').trim() || undefined,
    prd: String(form.get('prd') ?? '').trim() || undefined,
  }
}

export function DiscoveryPageView({ role, state, codeSources = { status: 'loading' }, systemId, onSubmit, feedback = { status: 'idle' } }: {
  codeSources?: DiscoveryCodeSourceState
  systemId?: string
  role: SystemRole
  state: PageState<DiscoveryRecord>
  onSubmit: (input: CreateDiscoveryInput) => void | Promise<void>
  feedback?: Feedback
}) {
  const [type, setType] = useState<CreateDiscoveryInput['type']>('code')
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    void onSubmit(createDiscoveryInput(new FormData(event.currentTarget)))
  }
  const requiresSource = discoveryNeedsCodeSource(type)
  const sourceUnavailable = requiresSource && codeSources.status !== 'ready'
  return (
    <Page eyebrow="单系统工作区" title="场景发现" description="从代码、短需求、PRD 或组合输入生成可解释的 P0 场景候选。">
      {canDiscover(role) && <form className="workflow-form" onSubmit={submit}>
        <h2>新建发现任务</h2>
        <label>发现方式<select name="type" value={type} onChange={(event) => setType(event.target.value as CreateDiscoveryInput['type'])}><option value="code">仅代码</option><option value="prompt">一句话需求</option><option value="prd">PRD 文档</option><option value="mixed">混合输入</option></select></label>
        <label>任务名称<input name="name" required placeholder="订单主链路发现" /></label>
        {requiresSource && codeSources.status === 'loading' && <div className="workspace-state">正在加载代码源…</div>}
        {requiresSource && codeSources.status === 'error' && <div className="workspace-state error-state"><strong>{codeSources.message}</strong></div>}
        {requiresSource && codeSources.status === 'empty' && <div className="workspace-state">请先添加启用的代码源。{systemId && <> <Link to={`/systems/${systemId}/code-sources`}>前往代码源</Link></>}</div>}
        {requiresSource && codeSources.status === 'ready' && <label>代码源<select name="codeSourceId" required defaultValue=""><option value="" disabled>请选择代码源</option>{codeSources.items.map((source) => <option key={source.id} value={source.id}>{source.name}（{source.sourceType} · {source.defaultRef || '默认引用未设置'}）</option>)}</select></label>}
        <label>一句话需求<textarea name="prompt" rows={3} placeholder="例如：用户支付后锁定库存" /></label>
        <label>PRD 文档<textarea name="prd" rows={6} placeholder="没有 PRD 时可留空；PRD/mixed 时粘贴正文" /></label>
        <button disabled={feedback.status === 'submitting' || sourceUnavailable} type="submit">{feedback.status === 'submitting' ? '正在生成…' : '生成场景候选'}</button>
      </form>}
      {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`}>{feedback.message}</div>}
      <DiscoveryListState state={state} />
    </Page>
  )
}

function DiscoveryListState({ state }: { state: PageState<DiscoveryRecord> }) {
  if (state.status === 'loading') return <div className="workspace-state">正在加载发现记录…</div>
  if (state.status === 'error') return <div className="workspace-state error-state"><strong>{state.message}</strong></div>
  if (state.status === 'empty') return <div className="workspace-state">尚无场景发现记录</div>
  return <div className="record-list">{state.items.map((item) => <article key={item.id}><strong>{item.name}</strong><span>{item.type} · {item.status}</span></article>)}</div>
}

function toState<T>(items: T[]): PageState<T> { return items.length ? { status: 'ready', items } : { status: 'empty' } }

export function DiscoveryPage() {
  const context = useOutletContext<SystemContextState>()
  const [state, setState] = useState<PageState<DiscoveryRecord>>({ status: 'loading' })
  const [codeSources, setCodeSources] = useState<DiscoveryCodeSourceState>({ status: 'loading' })
  const [feedback, setFeedback] = useState<Feedback>({ status: 'idle' })
  const system = context?.status === 'ready' ? context.system : undefined

  async function refresh() {
    if (!system) return
    const items = await apiClient.listDiscoveries(system.id)
    setState(toState(Array.isArray(items) ? items : []))
  }

  useEffect(() => {
    let active = true
    if (!system) return () => { active = false }
    setState({ status: 'loading' })
    apiClient.listDiscoveries(system.id).then((items) => { if (active) setState(toState(Array.isArray(items) ? items : [])) }).catch(() => { if (active) setState({ status: 'error', message: '场景发现记录加载失败' }) })
    return () => { active = false }
  }, [system?.id])

  useEffect(() => {
    let active = true
    if (!system || !canDiscover(system.myRole)) {
      setCodeSources({ status: 'loading' })
      return () => { active = false }
    }
    setCodeSources({ status: 'loading' })
    apiClient.listCodeSources(system.id)
      .then((items) => {
        if (!active) return
        const enabled = (Array.isArray(items) ? items : []).filter((item) => item.status === 'active')
        setCodeSources(enabled.length ? { status: 'ready', items: enabled } : { status: 'empty' })
      })
      .catch((reason) => { if (active) setCodeSources({ status: 'error', message: reason instanceof Error ? reason.message : '代码源加载失败' }) })
    return () => { active = false }
  }, [system?.id, system?.myRole])

  async function create(input: CreateDiscoveryInput) {
    if (!system || feedback.status === 'submitting') return
    setFeedback({ status: 'submitting', message: '正在分析输入并生成候选…' })
    try {
      if ((input.type === 'code' || input.type === 'mixed')) input.operations = await apiClient.listApiOperations(system.id)
      await apiClient.createDiscovery(system.id, input)
      setFeedback({ status: 'success', message: '发现任务完成，候选已生成。' })
      await refresh()
    } catch (reason) {
      setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '发现任务失败' })
    }
  }

  if (!system) return <DiscoveryPageView role="viewer" state={{ status: 'loading' }} codeSources={codeSources} onSubmit={create} />
  return <DiscoveryPageView role={system.myRole} systemId={system.id} state={state} codeSources={codeSources} feedback={feedback} onSubmit={create} />
}

function CandidateItem({ item, writable, promotable, submitting, onReview, onPromote }: { item: DiscoveryCandidate; writable: boolean; promotable: boolean; submitting: boolean; onReview: (candidate: DiscoveryCandidate, decision: 'accept' | 'reject', note: string) => void | Promise<void>; onPromote: (candidate: DiscoveryCandidate) => void | Promise<void> }) {
  const [note, setNote] = useState('')
  return <article className="candidate-card">
    <div className="candidate-heading"><div><span className="priority-badge">{item.priority}</span><h3>{item.name}</h3></div><strong>{Math.round(item.confidence * 100)}%</strong></div>
    <p>{item.description || '暂无补充描述'}</p>
    <div className="candidate-meta"><span>{item.requiresReview ? '需要人工核验' : '无需额外核验'}</span><span>{item.reviewStatus}</span></div>
    <div className="source-refs">{item.sourceRefs.map((source) => <code key={source}>{source}</code>)}</div>
    <ol>{item.steps.map((step) => <li key={step.key}>{step.name}{step.method && <small>{step.method} {step.path}</small>}</li>)}</ol>
    {writable && item.reviewStatus === 'pending' && <div className="candidate-actions"><input aria-label="核验备注" value={note} onChange={(event) => setNote(event.target.value)} placeholder="核验备注（可选）" /><button disabled={submitting} type="button" onClick={() => void onReview(item, 'accept', note)}>接受候选</button><button disabled={submitting} className="danger-action" type="button" onClick={() => void onReview(item, 'reject', note)}>拒绝候选</button></div>}
    {promotable && item.reviewStatus === 'accepted' && <div className="candidate-actions promote-actions"><span>候选已通过核验，可以幂等发布为正式场景草稿。</span><button disabled={submitting} type="button" onClick={() => void onPromote(item)}>发布为场景</button></div>}
  </article>
}

export function CandidateQueueView({ role, state, onReview, onPromote = () => undefined, feedback = { status: 'idle' } }: {
  role: SystemRole
  state: PageState<DiscoveryCandidate>
  onReview: (candidate: DiscoveryCandidate, decision: 'accept' | 'reject', note: string) => void | Promise<void>
  onPromote?: (candidate: DiscoveryCandidate) => void | Promise<void>
  feedback?: Feedback
}) {
  return <Page eyebrow="单系统工作区" title="待核验场景" description="核对候选的优先级、置信度、来源证据和 API 步骤。">
    {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`}>{feedback.message}</div>}
    {state.status === 'loading' && <div className="workspace-state">正在加载候选场景…</div>}
    {state.status === 'error' && <div className="workspace-state error-state"><strong>{state.message}</strong></div>}
    {state.status === 'empty' && <div className="workspace-state">尚无待核验候选</div>}
    {state.status === 'ready' && <div className="candidate-list">{state.items.map((item) => <CandidateItem key={item.id} item={item} writable={canReview(role)} promotable={canDiscover(role)} submitting={feedback.status === 'submitting'} onReview={onReview} onPromote={onPromote} />)}</div>}
  </Page>
}

async function loadCandidates(systemId: string): Promise<DiscoveryCandidate[]> {
  const discoveries = await apiClient.listDiscoveries(systemId)
  const settled = await Promise.allSettled(discoveries.map((item) => apiClient.listDiscoveryCandidates(systemId, item.id)))
  return settled.flatMap((result) => result.status === 'fulfilled' && Array.isArray(result.value) ? result.value : [])
}

export function ReviewPage() {
  const context = useOutletContext<SystemContextState>()
  const system = context?.status === 'ready' ? context.system : undefined
  const [state, setState] = useState<PageState<DiscoveryCandidate>>({ status: 'loading' })
  const [feedback, setFeedback] = useState<Feedback>({ status: 'idle' })
  const guard = useRef(createSubmissionGuard()).current

  async function refresh() {
    if (!system) return
    const items = await loadCandidates(system.id)
    setState(toState(items))
  }
  useEffect(() => {
    let active = true
    if (!system) return () => { active = false }
    setState({ status: 'loading' })
    loadCandidates(system.id).then((items) => { if (active) setState(toState(items)) }).catch(() => { if (active) setState({ status: 'error', message: '候选场景加载失败' }) })
    return () => { active = false }
  }, [system?.id])

  async function review(candidate: DiscoveryCandidate, decision: 'accept' | 'reject', note: string) {
    if (!system || guard.pending()) return
    setFeedback({ status: 'submitting', message: '正在提交核验结论…' })
    try {
      await guard.run(() => apiClient.reviewCandidate(system.id, candidate.discoveryId, candidate.id, decision, { note }))
      setFeedback({ status: 'success', message: decision === 'accept' ? '候选已接受。' : '候选已拒绝。' })
      await refresh()
    } catch (reason) {
      setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '核验失败' })
    }
  }
  async function promote(candidate: DiscoveryCandidate) {
    if (!system || guard.pending()) return
    setFeedback({ status: 'submitting', message: '正在发布场景…' })
    try {
      const result = await guard.run(() => apiClient.promoteCandidate(system.id, candidate.discoveryId, candidate.id))
      setFeedback({ status: 'success', message: result?.created ? '场景发布成功，已创建首个版本和步骤。' : '场景已存在，返回原发布结果。' })
    } catch (reason) {
      setFeedback({ status: 'error', message: reason instanceof Error ? reason.message : '场景发布失败' })
    }
  }
  if (!system) return <CandidateQueueView role="viewer" state={{ status: 'loading' }} onReview={review} />
  return <CandidateQueueView role={system.myRole} state={state} feedback={feedback} onReview={review} onPromote={promote} />
}
