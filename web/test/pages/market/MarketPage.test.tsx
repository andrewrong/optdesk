// The fixtures are written from the schema alone; each text asserted is read from a fixture field.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ChartSpec } from '@/charts/spec'
import { MarketPage, registration } from '@/pages/market/MarketPage'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

// The Chart is a stub that records the spec it is given.
const specs: Record<string, ChartSpec> = {}
vi.mock('@/charts/Chart', () => ({
  Chart: ({ spec, name }: { spec: ChartSpec; name: string }) => {
    specs[name] = spec
    return <div data-slot="chart" data-name={name} />
  },
}))

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
async function show(payload: Parameters<typeof MarketPage>[0]['payload']) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MarketPage payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

const figureTexts = (f: { kind: string; text: string }) => f.text
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1

describe('MarketPage', () => {
  it('shows the weather, indices, term and breadth in order with every payload text', async () => {
    const page = await show(ok)
    const text = page.textContent ?? ''
    expect(text).toContain(ok.weather.headline)
    for (const row of ok.indices.rows) {
      expect(text).toContain(row.name)
      for (const f of [row.iv, row.rv20, row.iv_minus_rv20, row.rank_3y]) expect(text).toContain(figureTexts(f))
    }
    expect(text).toContain(ok.term.sentence)
    for (const row of ok.breadth.rows) {
      expect(text).toContain(row.label)
      expect(text).toContain(row.figure.text)
    }
    const at = (name: string) => page.querySelector(`[data-block="${name}"]`) as Element
    const before = (a: Element, b: Element) =>
      expect(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    before(at('weather'), at('indices'))
    before(at('indices'), at('term'))
    before(at('term'), at('breadth'))
  })

  it('shows the cause of a missing figure and of a missing term, with no term chart', async () => {
    const page = await show(gap)
    const text = page.textContent ?? ''
    for (const row of gap.indices.rows.filter((r) => r.name.includes('N225') || r.name.includes('TOPX'))) {
      expect(text).toContain(row.name)
      for (const f of [row.iv, row.rv20, row.iv_minus_rv20, row.rank_3y]) expect(text).toContain(f.text)
    }
    expect(count(text, gap.term.text)).toBe(1)
    expect(page.querySelector('[data-block="term"] [data-slot="chart"]')).toBeNull()
    expect(page.querySelector('[data-block="term"] svg, [data-block="term"] [role="img"]')).toBeNull()
    expect(page.querySelector('[data-block="weather"] [data-slot="chart"]')).not.toBeNull()
  })

  it('lays the blocks in cells, the weather and the breadth across both columns', async () => {
    const page = await show(ok)
    const cells = Array.from(page.querySelectorAll('[data-cell]'))
    expect(cells.map((c) => c.getAttribute('data-cell'))).toEqual(['weather', 'indices', 'term', 'breadth'])
    expect(cells.map((c) => c.classList.contains('lg:col-span-2'))).toEqual([true, false, false, true])
    expect(page.querySelector('[data-page="market-page"]')?.classList.contains('min-w-0')).toBe(true)
    expect(cells[0]!.parentElement?.className).toContain('lg:grid-cols-2')
  })

  it('shows a missing chart, term and breadth as an Alert with the reason', async () => {
    const gaps = {
      chart: { kind: 'gap' as const, reason: 'no_data' as const, text: '天气图缺失' },
      term: { kind: 'gap' as const, reason: 'not_built' as const, text: '期限缺失' },
      breadth: { kind: 'gap' as const, reason: 'no_rule' as const, text: '宽度缺失' },
    }
    const page = await show({ ...ok, weather: { ...ok.weather, chart: gaps.chart }, term: gaps.term, breadth: gaps.breadth })
    for (const [cell, g] of [['weather', gaps.chart], ['term', gaps.term], ['breadth', gaps.breadth]] as const) {
      const alert = page.querySelector(`[data-cell="${cell}"] [role="alert"]`)
      expect(alert?.getAttribute('data-gap')).toBe(g.reason)
      expect(alert?.textContent).toBe(g.text)
    }
  })

  it('marks the term series as holed and keeps a null point null', async () => {
    const points = [
      { ...ok.term.points[0]!, today: { kind: 'gap' as const, reason: 'missing_quote' as const, text: '缺口' } },
      ok.term.points[1]!,
    ]
    await show({ ...ok, term: { ...ok.term, points } })
    const today = specs['market-term']!.series[0]!
    expect(today.hole).toBe(true)
    expect(today.points[0]!.value).toBeNull()
  })

  it('shows the stale label', async () => {
    const page = await show(stale)
    expect(page.textContent).toContain(stale.stale.text)
  })

  it('refuses a payload of another schema_version', () => {
    const checked = registration.page.validate(badVersion)
    expect(checked.ok).toBe(false)
  })
})
