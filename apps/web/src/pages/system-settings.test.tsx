import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { Environment, SystemMember } from '../api/types'
import { canManageMembers, EnvironmentSettingsView, MemberSettingsView, parseEnvironmentVariables, validateExternalSecretReference } from './system-settings'

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

const members: SystemMember[] = [
  { userId: 'user-1', displayName: '平台管理员', email: 'owner@example.test', role: 'owner', status: 'active' },
  { userId: 'user-2', displayName: '发布审核员', role: 'reviewer', status: 'disabled' },
]

describe('system settings member management', () => {
  it('loads members only for the system owner', () => {
    expect(canManageMembers('owner')).toBe(true)
    for (const role of ['maintainer', 'reviewer', 'runner', 'viewer'] as const) expect(canManageMembers(role)).toBe(false)
  })

  it('renders the real member identity, role and status for owners', () => {
    const html = renderToStaticMarkup(<MemberSettingsView role="owner" state={{ status: 'ready', items: members }} onUpsertMember={noop} />)
    expect(html).toContain('平台管理员')
    expect(html).toContain('owner@example.test')
    expect(html).toContain('发布审核员')
    expect(html).toContain('已停用')
    expect(html).toContain('更新角色')
    expect(html).toContain('添加或更新成员')
  })

  it('shows honest loading, empty and error states', () => {
    expect(renderToStaticMarkup(<MemberSettingsView role="owner" state={{ status: 'loading' }} onUpsertMember={noop} />)).toContain('正在加载成员')
    expect(renderToStaticMarkup(<MemberSettingsView role="owner" state={{ status: 'empty' }} onUpsertMember={noop} />)).toContain('尚未配置成员')
    expect(renderToStaticMarkup(<MemberSettingsView role="owner" state={{ status: 'error', message: '成员服务不可用' }} onUpsertMember={noop} />)).toContain('成员服务不可用')
  })

  it('does not expose member data or mutation controls to non-owners', () => {
    for (const role of ['maintainer', 'reviewer', 'runner', 'viewer'] as const) {
      const html = renderToStaticMarkup(<MemberSettingsView role={role} state={{ status: 'ready', items: members }} onUpsertMember={noop} />)
      expect(html).toContain('仅系统所有者可管理成员')
      expect(html).not.toContain('平台管理员')
      expect(html).not.toContain('<form')
    }
  })
})
