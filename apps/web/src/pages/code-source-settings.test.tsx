import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { CodeSource } from '../api/types'
import { CodeSourceSettingsView, parsePathList, validateCodeSourceInput } from './code-source-settings'

const gitSource: CodeSource = {
  id: 'source-1', systemId: 'system-a', name: '订单服务', sourceType: 'git',
  repositoryUrl: 'https://git.example.test/orders.git', defaultRef: 'main',
  includePaths: ['src', 'cmd'], excludePaths: ['vendor'], credentialRef: 'vault://git/orders',
  status: 'active', createdBy: 'user-1', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z',
}

const noop = vi.fn()

describe('code source settings', () => {
  it('shows real source identity and external credential reference to every system role', () => {
    for (const role of ['owner', 'maintainer', 'reviewer', 'runner', 'viewer'] as const) {
      const html = renderToStaticMarkup(<CodeSourceSettingsView role={role} state={{ status: 'ready', items: [gitSource] }} onCreate={noop} onUpdate={noop} />)
      expect(html).toContain('订单服务')
      expect(html).toContain('https://git.example.test/orders.git')
      expect(html).toContain('vault://git/orders')
    }
  })

  it('allows only owners and maintainers to create and edit', () => {
    for (const role of ['owner', 'maintainer'] as const) {
      const html = renderToStaticMarkup(<CodeSourceSettingsView role={role} state={{ status: 'ready', items: [gitSource] }} onCreate={noop} onUpdate={noop} />)
      expect(html).toContain('新建代码源')
      expect(html).toContain('编辑代码源')
      expect(html).toContain('<form')
      expect(html).toContain('只填写外部凭据引用')
      expect(html).not.toContain('name="password"')
    }
    for (const role of ['reviewer', 'runner', 'viewer'] as const) {
      const html = renderToStaticMarkup(<CodeSourceSettingsView role={role} state={{ status: 'ready', items: [gitSource] }} onCreate={noop} onUpdate={noop} />)
      expect(html).toContain('当前角色为只读')
      expect(html).not.toContain('<form')
    }
  })

  it('renders honest loading, empty, error and mutation feedback', () => {
    expect(renderToStaticMarkup(<CodeSourceSettingsView role="viewer" state={{ status: 'loading' }} onCreate={noop} onUpdate={noop} />)).toContain('正在加载代码源')
    expect(renderToStaticMarkup(<CodeSourceSettingsView role="viewer" state={{ status: 'empty' }} onCreate={noop} onUpdate={noop} />)).toContain('尚未配置代码源')
    expect(renderToStaticMarkup(<CodeSourceSettingsView role="viewer" state={{ status: 'error', message: '代码源服务不可用' }} onCreate={noop} onUpdate={noop} />)).toContain('代码源服务不可用')
    expect(renderToStaticMarkup(<CodeSourceSettingsView role="owner" state={{ status: 'empty' }} feedback={{ status: 'success', message: '代码源已创建' }} onCreate={noop} onUpdate={noop} />)).toContain('代码源已创建')
  })

  it('validates source-specific locations and parses path lists', () => {
    expect(parsePathList('src\n cmd,internal\n\nsrc')).toEqual(['src', 'cmd', 'internal'])
    expect(() => validateCodeSourceInput({ name: 'Git', sourceType: 'git', includePaths: [], excludePaths: [], status: 'active' })).toThrow('仓库地址')
    expect(() => validateCodeSourceInput({ name: 'Local', sourceType: 'local', includePaths: [], excludePaths: [], status: 'active' })).toThrow('本地路径')
    expect(() => validateCodeSourceInput({ name: 'Git', sourceType: 'git', repositoryUrl: 'https://user:secret@git.example.test/app.git', includePaths: [], excludePaths: [], status: 'active' })).toThrow('内嵌凭据')
    expect(() => validateCodeSourceInput({ name: 'Git', sourceType: 'git', repositoryUrl: 'https://git.example.test/app.git', credentialRef: 'plain-secret', includePaths: [], excludePaths: [], status: 'active' })).toThrow('外部凭据引用')
    expect(() => validateCodeSourceInput({ name: 'Git', sourceType: 'git', repositoryUrl: 'https://git.example.test/app.git', includePaths: ['../private'], excludePaths: [], status: 'active' })).toThrow('相对路径')
    expect(validateCodeSourceInput({ name: 'Git', sourceType: 'git', repositoryUrl: 'https://git.example.test/app.git', includePaths: [], excludePaths: [], status: 'active' }).repositoryUrl).toContain('app.git')
    expect(validateCodeSourceInput({ name: 'Local', sourceType: 'local', localPath: '/workspace/app', includePaths: [], excludePaths: [], status: 'active' }).localPath).toBe('/workspace/app')
  })
})
