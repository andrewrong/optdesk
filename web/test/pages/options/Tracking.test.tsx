// Issue #1827: the page shows the rows in payload order with the strings Python sent; the fixtures are the
// independent reference.
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { Tracking, validateTracking } from '@/pages/options/Tracking'
import { pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const dir = path.resolve(import.meta.dirname, '../../../fixtures/tracking')
const fixture = (name: string) => JSON.parse(readFileSync(path.join(dir, name), 'utf8'))
let root: Root | undefined
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<Tracking payload={payload as never} />)
  })
}
beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

type Cell = { text: string }
interface Row {
  symbol: string
  expiry_text: string
  strike: string
  status: string
  outcome: string
  premium: Cell
  underlying_move: Cell
  assigned: string | null
}

it('registers the page and validates the fixtures', () => {
  expect(pages.tracking).toBeDefined()
  for (const name of readdirSync(dir).filter(
    (name) => name.endsWith('.json') && (!name.startsWith('bad-') || name === 'bad-version.json'),
  )) {
    expect(validateTracking(fixture(name)).ok, name).toBe(!name.startsWith('bad-'))
  }
})

it('shows the rows in payload order, the split as sent and the 指派 flag as a badge', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  const trs = host.querySelectorAll('tbody tr')
  expect(trs).toHaveLength(payload.rows.length)
  payload.rows.forEach((row: Row, i: number) => {
    const t = trs[i]!.textContent ?? ''
    for (const cell of [
      row.symbol,
      row.expiry_text,
      row.strike,
      row.status,
      row.outcome,
      row.premium.text,
      row.underlying_move.text,
    ]) {
      expect(t).toContain(cell)
    }
    const badges = [...trs[i]!.querySelectorAll('[data-slot="badge"]')].map((b) => b.textContent)
    if (row.assigned !== null) expect(badges).toContain(row.assigned)
  })
  expect(payload.rows.some((row: Row) => row.assigned !== null)).toBe(true)
  expect(host.textContent).toContain(payload.pending)
  for (const line of payload.how) expect(host.textContent).toContain(line)
})

it('shows the page note once', async () => {
  const payload = fixture('gap.json')
  expect(typeof payload.note).toBe('string')
  await show(payload)
  expect((host.textContent ?? '').split(payload.note)).toHaveLength(2)
})
