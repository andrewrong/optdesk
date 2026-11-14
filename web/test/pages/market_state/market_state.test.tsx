// The market state page shows the texts of the contract fixtures and nothing else. Expected strings are read from them.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChartSpec } from '@/charts/spec'
import { MarketStatePage, registration } from '@/pages/market_state/MarketStatePage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.market_state_page.v1.${name}.json`),
      'utf8',
    ),
  )

vi.mock('@/charts/Chart', () => ({
  Chart: ({ spec }: { spec: ChartSpec }) => (
    <div
      data-chart-spec
      data-points={spec.series[0]!.points.length}
      data-categories={spec.categories.length}
      data-spec={JSON.stringify(spec)}
    />
  ),
}))
vi.mock('@/components/ui/chart', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/components/ui/chart')>()
  return { ...actual, ChartContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div> }
})
// recharts draws nothing in jsdom (no size): the parts that carry the data are replaced by markers of it.
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>()
  return {
    ...actual,
    ScatterChart: ({ children }: { children?: React.ReactNode }) => <svg data-scatter-chart>{children}</svg>,
    Scatter: ({ data, name }: { data: unknown[]; name: string }) => (
      <g data-scatter={name} data-points={data.length} data-data={JSON.stringify(data)} />
    ),
    BarChart: ({ data, layout }: { data: unknown[]; layout: string }) => (
      <svg data-bar-chart data-bars={data.length} data-layout={layout} />
    ),
  }
})

let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MarketStatePage payload={payload as never} />)
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
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})
const click = (el: Element | null) =>
  act(async () => {
    el?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
const rowOf = (id: string) => host.querySelector(`[data-market-row=${id}]`)

describe('market state page', () => {
  it('registers under its schema', () => {
    expect(registration.schema).toBe('market_state_page')
  })

  it('shows title, freshness text, five rows with window and day, in order', async () => {
    const p = fixture('ok')
    await show(p)
    expect(host.querySelector('h1')?.textContent).toBe(p.title)
    expect(host.querySelector('[data-freshness]')?.textContent).toBe(p.freshness_text)
    const rows = [...host.querySelectorAll('[data-market-row]')]
    expect(rows.map((r) => r.getAttribute('data-market-row'))).toEqual(p.market.rows.map((r: { id: string }) => r.id))
    p.market.rows.forEach((r: { window: string; day: string | { text: string }; figure: { text: string } }, i: number) => {
      expect(rows[i]!.querySelector('[data-window]')?.textContent).toBe(r.window)
      expect(rows[i]!.querySelector('[data-day]')?.textContent).toBe(typeof r.day === 'string' ? r.day : r.day.text)
      expect(rows[i]!.textContent).toContain(r.figure.text)
    })
  })

  it('opens the chart of the vix row with its points and caption, and closes it again', async () => {
    const p = fixture('ok')
    const vix = p.market.rows.find((r: { id: string }) => r.id === 'vix')
    await show(p)
    const button = rowOf('vix')!.querySelector('button')!
    expect(button.getAttribute('aria-expanded')).toBe('false')
    expect(rowOf('vix')!.querySelector('[data-chart-spec]')).toBeNull()
    await click(button)
    expect(button.getAttribute('aria-expanded')).toBe('true')
    const chart = rowOf('vix')!.querySelector('[data-chart-spec]')!
    expect(chart.getAttribute('data-points')).toBe(String(vix.history.points.length))
    expect(chart.getAttribute('data-categories')).toBe(String(vix.history.points.length))
    expect(rowOf('vix')!.querySelector('[data-history-caption]')?.textContent).toBe(vix.history.caption)
    const spec = JSON.parse(chart.getAttribute('data-spec')!)
    expect(spec.kind).toBe('line')
    expect(spec.categories).toEqual(vix.history.points.map((pt: { day: string }) => pt.day))
    expect(spec.series[0].points).toEqual(
      vix.history.points.map((pt: { value: number; text: string }) => ({ value: pt.value, text: pt.text })),
    )
    await click(button)
    expect(button.getAttribute('aria-expanded')).toBe('false')
    expect(rowOf('vix')!.querySelector('[data-chart-spec]')).toBeNull()
    expect(rowOf('vix')!.querySelector('[data-history-caption]')).toBeNull()
  })

  it('a row with a Gap history shows its text and opens no chart', async () => {
    const p = fixture('ok')
    const term = p.market.rows.find((r: { id: string }) => r.id === 'vix_term')
    await show(p)
    expect(rowOf('vix_term')!.querySelector('button')).toBeNull()
    expect(rowOf('vix_term')!.textContent).toContain(term.history.text)
    await click(rowOf('vix_term'))
    expect(rowOf('vix_term')!.querySelector('[data-chart-spec]')).toBeNull()
  })

  it('the gap fixture renders without a chart and without a number', async () => {
    const p = fixture('gap')
    await show(p)
    expect(host.querySelector('[data-chart-spec]')).toBeNull()
    expect(host.querySelectorAll('[data-market-row] button')).toHaveLength(0)
    p.market.rows.forEach((r: { id: string; label: string }) => {
      expect(rowOf(r.id)!.textContent!.replace(r.label, '')).not.toMatch(/\d/)
    })
    expect(host.querySelector('[data-freshness]')?.textContent).toBe(p.freshness_text)
  })

  it('shows no state word outside the four quadrant labels', async () => {
    await show(fixture('ok'))
    const text = [...host.querySelectorAll('*')]
      .filter((e) => e.children.length === 0 && !e.hasAttribute('data-quadrant'))
      .map((e) => e.textContent)
      .join(' ')
    for (const word of ['leading', 'lagging', 'risk-off', 'bullish', 'bearish']) {
      expect(text).not.toContain(word)
    }
  })

  it('shows the sector table with caption, end day, ranks and gap texts in payload order', async () => {
    const p = fixture('ok')
    await show(p)
    const table = host.querySelector('[data-sector-table]')!
    expect(table.querySelector('[data-sector-caption]')?.textContent).toBe(p.sectors.caption)
    expect(table.querySelector('[data-sector-end-day]')?.textContent).toBe(p.sectors.end_day)
    const rows = [...table.querySelectorAll('[data-sector-row]')]
    expect(rows.map((r) => r.getAttribute('data-sector-row'))).toEqual(p.sectors.rows.map((r: { symbol: string }) => r.symbol))
    const first = p.sectors.rows[0]
    expect(rows[0]!.textContent).toContain(`${first.rank_21.at} of ${first.rank_21.of}`)
    expect(rows[0]!.textContent).toContain(first.gap_63.text)
    const gap = p.sectors.rows.find((r: { symbol: string }) => r.symbol === 'XLRE')
    expect(table.querySelector('[data-sector-row=XLRE]')!.textContent).toContain(gap.gap_21.text)
    expect(table.querySelector('[data-sector-row=SMH]')!.textContent).toContain('not ranked')
  })

  it('draws one rotation series per row with a rotation, with the quadrant texts and captions', async () => {
    const p = fixture('ok')
    await show(p)
    const series = [...host.querySelectorAll('[data-scatter]')]
    const withRotation = p.sectors.rows.filter((r: { rotation: { kind?: string } }) => r.rotation.kind === undefined)
    expect(series).toHaveLength(withRotation.length)
    expect(host.querySelector('[data-scatter^=XLRE]')).toBeNull()
    expect(host.querySelector('[data-scatter^=XLF]')!.getAttribute('data-points')).toBe('4')
    const smh = host.querySelector('[data-scatter^=SMH]')!
    expect(smh.getAttribute('data-points')).toBe('5')
    expect(smh.getAttribute('data-scatter')).toContain('not ranked')
    for (const row of withRotation) {
      const drawn = JSON.parse(host.querySelector(`[data-scatter^=${row.symbol}]`)!.getAttribute('data-data')!)
      expect(drawn.map((pt: { text: string }) => pt.text)).toEqual(row.rotation.map((pt: { text: string }) => pt.text))
    }
    for (const q of p.sectors.rotation_axes.quadrants) {
      expect(host.querySelector(`[data-quadrant=${q.id}]`)?.textContent).toBe(q.text)
    }
    expect(host.querySelector('[data-x-caption]')?.textContent).toBe(p.sectors.rotation_axes.x_caption)
    expect(host.querySelector('[data-y-caption]')?.textContent).toBe(p.sectors.rotation_axes.y_caption)
    expect(host.querySelector('[data-rotation-note]')?.textContent).toBe(p.sectors.rotation_axes.note)
  })

  it('draws one vertical bar per row whose gap_63 is a value, with its caption', async () => {
    const p = fixture('ok')
    await show(p)
    const bars = host.querySelector('[data-bar-chart]')!
    expect(bars.getAttribute('data-layout')).toBe('vertical')
    const values = p.sectors.rows.filter((r: { gap_63: { kind: string } }) => r.gap_63.kind === 'value')
    expect(bars.getAttribute('data-bars')).toBe(String(values.length))
    expect(host.querySelector('[data-bars-caption]')?.textContent).toBe(p.sectors.bars_caption)
  })

  it('the gap fixture draws no rotation series', async () => {
    await show(fixture('gap'))
    expect(host.querySelectorAll('[data-scatter]')).toHaveLength(0)
  })
})
