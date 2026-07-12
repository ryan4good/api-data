import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { DiscoveryCandidate, ScenarioDetail, ScenarioRunDetail } from '../api/types'
import { CandidateQueueView, DiscoveryPageView } from './discovery'
import { RunDetailView, RunsPageView, parseInputVariables } from './runs'

const candidate: DiscoveryCandidate = {
  id: 'candidate-1', systemId: 'system-a', discoveryId: 'discovery-1', key: 'submit-order', name: '提交订单',
  priority: 'P0', confidence: 0.92, sourceRefs: ['code:router.go:42', 'prompt:用户下单'], requiresReview: true,
  steps: [{ key: 'submit', name: '提交', method: 'POST', path: '/orders' }], reviewStatus: 'pending',
  createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z',
}

const run: ScenarioRunDetail = {
  id: 'run-1', systemId: 'system-a', scenarioId: 'scenario-1', scenarioVersionId: 'version-1', environmentId: 'env-1',
  status: 'completed', outcome: 'failed', triggerType: 'manual', inputVariables: {}, outputVariables: {},
  summary: { totalSteps: 2, executedSteps: 2, failedStepId: 'step-2', stopAfterStepId: 'step-2' }, requestedBy: 'user-1',
  startedAt: '2026-07-12T00:00:00Z', finishedAt: '2026-07-12T00:00:01Z', createdAt: '2026-07-12T00:00:00Z',
  attempts: [{ id: 'attempt-1', systemId: 'system-a', runId: 'run-1', stepId: 'step-2', attemptNo: 1, position: 2, status: 'failed', durationMs: 120, assertions: [{ key: 'status', type: 'status', status: 'failed', expected: 200, actual: 500, message: '状态码不匹配' }],
    startedAt: '2026-07-12T00:00:00Z', finishedAt: '2026-07-12T00:00:01Z', createdAt: '2026-07-12T00:00:00Z' }],
}

const scenarioDetail: ScenarioDetail = {
  scenario: { id: 'scenario-1', systemId: 'system-a', key: 'submit-order', name: '提交订单', status: 'draft', currentVersionId: 'version-1', createdBy: 'user-1', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z' },
  version: { id: 'version-1', systemId: 'system-a', scenarioId: 'scenario-1', versionNo: 1, sourceType: 'candidate', createdBy: 'user-1', createdAt: '2026-07-12T00:00:00Z' },
  steps: [{ id: 'step-1', systemId: 'system-a', versionId: 'version-1', key: 'submit', name: '提交订单', position: 1, type: 'http', dependsOn: [], createdAt: '2026-07-12T00:00:00Z' }],
}

const noop = vi.fn()
const render = (node: React.ReactNode) => renderToStaticMarkup(<MemoryRouter>{node}</MemoryRouter>)

describe('discovery and runs pages', () => {
  it('makes all discovery inputs explicit and keeps viewers read-only', () => {
    const owner = render(<DiscoveryPageView role="owner" state={{ status: 'empty' }} onSubmit={noop} />)
    const viewer = render(<DiscoveryPageView role="viewer" state={{ status: 'empty' }} onSubmit={noop} />)
    expect(owner).toContain('仅代码')
    expect(owner).toContain('一句话需求')
    expect(owner).toContain('PRD 文档')
    expect(owner).toContain('混合输入')
    expect(owner).toContain('生成场景候选')
    expect(viewer).not.toContain('<form')
  })

  it('shows P0 evidence and only reviewers can accept or reject candidates', () => {
    const reviewer = render(<CandidateQueueView role="reviewer" state={{ status: 'ready', items: [candidate] }} onReview={noop} />)
    const viewer = render(<CandidateQueueView role="viewer" state={{ status: 'ready', items: [candidate] }} onReview={noop} />)
    expect(reviewer).toContain('P0')
    expect(reviewer).toContain('92%')
    expect(reviewer).toContain('code:router.go:42')
    expect(reviewer).toContain('需要人工核验')
    expect(reviewer).toContain('接受候选')
    expect(reviewer).toContain('拒绝候选')
    expect(viewer).not.toContain('接受候选')
  })

  it('lets only owners and maintainers idempotently promote accepted candidates', () => {
    const accepted = { ...candidate, reviewStatus: 'accepted' as const }
    const owner = render(<CandidateQueueView role="owner" state={{ status: 'ready', items: [accepted] }} onReview={noop} onPromote={noop} feedback={{ status: 'success', message: '场景已存在，返回原发布结果。' }} />)
    const reviewer = render(<CandidateQueueView role="reviewer" state={{ status: 'ready', items: [accepted] }} onReview={noop} onPromote={noop} />)
    const viewer = render(<CandidateQueueView role="viewer" state={{ status: 'ready', items: [accepted] }} onReview={noop} onPromote={noop} />)
    expect(owner).toContain('发布为场景')
    expect(owner).toContain('场景已存在，返回原发布结果')
    expect(reviewer).not.toContain('发布为场景')
    expect(viewer).not.toContain('发布为场景')
  })

  it('supports honest run states and execution through a selected stop step', () => {
    expect(render(<RunsPageView role="viewer" state={{ status: 'loading' }} onExecute={noop} />)).toContain('正在加载运行记录')
    expect(render(<RunsPageView role="viewer" state={{ status: 'error', message: '运行服务不可用' }} onExecute={noop} />)).toContain('运行服务不可用')
    expect(render(<RunsPageView role="viewer" state={{ status: 'empty' }} onExecute={noop} />)).toContain('尚无运行记录')
    const runner = render(<RunsPageView role="runner" state={{ status: 'empty' }} onExecute={noop} />)
    expect(runner).toContain('执行到指定步骤')
    expect(runner).toContain('stopAfterStepId')
    expect(runner).toContain('inputVariables')
  })

  it('prefers scenario version and step selectors when detail is available', () => {
    const html = render(<RunsPageView role="runner" state={{ status: 'empty' }} scenarios={{ status: 'ready', items: [scenarioDetail.scenario] }} scenarioDetails={{ status: 'ready', items: [scenarioDetail] }} onExecute={noop} />)
    expect(html).toContain('提交订单')
    expect(html).toContain('版本 1')
    expect(html).toContain('提交订单（步骤 1）')
    expect(html).toContain('完整执行')
  })

  it('shows attempts, assertions and role-scoped step retry', () => {
    const runner = render(<RunDetailView role="runner" state={{ status: 'ready', run }} onRetry={noop} />)
    const viewer = render(<RunDetailView role="viewer" state={{ status: 'ready', run }} onRetry={noop} />)
    expect(runner).toContain('第 1 次尝试')
    expect(runner).toContain('状态码不匹配')
    expect(runner).toContain('重试此步骤')
    expect(viewer).not.toContain('重试此步骤')
  })

  it('validates inputVariables as a JSON object', () => {
    expect(parseInputVariables('')).toEqual({})
    expect(parseInputVariables('{"orderId":"A1"}')).toEqual({ orderId: 'A1' })
    expect(() => parseInputVariables('[1]')).toThrow('输入变量必须是 JSON 对象')
  })
})
