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

const STALE_AFTER_SECONDS = 15 * 60

function secondsOfDay(hours: string, minutes: string, seconds: string): number {
  return Number(hours) * 3600 + Number(minutes) * 60 + Number(seconds)
}

export type QuoteMarket = 'US' | 'JP'

// The Nth Sunday (1-based) of a month, as a day of the month (UTC calendar).
function nthSunday(year: number, month: number, nth: number): number {
  const first = new Date(Date.UTC(year, month, 1)).getUTCDay()
  return 1 + ((7 - first) % 7) + 7 * (nth - 1)
}

// Minutes the exchange wall clock is ahead of UTC: Tokyo is fixed; New York is -5 h, or -4 h from the second Sunday of
// March 07:00 UTC to the first Sunday of November 06:00 UTC (US daylight saving time).
function exchangeOffsetMinutes(market: QuoteMarket, now: Date): number {
  if (market === 'JP') return 9 * 60
  const year = now.getUTCFullYear()
  const start = Date.UTC(year, 2, nthSunday(year, 2, 2), 7)
  const end = Date.UTC(year, 10, nthSunday(year, 10, 1), 6)
  return now.getTime() >= start && now.getTime() < end ? -4 * 60 : -5 * 60
}

// Decision 18: a quote older than 15 minutes is grey. `as_of` is the exchange-local HH:MM:SS of the day, so the clock
// is read as the exchange's wall time; a date text (a close archive value) or an empty time is not a time and is never
// called stale here, and a page that does not know the market (the stock page) judges nothing rather than guess a zone. A clock before the quote (never older) is not stale either.
export function isStaleQuote(asOf: string, now: Date, market: QuoteMarket | undefined): boolean {
  if (!market) return false
  const quoted = /^(\d{2}):(\d{2}):(\d{2})$/.exec(asOf)
  if (!quoted) return false
  const wall = new Date(now.getTime() + exchangeOffsetMinutes(market, now) * 60_000)
  const clock = secondsOfDay(String(wall.getUTCHours()), String(wall.getUTCMinutes()), String(wall.getUTCSeconds()))
  return clock - secondsOfDay(quoted[1] ?? '0', quoted[2] ?? '0', quoted[3] ?? '0') > STALE_AFTER_SECONDS
}

// `now` is the clock, injected so a test fixes it; by default the device clock at render time (no timer).
export function OptionQuoteCell({
  quote,
  now = new Date(),
  market,
}: {
  quote: Quote
  now?: Date
  market?: QuoteMarket
}) {
  const stale = isStaleQuote(quote.as_of, now, market)
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
  now,
  market,
}: {
  items: { id: string; label?: string; quote: Quote }[]
  now?: Date
  market?: QuoteMarket
}) {
  if (items.length === 0) return null
  return (
    <ul className="space-y-0.5" data-quotes>
      {items.map(({ id, label, quote }) => (
        <li key={id} data-quote-row={id}>
          {label ? <span className="mr-2 text-xs">{label}</span> : null}
          <OptionQuoteCell quote={quote} now={now} market={market} />
        </li>
      ))}
    </ul>
  )
}
