import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { BusinessSystem } from '../api/types'
import { SystemContextView } from './SystemLayout'

const system: BusinessSystem = {
  id: '11111111-1111-1111-1111-111111111111',
  code: 'order-center',
  name: '订单中心',
  status: 'active',
  myRole: 'viewer',
  createdAt: '2026-07-11T10:00:00Z',
  updatedAt: '2026-07-11T10:00:00Z',
}

function render(state: Parameters<typeof SystemContextView>[0]) {
  return renderToStaticMarkup(<MemoryRouter><SystemContextView {...state} /></MemoryRouter>)
}

describe('SystemContextView', () => {
  it('renders loading and error states without inventing a system name', () => {
    expect(render({ status: 'loading' })).toContain('正在加载工作空间')
    expect(render({ status: 'error' })).toContain('工作空间不可用')
  })

  it('renders the authorized system detail and role', () => {
    const html = render({ status: 'ready', system })

    expect(html).toContain('订单中心')
    expect(html).toContain('order-center')
    expect(html).toContain('只读成员')
  })
})
