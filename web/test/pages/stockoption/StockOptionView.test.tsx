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
      ok.contracts.rows[0]?.cells[10],
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
