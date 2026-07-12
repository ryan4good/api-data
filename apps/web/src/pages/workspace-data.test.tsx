import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { BusinessSystem } from '../api/types'
import { loadWorkspaceResources, SystemWorkspaceView, type WorkspaceResourcesState } from './system'

const system: BusinessSystem = {
  id: 'system-a', code: 'oms', name: '订单中心', status: 'active', myRole: 'owner',
  createdAt: '2026-07-11T10:00:00Z', updatedAt: '2026-07-11T10:00:00Z',
}

function render(resources: WorkspaceResourcesState) {
  return renderToStaticMarkup(<MemoryRouter><SystemWorkspaceView state={{ status: 'ready', system }} resources={resources} /></MemoryRouter>)
}

describe('workspace resource data', () => {
  it('shows independent loading states', () => {
    const html = render({
      scans: { status: 'loading' }, operations: { status: 'loading' }, imports: { status: 'loading' },
      scenarios: { status: 'loading' }, runs: { status: 'loading' },
    })

    expect(html).toContain('正在加载扫描记录')
    expect(html).toContain('正在加载 API 资产')
    expect(html).toContain('正在加载场景导入记录')
    expect(html).toContain('正在加载业务场景')
    expect(html).toContain('正在加载运行记录')
  })

  it('preserves successful resources when another resource fails', () => {
    const html = render({
      scans: { status: 'error', message: '扫描服务暂不可用' },
      operations: { status: 'ready', items: [{ id: 'op-1', systemId: 'system-a', scanRunId: 'scan-1', operationKey: 'GET /orders', method: 'GET', path: '/orders', verificationStatus: 'verified', lifecycleStatus: 'active', contentHash: 'hash', createdAt: '2026-07-11T10:00:00Z', updatedAt: '2026-07-11T10:00:00Z' }] },
      imports: { status: 'ready', items: [] },
      scenarios: { status: 'error', message: '场景列表加载失败' },
      runs: { status: 'ready', items: [{
        id: 'run-1', systemId: 'system-a', scenarioId: 'scenario-1', scenarioVersionId: 'version-1', environmentId: 'environment-1',
        status: 'finished', outcome: 'passed', triggerType: 'manual', summary: { totalSteps: 2, executedSteps: 2 }, requestedBy: 'user-1',
        startedAt: '2026-07-11T10:00:00Z', finishedAt: '2026-07-11T10:00:01Z', createdAt: '2026-07-11T10:00:00Z', attempts: [],
      }] },
    })

    expect(html).toContain('扫描服务暂不可用')
    expect(html).toContain('API 资产 1 个')
    expect(html).toContain('GET')
    expect(html).toContain('/orders')
    expect(html).toContain('尚无场景导入记录')
    expect(html).toContain('场景列表加载失败')
    expect(html).toContain('运行记录 1 条')
    expect(html).toContain('finished · passed')
  })

  it('shows real scenario and run counts while limiting each preview to three items', () => {
    const html = render({
      scans: { status: 'ready', items: [] }, operations: { status: 'ready', items: [] }, imports: { status: 'ready', items: [] },
      scenarios: { status: 'ready', items: [
        { id: 'scenario-1', systemId: 'system-a', name: '创建订单', status: 'active' },
        { id: 'scenario-2', systemId: 'system-a', name: '支付订单', status: 'draft' },
        { id: 'scenario-3', systemId: 'system-a', name: '取消订单', status: 'active' },
        { id: 'scenario-4', systemId: 'system-a', name: '隐藏的第四项', status: 'active' },
      ] },
      runs: { status: 'ready', items: [
        { id: 'run-1', systemId: 'system-a', scenarioId: 'scenario-1', scenarioVersionId: 'version-1', environmentId: 'environment-1', status: 'finished', outcome: 'passed', triggerType: 'manual', summary: { totalSteps: 1, executedSteps: 1 }, requestedBy: 'user-1', startedAt: '2026-07-11T10:00:00Z', finishedAt: '2026-07-11T10:00:01Z', createdAt: '2026-07-11T10:00:00Z', attempts: [] },
      ] },
    })

    expect(html).toContain('业务场景 4 个')
    expect(html).toContain('创建订单 · active')
    expect(html).toContain('支付订单 · draft')
    expect(html).not.toContain('隐藏的第四项')
    expect(html).toContain('运行记录 1 条')
    expect(html).toContain('finished · passed')
  })

  it('loads all resources concurrently and keeps partial failures', async () => {
    const listScans = vi.fn().mockRejectedValue(new Error('scan failed'))
    const listApiOperations = vi.fn().mockResolvedValue([{ id: 'op-1' }])
    const listScenarioImports = vi.fn().mockResolvedValue([{ id: 'import-1' }])
    const listScenarios = vi.fn().mockResolvedValue([{ id: 'scenario-1' }])
    const listScenarioRuns = vi.fn().mockRejectedValue(new Error('run failed'))

    const result = await loadWorkspaceResources({ listScans, listApiOperations, listScenarioImports, listScenarios, listScenarioRuns }, 'system-a')

    expect(listScans).toHaveBeenCalledWith('system-a')
    expect(listApiOperations).toHaveBeenCalledWith('system-a')
    expect(listScenarioImports).toHaveBeenCalledWith('system-a')
    expect(listScenarios).toHaveBeenCalledWith('system-a')
    expect(listScenarioRuns).toHaveBeenCalledWith('system-a')
    expect(result.scans.status).toBe('error')
    expect(result.operations).toMatchObject({ status: 'ready', items: [{ id: 'op-1' }] })
    expect(result.imports).toMatchObject({ status: 'ready', items: [{ id: 'import-1' }] })
    expect(result.scenarios).toMatchObject({ status: 'ready', items: [{ id: 'scenario-1' }] })
    expect(result.runs.status).toBe('error')
  })
})
