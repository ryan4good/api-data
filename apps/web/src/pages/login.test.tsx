import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { LoginPage } from './login'

describe('LoginPage', () => {
  it('renders an accessible credential form', () => {
    const html = renderToStaticMarkup(<MemoryRouter><LoginPage /></MemoryRouter>)
    expect(html).toContain('登录 BizDevOps')
    expect(html).toContain('type="email"')
    expect(html).toContain('type="password"')
    expect(html).toContain('进入平台')
  })
})
