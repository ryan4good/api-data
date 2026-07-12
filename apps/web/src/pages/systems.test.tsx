import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { BusinessSystem } from '../api/types'
import { SystemsPageView } from './global'

const system: BusinessSystem = {
  id: '11111111-1111-1111-1111-111111111111',
  code: 'order-center',
  name: '订单中心',
  description: '订单全生命周期服务',
  status: 'active',
  myRole: 'maintainer',
  createdAt: '2026-07-11T10:00:00Z',
  updatedAt: '2026-07-11T10:00:00Z',
}

function render(state: Parameters<typeof SystemsPageView>[0]) {
  return renderToStaticMarkup(<MemoryRouter><SystemsPageView {...state} /></MemoryRouter>)
}

describe('SystemsPageView', () => {
  it('renders loading, error and empty states', () => {
    expect(render({ status: 'loading' })).toContain('正在加载授权业务系统')
    expect(render({ status: 'error', message: '暂时无法连接服务' })).toContain('暂时无法连接服务')
    expect(render({ status: 'empty' })).toContain('暂无授权业务系统')
  })

  it('shows authorized systems, myRole and a workspace link', () => {
    const html = render({ status: 'ready', systems: [system] })

    expect(html).toContain('订单中心')
    expect(html).toContain('维护者')
    expect(html).toContain('order-center')
    expect(html).toContain('/systems/11111111-1111-1111-1111-111111111111/overview')
  })
})
