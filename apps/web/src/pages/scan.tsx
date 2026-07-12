import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useOutletContext } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { CreateScanInput, RunScanInput, ScanRun, SystemRole } from '../api/types'
import { Page } from '../components/Page'
import type { SystemContextState } from '../layouts/SystemLayout'
import { createSubmissionGuard } from './workflow-mutations'

export type ScanPageState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: ScanRun[] }

export interface ScanFeedback {
  status: 'idle' | 'submitting' | 'success' | 'error'
  message?: string
}

const canManageScans = (role: SystemRole) => role === 'owner' || role === 'maintainer'

export function scanItemsState(items: ScanRun[]): ScanPageState {
  return items.length > 0 ? { status: 'ready', items } : { status: 'empty' }
}

function optionalText(form: FormData, name: string): string | undefined {
  return String(form.get(name) ?? '').trim() || undefined
}

export function createScanInput(form: FormData): CreateScanInput {
  return {
    codeSourceId: String(form.get('codeSourceId') ?? '').trim(),
    sourceRef: optionalText(form, 'sourceRef'),
    sourceCommit: optionalText(form, 'sourceCommit'),
    language: optionalText(form, 'language'),
    framework: optionalText(form, 'framework'),
  }
}

export function runScanInput(form: FormData): RunScanInput {
  return { repositoryRoot: String(form.get('repositoryRoot') ?? '').trim() }
}

function CreateScanForm({ disabled, onCreate }: {
  disabled: boolean
  onCreate: (input: CreateScanInput) => void | Promise<void>
}) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    void onCreate(createScanInput(form))
  }

  return (
    <form className="workflow-form" onSubmit={submit}>
      <h2>创建扫描记录</h2>
      <label>代码源 ID<input name="codeSourceId" required placeholder="代码源 UUID" /></label>
      <label>源码引用<input name="sourceRef" placeholder="例如 refs/heads/main" /></label>
      <label>提交版本<input name="sourceCommit" placeholder="例如 Git commit SHA" /></label>
      <label>语言<input name="language" placeholder="例如 go" /></label>
      <label>框架<input name="framework" placeholder="例如 gin" /></label>
      <button disabled={disabled} type="submit">{disabled ? '正在提交…' : '创建扫描记录'}</button>
    </form>
  )
}

function RunScanForm({ scan, disabled, onRun }: {
  scan: ScanRun
  disabled: boolean
  onRun: (scanId: string, input: RunScanInput) => void | Promise<void>
}) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    void onRun(scan.id, runScanInput(form))
  }

  return (
    <form className="candidate-actions" onSubmit={submit}>
      <label>
        仓库目录
        <input name="repositoryRoot" required placeholder="服务器上的仓库绝对路径" />
      </label>
      <button disabled={disabled} type="submit">执行扫描</button>
    </form>
  )
}

function value(value: string | undefined): string {
  return value || '—'
}

function ScanList({ role, state, submitting, onRun }: {
  role: SystemRole
  state: ScanPageState
  submitting: boolean
  onRun: (scanId: string, input: RunScanInput) => void | Promise<void>
}) {
  if (state.status === 'loading') return <div className="workspace-state">正在加载扫描记录…</div>
  if (state.status === 'error') return <div className="workspace-state error-state"><strong>{state.message}</strong></div>
  if (state.status === 'empty') return <div className="workspace-state">尚无代码扫描记录</div>

  return (
    <div className="record-list">
      {state.items.map((scan) => (
        <article key={scan.id}>
          <strong>{value(scan.sourceRef)}</strong>
          <dl>
            <div><dt>源码引用</dt><dd>{value(scan.sourceRef)}</dd></div>
            <div><dt>提交版本</dt><dd>{value(scan.sourceCommit)}</dd></div>
            <div><dt>语言</dt><dd>{value(scan.language)}</dd></div>
            <div><dt>框架</dt><dd>{value(scan.framework)}</dd></div>
            <div><dt>状态</dt><dd>{scan.status}</dd></div>
            <div><dt>创建时间</dt><dd>{scan.createdAt}</dd></div>
          </dl>
          {scan.errorMessage && <div className="error-state"><strong>{scan.errorMessage}</strong></div>}
          {canManageScans(role) && <RunScanForm scan={scan} disabled={submitting} onRun={onRun} />}
        </article>
      ))}
    </div>
  )
}

export function ScanPageView({ role, state, onCreate, onRun, feedback = { status: 'idle' } }: {
  role: SystemRole
  state: ScanPageState
  onCreate: (input: CreateScanInput) => void | Promise<void>
  onRun: (scanId: string, input: RunScanInput) => void | Promise<void>
  feedback?: ScanFeedback
}) {
  const writable = canManageScans(role)
  return (
    <Page eyebrow="单系统工作区" title="代码扫描" description="配置代码来源并执行扫描，查看当前业务系统内的真实扫描记录。">
      {writable && <CreateScanForm disabled={feedback.status === 'submitting'} onCreate={onCreate} />}
      {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`}>{feedback.message}</div>}
      <ScanList role={role} state={state} submitting={feedback.status === 'submitting'} onRun={onRun} />
    </Page>
  )
}

function failureMessage(reason: unknown, fallback: string): string {
  return reason instanceof Error ? reason.message : fallback
}

export function ScanPage() {
  const context = useOutletContext<SystemContextState>()
  const system = context?.status === 'ready' ? context.system : undefined
  const [state, setState] = useState<ScanPageState>({ status: 'loading' })
  const [feedback, setFeedback] = useState<ScanFeedback>({ status: 'idle' })
  const guard = useRef(createSubmissionGuard()).current

  async function refresh() {
    if (!system) return
    const items = await apiClient.listScans(system.id)
    setState(scanItemsState(Array.isArray(items) ? items : []))
  }

  useEffect(() => {
    let active = true
    if (context?.status === 'error') {
      setState({ status: 'error', message: '无法加载当前业务系统' })
      return () => { active = false }
    }
    if (!system) {
      setState({ status: 'loading' })
      return () => { active = false }
    }
    setState({ status: 'loading' })
    apiClient.listScans(system.id)
      .then((items) => { if (active) setState(scanItemsState(Array.isArray(items) ? items : [])) })
      .catch((reason) => { if (active) setState({ status: 'error', message: failureMessage(reason, '扫描记录加载失败') }) })
    return () => { active = false }
  }, [context?.status, system?.id])

  async function create(input: CreateScanInput) {
    if (!system || guard.pending()) return
    setFeedback({ status: 'submitting', message: '正在创建扫描记录…' })
    try {
      await guard.run(() => apiClient.createScan(system.id, input))
      setFeedback({ status: 'success', message: '扫描记录已创建。' })
      try {
        await refresh()
      } catch (reason) {
        setFeedback({ status: 'error', message: `扫描记录已创建，但列表刷新失败：${failureMessage(reason, '未知错误')}` })
      }
    } catch (reason) {
      setFeedback({ status: 'error', message: failureMessage(reason, '扫描记录创建失败') })
    }
  }

  async function run(scanId: string, input: RunScanInput) {
    if (!system || guard.pending()) return
    setFeedback({ status: 'submitting', message: '正在执行代码扫描…' })
    try {
      const result = await guard.run(() => apiClient.runScan(system.id, scanId, input))
      setFeedback({ status: 'success', message: `扫描任务已提交，状态：${result?.status ?? '未知'}` })
      try {
        await refresh()
      } catch (reason) {
        setFeedback({ status: 'error', message: `扫描任务已提交，但列表刷新失败：${failureMessage(reason, '未知错误')}` })
      }
    } catch (reason) {
      setFeedback({ status: 'error', message: failureMessage(reason, '扫描执行失败') })
    }
  }

  return <ScanPageView role={system?.myRole ?? 'viewer'} state={state} feedback={feedback} onCreate={create} onRun={run} />
}
