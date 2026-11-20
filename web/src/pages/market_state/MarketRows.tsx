// The five market rows: a row opens its chart below it. Every text, day and point is the payload's; nothing is parsed.
import { useState } from 'react'
import { Chart } from '@/charts/Chart'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import type { MarketRow as RowV2 } from '@/generated/market_state_page.v2'
import type { Payload } from '@/types/market_state_page'

// The v1 payload has no range or hint; the v2 payload may send both.
type Row = Payload['market']['rows'][number] & Pick<RowV2, 'range' | 'hint'>
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

function HintMark({ hint }: { hint: string }) {
  const [open, setOpen] = useState(false)
  return (
    <TooltipProvider>
      <Tooltip open={open} onOpenChange={setOpen}>
        <TooltipTrigger asChild>
          <span
            role="button"
            tabIndex={0}
            data-hint-mark
            className="ml-1 cursor-help text-muted-foreground"
            onClick={(event) => {
              event.stopPropagation()
              setOpen(!open)
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                event.stopPropagation()
                setOpen(!open)
              }
            }}
          >
            ⓘ
          </span>
        </TooltipTrigger>
        <TooltipContent>{hint}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function RowText({ row }: { row: Row }) {
  const { figure, percentile } = row
  return (
    <span className="grid gap-1 text-left">
      <span className="font-medium">
        {row.label}
        {row.hint ? <HintMark hint={row.hint} /> : null}
      </span>
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
