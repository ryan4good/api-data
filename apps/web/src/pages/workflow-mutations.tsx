import { useRef, useState, type FormEvent } from 'react'
import type {
  ScenarioImport,
  SystemRole,
  UploadScenarioImportInput,
} from '../api/types'
import type { ResourceState } from './system'

export interface MutationFeedback {
  status: 'idle' | 'submitting' | 'success' | 'error'
  message?: string
}

export function parseScenarioDocument(source: string): unknown {
  try {
    const document: unknown = JSON.parse(source)
    if (document === null || typeof document !== 'object') throw new Error('not an object')
    return document
  } catch {
    throw new Error('场景 JSON 格式不正确')
  }
}

export function createSubmissionGuard() {
  let inFlight = false
  return {
    pending: () => inFlight,
    async run<T>(action: () => Promise<T>): Promise<T | undefined> {
      if (inFlight) return undefined
      inFlight = true
      try {
        return await action()
      } finally {
        inFlight = false
      }
    },
  }
}

export async function loadPreservingOnFailure<T>(
  previous: ResourceState<T>,
  loader: () => Promise<T[]>,
): Promise<ResourceState<T>> {
  try {
    const items = await loader()
    return { status: 'ready', items: Array.isArray(items) ? items : [] }
  } catch {
    return previous
  }
}

interface WorkflowMutationPanelProps {
  role: SystemRole
  imports: ScenarioImport[]
  onUploadImport: (input: UploadScenarioImportInput) => Promise<ScenarioImport>
  onConfirmScripts: (importId: string) => Promise<unknown>
  onApplyImport: (importId: string) => Promise<unknown>
  initialFeedback?: MutationFeedback
}

function value(form: FormData, key: string): string {
  return String(form.get(key) ?? '').trim()
}

function errorMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : '操作失败，请稍后重试'
}

export function WorkflowMutationPanel({
  role,
  imports,
  onUploadImport,
  onConfirmScripts,
  onApplyImport,
  initialFeedback = { status: 'idle' },
}: WorkflowMutationPanelProps) {
  const [feedback, setFeedback] = useState<MutationFeedback>(initialFeedback)
  const guard = useRef(createSubmissionGuard()).current
  const manages = role === 'owner' || role === 'maintainer'
  const reviews = manages || role === 'reviewer'
  const submitting = feedback.status === 'submitting'

  async function submit<T>(pendingMessage: string, successMessage: string, action: () => Promise<T>): Promise<T | undefined> {
    if (guard.pending()) return undefined
    setFeedback({ status: 'submitting', message: pendingMessage })
    try {
      const result = await guard.run(action)
      setFeedback({ status: 'success', message: successMessage })
      return result
    } catch (reason) {
      setFeedback({ status: 'error', message: errorMessage(reason) })
      return undefined
    }
  }

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    let document: unknown
    try {
      document = parseScenarioDocument(value(form, 'document'))
    } catch (reason) {
      setFeedback({ status: 'error', message: errorMessage(reason) })
      return
    }
    await submit('正在上传并分析场景…', '场景导入成功，已请求刷新导入记录。', () => onUploadImport({
      fileName: value(form, 'fileName'), document,
    }))
  }

  if (!manages && !reviews) {
    return <section className="mutation-panel permission-note">当前角色没有可执行的写操作，资源保持只读。</section>
  }

  return (
    <section className="mutation-panel" aria-label="工作流操作区">
      <div className="mutation-heading"><div><h2>工作流操作</h2><p>写操作仅作用于当前业务系统；提交成功后刷新对应资源。</p></div></div>
      {feedback.status !== 'idle' && <div className={`mutation-feedback ${feedback.status}`} role="status">{feedback.message}</div>}
      {manages && (
        <div className="mutation-forms">
          <form onSubmit={upload}>
            <h3>上传场景 JSON</h3>
            <label>文件名<input name="fileName" required placeholder="orders.postman_collection.json" /></label>
            <label>JSON 内容<textarea name="document" required rows={7} placeholder="粘贴 Scenario Bundle 或 Postman Collection JSON" /></label>
            <button type="submit" disabled={submitting}>上传并分析</button>
          </form>
        </div>
      )}
      {imports.length > 0 && (
        <div className="import-actions">
          <h3>场景导入核验与应用</h3>
          {imports.map((item) => (
            <div className="import-action-row" key={item.id}>
              <span><strong>{item.fileName}</strong><small>{item.status}</small></span>
              <div>
                {reviews && item.status === 'ready' && <button type="button" disabled={submitting} onClick={() => void submit('正在确认脚本…', '脚本已确认。', () => onConfirmScripts(item.id))}>确认脚本</button>}
                {manages && item.status === 'ready' && <button type="button" disabled={submitting} onClick={() => void submit('正在应用导入…', '导入已应用。', () => onApplyImport(item.id))}>应用导入</button>}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
