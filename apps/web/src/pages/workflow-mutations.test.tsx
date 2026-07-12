import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { ScenarioImport } from '../api/types'
import {
  WorkflowMutationPanel,
  createSubmissionGuard,
  loadPreservingOnFailure,
  parseScenarioDocument,
} from './workflow-mutations'

const scenarioImport: ScenarioImport = {
  id: 'import-1', systemId: 'system-a', format: 'postman_2_1', fileName: 'orders.json', status: 'ready',
  createdAt: '2026-07-12T00:00:00Z',
}

const callbacks = {
  onUploadImport: vi.fn(),
  onConfirmScripts: vi.fn(), onApplyImport: vi.fn(),
}

describe('workflow mutations', () => {
  it('rejects malformed JSON before upload', () => {
    expect(() => parseScenarioDocument('{not json')).toThrow('场景 JSON 格式不正确')
    expect(parseScenarioDocument('{"schemaVersion":"1.0"}')).toEqual({ schemaVersion: '1.0' })
  })

  it('prevents duplicate submissions while the first is pending', async () => {
    let release!: () => void
    const pending = new Promise<void>((resolve) => { release = resolve })
    const action = vi.fn(() => pending)
    const guard = createSubmissionGuard()

    const first = guard.run(action)
    const duplicate = guard.run(action)

    expect(action).toHaveBeenCalledTimes(1)
    expect(guard.pending()).toBe(true)
    release()
    await first
    await duplicate
    expect(guard.pending()).toBe(false)
  })

  it('keeps existing resource data when a refresh fails', async () => {
    const previous = { status: 'ready' as const, items: [scenarioImport] }
    await expect(loadPreservingOnFailure(previous, () => Promise.reject(new Error('offline')))).resolves.toBe(previous)
  })

  it('shows owner forms, reviewer confirmation and keeps viewers read-only', () => {
    const owner = renderToStaticMarkup(<WorkflowMutationPanel role="owner" imports={[scenarioImport]} {...callbacks} />)
    const reviewer = renderToStaticMarkup(<WorkflowMutationPanel role="reviewer" imports={[scenarioImport]} {...callbacks} />)
    const viewer = renderToStaticMarkup(<WorkflowMutationPanel role="viewer" imports={[scenarioImport]} {...callbacks} />)

    expect(owner).not.toContain('代码源 ID')
    expect(owner).toContain('上传场景 JSON')
    expect(owner).toContain('应用导入')
    expect(reviewer).toContain('确认脚本')
    expect(reviewer).not.toContain('代码源 ID')
    expect(reviewer).not.toContain('应用导入')
    expect(viewer).toContain('当前角色没有可执行的写操作')
    expect(viewer).not.toContain('<form')
  })

  it('renders submitting, success and error feedback without replacing resource summaries', () => {
    const submitting = renderToStaticMarkup(<WorkflowMutationPanel role="owner" imports={[scenarioImport]} initialFeedback={{ status: 'submitting', message: '正在提交…' }} {...callbacks} />)
    const failed = renderToStaticMarkup(<WorkflowMutationPanel role="owner" imports={[scenarioImport]} initialFeedback={{ status: 'error', message: '创建失败，已有扫描记录仍保留' }} {...callbacks} />)

    expect(submitting).toContain('正在提交')
    expect(submitting).toContain('disabled')
    expect(failed).toContain('创建失败，已有扫描记录仍保留')
    expect(failed).toContain('orders.json')
  })
})
