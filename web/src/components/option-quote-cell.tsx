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

export function quoteText(quote: Quote): string {
  const parts: string[] = [quote.as_of]
  if (quote.data_type) parts.push(quote.data_type)
  parts.push(SOURCE_WORD[quote.source])
  return parts.join(' · ')
}

export function OptionQuoteCell({ quote }: { quote: Quote }) {
  return (
    <span className="text-xs text-muted-foreground" data-quote>
      <span data-quote-time>{quote.as_of}</span>{' '}
      {quote.data_type ? (
        <span data-quote-type className={cn(quote.data_type !== 'live' && 'text-destructive')}>
          {quote.data_type}
        </span>
      ) : null}{' '}
      <span data-quote-source>{SOURCE_WORD[quote.source]}</span>
    </span>
  )
}
