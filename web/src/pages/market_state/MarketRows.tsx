// The five market rows: a row opens its chart below it. Every text, day and point is the payload's; nothing is parsed.
import { useState } from 'react'
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import type { MarketRow as RowV2 } from '@/generated/market_state_page.v2'
import type { Payload } from '@/types/market_state_page'
import { HintMark } from './HintMark'

// The v1 payload has no range or hint; the v2 payload may send both.
type Row = Payload['market']['rows'][number] & Pick<RowV2, 'range' | 'hint' | 'price_chart'>
type HistoryOk = Extract<Row['history'], { points: unknown }>
type PriceChart = NonNullable<RowV2['price_chart']>
type Range = NonNullable<RowV2['range']>

const TONE = {
  normal: 'bg-muted text-foreground',
  caution: 'bg-amber-100 text-amber-900',
  alarm: 'bg-red-100 text-red-900',
} as const

function RowRange({ range }: { range: Range }) {
  if ('kind' in range) {
    return (
      <span className="text-sm text-muted-foreground" data-range-gap>
        {range.text}
      </span>
    )
  }
  return (
    <span className="grid gap-1">
      <span className={`w-fit rounded px-2 py-0.5 text-sm ${TONE[range.state]}`} data-range-state={range.state}>
        {range.state_text}
      </span>
      {range.cuts.map((cut) => (
        <span key={cut.level} className="text-sm text-muted-foreground" data-range-cut={cut.level}>
          {cut.text}
        </span>
      ))}
      <span className="text-sm text-muted-foreground" data-range-notice>
        {range.notice}
      </span>
    </span>
  )
}

function RowText({ row }: { row: Row }) {
  const { figure, percentile } = row
  return (
    <span className="grid gap-1 text-left">
      <span className="font-medium">{row.label}</span>
      <span>{figure.text}</span>
      {percentile ? (
        <span className="text-sm text-muted-foreground">
          {percentile.text} · {percentile.window}
        </span>
      ) : null}
      <span className="text-sm text-muted-foreground" data-window>
        {row.window}
      </span>
      <span className="text-sm text-muted-foreground" data-day>
        {typeof row.day === 'string' ? row.day : row.day.text}
      </span>
      {row.range ? <RowRange range={row.range} /> : null}
    </span>
  )
}

// A day without an average or band point is a gap of the line, never a zero.
const pointOf = (value: number | undefined, text: string | undefined) =>
  value === undefined ? { value: null } : { value, text }

function priceSpec(label: string, chart: PriceChart): ChartSpec {
  const { points } = chart
  return {
    kind: 'line',
    legend: true,
    categories: points.map((point) => point.day),
    unit: chart.unit,
    series: [
      { name: label, points: points.map((point) => ({ value: point.close, text: point.close_text })) },
      {
        name: '200-day average',
        dashed: true,
        points: points.map((point) => pointOf(point.ma200, point.ma200_text)),
      },
    ],
    band: {
      name: 'Bollinger band',
      low: points.map((point) => pointOf(point.band_low, point.band_low_text)),
      high: points.map((point) => pointOf(point.band_high, point.band_high_text)),
    },
  }
}

function historySpec(label: string, history: HistoryOk): ChartSpec {
  return {
    kind: 'line',
    categories: history.points.map((point) => point.day),
    unit: history.unit,
    series: [{ name: label, points: history.points.map(({ value, text }) => ({ value, text })) }],
  }
}

// The chart of a row: price_chart when it has one, else its history; none for a history that is a gap.
function chartOf(row: Row): { source: PriceChart | HistoryOk; spec: ChartSpec } | null {
  if (row.price_chart) return { source: row.price_chart, spec: priceSpec(row.label, row.price_chart) }
  if ('kind' in row.history) return null
  return { source: row.history, spec: historySpec(row.label, row.history) }
}

function MarketRow({ row }: { row: Row }) {
  const [open, setOpen] = useState(false)
  const chart = chartOf(row)
  if (!chart) {
    const { history } = row
    return (
      <li className="relative rounded-md border p-3" data-market-row={row.id}>
        {row.hint ? <HintMark hint={row.hint} className="absolute right-3 top-3" /> : null}
        <RowText row={row} />
        <p className="mt-2 text-sm text-muted-foreground" data-history-gap>
          {'kind' in history ? history.text : null}
        </p>
      </li>
    )
  }
  return (
    <li className="relative rounded-md border" data-market-row={row.id}>
      {row.hint ? <HintMark hint={row.hint} className="absolute right-3 top-3" /> : null}
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="w-full cursor-pointer p-3 text-left"
      >
        <RowText row={row} />
      </button>
      {open ? (
        <div className="border-t p-3">
          <Chart
            name={`market-state-${row.id}`}
            ariaText={chart.source.aria_text}
            spec={chart.spec}
          />
          <p className="mt-2 text-sm text-muted-foreground" data-history-caption>
            {chart.source.caption}
          </p>
        </div>
      ) : null}
    </li>
  )
}

export function MarketRows({ block }: { block: Payload['market'] }) {
  return (
    <section aria-label={block.title} data-market-rows>
      <h2 className="mb-2 text-lg font-medium">{block.title}</h2>
      <ul className="grid gap-2">
        {block.rows.map((row) => (
          <MarketRow key={row.id} row={row} />
        ))}
      </ul>
    </section>
  )
}
