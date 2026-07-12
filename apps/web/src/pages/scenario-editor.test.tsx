import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { BusinessSystem, ScenarioDetail } from '../api/types'
import { ScenarioEditorView, detailToScenarioDraft, saveScenarioRevision, validateScenarioDraft } from './scenario-editor'

const system: BusinessSystem = {
  id: '11111111-1111-4111-8111-111111111111', code: 'oms', name: '订单中心', status: 'active', myRole: 'owner',
  createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z',
}
const detail: ScenarioDetail = {
  scenario: { id: '22222222-2222-4222-8222-222222222222', systemId: system.id, key: 'place-order', name: '创建订单', description: '订单主链路', status: 'active', currentVersionId: 'version-1' },
  version: { id: 'version-1', systemId: system.id, scenarioId: '22222222-2222-4222-8222-222222222222', versionNo: 3, sourceType: 'manual', createdBy: 'user-1', createdAt: '2026-07-12T00:00:00Z' },
  steps: [
    { id: 'step-1', systemId: system.id, versionId: 'version-1', key: 'login', name: '登录', position: 1, type: 'http', dependsOn: [], requestConfig: { method: 'POST', path: '/login' }, createdAt: '2026-07-12T00:00:00Z' },
    { id: 'step-2', systemId: system.id, versionId: 'version-1', key: 'create', name: '创建订单', position: 2, type: 'http', operationId: '33333333-3333-4333-8333-333333333333', dependsOn: ['login'], requestConfig: { method: 'POST', path: '/orders' }, createdAt: '2026-07-12T00:00:00Z' },
  ],
}
const render = (node: React.ReactNode) => renderToStaticMarkup(<MemoryRouter>{node}</MemoryRouter>)

describe('scenario editor', () => {
  it('shows an honest creation path when no scenario id exists', () => {
    const html = render(<ScenarioEditorView system={system} scenarioId="" state={{ status: 'no-scenario' }} onSave={vi.fn()} />)
    expect(html).toContain('当前没有直接创建空白场景的接口')
    expect(html).toContain(`/systems/${system.id}/review`)
    expect(html).toContain(`/systems/${system.id}/discovery`)
    expect(html).not.toContain('<form')
  })

  it('renders backend detail as an editable revision for owner and maintainer', () => {
    for (const role of ['owner', 'maintainer'] as const) {
      const html = render(<ScenarioEditorView system={{ ...system, myRole: role }} scenarioId={detail.scenario.id} state={{ status: 'ready', detail }} onSave={vi.fn()} />)
      expect(html).toContain('<form')
      expect(html).toContain('创建订单')
      expect(html).toContain('订单主链路')
      expect(html).toContain('login')
      expect(html).toContain('/login')
      expect(html).toContain('新增步骤')
      expect(html).toContain('保存为新版本')
      expect(html).toContain('当前版本 3')
    }
  })

  it('keeps reviewer, runner and viewer strictly read-only', async () => {
    for (const role of ['reviewer', 'runner', 'viewer'] as const) {
      const readOnlySystem = { ...system, myRole: role }
      const html = render(<ScenarioEditorView system={readOnlySystem} scenarioId={detail.scenario.id} state={{ status: 'ready', detail }} onSave={vi.fn()} />)
      expect(html).toContain('只读查看')
      expect(html).toContain('创建订单')
      expect(html).not.toContain('<form')
      expect(html).not.toContain('保存为新版本')

      const updateScenario = vi.fn()
      await expect(saveScenarioRevision(role, { updateScenario }, system.id, detail.scenario.id, validateScenarioDraft(detailToScenarioDraft(detail)))).rejects.toThrow('没有场景编辑权限')
      expect(updateScenario).not.toHaveBeenCalled()
    }
  })

  it('validates request JSON, unique keys, dependencies and cycles before saving', () => {
    const draft = detailToScenarioDraft(detail)
    expect(validateScenarioDraft(draft)).toEqual(expect.objectContaining({
      name: '创建订单', status: 'active', steps: [
        expect.objectContaining({ key: 'login', requestConfig: { method: 'POST', path: '/login' } }),
        expect.objectContaining({ key: 'create', dependsOn: ['login'] }),
      ],
    }))
    expect(() => validateScenarioDraft({ ...draft, steps: [{ ...draft.steps[0], requestConfig: '[]' }] })).toThrow('JSON 对象')
    expect(() => validateScenarioDraft({ ...draft, steps: [draft.steps[0], { ...draft.steps[1], key: 'login' }] })).toThrow('步骤 Key 不能重复')
    expect(() => validateScenarioDraft({ ...draft, steps: [{ ...draft.steps[0], dependsOn: 'missing' }] })).toThrow('不存在的步骤')
    expect(() => validateScenarioDraft({ ...draft, steps: [{ ...draft.steps[0], dependsOn: 'create' }, { ...draft.steps[1], dependsOn: 'login' }] })).toThrow('循环依赖')
  })

  it('renders loading, backend error, validation and saving feedback', () => {
    expect(render(<ScenarioEditorView system={system} scenarioId={detail.scenario.id} state={{ status: 'loading' }} onSave={vi.fn()} />)).toContain('正在加载场景详情')
    expect(render(<ScenarioEditorView system={system} scenarioId={detail.scenario.id} state={{ status: 'error', message: '场景不存在' }} onSave={vi.fn()} />)).toContain('场景不存在')
    expect(render(<ScenarioEditorView system={system} scenarioId={detail.scenario.id} state={{ status: 'ready', detail }} feedback={{ status: 'saving', message: '正在保存' }} onSave={vi.fn()} />)).toContain('正在保存')
    expect(render(<ScenarioEditorView system={system} scenarioId={detail.scenario.id} state={{ status: 'ready', detail }} feedback={{ status: 'success', message: '已生成版本 4' }} onSave={vi.fn()} />)).toContain('已生成版本 4')
    expect(render(<ScenarioEditorView system={system} scenarioId={detail.scenario.id} state={{ status: 'ready', detail }} feedback={{ status: 'error', message: '步骤 Key 不能重复' }} onSave={vi.fn()} />)).toContain('步骤 Key 不能重复')
  })
})
