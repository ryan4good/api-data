import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { ManagementOverview, ManagementSystemOverview } from '../api/types'
import { ManagementOverviewView, SystemManagementOverviewView } from './management'

const oms: ManagementSystemOverview = {
  systemId: 'system-oms', code: 'oms', name: '订单中心', myRole: 'owner', apiAssetCount: 30,
  p0CandidateCount: 4, scenarioCount: 12, runs24h: { succeeded: 18, failed: 2, running: 1 }, riskCount: 1,
}
const wms: ManagementSystemOverview = {
  systemId: 'system-wms', code: 'wms', name: '库存中心', myRole: 'reviewer', apiAssetCount: 40,
  p0CandidateCount: 3, scenarioCount: 9, runs24h: { succeeded: 10, failed: 1, running: 0 }, riskCount: 0,
}
const overview: ManagementOverview = {
  accessScope: 'authorized', systemCount: 2, apiAssetCount: 100, p0CandidateCount: 7, scenarioCount: 21,
  runs24h: { succeeded: 28, failed: 3, running: 1 },
  risks: [{ id: 'risk-1', level: 'high', title: '库存分配 P0 场景连续失败', systemId: 'system-oms', systemName: '订单中心' }],
  systems: [oms, wms], partial: false, generatedAt: '2026-07-12T10:00:00Z',
}
const render = (node: React.ReactNode) => renderToStaticMarkup(<MemoryRouter>{node}</MemoryRouter>)

describe('management overview', () => {
  it('renders loading, error and empty states without fake metrics', () => {
    expect(render(<ManagementOverviewView state={{ status: 'loading' }} />)).toContain('正在加载授权管理总览')
    expect(render(<ManagementOverviewView state={{ status: 'error', message: '聚合服务不可用' }} />)).toContain('聚合服务不可用')
    expect(render(<ManagementOverviewView state={{ status: 'empty' }} />)).toContain('暂无授权业务系统数据')
  })

  it('uses backend aggregate totals and authorized scope without summing rows', () => {
    const html = render(<ManagementOverviewView state={{ status: 'ready', overview }} />)
    expect(html).toContain('仅授权系统')
    expect(html).toContain('100')
    expect(html).not.toContain('API 资产</span><strong>70')
    expect(html).toContain('P0 候选')
    expect(html).toContain('成功 28')
    expect(html).toContain('失败 3')
    expect(html).toContain('进行中 1')
    expect(html).toContain('库存分配 P0 场景连续失败')
  })

  it('filters only returned systems and provides workspace drill-down', () => {
    const html = render(<ManagementOverviewView state={{ status: 'ready', overview }} selectedSystemId="system-oms" />)
    expect(html).toContain('订单中心')
    expect(html).not.toContain('/systems/system-wms/overview')
    expect(html).toContain('/systems/system-oms/overview')
  })

  it('marks partial metrics as unavailable rather than zero', () => {
    const partial: ManagementOverview = { ...overview, apiAssetCount: undefined, partial: true, unavailableMetrics: ['apiAssetCount'] }
    const html = render(<ManagementOverviewView state={{ status: 'ready', overview: partial }} />)
    expect(html).toContain('部分指标暂不可用')
    expect(html).toContain('—')
  })

  it('renders a system projection that links back into the existing workspace', () => {
    const html = render(<SystemManagementOverviewView state={{ status: 'ready', overview: oms }} />)
    expect(html).toContain('订单中心管理摘要')
    expect(html).toContain('API 资产')
    expect(html).toContain('/systems/system-oms/runs')
  })
})
