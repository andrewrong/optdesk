// The core page (core_page v1, board 15): the sentence, the dot bar scale with the treasury line and one fold per
// company that opens to its returns. Every text and every `marker_percent` is read from the payload; the view
// compares, sums and formats nothing, and gives no decision.
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/core_page'
import { CardNotes } from '../home/CardNotes'
import { StaleAlert } from '../home/StaleAlert'

type Row = Payload['rows'][number]

// The bar: a shadcn Progress track, the dots at the producer's positions and the treasury line as a second marker.
function DotBar({ dots, treasury }: { dots: readonly number[]; treasury: number }) {
  return (
    <div className="relative py-1" data-bar>
      <Progress value={0} />
      <span
        data-treasury
        aria-hidden
        className="absolute top-0 h-3 w-0.5 bg-foreground"
        style={{ left: `${treasury}%` }}
      />
      {dots.map((percent, index) => (
        <span
          key={index}
          data-dot
          aria-hidden
          className="absolute top-0.5 size-2 -translate-x-1 rounded-full bg-primary"
          style={{ left: `${percent}%` }}
        />
      ))}
    </div>
  )
}

function RowMarks({ row }: { row: Row }) {
  return (
    <>
      {row.flag ? <Badge variant="warn">{row.flag}</Badge> : null}
      {row.cyclical ? <Badge variant="secondary">Cyclical</Badge> : null}
    </>
  )
}

function CoreRowItem({ row, treasury }: { row: Row; treasury: number }) {
  const { returns } = row
  if ('kind' in returns) {
    return (
      <div data-row={row.symbol} data-gap className="flex flex-wrap items-center gap-2 border-b py-3">
        <span className="font-medium">{row.symbol}</span>
        <span>{row.price_text}</span>
        <RowMarks row={row} />
        <span className="text-sm text-muted-foreground">{returns.text}</span>
      </div>
    )
  }
  const [first] = returns
  return (
    <AccordionItem value={row.symbol} data-row={row.symbol}>
      <AccordionTrigger>
        <span className="flex flex-wrap items-center gap-2">
          <span className="font-medium">{row.symbol}</span>
          <span>{row.price_text}</span>
          <span>{first.text}</span>
          <RowMarks row={row} />
        </span>
      </AccordionTrigger>
      <AccordionContent>
        <ul className="space-y-3">
          {returns.map((point, index) => (
            <li key={`${index}-${point.assumption}`} data-return>
              <div className="flex justify-between text-sm">
                <span>{point.assumption}</span>
                <span>{point.text}</span>
              </div>
              <DotBar dots={[point.marker_percent]} treasury={treasury} />
            </li>
          ))}
        </ul>
      </AccordionContent>
    </AccordionItem>
  )
}

export function CorePage({ payload }: { payload: Payload }) {
  const { scale } = payload
  return (
    <main className="mx-auto max-w-3xl space-y-4 p-4">
      <Card>
        <CardHeader>
          <CardTitle>{payload.title}</CardTitle>
          <CardDescription>{payload.sentence}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm">{payload.basis_note}</p>
          {payload.stale && payload.stale_after ? (
            <StaleAlert
              stale={{ text: payload.stale.text, stale_after: payload.stale_after }}
              generatedAt={payload.generated_at}
            />
          ) : payload.stale ? (
            <Alert data-stale={payload.stale.is_stale}>
              <AlertDescription>{payload.stale.text}</AlertDescription>
            </Alert>
          ) : null}
          <CardNotes notes={payload.notes} gaps={payload.gaps} />
        </CardContent>
      </Card>
      <Card data-scale>
        <CardContent className="space-y-1">
          <DotBar dots={[]} treasury={scale.treasury.marker_percent} />
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>{scale.low_text}</span>
            <span data-treasury-text>{scale.treasury.text}</span>
            <span>{scale.high_text}</span>
          </div>
        </CardContent>
      </Card>
      <Accordion type="multiple">
        {payload.rows.map((row, index) => (
          <CoreRowItem key={`${index}-${row.symbol}`} row={row} treasury={scale.treasury.marker_percent} />
        ))}
      </Accordion>
      <p className="text-sm">{payload.jp_note}</p>
      <ul className="space-y-1 text-xs text-muted-foreground">
        {payload.legend.map((line, index) => (
          <li key={`${index}-${line}`}>{line}</li>
        ))}
      </ul>
      <p className="text-xs text-muted-foreground">{payload.snapshot_text}</p>
    </main>
  )
}

export const registration = { schema: 'core_page', page: page<Payload>(validate, CorePage) }
