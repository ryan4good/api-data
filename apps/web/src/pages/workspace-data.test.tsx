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
    })

    expect(html).toContain('正在加载扫描记录')
    expect(html).toContain('正在加载 API 资产')
    expect(html).toContain('正在加载场景导入记录')
  })

  it('preserves successful resources when another resource fails', () => {
    const html = render({
      scans: { status: 'error', message: '扫描服务暂不可用' },
      operations: { status: 'ready', items: [{ id: 'op-1', systemId: 'system-a', scanRunId: 'scan-1', operationKey: 'GET /orders', method: 'GET', path: '/orders', verificationStatus: 'verified', lifecycleStatus: 'active', contentHash: 'hash', createdAt: '2026-07-11T10:00:00Z', updatedAt: '2026-07-11T10:00:00Z' }] },
      imports: { status: 'ready', items: [] },
    })

    expect(html).toContain('扫描服务暂不可用')
    expect(html).toContain('API 资产 1 个')
    expect(html).toContain('GET')
    expect(html).toContain('/orders')
    expect(html).toContain('尚无场景导入记录')
  })

  it('loads all resources concurrently and keeps partial failures', async () => {
    const listScans = vi.fn().mockRejectedValue(new Error('scan failed'))
    const listApiOperations = vi.fn().mockResolvedValue([{ id: 'op-1' }])
    const listScenarioImports = vi.fn().mockResolvedValue([{ id: 'import-1' }])

    const result = await loadWorkspaceResources({ listScans, listApiOperations, listScenarioImports }, 'system-a')

    expect(listScans).toHaveBeenCalledWith('system-a')
    expect(listApiOperations).toHaveBeenCalledWith('system-a')
    expect(listScenarioImports).toHaveBeenCalledWith('system-a')
    expect(result.scans.status).toBe('error')
    expect(result.operations).toMatchObject({ status: 'ready', items: [{ id: 'op-1' }] })
    expect(result.imports).toMatchObject({ status: 'ready', items: [{ id: 'import-1' }] })
  })
})
