import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { AuthenticatedUserView } from './AppShell'

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
