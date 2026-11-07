// The fixtures of contracts/fixtures are the independent reference: the page shows their texts as written.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { OptionDataPage, validateOptionDataPage } from '@/pages/optiondata/OptionDataPage'
import type { OptionDataPagePayload } from '@/pages/optiondata/OptionDataPage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const dir = path.resolve(import.meta.dirname, '../../../../contracts/fixtures')
const fixture = (name: string) =>
  JSON.parse(readFileSync(path.join(dir, `page.option_data_page.v1.${name}.json`), 'utf8'))

let root: Root | undefined
let host: HTMLElement
async function show(payload: OptionDataPagePayload) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionDataPage payload={payload} />)
  })
}
beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const rowsOf = () => host.querySelectorAll('tbody tr')

describe('option data page', () => {
  it('ok shows 6 or more rows and 3 series', async () => {
    const payload = fixture('ok')
    await show(payload)
    expect(rowsOf().length).toBeGreaterThanOrEqual(6)
    expect(host.querySelectorAll('[role="tab"]')).toHaveLength(3)
    expect(host.textContent).toContain(payload.title)
    expect(host.textContent).toContain(payload.sentence)
    expect(host.textContent).toContain(payload.chain_title)
    expect(host.textContent).toContain(payload.underlying.price.text)
  })

  it('shows the gap text for the status-2 row and the null delta, never 0', async () => {
    const payload = fixture('ok')
    await show(payload)
    const rows: { status: { code: number }; delta: { kind: string; text: string } }[] =
      payload.chain.rows
    const noQuote = rows.findIndex((r) => r.status.code === 2)
    const nullDelta = rows.findIndex((r) => r.status.code !== 2 && r.delta.kind === 'gap')
    expect(noQuote).toBeGreaterThanOrEqual(0)
    expect(nullDelta).toBeGreaterThanOrEqual(0)
    const cells = (i: number) => [...rowsOf()[i]!.querySelectorAll('td')].map((c) => c.textContent)
    expect(cells(noQuote)).toContain('no quote')
    expect(cells(noQuote)).toContain(payload.chain.rows[noQuote].bid.text)
    expect(cells(nullDelta)).toContain(payload.chain.rows[nullDelta].delta.text)
    expect(cells(nullDelta)).not.toContain('0')
    expect(cells(noQuote)).not.toContain('0')
  })

  it('terms shows the strike, expiry and right text', async () => {
    const payload = fixture('terms')
    await show(payload)
    const row = payload.chain.rows[0]
    const text = rowsOf()[0]!.textContent
    expect(text).toContain(row.strike.text)
    expect(text).toContain(row.expiry)
    expect(text).toContain(row.right)
  })

  it('gap shows the empty-table message and no chart', async () => {
    const payload = fixture('gap')
    await show(payload)
    expect(rowsOf()).toHaveLength(0)
    expect(host.querySelector('[data-chain-empty]')?.textContent).toBe(payload.sentence)
    expect(host.textContent).toContain(payload.gaps[0].text)
    expect(host.querySelector('[data-slot="chart"]')).toBeNull()
  })

  it('stale shows the stale text', async () => {
    const payload = fixture('stale')
    await show(payload)
    expect(host.querySelector('[data-stale="true"]')?.textContent).toBe(payload.stale.text)
  })

  it('bad-version is rejected by validate and the others are accepted', () => {
    const bad = validateOptionDataPage(fixture('bad-version'))
    expect(bad.ok).toBe(false)
    for (const name of ['ok', 'terms', 'gap', 'stale']) {
      expect(validateOptionDataPage(fixture(name)).ok, name).toBe(true)
    }
  })

  it('choices shows three links of the fixture, the current one marked', async () => {
    const payload = fixture('choices')
    await show(payload)
    const nav = host.querySelectorAll('[data-underlying-links] a')
    expect([...nav].map((a) => a.getAttribute('href'))).toEqual(
      payload.underlyings.map((c: { href: string }) => c.href),
    )
    const marked = [...nav].filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked).toHaveLength(1)
    expect(marked[0]?.getAttribute('href')).toBe('/options/data/9000')
  })

  it('ok shows no choices block', async () => {
    await show(fixture('ok'))
    expect(host.querySelector('[data-underlying-links]')).toBeNull()
  })

  it('validate rejects a non-string href', () => {
    const bad = fixture('choices')
    bad.underlyings[0].href = 123
    expect(validateOptionDataPage(bad).ok).toBe(false)
  })

  it('validate still accepts ok and choices', () => {
    expect(validateOptionDataPage(fixture('ok')).ok).toBe(true)
    expect(validateOptionDataPage(fixture('choices')).ok).toBe(true)
  })
})
