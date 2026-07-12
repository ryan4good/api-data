import { useEffect, useState, type ReactNode } from 'react'
import { Link, useOutletContext } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { ApiClient } from '../api/client'
import type { ApiOperation, ScanRun, ScenarioImport, ScenarioRunDetail, ScenarioSummary, SystemRole } from '../api/types'
import { Page, PlaceholderPanel } from '../components/Page'
import { systemPath } from '../navigation'
import type { SystemContextState } from '../layouts/SystemLayout'
import { WorkflowMutationPanel } from './workflow-mutations'
import type { UploadScenarioImportInput } from '../api/types'
import { SystemManagementOverview } from './management'

const roleLabels: Record<SystemRole, string> = {
  owner: '所有者',
  maintainer: '维护者',
  reviewer: '核验者',
  runner: '执行者',
  viewer: '只读成员',
}

interface RoleCapabilities {
  manageAssets: boolean
  importScenarios: boolean
  reviewScenarios: boolean
  editScenarios: boolean
  runScenarios: boolean
}

export function capabilitiesFor(role: SystemRole): RoleCapabilities {
  if (role === 'owner' || role === 'maintainer') {
    return { manageAssets: true, importScenarios: true, reviewScenarios: true, editScenarios: true, runScenarios: true }
  }
  return {
    manageAssets: false,
    importScenarios: false,
    reviewScenarios: role === 'reviewer',
    editScenarios: false,
    runScenarios: role === 'runner',
  }
}

function WorkspaceAction({ to, children }: { to: string; children: string }) {
  return <Link to={to}><button type="button">{children}</button></Link>
}

function WorkspaceCard({ title, description, children, browseTo, browseLabel, actions }: {
  title: string
  description: string
  children: ReactNode
  browseTo: string
  browseLabel: string
  actions?: ReactNode
}) {
  return (
    <section className="workspace-card">
      <div className="workspace-card-heading"><div><h2>{title}</h2><p>{description}</p></div><Link className="secondary-action" to={browseTo}>{browseLabel}</Link></div>
      <div className="workspace-resource-list">{children}</div>
      {actions && <div className="workspace-actions">{actions}</div>}
    </section>
  )
}

export type ResourceState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; items: T[] }

export interface WorkspaceResourcesState {
  scans: ResourceState<ScanRun>
  operations: ResourceState<ApiOperation>
  imports: ResourceState<ScenarioImport>
  scenarios: ResourceState<ScenarioSummary>
  runs: ResourceState<ScenarioRunDetail>
}

const emptyResources: WorkspaceResourcesState = {
  scans: { status: 'ready', items: [] },
  operations: { status: 'ready', items: [] },
  imports: { status: 'ready', items: [] },
  scenarios: { status: 'ready', items: [] },
  runs: { status: 'ready', items: [] },
}

export const loadingWorkspaceResources = (): WorkspaceResourcesState => ({
  scans: { status: 'loading' }, operations: { status: 'loading' }, imports: { status: 'loading' },
  scenarios: { status: 'loading' }, runs: { status: 'loading' },
})

type WorkspaceResourceClient = Pick<ApiClient, 'listScans' | 'listApiOperations' | 'listScenarioImports' | 'listScenarios' | 'listScenarioRuns'>

function settledResource<T>(result: PromiseSettledResult<T[]>, failureMessage: string): ResourceState<T> {
  return result.status === 'fulfilled'
    ? { status: 'ready', items: Array.isArray(result.value) ? result.value : [] }
    : { status: 'error', message: failureMessage }
}

export async function loadWorkspaceResources(client: WorkspaceResourceClient, systemId: string): Promise<WorkspaceResourcesState> {
  const [scans, operations, imports, scenarios, runs] = await Promise.allSettled([
    client.listScans(systemId),
    client.listApiOperations(systemId),
    client.listScenarioImports(systemId),
    client.listScenarios(systemId),
    client.listScenarioRuns(systemId),
  ])
  return {
    scans: settledResource(scans, '扫描记录加载失败'),
    operations: settledResource(operations, 'API 资产加载失败'),
    imports: settledResource(imports, '场景导入记录加载失败'),
    scenarios: settledResource(scenarios, '业务场景加载失败'),
    runs: settledResource(runs, '运行记录加载失败'),
  }
}

function ResourceStatus<T>({ state, loading, empty, summary, item }: {
  state: ResourceState<T>
  loading: string
  empty: string
  summary: (count: number) => string
  item: (value: T) => ReactNode
}) {
  if (state.status === 'loading') return <div className="workspace-resource muted">{loading}</div>
  if (state.status === 'error') return <div className="workspace-resource error-state"><strong>{state.message}</strong></div>
  if (state.items.length === 0) return <div className="workspace-resource workspace-empty"><strong>{empty}</strong></div>
  return (
    <div className="workspace-resource">
      <strong>{summary(state.items.length)}</strong>
      <ul>{state.items.slice(0, 3).map((value, index) => <li key={index}>{item(value)}</li>)}</ul>
    </div>
  )
}

export function SystemWorkspaceView({ state, resources = emptyResources, mutationPanel }: { state: SystemContextState; resources?: WorkspaceResourcesState; mutationPanel?: ReactNode }) {
  if (state.status === 'loading') {
    return <Page title="系统工作区" description="正在读取你的授权范围。"><div className="workspace-state">正在加载业务系统…</div></Page>
  }
  if (state.status === 'error') {
    return <Page title="系统工作区" description="当前请求未返回可用的系统上下文。"><div className="workspace-state error-state"><strong>无法加载当前业务系统</strong><p>请稍后重试，或确认你仍拥有该系统权限。</p></div></Page>
  }

  const { system } = state
  const capabilities = capabilitiesFor(system.myRole)
  const path = (segment: Parameters<typeof systemPath>[1]) => systemPath(system.id, segment)

  return (
    <Page
      eyebrow={`${system.code} / 单系统工作区`}
      title={system.name}
      description={`当前身份：${roleLabels[system.myRole]}。代码资产、业务场景与执行记录均隔离在当前业务系统内。`}
    >
      {system.myRole === 'viewer' && <div className="permission-note">当前角色仅可查看，变更、核验和执行操作已隐藏。</div>}
      {mutationPanel}
      <div className="workspace-grid">
        <WorkspaceCard
          title="代码扫描与 API 资产"
          description="从当前系统的代码仓库识别接口，并进入 API 资产库核对来源。"
          browseTo={path('apis')}
          browseLabel="查看 API 资产"
          actions={capabilities.manageAssets ? <WorkspaceAction to={path('scan')}>开始代码扫描</WorkspaceAction> : undefined}
        >
          <ResourceStatus state={resources.scans} loading="正在加载扫描记录…" empty="尚无代码扫描记录" summary={(count) => `扫描记录 ${count} 条`} item={(scan) => `${scan.status}${scan.sourceRef ? ` · ${scan.sourceRef}` : ''}`} />
          <ResourceStatus state={resources.operations} loading="正在加载 API 资产…" empty="尚无 API 资产" summary={(count) => `API 资产 ${count} 个`} item={(operation) => <><b className="http-method">{operation.method}</b> {operation.path}</>} />
        </WorkspaceCard>
        <WorkspaceCard
          title="场景导入与人工核验"
          description="场景可来自代码分析，也可导入 Postman 类 JSON；候选结果需在本系统内确认。"
          browseTo={path('review')}
          browseLabel="查看核验队列"
          actions={<>
            {capabilities.importScenarios && <WorkspaceAction to={path('editor')}>导入场景 JSON</WorkspaceAction>}
            {capabilities.reviewScenarios && <WorkspaceAction to={path('review')}>人工核验</WorkspaceAction>}
          </>}
        >
          <ResourceStatus state={resources.imports} loading="正在加载场景导入记录…" empty="尚无场景导入记录" summary={(count) => `场景导入 ${count} 条`} item={(scenarioImport) => `${scenarioImport.fileName} · ${scenarioImport.status}`} />
        </WorkspaceCard>
        <WorkspaceCard
          title="场景列表与编排"
          description="查看本系统场景，进入编排器增删单个步骤或选择执行截止步骤。"
          browseTo={path('discovery')}
          browseLabel="查看场景列表"
          actions={capabilities.editScenarios ? <WorkspaceAction to={path('editor')}>创建场景</WorkspaceAction> : undefined}
        >
          <ResourceStatus state={resources.scenarios} loading="正在加载业务场景…" empty="尚无业务场景" summary={(count) => `业务场景 ${count} 个`} item={(scenario) => <Link to={`${path('editor')}/${scenario.id}`}>{scenario.name} · {scenario.status}</Link>} />
        </WorkspaceCard>
        <WorkspaceCard
          title="执行与结果"
          description="从场景入口发起运行，在当前系统范围查看步骤日志与结果记录。"
          browseTo={path('runs')}
          browseLabel="查看运行记录"
          actions={capabilities.runScenarios ? <WorkspaceAction to={path('runs')}>执行场景</WorkspaceAction> : undefined}
        >
          <ResourceStatus state={resources.runs} loading="正在加载运行记录…" empty="尚无运行记录" summary={(count) => `运行记录 ${count} 条`} item={(run) => `${run.status} · ${run.outcome}`} />
        </WorkspaceCard>
      </div>
    </Page>
  )
}

type Capability = keyof RoleCapabilities

export function SystemModulePageView({ state, title, description, action, capability }: {
  state: SystemContextState
  title: string
  description: string
  action?: string
  capability?: Capability
}) {
  if (state.status === 'loading') return <Page title={title} description="正在加载业务系统…"><div className="workspace-state">正在加载业务系统…</div></Page>
  if (state.status === 'error') return <Page title={title} description={description}><div className="workspace-state error-state"><strong>无法加载当前业务系统</strong></div></Page>
  const canAct = !capability || capabilitiesFor(state.system.myRole)[capability]
  return (
    <Page eyebrow={`${state.system.name} / 工作空间`} title={title} description={description} action={action && canAct && <button type="button">{action}</button>}>
      <PlaceholderPanel title="功能工作区">
        <div className="empty-state"><span>◇</span><strong>当前系统暂无{title}数据</strong><p>这里只会展示 {state.system.name} 范围内的数据。</p></div>
      </PlaceholderPanel>
    </Page>
  )
}

function StaticPage(props: Omit<Parameters<typeof SystemModulePageView>[0], 'state'>) {
  const state = useOutletContext<SystemContextState>()
  return <SystemModulePageView state={state ?? { status: 'loading' }} {...props} />
}

export function SystemOverviewPage() {
  const state = useOutletContext<SystemContextState>()
  const [resources, setResources] = useState<WorkspaceResourcesState>(loadingWorkspaceResources)
  const systemId = state?.status === 'ready' ? state.system.id : undefined

  useEffect(() => {
    let active = true
    if (!systemId) {
      setResources(loadingWorkspaceResources())
      return () => { active = false }
    }
    setResources(loadingWorkspaceResources())
    loadWorkspaceResources(apiClient, systemId).then((next) => { if (active) setResources(next) })
    return () => { active = false }
  }, [systemId])

  async function refreshImports() {
    if (!systemId) return
    try {
      const items = await apiClient.listScenarioImports(systemId)
      setResources((current) => ({ ...current, imports: { status: 'ready', items: Array.isArray(items) ? items : [] } }))
    } catch {
      // Preserve the previous import snapshot on a partial refresh failure.
    }
  }

  async function uploadImport(input: UploadScenarioImportInput) {
    if (!systemId) throw new Error('当前业务系统尚未加载完成')
    const result = await apiClient.uploadScenarioImport(systemId, input)
    await refreshImports()
    return result
  }

  async function confirmScripts(importId: string) {
    if (!systemId) throw new Error('当前业务系统尚未加载完成')
    const result = await apiClient.confirmScenarioImportScripts(systemId, importId)
    await refreshImports()
    return result
  }

  async function applyImport(importId: string) {
    if (!systemId) throw new Error('当前业务系统尚未加载完成')
    const result = await apiClient.applyScenarioImport(systemId, importId)
    await refreshImports()
    return result
  }

  const visibleImports = resources.imports.status === 'ready' ? resources.imports.items : []
  const mutationPanel = state?.status === 'ready' ? (
    <>
      <SystemManagementOverview systemId={state.system.id} />
      <WorkflowMutationPanel
        role={state.system.myRole}
        imports={visibleImports}
        onUploadImport={uploadImport}
        onConfirmScripts={confirmScripts}
        onApplyImport={applyImport}
      />
    </>
  ) : undefined

  return <SystemWorkspaceView state={state ?? { status: 'loading' }} resources={resources} mutationPanel={mutationPanel} />
}
export const DiscoveryPage = () => <StaticPage title="场景发现" description="从调用关系、测试代码和接口语义生成可解释的场景候选。" action="生成候选" capability="manageAssets" />
export const ReviewPage = () => <StaticPage title="待核验场景" description="集中处理候选场景、导入冲突和缺失的变量依赖。" action="人工核验" capability="reviewScenarios" />
export const RunsPage = () => <StaticPage title="运行记录" description="追踪场景执行状态、步骤日志、断言结果和脱敏后的请求响应。" action="立即执行" capability="runScenarios" />
