import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { ScanRun } from '../api/types'
import { createScanInput, runScanInput, ScanPageView, scanItemsState } from './scan'

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

const noop = vi.fn()
const render = (node: React.ReactNode) => renderToStaticMarkup(node)

describe('ScanPageView', () => {
  it('renders honest loading, error and empty states', () => {
    expect(render(<ScanPageView role="viewer" state={{ status: 'loading' }} onCreate={noop} onRun={noop} />)).toContain('正在加载扫描记录')
    expect(render(<ScanPageView role="viewer" state={{ status: 'error', message: '扫描服务不可用' }} onCreate={noop} onRun={noop} />)).toContain('扫描服务不可用')
    expect(render(<ScanPageView role="viewer" state={{ status: 'empty' }} onCreate={noop} onRun={noop} />)).toContain('尚无代码扫描记录')
  })

  it('shows source, stack, status and creation time from the API record', () => {
    const html = render(<ScanPageView role="viewer" state={{ status: 'ready', items: [scan] }} onCreate={noop} onRun={noop} />)

    expect(html).toContain('refs/heads/main')
    expect(html).toContain('8f29c43')
    expect(html).toContain('go')
    expect(html).toContain('gin')
    expect(html).toContain('succeeded')
    expect(html).toContain('2026-07-12T03:30:00Z')
  })

  it('shows create and run inputs only to owners and maintainers', () => {
    const owner = render(<ScanPageView role="owner" state={{ status: 'ready', items: [scan] }} onCreate={noop} onRun={noop} />)
    const maintainer = render(<ScanPageView role="maintainer" state={{ status: 'ready', items: [scan] }} onCreate={noop} onRun={noop} />)
    const reviewer = render(<ScanPageView role="reviewer" state={{ status: 'ready', items: [scan] }} onCreate={noop} onRun={noop} />)

    for (const writable of [owner, maintainer]) {
      expect(writable).toContain('name="codeSourceId"')
      expect(writable).toContain('name="sourceRef"')
      expect(writable).toContain('name="sourceCommit"')
      expect(writable).toContain('name="language"')
      expect(writable).toContain('name="framework"')
      expect(writable).toContain('name="repositoryRoot"')
      expect(writable).toContain('创建扫描记录')
      expect(writable).toContain('执行扫描')
    }
    expect(reviewer).not.toContain('<form')
    expect(reviewer).not.toContain('>执行扫描</button>')
  })

  it('preserves records while showing real mutation feedback', () => {
    const html = render(<ScanPageView role="owner" state={{ status: 'ready', items: [scan] }} feedback={{ status: 'error', message: '仓库目录不存在' }} onCreate={noop} onRun={noop} />)

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

  it('omits blank optional create fields and maps RunScanInput', () => {
    const create = new FormData()
    create.set('codeSourceId', 'source-1')
    create.set('sourceRef', '  ')
    const run = new FormData()
    run.set('repositoryRoot', ' /srv/orders ')

    expect(createScanInput(create)).toEqual({
      codeSourceId: 'source-1', sourceRef: undefined, sourceCommit: undefined, language: undefined, framework: undefined,
    })
    expect(runScanInput(run)).toEqual({ repositoryRoot: '/srv/orders' })
  })
})
