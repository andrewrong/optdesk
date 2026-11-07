// Issue #1126: the shell dispatches on `payload.schema` through the page registry.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { OptionDataPage } from '@/pages/optiondata/OptionDataPage'
import { App } from '@/shell/App'
import { page } from '@/shell/page'
import { discoverCards, pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const OK = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, '../../fixtures/advice-page/ok.json'), 'utf8'),
)
const HELLO_TEXT = 'option-trader web shell, dev page, not a report'

let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<App payload={payload} />)
  })
}

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null) // jsdom has no canvas
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  await act(async () => root?.unmount())
  root = null
  host?.remove()
})

describe('page registry', () => {
  it('has the advice and household pages', () => {
    expect(Object.keys(pages)).toContain('advice_page')
    expect(Object.keys(pages)).toContain('household_page')
  })

  it('registers option_data_page with the option data page component', () => {
    expect(pages['option_data_page']?.component).toBe(OptionDataPage)
  })

  it('renders the top navigation bar on a registry page', async () => {
    await show(OK)
    expect(host.querySelector('nav[aria-label="主导航"]')).not.toBeNull()
  })

  it('renders the advice page for an advice_page payload', async () => {
    await show(OK)
    expect(host.textContent).toContain(OK.title)
    expect(host.textContent).not.toContain(HELLO_TEXT)
  })

  it('falls back to the hello page for an unknown schema, none, or a name on the prototype', async () => {
    for (const payload of [{ schema: 'nope' }, { schema: 'constructor' }, {}, undefined, 'x']) {
      await act(async () => root?.unmount())
      host?.remove()
      await show(payload)
      expect(host.textContent).toContain(HELLO_TEXT)
    }
  })

  it('throws for a payload that does not validate, so the error boundary can show the notice', async () => {
    await expect(show({ ...OK, schema_version: 2 })).rejects.toThrow(/Unsupported schema_version/)
  })
})

describe('discoverCards', () => {
  const card = page<{ n: number }>(
    () => ({ ok: true, payload: { n: 1 } }),
    () => null,
  )

  it('adds a card by its registration and skips a module without one', () => {
    const found = discoverCards({
      './AlphaCard.tsx': { registration: { schema: 'alpha_summary', page: card } },
      './OptionsSummaryCard.tsx': { OptionsSummaryCard: () => null },
      './Empty.tsx': null,
    })

    expect(Object.keys(found)).toEqual(['alpha_summary'])
    expect(found.alpha_summary).toBe(card)
  })

  it('throws on a duplicate schema, among the modules or against a taken one', () => {
    const registration = { schema: 'alpha_summary', page: card }

    expect(() =>
      discoverCards({ './A.tsx': { registration }, './B.tsx': { registration } }),
    ).toThrow(/Duplicate page schema "alpha_summary"/)
    expect(() => discoverCards({ './A.tsx': { registration } }, { alpha_summary: card })).toThrow(
      /Duplicate/,
    )
  })

  it('throws on duplicate __proto__ schema registrations', () => {
    // Issue #1155 requires duplicate schemas to throw.
    const registration = { schema: '__proto__', page: card }

    expect(() =>
      discoverCards({
        './A.tsx': { registration },
        './B.tsx': { registration },
      }),
    ).toThrow(/Duplicate page schema/)
  })
})
