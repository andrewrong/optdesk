// The fixtures are written from the schema alone: the page shows their texts as written.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { StockOptionView, registration } from '@/pages/stockoption/view'
import { validate, type Payload } from '@/types/stock_option_page'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion: unknown = JSON.parse(
  readFileSync(path.join(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)

let root: Root | undefined
let host: HTMLElement | undefined
async function show(payload: Payload): Promise<string> {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(<StockOptionView payload={payload} />)
  })
  return host.textContent ?? ''
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

describe('stock option view', () => {
  it('shows every block of ok', async () => {
    const page = await show(ok)
    for (const text of [
      ok.title,
      ok.top.symbol,
      ok.top.spot,
      ok.top.trend,
      ok.top.policy,
      ok.cards[0]?.title,
      ok.cards[0]?.advice,
      ok.cards[0]?.ladder[0].text,
      ok.matrix.facts,
      ok.matrix.legend,
      ok.matrix.cells[0]?.yield_text,
      ok.maps[0]?.heading,
      ok.maps[0]?.description,
      ok.maps[0]?.bands[0]?.text,
      ok.maps[0]?.marks[0]?.text,
      ok.contracts.basis,
      ok.contracts.notes[0],
      ok.contracts.columns[0],
      ok.contracts.rows[0]?.cells[6],
      ok.held[0]?.headline,
      ok.held[0]?.earned,
      ok.held[0]?.progress?.note,
      ok.notice,
      ok.footer,
    ]) {
      expect(text).toBeTruthy()
      expect(page).toContain(text as string)
    }
    expect(page).not.toContain('Gap the quotes')
  })

  it('prints top.iv as written and top.earnings in a secondary badge', async () => {
    const page = await show(ok)
    expect(page).toContain('IV 24.1% · HV30 缺 · IVP 38')
    expect(page).toContain(ok.top.iv)
    const badge = [...(host?.querySelectorAll('[data-slot="badge"]') ?? [])].find(
      (el) => el.textContent === ok.top.earnings,
    )
    expect(badge).toBeTruthy()
    expect(badge?.getAttribute('data-variant')).toBe('secondary')
  })

  it('switches each matrix cell from yield_text to premium on the toggle', async () => {
    const before = await show(ok)
    const cell = ok.matrix.cells[0]
    if (!cell) throw new Error('ok has no matrix cell')
    expect(before).toContain(cell.yield_text)
    expect(before).not.toContain(cell.premium)
    const toggle = host?.querySelector('[data-block="matrix"] [data-slot="toggle"]') as HTMLElement
    expect(toggle).toBeTruthy()
    await act(async () => toggle.click())
    const after = host?.textContent ?? ''
    expect(after).toContain(cell.premium)
    expect(after).not.toContain(cell.yield_text)
  })

  it('shows the Θ, Γ and Vega columns and every cell of every contract row', async () => {
    const page = await show(ok)
    const ths = [...(host?.querySelectorAll('[data-block="contracts"] th') ?? [])]
    for (const term of ['Θ', 'Γ', 'Vega']) {
      expect(ths.some((th) => th.textContent?.includes(term))).toBe(true)
    }
    for (const row of ok.contracts.rows) {
      for (const cell of row.cells) expect(page).toContain(cell)
      expect(row.cells).toHaveLength(ok.contracts.columns.length)
    }
  })

  it('shows the stale label of stale', async () => {
    const page = await show(stale)
    expect(page).toContain(stale.stale.text)
    expect(page).toContain(stale.top.symbol)
  })

  it('shows the gap lines of gap in place of the blocks', async () => {
    const page = await show(gap)
    expect(page).toContain(gap.shared_gap)
    expect(page).toContain(gap.no_card)
    expect(page).toContain(gap.matrix.empty)
    expect(page).toContain(gap.contracts.empty)
    expect(page).not.toContain(ok.cards[0]?.title ?? '')
    expect(page).not.toContain('Notice read the card first')
  })

  it('refuses a schema_version it does not read', () => {
    const result = validate(badVersion)
    expect(result.ok).toBe(false)
    expect(validate(ok).ok).toBe(true)
  })

  it('registers under its schema name', () => {
    expect(registration.schema).toBe(ok.schema)
  })
})
