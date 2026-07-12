import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { AuthenticatedUserView, NavigationMenu } from './AppShell'

describe('AuthenticatedUserView', () => {
  it('shows the signed-in identity and a logout action', () => {
    const html = renderToStaticMarkup(<AuthenticatedUserView user={{
      id: 'user-1',
      email: 'admin@example.com',
      displayName: '平台管理员',
      platformRole: 'admin',
    }} onLogout={() => undefined} />)

    expect(html).toContain('平台管理员')
    expect(html).toContain('admin@example.com')
    expect(html).toContain('退出登录')
  })
})

describe('NavigationMenu', () => {
  it('renders first-level group labels and second-level route links', () => {
    const html = renderToStaticMarkup(<MemoryRouter initialEntries={['/systems/a/scan']}><NavigationMenu groups={[
      { label: '资产管理', items: [{ label: '代码扫描', to: '/systems/a/scan', icon: '⌕' }, { label: 'API 资产', to: '/systems/a/apis', icon: '⇄' }] },
      { label: '系统管理', items: [{ label: '成员与权限', to: '/systems/a/settings/members', icon: '♙' }] },
    ]} /></MemoryRouter>)
    expect(html).toContain('资产管理')
    expect(html).toContain('系统管理')
    expect(html).toContain('href="/systems/a/scan"')
    expect(html).toContain('nav-item nav-subitem active')
  })
})
