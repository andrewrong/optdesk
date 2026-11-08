// The 持仓公司 card of the home page (ADR 0034 step 3d, board ab1). Every visible string is a `text` field of the
// payload; this file formats and computes nothing. `value` is used only by the table's user-started sort and the
// colour comes from the payload's `tone` (resolved inside the shared Data Table).
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { validate, type Payload } from '@/types/held_companies_summary'
import { page } from '@/shell/page'
import { CardNotes } from './CardNotes'
import { DataTimesFold } from './DataTimesFold'
import { StaleNotice } from './StaleNotice'
import { TitleLink } from './OverviewCard'

// The first column is left-aligned, the figure columns right-aligned.
const ALIGN: Record<string, Column['align']> = {
  price: 'right',
  earnings: 'right',
  day_change: 'right',
  five_day_change: 'right',
}

type Row = Payload['card']['rows'][number]
type Mark = Row['day_sharp']

// A mark from the payload: the Data Table shows a Badge for a `sharp` mark and nothing for `none`.
const sharpOf = (mark: Mark) => (mark.kind === 'sharp' ? { text: mark.text } : null)

export function HeldCompaniesCard({
  payload,
  showGaps = true,
  titleHref,
}: {
  payload: Payload
  showGaps?: boolean
  titleHref?: string
}) {
  const { card } = payload
  const columns: Column[] = card.columns.map(({ id, label }) => ({ id, label, align: ALIGN[id] }))
  const rows = card.rows.map((row) => ({
    company: { text: row.symbol, value: row.symbol } as Cell,
    price: row.price as Cell,
    earnings: ('reason' in row.earnings
      ? row.earnings
      : { text: row.earnings.text, value: row.earnings.date }) as Cell,
    day_change: { ...row.day_change, sharp: sharpOf(row.day_sharp) } as Cell,
    five_day_change: { ...row.five_day_change, sharp: sharpOf(row.five_day_sharp) } as Cell,
  }))
  return (
    <Card data-card="held-companies-summary">
      <CardHeader>
        <CardTitle>
          <TitleLink href={titleHref}>{card.title}</TitleLink>
        </CardTitle>
        <CardDescription>{card.say}</CardDescription>
        <CardDescription>{card.subtitle}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <StaleNotice text={card.stale.text} isStale={card.stale.is_stale} />
        <div data-companies>
          <DataTable columns={columns} rows={rows} />
        </div>
        <p className="text-xs text-muted-foreground" data-sharp-note>
          {card.sharp_note}
        </p>
        <CardNotes notes={card.notes} gaps={card.gaps} showGaps={showGaps} />
        <DataTimesFold text={card.data_times_text} />
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'held_companies_summary',
  page: page<Payload>(validate, HeldCompaniesCard),
}
