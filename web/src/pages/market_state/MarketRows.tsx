// The five market rows: a row opens its chart below it. Every text, day and point is the payload's; nothing is parsed.
import { useState } from 'react'
import { Chart } from '@/charts/Chart'
import { unitSuffix } from '@/charts/unitSuffix'
import type { Payload } from '@/types/market_state_page'

type Row = Payload['market']['rows'][number]

function RowText({ row }: { row: Row }) {
  const { figure, percentile } = row
  return (
    <span className="grid gap-1 text-left">
      <span className="font-medium">{row.label}</span>
      <span>
        {figure.kind === 'value' ? (
          <>
            {figure.text}
            {unitSuffix(figure.unit) === '' ? null : (
              <span className="ml-1 text-muted-foreground">{unitSuffix(figure.unit)}</span>
            )}
          </>
        ) : (
          figure.text
        )}
      </span>
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
    </span>
  )
}

function MarketRow({ row }: { row: Row }) {
  const [open, setOpen] = useState(false)
  const { history } = row
  if ('kind' in history) {
    return (
      <li className="rounded-md border p-3" data-market-row={row.id}>
        <RowText row={row} />
        <p className="mt-2 text-sm text-muted-foreground" data-history-gap>
          {history.text}
        </p>
      </li>
    )
  }
  return (
    <li className="rounded-md border" data-market-row={row.id}>
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
            ariaText={history.aria_text}
            spec={{
              kind: 'line',
              categories: history.points.map((point) => point.day),
              unit: history.unit,
              series: [
                { name: row.label, points: history.points.map(({ value, text }) => ({ value, text })) },
              ],
            }}
          />
          <p className="mt-2 text-sm text-muted-foreground" data-history-caption>
            {history.caption}
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
