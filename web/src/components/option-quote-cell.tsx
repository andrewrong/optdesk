// The quote cell of a contract's prices (docs/design/option-open-pull.md Decision 18): the time, the data type as a
// word and the source, copied from the row's `quote`. Nothing is computed here; the two words below only name the
// payload's codes, and a data type other than live is shown in red words.
import type { Quote } from '@/generated/option_page.v2'
import { cn } from '@/lib/utils'

const SOURCE_WORD = {
  open_pull: 'open pull',
  refresh: 'refresh',
  close_archive: 'close archive',
} as const

export function OptionQuoteCell({ quote }: { quote: Quote }) {
  const stale = quote.stale
  return (
    <span
      className={cn('text-xs text-muted-foreground', stale && 'opacity-50')}
      data-quote
      {...(stale ? { 'data-quote-stale': '' } : {})}
    >
      <span data-quote-time>{quote.as_of}</span>{' '}
      {quote.data_type ? (
        <span data-quote-type className={cn(quote.data_type !== 'live' && 'text-destructive')}>
          {quote.data_type}
        </span>
      ) : null}{' '}
      <span data-quote-source>{SOURCE_WORD[quote.source]}</span>
      {quote.skew ? <span data-quote-skew> · skew</span> : null}
      {quote.outside_band ? <span data-quote-outside-band> · outside the planned band</span> : null}
    </span>
  )
}

// One quote cell per row of a table, in the order of the rows; `label` is the row's own text from the payload.
export function OptionQuoteList({
  items,
}: {
  items: { id: string; label?: string; quote: Quote }[]
}) {
  if (items.length === 0) return null
  return (
    <ul className="space-y-0.5" data-quotes>
      {items.map(({ id, label, quote }) => (
        <li key={id} data-quote-row={id}>
          {label ? <span className="mr-2 text-xs">{label}</span> : null}
          <OptionQuoteCell quote={quote} />
        </li>
      ))}
    </ul>
  )
}
