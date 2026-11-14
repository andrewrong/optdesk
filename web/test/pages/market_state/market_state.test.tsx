// The market state page shows the texts of the contract fixtures and nothing else. Expected strings are read from them.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChartSpec } from '@/charts/spec'
import { ChartTooltip } from '@/components/ui/chart'
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
  return {
    ...actual,
    ChartContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
    // The tooltip content is shown as if the pointer rested on a point of the unranked fund.
    ChartTooltip: ({ content }: { content: React.ReactElement }) =>
      React.cloneElement(content, {
        active: true,
        payload: [{ payload: { fund: 'SMH', note: 'not ranked', day: 'Nov 6', text: 'x +1.0, y -0.5' } }],
      } as object),
  }
})
// recharts draws nothing in jsdom (no size): the parts that carry the data are replaced by markers of it.
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>()
  return {
    ...actual,
    ScatterChart: ({ children }: { children?: React.ReactNode }) => <svg data-scatter-chart>{children}</svg>,
    // A series draws its shape per point and the content of its label list per point, as recharts does.
    Scatter: ({ data, name, shape, children }: { data: unknown[]; name: string; shape: React.ReactElement; children?: React.ReactElement<{ content: React.ReactElement }> }) => (
      <g data-scatter={name} data-points={data.length} data-data={JSON.stringify(data)}>
        {data.map((_, i) => React.cloneElement(shape, { key: i, cx: 10 * i, cy: 5, index: i } as object))}
        {data.map((_, i) => React.cloneElement(children!.props.content, { key: i, x: 10 * i, y: 5, index: i } as object))}
      </g>
    ),
    // Outside a real chart there is no scale: a fixed one stands in, so the arrow can be drawn.
    // Only the axes of the rotation map carry a title; the bar chart's axes are the real ones.
    XAxis: (props: React.ComponentProps<typeof actual.XAxis>) =>
      props.label ? (
        <g data-axis="x" data-padding={JSON.stringify(props.padding)}>
          <text>{(props.label as { value: string }).value}</text>
        </g>
      ) : (
        <actual.XAxis {...props} />
      ),
    YAxis: (props: React.ComponentProps<typeof actual.YAxis>) =>
      props.label ? (
        <g data-axis="y" data-padding-top={(props.padding as { top: number }).top}>
          <text>{(props.label as { value: string }).value}</text>
        </g>
      ) : (
        <actual.YAxis {...props} />
      ),
    useCartesianScale: (pt: { x: number; y: number }) => ({ x: pt.x, y: pt.y }),
    ReferenceArea: ({ x1, x2, y1, y2, label }: { x1?: number; x2?: number; y1?: number; y2?: number; label: { value: string; position: string } }) => (
      <g data-quadrant-area={`${x1 === undefined ? 'left' : 'right'}-${y1 === undefined ? 'bottom' : 'top'}`} data-corner={label.position} data-y2={y2} data-x2={x2}>
        <text>{label.value}</text>
      </g>
    ),
    // The bar chart is marked and also drawn for real at a fixed size, so the test reads what recharts lays out.
    BarChart: (props: React.ComponentProps<typeof actual.BarChart>) => (
      <>
        <svg data-bar-chart data-bars={(props.data as unknown[]).length} data-layout={props.layout} />
        <actual.BarChart {...props} width={700} height={360}>
          {React.Children.toArray(props.children).filter((c) => !(React.isValidElement(c) && c.type === ChartTooltip))}
        </actual.BarChart>
      </>
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
      .filter((e) => e.children.length === 0 && !e.closest('[data-quadrant-area]'))
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

  it('draws one series per drawn fund with its points, a today label and an arrow', async () => {
    const p = fixture('ok')
    await show(p)
    const series = [...host.querySelectorAll('[data-scatter]')]
    const withRotation = p.sectors.rows.filter((r: { rotation: { kind?: string } }) => r.rotation.kind === undefined)
    expect(series).toHaveLength(withRotation.length)
    expect(host.querySelector('[data-scatter=XLRE]')).toBeNull()
    expect(host.querySelector('[data-scatter=XLF]')!.getAttribute('data-points')).toBe('4')
    expect(host.querySelector('[data-scatter=SMH]')!.getAttribute('data-points')).toBe('5')
    for (const row of withRotation) {
      const s = host.querySelector(`[data-scatter=${row.symbol}]`)!
      const drawn = JSON.parse(s.getAttribute('data-data')!)
      expect(drawn.map((pt: { text: string }) => pt.text)).toEqual(row.rotation.map((pt: { text: string }) => pt.text))
      expect(drawn.map((pt: { x: number }) => pt.x)).toEqual(row.rotation.map((pt: { x: number }) => pt.x))
      expect(s.querySelectorAll('[data-tail-point]')).toHaveLength(row.rotation.length)
      expect([...s.querySelectorAll('[data-today-label]')].map((e) => e.textContent)).toEqual([row.symbol])
      expect(host.querySelectorAll(`[data-arrow=${row.symbol}]`)).toHaveLength(1)
      const last = s.querySelector(`[data-tail-point="${row.rotation.length - 1}"]`)!
      const first = s.querySelector('[data-tail-point="0"]')!
      expect(Number(last.getAttribute('r'))).toBeGreaterThan(Number(first.getAttribute('r')))
      expect(Number(first.getAttribute('opacity'))).toBeLessThan(1)
    }
  })

  it('draws each fund in its own fixed colour and SMH as a hollow neutral ring', async () => {
    await show(fixture('ok'))
    const stroke = (symbol: string) => host.querySelector(`[data-scatter=${symbol}] [data-tail-point]`)!.getAttribute('stroke')
    const colours = ['XLK', 'XLC', 'XLY', 'XLF', 'XLI', 'XLE', 'XLV', 'XLB', 'XLU', 'XLP'].map(stroke)
    expect(new Set(colours).size).toBe(colours.length)
    expect(colours).not.toContain(stroke('SMH'))
    expect(stroke('SMH')).toBe('#6b7280')
    const ring = host.querySelector('[data-scatter=SMH] [data-tail-point]')!
    expect(ring.getAttribute('fill')).toBe('var(--background)')
  })

  it('gives XLC the eleventh hue whatever its rank', async () => {
    await show(fixture('ok'))
    const xlc = host.querySelector('[data-scatter=XLC] [data-tail-point]')!
    expect(xlc.getAttribute('stroke')).toBe('#38b6e8')
  })

  it('titles the axes with the captions and leaves room above the top tick', async () => {
    const p = fixture('ok')
    await show(p)
    expect(host.querySelector('[data-axis=x]')?.textContent).toBe(p.sectors.rotation_axes.x_caption)
    expect(host.querySelector('[data-axis=y]')?.textContent).toBe(p.sectors.rotation_axes.y_caption)
    expect(Number(host.querySelector('[data-axis=y]')!.getAttribute('data-padding-top'))).toBeGreaterThan(0)
  })

  it('the tooltip shows the fund with its rank note, the day and the point text', async () => {
    await show(fixture('ok'))
    const tip = host.querySelector('[data-rotation-tip]')!
    expect(tip.textContent).toBe('SMH (not ranked)Nov 6x +1.0, y -0.5')
  })

  it('names the quadrants by fixed words at the four corners and shows no producer sentence', async () => {
    const p = fixture('ok')
    await show(p)
    const word = (corner: string) => host.querySelector(`[data-corner=${corner}]`)?.textContent
    expect(word('insideTopRight')).toBe('Leading')
    expect(word('insideTopLeft')).toBe('Improving')
    expect(word('insideBottomLeft')).toBe('Lagging')
    expect(word('insideBottomRight')).toBe('Weakening')
    const map = host.querySelector('[data-rotation-map]')!.textContent!
    for (const q of p.sectors.rotation_axes.quadrants) expect(map).not.toContain(q.text)
    expect(host.querySelector('[data-x-caption]')?.textContent).toBe(p.sectors.rotation_axes.x_caption)
    expect(host.querySelector('[data-y-caption]')?.textContent).toBe(p.sectors.rotation_axes.y_caption)
    expect(host.querySelector('[data-rotation-note]')?.textContent).toBe(p.sectors.rotation_axes.note)
  })

  it('keeps "not ranked" out of the legend names and in the SMH points for the tooltip', async () => {
    await show(fixture('ok'))
    expect(host.querySelector('[data-scatter=SMH]')).not.toBeNull()
    const outsideTip = [...host.querySelectorAll('[data-rotation-map] *')]
      .filter((e) => e.children.length === 0 && !e.closest('[data-rotation-tip]'))
      .map((e) => e.textContent)
      .join(' ')
    expect(outsideTip).not.toContain('not ranked')
    const smh = JSON.parse(host.querySelector('[data-scatter=SMH]')!.getAttribute('data-data')!)
    expect(smh[0].note).toContain('not ranked')
    const xlk = JSON.parse(host.querySelector('[data-scatter=XLK]')!.getAttribute('data-data')!)
    expect(xlk[0].note).toBe('')
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

  it('leaves room beyond the extreme bars and puts each label at the outer end of its bar, on one line', async () => {
    const p = fixture('ok')
    await show(p)
    const rows = p.sectors.rows.filter((r: { gap_63: { kind: string } }) => r.gap_63.kind === 'value') as {
      symbol: string
      gap_63: { value: number; text: string }
    }[]
    const values = rows.map((r) => r.gap_63.value)
    const negative = rows.find((r) => r.gap_63.value < 0)!
    const positive = rows.find((r) => r.gap_63.value > 0)!
    const ticks = [...host.querySelectorAll('.recharts-cartesian-axis-tick-value[orientation=bottom]')].map((t) => parseFloat(t.textContent ?? ''))
    // The ends of the axis (its first and last tick) lie at least 4 points beyond the extreme bars.
    expect(Math.min(...ticks)).toBeLessThanOrEqual(Math.min(...values) - 4)
    expect(Math.max(...ticks)).toBeGreaterThanOrEqual(Math.max(...values) + 4)
    const label = (text: string) => [...host.querySelectorAll('text.recharts-label')].find((t) => t.textContent === text)!
    expect(label(negative.gap_63.text).getAttribute('text-anchor')).toBe('end')
    expect(label(positive.gap_63.text).getAttribute('text-anchor')).toBe('start')
    for (const r of rows) expect(label(r.gap_63.text).querySelectorAll('tspan')).toHaveLength(1)
    // No label is drawn over the y axis tick text: the left edge of the plot is the x of the tick texts.
    const tickX = Number(host.querySelector('text.recharts-cartesian-axis-tick-value[orientation=left]')!.getAttribute('x'))
    const textRight = Number(label(negative.gap_63.text).getAttribute('x'))
    expect(textRight).toBeGreaterThan(tickX)
  })

  it('a market row text ends with the figure text, with no stray unit after it', async () => {
    const p = fixture('ok')
    await show(p)
    for (const row of p.market.rows) {
      const first = rowOf(row.id)!.querySelector('span > span:nth-child(2)')!
      expect(first.textContent).toBe(row.figure.text)
      expect(first.textContent).not.toMatch(/% %$/)
    }
  })

  it('writes the bar axis ticks as whole numbers in pp', async () => {
    await show(fixture('ok'))
    const ticks = [...host.querySelectorAll('.recharts-cartesian-axis-tick-value[orientation=bottom]')].map((t) => t.textContent ?? '')
    expect(ticks.length).toBeGreaterThan(1)
    for (const t of ticks) expect(t).toMatch(/^-?\d+ pp$/)
  })

  it('the gap fixture draws no rotation series', async () => {
    await show(fixture('gap'))
    expect(host.querySelectorAll('[data-scatter]')).toHaveLength(0)
  })
})
