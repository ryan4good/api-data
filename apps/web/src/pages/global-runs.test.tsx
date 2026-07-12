import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { BusinessSystem, ScenarioRunDetail } from '../api/types'
import { GlobalRunDetailView, GlobalRunsPageView, loadGlobalRuns } from './global-runs'

const systems: BusinessSystem[] = [
  { id: 'system-a', code: 'oms', name: '订单中心', status: 'active', myRole: 'owner', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z' },
  { id: 'system-b', code: 'crm', name: '会员中心', status: 'active', myRole: 'viewer', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z' },
]

const run: ScenarioRunDetail = {
  id: 'run/1', systemId: 'system-a', scenarioId: 'scenario-1', scenarioVersionId: 'version-1', environmentId: 'env-1',
  status: 'completed', outcome: 'succeeded', triggerType: 'manual', summary: { totalSteps: 2, executedSteps: 2 }, requestedBy: 'user-1',
  startedAt: '2026-07-12T10:00:00.000Z', finishedAt: '2026-07-12T10:00:01.500Z', createdAt: '2026-07-12T10:00:00.000Z',
  attempts: [{
    id: 'attempt-1', systemId: 'system-a', runId: 'run/1', stepId: 'step-1', attemptNo: 1, position: 1, status: 'passed',
    durationMs: 1500, assertions: [{ key: 'status-code', type: 'equals', status: 'passed', expected: 200, actual: 200 }],
    startedAt: '2026-07-12T10:00:00.000Z', finishedAt: '2026-07-12T10:00:01.500Z', createdAt: '2026-07-12T10:00:00.000Z',
  }],
}

const render = (node: React.ReactNode) => renderToStaticMarkup(<MemoryRouter>{node}</MemoryRouter>)

describe('global runs', () => {
  it('aggregates authorized systems and keeps successful results when one system fails', async () => {
    const client = {
      listSystems: vi.fn().mockResolvedValue(systems),
      listScenarioRuns: vi.fn().mockImplementation((systemId: string) => systemId === 'system-a' ? Promise.resolve([run]) : Promise.reject(new Error('forbidden'))),
    }

    const state = await loadGlobalRuns(client)

    expect(client.listScenarioRuns).toHaveBeenCalledTimes(2)
    expect(state).toMatchObject({ status: 'ready', partial: true, failedSystemNames: ['会员中心'] })
    if (state.status !== 'ready') throw new Error('expected ready state')
    expect(state.records).toEqual([{ system: systems[0], run }])
  })

  it('renders honest loading, error, empty and partial states', () => {
    expect(render(<GlobalRunsPageView state={{ status: 'loading' }} />)).toContain('正在加载全局运行记录')
    expect(render(<GlobalRunsPageView state={{ status: 'error', message: '运行聚合失败' }} />)).toContain('运行聚合失败')
    expect(render(<GlobalRunsPageView state={{ status: 'empty' }} />)).toContain('暂无可查看的运行记录')

    const html = render(<GlobalRunsPageView state={{ status: 'ready', records: [{ system: systems[0], run }], partial: true, failedSystemNames: ['会员中心'] }} />)
    expect(html).toContain('部分系统运行记录暂不可用')
    expect(html).toContain('会员中心')
    expect(html).toContain('订单中心')
    expect(html).toContain('scenario-1')
    expect(html).toContain('1.5s')
    expect(html).toContain('/runs/run%2F1?systemId=system-a')
  })

  it('renders real run detail fields and step attempts without fabricated labels', () => {
    const html = render(<GlobalRunDetailView state={{ status: 'ready', system: systems[0], run }} />)
    expect(html).toContain('run/1')
    expect(html).toContain('订单中心')
    expect(html).toContain('env-1')
    expect(html).toContain('manual')
    expect(html).toContain('step-1')
    expect(html).toContain('status-code')
    expect(html).toContain('passed')
    expect(html).not.toContain('生产验证环境')
    expect(html).not.toContain('平台管理员触发')
  })

  it('renders missing system context and backend errors explicitly', () => {
    expect(render(<GlobalRunDetailView state={{ status: 'error', message: '缺少业务系统标识' }} />)).toContain('缺少业务系统标识')
    expect(render(<GlobalRunDetailView state={{ status: 'loading' }} />)).toContain('正在加载运行详情')
  })
})
