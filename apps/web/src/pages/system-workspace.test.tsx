import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { BusinessSystem, SystemRole } from '../api/types'
import { SystemModulePageView, SystemWorkspaceView } from './system'

function system(myRole: SystemRole): BusinessSystem {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    code: 'order-center',
    name: '订单中心',
    status: 'active',
    myRole,
    createdAt: '2026-07-11T10:00:00Z',
    updatedAt: '2026-07-11T10:00:00Z',
  }
}

function render(role: SystemRole) {
  return renderToStaticMarkup(
    <MemoryRouter>
      <SystemWorkspaceView state={{ status: 'ready', system: system(role) }} />
    </MemoryRouter>,
  )
}

describe('SystemWorkspaceView', () => {
  it('renders loading and error states without inventing system statistics', () => {
    const loading = renderToStaticMarkup(<SystemWorkspaceView state={{ status: 'loading' }} />)
    const error = renderToStaticMarkup(<SystemWorkspaceView state={{ status: 'error' }} />)

    expect(loading).toContain('正在加载业务系统')
    expect(error).toContain('无法加载当前业务系统')
    expect(loading).not.toContain('326')
  })

  it('shows the current system, role and honest empty asset states', () => {
    const html = render('viewer')

    expect(html).toContain('订单中心')
    expect(html).toContain('只读成员')
    expect(html).toContain('尚无代码扫描记录')
    expect(html).toContain('尚无 API 资产')
    expect(html).toContain('尚无业务场景')
    expect(html).toContain('/systems/11111111-1111-1111-1111-111111111111/apis')
  })

  it('lets an owner scan, import, review, edit and run', () => {
    const html = render('owner')

    expect(html).toContain('开始代码扫描')
    expect(html).toContain('导入场景 JSON')
    expect(html).toContain('人工核验')
    expect(html).toContain('创建场景')
    expect(html).toContain('执行场景')
  })

  it('gives reviewers verification and runners execution without management actions', () => {
    const reviewer = render('reviewer')
    const runner = render('runner')

    expect(reviewer).toContain('人工核验')
    expect(reviewer).not.toContain('开始代码扫描')
    expect(reviewer).not.toContain('执行场景')
    expect(runner).toContain('执行场景')
    expect(runner).not.toContain('<button type="button">人工核验</button>')
    expect(runner).not.toContain('导入场景 JSON')
  })

  it('keeps viewers read-only', () => {
    const html = render('viewer')

    expect(html).toContain('当前角色仅可查看')
    expect(html).not.toContain('<button')
  })

  it('uses the authorized system context and hides forbidden module actions', () => {
    const viewer = renderToStaticMarkup(<SystemModulePageView state={{ status: 'ready', system: system('viewer') }} title="代码扫描" description="扫描代码" action="新建扫描任务" capability="manageAssets" />)
    const maintainer = renderToStaticMarkup(<SystemModulePageView state={{ status: 'ready', system: system('maintainer') }} title="代码扫描" description="扫描代码" action="新建扫描任务" capability="manageAssets" />)

    expect(viewer).toContain('订单中心 / 工作空间')
    expect(viewer).not.toContain('<button')
    expect(maintainer).toContain('<button')
    expect(maintainer).toContain('新建扫描任务')
  })
})
