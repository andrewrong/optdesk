// The Macro card of the home page (ADR 0034 step 3b, board ab1). Every visible string is a `text` field of the
// payload; this file formats and computes nothing. `value` is used only by the table's user-started sort.
import { EaseGauge } from '@/components/ease-gauge/EaseGauge'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/macro_summary'
export { validate as validateMacro } from '@/types/macro_summary'
export type { Payload as MacroPayload } from '@/types/macro_summary'
import { StaleAlert } from './StaleAlert'
import { CardNotes } from './CardNotes'
import { DataTimesFold } from './DataTimesFold'
import { TitleLink } from './OverviewCard'

const COLUMNS: Column[] = [
  { id: 'date', label: 'Date' },
  { id: 'name', label: 'Event' },
  { id: 'previous', label: 'Previous', align: 'right' },
]

export function MacroCard({
  payload,
  showGaps = true,
  titleHref,
}: {
  payload: Payload
  showGaps?: boolean
  titleHref?: string
}) {
  const { card } = payload
  const { ease } = card
  const rows = card.events.map((event) => ({
    date: { text: event.when_text, value: event.date } as Cell,
    name: { text: event.name_text, value: event.name_text } as Cell,
    previous: (typeof event.previous === 'string'
      ? { text: event.previous }
      : event.previous) as Cell,
  }))
  return (
    <Card data-card="macro-summary">
      <CardHeader>
        <CardTitle>
          <TitleLink href={titleHref}>{card.title}</TitleLink>
        </CardTitle>
        <CardDescription>{card.say}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <StaleAlert stale={card.stale} generatedAt={payload.generated_at} />
        <Accordion type="single" collapsible data-ease-fold>
          <AccordionItem value="ease">
            <AccordionTrigger>Ease score</AccordionTrigger>
            <AccordionContent>
              {'reason' in ease ? (
                <Badge
                  variant="secondary"
                  className="h-auto whitespace-normal text-left"
                  data-ease-gap
                >
                  {ease.text}
                </Badge>
              ) : (
                <div className="space-y-1" data-ease>
                  <div className="text-2xl font-semibold" data-ease-figure>
                    {ease.figure.text}
                  </div>
                  <EaseGauge ease={ease} />
                  <div className="text-xs text-muted-foreground" data-neutral-band>
                    Neutral band {ease.neutral_band.low.text} – {ease.neutral_band.high.text}
                  </div>
                  <p>{ease.reading_text}</p>
                  <div className="text-xs text-muted-foreground" data-week-change>
                    vs last week {ease.week_change.text}
                  </div>
                </div>
              )}
            </AccordionContent>
          </AccordionItem>
        </Accordion>
        <Separator />
        <div data-events>
          <DataTable columns={COLUMNS} rows={rows} />
        </div>
        <CardNotes notes={card.notes} gaps={card.gaps} showGaps={showGaps} />
        <DataTimesFold text={card.data_times_text} />
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'macro_summary',
  page: page<Payload>(validate, MacroCard),
}
