import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { Environment } from '../api/types'
import { EnvironmentSettingsView, parseEnvironmentVariables, validateExternalSecretReference } from './system-settings'

const environment: Environment = {
  id: 'env-1', systemId: 'system-a', key: 'staging', name: '预发布', status: 'active',
  variables: { BASE_URL: 'https://staging.example.test', REGION: 'ap-shanghai' }, createdBy: 'user-1',
  createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z',
}

const noop = vi.fn()
const render = (role: 'owner' | 'maintainer' | 'viewer', references = { 'env-1': { status: 'ready' as const, items: [{
  id: 'ref-1', systemId: 'system-a', environmentId: 'env-1', variableKey: 'DB_PASSWORD', secretRef: role === 'viewer' ? undefined : 'vault://bizdevops/staging/db-password', createdBy: 'user-1', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z',
}] } }) => renderToStaticMarkup(<EnvironmentSettingsView role={role} state={{ status: 'ready', items: [environment] }} references={references} onUpsertEnvironment={noop} onCreateSecretReference={noop} />)

describe('system settings environment page', () => {
  it('shows environment identity, status and non-sensitive variables', () => {
    const html = render('viewer')
    expect(html).toContain('staging')
    expect(html).toContain('预发布')
    expect(html).toContain('启用')
    expect(html).toContain('BASE_URL')
    expect(html).toContain('https://staging.example.test')
  })

  it('lets only owners and maintainers manage environments and external references', () => {
    for (const role of ['owner', 'maintainer'] as const) {
      const html = render(role)
      expect(html).toContain('新建环境')
      expect(html).toContain('更新环境')
      expect(html).toContain('新增外部密钥引用')
      expect(html).toContain('vault://')
      expect(html).not.toContain('真实密钥值')
    }
    const viewer = render('viewer')
    expect(viewer).not.toContain('<form')
    expect(viewer).not.toContain('更新环境')
  })

  it('does not fabricate references redacted by the backend', () => {
    const html = render('viewer')
    expect(html).toContain('DB_PASSWORD')
    expect(html).toContain('引用位置已隐藏')
    expect(html).not.toContain('vault://bizdevops/staging/db-password')
  })

  it('keeps secret-reference loading and errors independent per environment', () => {
    const second = { ...environment, id: 'env-2', key: 'prod', name: '生产' }
    const html = renderToStaticMarkup(<EnvironmentSettingsView role="viewer" state={{ status: 'ready', items: [environment, second] }} references={{
      'env-1': { status: 'loading' }, 'env-2': { status: 'error', message: '引用服务不可用' },
    }} onUpsertEnvironment={noop} onCreateSecretReference={noop} />)
    expect(html).toContain('正在加载密钥引用')
    expect(html).toContain('引用服务不可用')
    expect(html).toContain('staging')
    expect(html).toContain('prod')
  })

  it('accepts only JSON objects with non-sensitive variables and external secret schemes', () => {
    expect(parseEnvironmentVariables('{"BASE_URL":"https://example.test","RETRIES":"3"}')).toEqual({ BASE_URL: 'https://example.test', RETRIES: '3' })
    expect(() => parseEnvironmentVariables('{"PASSWORD":"plaintext"}')).toThrow('敏感变量')
    expect(() => parseEnvironmentVariables('["bad"]')).toThrow('JSON 对象')
    expect(validateExternalSecretReference('vault://path/to/secret')).toBe(true)
    expect(validateExternalSecretReference('secret://name')).toBe(true)
    expect(validateExternalSecretReference('aws-secrets://name')).toBe(true)
    expect(validateExternalSecretReference('gcp-secret://name')).toBe(true)
    expect(validateExternalSecretReference('https://example.test/secret')).toBe(false)
    expect(validateExternalSecretReference('vault://')).toBe(false)
  })
})
