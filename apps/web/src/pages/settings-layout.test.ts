// @ts-expect-error Vitest runs in Node; the browser bundle intentionally omits Node typings.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const stylesheet = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')

describe('system settings layout contract', () => {
  it('defines the page, card and responsive form selectors used by settings components', () => {
    for (const selector of [
      '.settings-page',
      '.workspace-page',
      '.page-card',
      '.settings-form',
      '.settings-form input',
      '.settings-form textarea',
      '.settings-form select',
      '.settings-page .status',
    ]) expect(stylesheet).toContain(selector)
    expect(stylesheet).toMatch(/@media \(max-width: 900px\)[\s\S]*\.settings-form/)
  })
})
