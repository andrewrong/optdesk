// Board 8: all figures and prose are supplied by Python; tables only sort on demand.
import { ChevronRight, Info } from 'lucide-react'
import { EaseGauge } from '@/components/ease-gauge/EaseGauge'
import {
  DataTable,
  SOURCE_STATUS_CLASS,
  TONE_CLASS,
  type Column,
} from '@/components/data-table/data-table'
import { MissingValue } from '@/components/data-table/missing-value'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
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
import { validate, type MacroPageV2, type Payload } from '@/types/macro_page'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { Hinted, LeadBlock } from './LeadBlock'
import { LiquidityBlock } from './LiquidityBlock'
export { validate as validateMacroPage } from '@/types/macro_page'
export type { Payload as MacroPagePayload } from '@/types/macro_page'

const EVENTS: Column[] = [
  { id: 'date', label: 'Date' },
  { id: 'name', label: 'Event' },
  { id: 'previous', label: 'Previous' },
  { id: 'actual', label: 'Actual' },
  { id: 'days_to', label: 'Days to' },
  { id: 'window', label: 'Window' },
]
const INDICATOR_HEADS = ['Indicator · timeliness', 'Latest', 'Change', 'Data day', 'Source', 'Detail']
// Group headings; the payload sends the key only (docs/design/macro-page-v2.md Decision 5).
const GROUP_TITLE = {
  rates: 'Rates and curve',
  credit: 'Credit and conditions',
  dollar_japan: 'Dollar and Japan',
} as const

type Indicator = MacroPageV2['indicators'][number]
type EventRow = MacroPageV2['events'][number]

function IndicatorRow({ indicator }: { indicator: Indicator }) {
  const latest = indicator.latest
  const gap = 'reason' in latest
  const tone = 'tone' in latest ? latest.tone : undefined
  return (
    <TableRow data-indicator>
      <TableCell className="py-1.5">
        <Hinted hint={indicator.hint}>{indicator.name}</Hinted>{' '}
        <Badge variant="secondary" className={SOURCE_STATUS_CLASS[indicator.status]}>
          {indicator.label}
        </Badge>
      </TableCell>
      <TableCell className="py-1.5">
        {gap ? (
          <>
            <MissingValue text={latest.text} />{' '}
            <span className="text-muted-foreground text-xs" data-missing-reason>
              {latest.text}
            </span>
          </>
        ) : tone && tone !== 'none' ? (
          <span className={TONE_CLASS[tone]}>{latest.text}</span>
        ) : (
          latest.text
        )}
      </TableCell>
      <TableCell className="py-1.5" data-change>
        {indicator.change_text}
      </TableCell>
      <TableCell className="py-1.5" data-data-day>
        {indicator.data_day}
      </TableCell>
      <TableCell className="py-1.5">{indicator.source_text}</TableCell>
      <TableCell className="py-1.5">
        {indicator.detail_href && (
          <Button asChild variant="link" size="sm">
            <a href={indicator.detail_href}>Detail</a>
          </Button>
        )}
      </TableCell>
    </TableRow>
  )
}

// The rows in payload order, cut into runs by `group`; a payload without groups is one run.
function groupRows(indicators: Indicator[]) {
  const groups: { key: string; rows: Indicator[] }[] = []
  for (const indicator of indicators) {
    const key = indicator.group ?? ''
    const found = groups.find((g) => g.key === key)
    if (found) found.rows.push(indicator)
    else groups.push({ key, rows: [indicator] })
  }
  return groups
}

export function MacroPage({ payload }: { payload: Payload }) {
  const { ease } = payload
  const events = (payload.events as EventRow[]).map((event) => ({
    date: { text: event.when_text, value: event.date },
    name: { text: event.name_text, value: event.name_text },
    previous: typeof event.previous === 'string' ? { text: event.previous } : event.previous,
    ...(event.actual !== undefined && { actual: { text: event.actual.text } }),
    ...(event.days_to !== undefined && { days_to: { text: String(event.days_to) } }),
    ...(event.in_window === true && { window: { text: 'in window', badge: true } }),
  }))
  return (
    <div className="min-w-0 space-y-6" data-page="macro-page">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      {'lead' in payload && payload.lead && <LeadBlock lead={payload.lead} />}
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
          {groupRows(payload.indicators as Indicator[]).map((group) => (
            <div key={group.key} data-indicator-group={group.key || undefined}>
              {group.key in GROUP_TITLE && (
                <h3 className="mt-3 text-sm font-medium">
                  {GROUP_TITLE[group.key as keyof typeof GROUP_TITLE]}
                </h3>
              )}
              <Table>
                <TableHeader>
                  <TableRow>
                    {INDICATOR_HEADS.map((head) => (
                      <TableHead key={head}>{head}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.rows.map((indicator, i) => (
                    <IndicatorRow key={i} indicator={indicator} />
                  ))}
                </TableBody>
              </Table>
            </div>
          ))}
        </CardContent>
      </Card>
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
      <Card data-notes>
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
