// Issue #1102 acceptance 5: the app shell shows a fail-closed notice until the payload parsed and
// the first React commit happened, and shows it again when a component throws.
import { act, type ReactNode } from 'react'
import type { Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '@/shell/App'
import { mountShell } from '@/shell/mount'
import { loadPayload } from '@/vm'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const HELLO = '{"schema":"_dev/hello","schema_version":1}'

function page(payload: string): void {
  document.body.innerHTML =
    '<div id="fail-closed" role="alert">page did not load, do not act</div>' +
    '<div id="root"></div>' +
    `<script type="application/json" id="vm">${payload}</script>`
}
const notice = () => document.getElementById('fail-closed') as HTMLElement

let root: Root | null = null
async function mount(app: (payload: unknown) => ReactNode): Promise<void> {
  await act(async () => {
    root = mountShell(document, app)
  })
}
beforeEach(() => vi.spyOn(console, 'error').mockImplementation(() => {}))
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  vi.restoreAllMocks()
})

describe('payload', () => {
  it('parses the embedded JSON', () => {
    page(HELLO)
    expect(loadPayload(document)).toEqual({ schema: '_dev/hello', schema_version: 1 })
  })

  it('round-trips a closing script tag written as the publisher escapes it', () => {
    // The publisher writes every "<" as the six characters \u003c, so the page never holds a raw "<".
    page('{"note":"\\u003c/script>"}')
    expect(document.getElementById('vm')?.textContent).not.toContain('<')
    expect(loadPayload(document)).toEqual({ note: '</script>' })
  })

  it('throws when #vm is missing or not JSON', () => {
    page('not json')
    expect(() => loadPayload(document)).toThrow()
    document.body.innerHTML = ''
    expect(() => loadPayload(document)).toThrow()
  })
})

describe('shell', () => {
  it('shows the notice before the mount, hides it after the first commit and renders the Card', async () => {
    page(HELLO)
    expect(notice().hidden).toBe(false)
    await mount(() => <App />)
    expect(notice().hidden).toBe(true)
    expect(document.body.textContent).toContain('option-trader web shell, dev page, not a report')
  })

  it('keeps the notice visible when #vm is not JSON, and renders nothing', async () => {
    page('{nope')
    await mount(() => <App />)
    expect(notice().hidden).toBe(false)
    expect(document.getElementById('root')?.textContent).toBe('')
  })

  it('shows the notice again when a child throws', async () => {
    page(HELLO)
    function Boom(): ReactNode {
      throw new Error('boom')
    }
    await mount(() => <Boom />)
    expect(notice().hidden).toBe(false)
  })

  it('shows the notice again when a child throws after the first commit', async () => {
    page(HELLO)
    let trigger = () => {}
    const { useState } = await import('react')
    function Later(): ReactNode {
      const [bad, setBad] = useState(false)
      trigger = () => setBad(true)
      if (bad) throw new Error('late')
      return <p>fine</p>
    }
    await mount(() => <Later />)
    expect(notice().hidden).toBe(true)
    await act(async () => trigger())
    expect(notice().hidden).toBe(false)
  })
})
