// The fixtures are written from the schema alone; each text asserted is read from a fixture field.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { registration, View } from '@/pages/ledger/view'
import { pages } from '@/shell/pages'
import { validate } from '@/types/ledger_page'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
// jsdom has no ResizeObserver, which the chart container needs.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: typeof ok | typeof gap | typeof stale) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<View payload={payload} />)
  })
  return host.textContent ?? ''
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

// A fixture field that must exist: fails the test (not the type checker) when it is absent.
function req<T>(value: T | null | undefined): T {
  if (value === null || value === undefined) throw new Error('fixture field missing')
  return value
}
const on = (text: string | null | undefined, page: string) => expect(page).toContain(req(text))

describe('ledger view', () => {
  it('shows header, overview, sections, shares, notes and footer of ok', async () => {
    const page = await show(ok)
    on(ok.header.title, page)
    on(ok.header.through, page)
    on(ok.header.fx_note, page)
    for (const row of ok.overview) {
      on(row.title, page)
      on(row.count, page)
    }
    on(req(req(ok.overview[0]).total).text, page)
    const section = req(ok.sections[0])
    on(section.title, page)
    on(section.strategy_columns[0], page)
    on(section.strategies[0].cells[0].text, page)
    on(section.open_title, page)
    on(req(section.open[0]).label, page)
    on(section.lead.caption, page)
    on(section.more[0], page)
    const share = req(ok.shares[0])
    on(share.title, page)
    on(share.lead, page)
    on(req(req(share.rows[0]).cells[0]).text, page)
    on(req(req(share.sales[0]).cells[3]).text, page)
    on(req(ok.notes[0]).term, page)
    on(req(ok.notes[0]).text, page)
    on(req(ok.footer[0]), page)
    on(ok.title, page)
    on(ok.stale.text, page)
    // ok.nothing_closed is null: the line is absent and no gap is listed (ok.gaps is empty).
    expect(ok.nothing_closed).toBeNull()
    expect(host.querySelector('[data-nothing-closed]')).toBeNull()
    expect(ok.gaps).toHaveLength(0)
    expect(host.querySelector('[data-gaps]')).toBeNull()
  })

  it('draws one line chart of the section points with its summary, and none without a chart', async () => {
    const first = req(ok.sections[0])
    const chart = req(first.chart)
    const page = await show(ok)
    // One line chart per section that has a chart; the JPY section has none.
    const drawn = ok.sections.filter((s) => s.chart !== null)
    expect(drawn.length).toBeLessThan(ok.sections.length)
    expect(host.querySelectorAll('[data-chart]')).toHaveLength(drawn.length)
    expect(host.querySelector('[data-chart]')?.getAttribute('aria-label')).toBe(chart.summary)
    expect(host.querySelector('[data-points]')?.getAttribute('data-points')).toBe(String(chart.points.length))
    on(chart.summary, page)
    expect(page).toContain('Ann.')
    on(ok.header.fx_note, page)
    on('n<30', page)
    const without = structuredClone(ok)
    for (const section of without.sections) section.chart = null as never
    await act(async () => root?.unmount())
    host.remove()
    await show(without)
    expect(host.querySelector('[data-chart]')).toBeNull()
  })

  it('carries ten or more points in each USD section and approx_usd text in the JPY one', () => {
    for (const section of ok.sections.filter((s) => s.key.endsWith('_usd'))) {
      expect(req(section.chart).points.length).toBeGreaterThanOrEqual(10)
    }
    const jpy = req(ok.sections.find((s) => s.key.endsWith('_jpy')))
    expect(req(jpy.lead.total).text).toContain('approx_usd')
  })

  it('shows the nothing_closed line and the stale label of stale', async () => {
    const page = await show(stale)
    on(stale.nothing_closed, page)
    on(stale.stale.text, page)
    on(stale.header.title, page)
    on(req(stale.sections[0]).title, page)
    on(req(stale.shares[0]).title, page)
    on(req(stale.notes[0]).text, page)
    on(req(stale.footer[0]), page)
    expect(req(stale.overview[0]).title.length).toBeGreaterThan(0)
    on(req(stale.overview[0]).title, page)
    expect(host.querySelector('[data-stale="true"]')).not.toBeNull()
  })

  it('shows the gaps and totals_differ of gap', async () => {
    const page = await show(gap)
    on(req(gap.gaps[0]).text, page)
    on(req(gap.sections[0]).totals_differ, page)
    on(req(gap.sections[0]).lead.caption, page)
    on(gap.header.title, page)
    on(req(gap.notes[0]).text, page)
    on(req(gap.footer[0]), page)
    on(req(gap.shares[0]).note, page)
    expect(gap.overview.length).toBe(2)
    expect(gap.sections.length).toBe(1)
  })

  it('validate refuses bad-version and accepts ok', () => {
    const refused = validate(badVersion)
    expect(refused.ok).toBe(false)
    expect(validate(ok).ok).toBe(true)
  })

  it('registers itself under ledger_page', () => {
    expect(registration.schema).toBe('ledger_page')
    expect(pages.ledger_page).toBe(registration.page)
  })
})
