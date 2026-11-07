// The option data page: every figure and sentence is the text the builder wrote; this file computes nothing.
import { Info, TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { validate, type Payload, type Result } from '@/types/option_data_page'
import { ChainTable } from './ChainTable'
import { QuoteChart } from './QuoteChart'
import { UnderlyingLinks } from './UnderlyingLinks'

// The shared validator checks the version only; the page also refuses an `underlyings` entry whose `href` is not a string.
export function validateOptionDataPage(input: unknown): Result {
  const checked = validate(input)
  if (!checked.ok) return checked
  const list: unknown = checked.payload.underlyings
  if (list === undefined) return checked
  const sound =
    Array.isArray(list) &&
    list.every(
      (entry: unknown) =>
        typeof entry === 'object' &&
        entry !== null &&
        'href' in entry &&
        typeof entry.href === 'string',
    )
  if (sound) return checked
  return {
    ok: false,
    gap: {
      reason: 'invalid_payload',
      text: 'The payload does not match its schema. Do not act on this card.',
    },
    errors: [],
  }
}
export type { Payload as OptionDataPagePayload } from '@/types/option_data_page'

export function OptionDataPage({ payload }: { payload: Payload }) {
  const { underlying, chain, stale } = payload
  const hasRows = chain.rows[0] !== undefined
  const hasSeries = payload.quote_series[0] !== undefined
  return (
    <div className="min-w-0 space-y-6" data-page="option-data-page">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
      </header>
      {stale ? (
        <Alert variant={stale.is_stale ? 'destructive' : 'default'} data-stale={stale.is_stale}>
          <TriangleAlert aria-hidden />
          <AlertDescription>{stale.text}</AlertDescription>
        </Alert>
      ) : null}
      {payload.gaps.map((gap, index) => (
        <Alert key={`${index}-${gap.text}`} data-gap={gap.reason}>
          <Info aria-hidden />
          <AlertDescription>{gap.text}</AlertDescription>
        </Alert>
      ))}
      <Card data-underlying>
        <CardHeader>
          <CardTitle>{underlying.symbol ?? underlying.label}</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-4">
          <span data-underlying-price>{underlying.price.text}</span>
          <span className="text-muted-foreground">{underlying.session_day}</span>
        </CardContent>
      </Card>
      {payload.underlyings ? <UnderlyingLinks underlyings={payload.underlyings} /> : null}
      <Card>
        <CardHeader>
          <CardTitle>{payload.chain_title}</CardTitle>
        </CardHeader>
        <CardContent>
          {hasRows ? (
            <ChainTable rows={chain.rows} />
          ) : (
            <p data-chain-empty className="text-muted-foreground">
              {payload.sentence}
            </p>
          )}
        </CardContent>
      </Card>
      {hasSeries ? (
        <Card>
          <CardHeader>
            <CardTitle>{payload.series_title}</CardTitle>
          </CardHeader>
          <CardContent>
            <QuoteChart series={payload.quote_series} />
          </CardContent>
        </Card>
      ) : null}
      <section>
        <h2 className="text-sm font-medium">{payload.notes_title}</h2>
        {payload.notes.map((note, index) => (
          <p key={`${index}-${note}`} className="text-xs text-muted-foreground">
            {note}
          </p>
        ))}
      </section>
    </div>
  )
}
