import { useEffect, useState } from 'react'
import { useOutletContext } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { ApiOperation } from '../api/types'
import { Page } from '../components/Page'
import type { SystemContextState } from '../layouts/SystemLayout'
import './api-assets.css'

export type ApiAssetsState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: ApiAsset[] }

type ApiAsset = ApiOperation & { tags?: string[] }
export interface ApiAssetFilters { method: string; status: string; path: string }

function normalized(value?: string): string { return value?.trim().toLocaleLowerCase() ?? '' }

export function filterApiOperations(items: ApiAsset[], filters: ApiAssetFilters): ApiAsset[] {
  const method = normalized(filters.method)
  const status = normalized(filters.status)
  const path = normalized(filters.path)
  return items.filter((item) => {
    const matchesMethod = !method || normalized(item.method) === method
    const matchesStatus = !status || normalized(item.verificationStatus) === status || normalized(item.lifecycleStatus) === status
    const matchesPath = !path || normalized(item.path).includes(path)
    return matchesMethod && matchesStatus && matchesPath
  })
}

function text(value?: string): string { return value?.trim() || '—' }
function tags(value?: string[]): string { return value?.filter(Boolean).join('、') || '—' }
function dateTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

export function ApiAssetsPageView({ state, methodFilter, statusFilter, pathFilter, onMethodFilter, onStatusFilter, onPathFilter }: {
  state: ApiAssetsState
  methodFilter: string
  statusFilter: string
  pathFilter: string
  onMethodFilter: (value: string) => void
  onStatusFilter: (value: string) => void
  onPathFilter: (value: string) => void
}) {
  if (state.status === 'loading') return <Page title="API 资产" description="读取当前业务系统的真实接口资产。"><div className="workspace-state">正在加载 API 资产…</div></Page>
  if (state.status === 'error') return <Page title="API 资产" description="当前业务系统的接口资产暂不可用。"><div className="workspace-state error-state"><strong>{state.message}</strong></div></Page>
  if (state.status === 'empty') return <Page title="API 资产" description="仅展示扫描入库的真实接口资产。"><div className="workspace-state">暂无 API 资产</div></Page>

  const methods = [...new Set(state.items.map((item) => item.method).filter(Boolean))].sort()
  const statuses = [...new Set(state.items.flatMap((item) => [item.verificationStatus, item.lifecycleStatus]).filter((value): value is string => Boolean(value)))].sort()
  const visibleItems = filterApiOperations(state.items, { method: methodFilter, status: statusFilter, path: pathFilter })
  return <Page eyebrow="单系统工作区" title="API 资产" description="展示扫描入库的接口定义，可在当前结果内按方法、状态和路径筛选。">
    <section className="api-assets-filters" aria-label="API 资产筛选">
      <label>方法<select value={methodFilter} onChange={(event) => onMethodFilter(event.target.value)}><option value="">全部方法</option>{methods.map((method) => <option key={method} value={method}>{method}</option>)}</select></label>
      <label>状态<select value={statusFilter} onChange={(event) => onStatusFilter(event.target.value)}><option value="">全部状态</option>{statuses.map((status) => <option key={status} value={status}>{status}</option>)}</select></label>
      <label>路径<input value={pathFilter} onChange={(event) => onPathFilter(event.target.value)} placeholder="输入路径片段" /></label>
      <strong>共 {visibleItems.length} 条<span>（总计 {state.items.length} 条）</span></strong>
    </section>
    {visibleItems.length === 0 ? <div className="workspace-state">当前筛选条件下没有 API 资产</div> : <div className="api-assets-table" role="table" aria-label="API 资产列表">
      <div className="api-assets-row api-assets-head" role="row"><span>方法 / 路径</span><span>摘要</span><span>标签</span><span>核验状态</span><span>生命周期</span><span>更新时间</span></div>
      {visibleItems.map((item) => <article className="api-assets-row" role="row" key={item.id}>
        <span><b className={`api-method method-${item.method.toLocaleLowerCase()}`}>{text(item.method)}</b><code>{text(item.path)}</code></span>
        <span>{text(item.summary)}</span>
        <span>{tags(item.tags)}</span>
        <span>{text(item.verificationStatus)}</span>
        <span>{text(item.lifecycleStatus)}</span>
        <span>{dateTime(item.updatedAt)}</span>
      </article>)}
    </div>}
  </Page>
}

export function ApiAssetsPage() {
  const context = useOutletContext<SystemContextState>()
  const systemId = context?.status === 'ready' ? context.system.id : ''
  const [state, setState] = useState<ApiAssetsState>({ status: 'loading' })
  const [methodFilter, setMethodFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [pathFilter, setPathFilter] = useState('')

  useEffect(() => {
    let active = true
    if (context?.status === 'error') {
      setState({ status: 'error', message: '业务系统上下文加载失败' })
      return () => { active = false }
    }
    if (!systemId) {
      setState({ status: 'loading' })
      return () => { active = false }
    }
    setState({ status: 'loading' })
    apiClient.listApiOperations(systemId).then((items) => {
      if (!active) return
      const operations = Array.isArray(items) ? items as ApiAsset[] : []
      setState(operations.length ? { status: 'ready', items: operations } : { status: 'empty' })
    }).catch((reason: unknown) => {
      if (active) setState({ status: 'error', message: reason instanceof Error ? reason.message : 'API 资产加载失败' })
    })
    return () => { active = false }
  }, [context?.status, systemId])

  return <ApiAssetsPageView state={state} methodFilter={methodFilter} statusFilter={statusFilter} pathFilter={pathFilter} onMethodFilter={setMethodFilter} onStatusFilter={setStatusFilter} onPathFilter={setPathFilter} />
}
