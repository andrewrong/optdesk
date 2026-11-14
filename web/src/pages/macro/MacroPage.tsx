// Board 8: all figures and prose are supplied by Python; tables only sort on demand.
import { ChevronRight, Info } from 'lucide-react'
import { EaseGauge } from '@/components/ease-gauge/EaseGauge'
import { DataTable, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Separator } from '@/components/ui/separator'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/macro_page'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { LiquidityBlock } from './LiquidityBlock'
export { validate as validateMacroPage } from '@/types/macro_page'
export type { Payload as MacroPagePayload } from '@/types/macro_page'

const EVENTS: Column[] = [
  { id: 'date', label: 'Date' },
  { id: 'name', label: 'Event' },
  { id: 'previous', label: 'Previous' },
]
const INDICATORS: Column[] = [
  { id: 'name', label: 'Indicator · timeliness' },
  { id: 'latest', label: 'Latest' },
  { id: 'source', label: 'Source' },
  { id: 'detail', label: 'Detail' },
]

export function MacroPage({ payload }: { payload: Payload }) {
  const { ease } = payload
  const events = payload.events.map((event) => ({
    date: { text: event.when_text, value: event.date },
    name: { text: event.name_text, value: event.name_text },
    previous: typeof event.previous === 'string' ? { text: event.previous } : event.previous,
  }))
  const indicators = payload.indicators.map((indicator) => ({
    name: {
      text: indicator.name,
      value: indicator.name,
      source: indicator.label,
      source_status: indicator.status,
    },
    latest:
      'reason' in indicator.latest
        ? indicator.latest
        : { ...indicator.latest, tone: indicator.latest.tone ?? undefined },
    source: { text: indicator.source_text },
    ...(indicator.detail_href && { detail: { text: 'Detail', href: indicator.detail_href } }),
  }))
  return (
    <div className="min-w-0 space-y-6" data-page="macro-page">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      <Card>
        <CardContent>
          <Accordion type="single" collapsible data-ease-fold>
            <AccordionItem value="ease">
              <AccordionTrigger>{payload.ease_title}</AccordionTrigger>
              <AccordionContent className="space-y-3">
                {'reason' in ease ? (
                  <Alert data-ease-gap>
                    <Info aria-hidden />
                    <AlertDescription>{ease.text}</AlertDescription>
                  </Alert>
                ) : (
                  <>
                    <div className="text-2xl font-semibold" data-ease-figure>
                      {ease.figure.text}
                    </div>
                    <EaseGauge ease={ease} />
                    <p className="text-xs text-muted-foreground" data-neutral-band>
                      {ease.neutral_band.low.text} – {ease.neutral_band.high.text}
                    </p>
                    <Separator />
                    <p>{ease.reading_text}</p>
                  </>
                )}
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </CardContent>
      </Card>
      {'liquidity' in payload && (
        <LiquidityBlock title={payload.liquidity_title} liquidity={payload.liquidity} />
      )}
      <Card className="min-w-0" data-events>
        <CardHeader>
          <CardTitle>{payload.events_title}</CardTitle>
        </CardHeader>
        <CardContent className="min-w-0">
          <DataTable columns={EVENTS} rows={events} />
        </CardContent>
      </Card>
      <Card className="min-w-0" data-indicators>
        <CardHeader>
          <CardTitle>{payload.indicators_title}</CardTitle>
        </CardHeader>
        <CardContent className="min-w-0">
          <DataTable columns={INDICATORS} rows={indicators} />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardDescription className="flex items-center gap-2">
            <ChevronRight aria-hidden />
            {payload.notes_title}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          <CardNotes notes={payload.notes} gaps={payload.gaps} />
        </CardContent>
      </Card>
    </div>
  )
}

export const registration = {
  schema: 'macro_page',
  page: page<Payload>(validate, MacroPage),
}
