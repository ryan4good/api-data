import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { ApiOperation } from '../api/types'
import { ApiAssetsPageView, filterApiOperations } from './api-assets'

type OperationWithTags = ApiOperation & { tags?: string[] }

const operations: OperationWithTags[] = [
  {
    id: 'operation-1',
    systemId: 'system-a',
    method: 'GET',
    path: '/orders/{id}',
    summary: '查询订单',
    tags: ['orders', 'read'],
    verificationStatus: 'verified',
    lifecycleStatus: 'active',
    updatedAt: '2026-07-12T08:30:00Z',
  },
  {
    id: 'operation-2',
    systemId: 'system-a',
    method: 'POST',
    path: '/payments',
  },
]

const render = (state: Parameters<typeof ApiAssetsPageView>[0]['state']) => renderToStaticMarkup(
  <ApiAssetsPageView
    state={state}
    methodFilter=""
    statusFilter=""
    pathFilter=""
    onMethodFilter={vi.fn()}
    onStatusFilter={vi.fn()}
    onPathFilter={vi.fn()}
  />,
)

describe('ApiAssetsPageView', () => {
  it('renders honest loading, error and empty states', () => {
    expect(render({ status: 'loading' })).toContain('正在加载 API 资产')
    expect(render({ status: 'error', message: 'API 资产服务不可用' })).toContain('API 资产服务不可用')
    expect(render({ status: 'empty' })).toContain('暂无 API 资产')
  })

  it('renders only backend operation fields and an honest missing-field placeholder', () => {
    const html = render({ status: 'ready', items: operations })

    expect(html).toContain('GET')
    expect(html).toContain('/orders/{id}')
    expect(html).toContain('查询订单')
    expect(html).toContain('orders')
    expect(html).toContain('verified')
    expect(html).toContain('active')
    expect(html).toContain('2026')
    expect(html).toContain('共 2 条')
    expect(html).toContain('全部方法')
    expect(html).toContain('全部状态')
    expect(html).toContain('输入路径片段')
    expect(html).toContain('—')
    expect(html).not.toContain('<button')
    expect(html).not.toContain('新建 API')
  })
})

describe('filterApiOperations', () => {
  it('filters by method, either backend status and a case-insensitive path fragment', () => {
    expect(filterApiOperations(operations, { method: 'get', status: '', path: '' })).toEqual([operations[0]])
    expect(filterApiOperations(operations, { method: '', status: 'VERIFIED', path: '' })).toEqual([operations[0]])
    expect(filterApiOperations(operations, { method: '', status: 'ACTIVE', path: '' })).toEqual([operations[0]])
    expect(filterApiOperations(operations, { method: '', status: '', path: 'ORDERS' })).toEqual([operations[0]])
    expect(filterApiOperations(operations, { method: 'POST', status: 'verified', path: '' })).toEqual([])
  })
})
