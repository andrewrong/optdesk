// The 宏观 card of the home page (ADR 0034 step 3b, board ab1). Every visible string is a `text` field of the
// payload; this file formats and computes nothing. `value` is used only by the table's user-started sort.
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import type { Payload } from '@/types/macro_summary'
export { validate as validateMacro } from '@/types/macro_summary'
export type { Payload as MacroPayload } from '@/types/macro_summary'
import { StaleAlert } from './StaleAlert'
import { CardNotes } from './CardNotes'
import { DataTimesFold } from './DataTimesFold'

const COLUMNS: Column[] = [
  { id: 'date', label: '日期' },
  { id: 'name', label: '事件' },
  { id: 'previous', label: '前值', align: 'right' },
]

export function MacroCard({ payload, showGaps = true }: { payload: Payload; showGaps?: boolean }) {
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
        <CardTitle>{card.title}</CardTitle>
        <CardDescription>{card.say}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <StaleAlert stale={card.stale} generatedAt={payload.generated_at} />
        {'reason' in ease ? (
          <Badge variant="secondary" className="h-auto whitespace-normal text-left" data-ease-gap>
            {ease.text}
          </Badge>
        ) : (
          <div className="space-y-1" data-ease>
            <div className="text-2xl font-semibold" data-ease-figure>
              {ease.figure.text}
            </div>
            <Progress value={ease.marker_percent.value} aria-label={ease.aria_text} data-marker />
            <div className="flex justify-between text-xs text-muted-foreground" data-ticks>
              {ease.ticks.map((tick, index) => (
                <span key={`${index}-${tick}`}>{tick}</span>
              ))}
            </div>
            <div className="text-xs text-muted-foreground" data-neutral-band>
              中性区间 {ease.neutral_band.low.text} – {ease.neutral_band.high.text}
            </div>
            <p>{ease.reading_text}</p>
            <div className="text-xs text-muted-foreground" data-week-change>
              {ease.week_change.text}
            </div>
          </div>
        )}
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
