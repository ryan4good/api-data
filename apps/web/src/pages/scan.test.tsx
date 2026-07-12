import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { CodeSource, ScanRun } from '../api/types'
import { createScanInput, ScanPageView, scanItemsState } from './scan'

const scan: ScanRun = {
  id: 'scan-1',
  systemId: 'system-a',
  codeSourceId: 'source-1',
  sourceRef: 'refs/heads/main',
  sourceCommit: '8f29c43',
  language: 'go',
  framework: 'gin',
  status: 'succeeded',
  createdAt: '2026-07-12T03:30:00Z',
}

const source: CodeSource = {
  id: 'source-1', systemId: 'system-a', name: '订单服务仓库', sourceType: 'git', repositoryUrl: 'https://example.test/orders.git',
  defaultRef: 'main', includePaths: [], excludePaths: [], status: 'active', createdBy: 'user-1',
  createdAt: '2026-07-12T03:00:00Z', updatedAt: '2026-07-12T03:00:00Z',
}
const localSource: CodeSource = {
  ...source, id: 'source-local', name: '本地订单工作区', sourceType: 'local', repositoryUrl: undefined, localPath: '/srv/orders',
}
const disabledSource: CodeSource = { ...localSource, id: 'source-disabled', name: '停用工作区', status: 'disabled' }

const noop = vi.fn()
const render = (node: React.ReactNode) => renderToStaticMarkup(<MemoryRouter>{node}</MemoryRouter>)

describe('ScanPageView', () => {
  it('renders honest loading, error and empty states', () => {
    expect(render(<ScanPageView role="viewer" state={{ status: 'loading' }} codeSources={{ status: 'loading' }} onCreate={noop} onRun={noop} />)).toContain('正在加载扫描记录')
    expect(render(<ScanPageView role="viewer" state={{ status: 'error', message: '扫描服务不可用' }} codeSources={{ status: 'loading' }} onCreate={noop} onRun={noop} />)).toContain('扫描服务不可用')
    expect(render(<ScanPageView role="viewer" state={{ status: 'empty' }} codeSources={{ status: 'loading' }} onCreate={noop} onRun={noop} />)).toContain('尚无代码扫描记录')
  })

  it('shows source, stack, status and creation time from the API record', () => {
    const html = render(<ScanPageView role="viewer" state={{ status: 'ready', items: [scan] }} codeSources={{ status: 'ready', items: [source] }} onCreate={noop} onRun={noop} />)

    expect(html).toContain('refs/heads/main')
    expect(html).toContain('8f29c43')
    expect(html).toContain('go')
    expect(html).toContain('gin')
    expect(html).toContain('succeeded')
    expect(html).toContain('2026-07-12T03:30:00Z')
  })

  it('shows create and run inputs only to owners and maintainers', () => {
    const localScan = { ...scan, codeSourceId: localSource.id }
    const owner = render(<ScanPageView role="owner" state={{ status: 'ready', items: [localScan] }} codeSources={{ status: 'ready', items: [source, localSource] }} onCreate={noop} onRun={noop} />)
    const maintainer = render(<ScanPageView role="maintainer" state={{ status: 'ready', items: [localScan] }} codeSources={{ status: 'ready', items: [source, localSource] }} onCreate={noop} onRun={noop} />)
    const reviewer = render(<ScanPageView role="reviewer" state={{ status: 'ready', items: [scan] }} codeSources={{ status: 'ready', items: [source] }} onCreate={noop} onRun={noop} />)

    for (const writable of [owner, maintainer]) {
      expect(writable).toContain('name="codeSourceId"')
      expect(writable).toContain('订单服务仓库')
      expect(writable).toContain('git · main')
      expect(writable).toContain('name="sourceRef"')
      expect(writable).toContain('name="sourceCommit"')
      expect(writable).toContain('name="language"')
      expect(writable).toContain('name="framework"')
      expect(writable).toContain('按已登记代码源配置执行')
      expect(writable).not.toContain('服务器上的仓库绝对路径')
      expect(writable).toContain('创建扫描记录')
      expect(writable).toContain('执行扫描')
    }
    expect(reviewer).not.toContain('<form')
    expect(reviewer).not.toContain('>执行扫描</button>')
  })

  it('only offers execution for active local sources and explains every unavailable association', () => {
    const scans = [
      scan,
      { ...scan, id: 'scan-local', codeSourceId: localSource.id },
      { ...scan, id: 'scan-disabled', codeSourceId: disabledSource.id },
      { ...scan, id: 'scan-missing', codeSourceId: 'source-missing' },
    ]
    const html = render(<ScanPageView role="owner" state={{ status: 'ready', items: scans }} codeSources={{ status: 'ready', items: [source, localSource, disabledSource] }} onCreate={noop} onRun={noop} />)
    expect(html).toContain('Git 检出工作区尚未配置，当前不可直接执行')
    expect(html.match(/代码源当前不可执行/g)).toHaveLength(2)
    expect(html.match(/>执行扫描<\/button>/g)).toHaveLength(1)
    expect(html).toContain('停用工作区')
  })

  it('disables creation honestly when active code sources are absent or unavailable without hiding records', () => {
    const empty = render(<ScanPageView role="owner" systemId="system-a" state={{ status: 'ready', items: [scan] }} codeSources={{ status: 'empty' }} onCreate={noop} onRun={noop} />)
    const failed = render(<ScanPageView role="owner" state={{ status: 'ready', items: [scan] }} codeSources={{ status: 'error', message: '代码源服务不可用' }} onCreate={noop} onRun={noop} />)
    expect(empty).toContain('/systems/system-a/settings')
    expect(empty).toContain('请先在系统设置中添加启用的代码源')
    expect(empty).toContain('disabled=""')
    expect(failed).toContain('代码源服务不可用')
    expect(failed).toContain('refs/heads/main')
  })

  it('preserves records while showing real mutation feedback', () => {
    const html = render(<ScanPageView role="owner" state={{ status: 'ready', items: [scan] }} codeSources={{ status: 'ready', items: [source] }} feedback={{ status: 'error', message: '仓库目录不存在' }} onCreate={noop} onRun={noop} />)

    expect(html).toContain('仓库目录不存在')
    expect(html).toContain('refs/heads/main')
  })
})

describe('scanItemsState', () => {
  it('maps API arrays to empty or ready without inventing records', () => {
    expect(scanItemsState([])).toEqual({ status: 'empty' })
    expect(scanItemsState([scan])).toEqual({ status: 'ready', items: [scan] })
  })
})

describe('scan form input mapping', () => {
  it('maps and trims every CreateScanInput field', () => {
    const form = new FormData()
    form.set('codeSourceId', ' source-1 ')
    form.set('sourceRef', ' refs/heads/main ')
    form.set('sourceCommit', ' 8f29c43 ')
    form.set('language', ' go ')
    form.set('framework', ' gin ')

    expect(createScanInput(form)).toEqual({
      codeSourceId: 'source-1', sourceRef: 'refs/heads/main', sourceCommit: '8f29c43', language: 'go', framework: 'gin',
    })
  })

  it('omits blank optional create fields', () => {
    const create = new FormData()
    create.set('codeSourceId', 'source-1')
    create.set('sourceRef', '  ')

    expect(createScanInput(create)).toEqual({
      codeSourceId: 'source-1', sourceRef: undefined, sourceCommit: undefined, language: undefined, framework: undefined,
    })
  })
})
