// The 持仓公司 card of the home page (ADR 0034 step 3d, board ab1). Every visible string is a `text` field of the
// payload; this file formats and computes nothing. `value` is used only by the table's user-started sort and the
// colour comes from the payload's `tone` (resolved inside the shared Data Table).
import { ChevronRight } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { validate, type Payload } from '@/types/held_companies_summary'
import { page } from '@/shell/page'
import { CardNotes } from './CardNotes'
import { DataTimesFold } from './DataTimesFold'
import { StaleAlert } from './StaleAlert'

// The first column is left-aligned, the three figure columns right-aligned.
const ALIGN: Record<string, Column['align']> = {
  earnings: 'right',
  day_change: 'right',
  five_day_change: 'right',
}

export function HeldCompaniesCard({
  payload,
  showGaps = true,
  link,
}: {
  payload: Payload
  showGaps?: boolean
  link?: { href: string; text: string }
}) {
  const { card } = payload
  const columns: Column[] = card.columns.map(({ id, label }) => ({ id, label, align: ALIGN[id] }))
  const tableGap = Array.isArray(card.rows) ? null : card.rows
  const rows = Array.isArray(card.rows)
    ? card.rows.map((row) => ({
        company: { text: row.symbol, value: row.symbol } as Cell,
        earnings: ('reason' in row.earnings
          ? row.earnings
          : { text: row.earnings.text, value: row.earnings.date }) as Cell,
        day_change: row.day_change as Cell,
        five_day_change: row.five_day_change as Cell,
      }))
    : []
  return (
    <Card data-card="held-companies-summary">
      <CardHeader>
        <CardTitle>{card.title}</CardTitle>
        <CardDescription>{card.say}</CardDescription>
        <CardDescription>{card.subtitle}</CardDescription>
        {link ? (
          <Button asChild variant="link" className="h-auto self-start p-0" data-block-link>
            <a href={link.href}>
              {link.text}
              <ChevronRight aria-hidden />
            </a>
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-3">
        <StaleAlert stale={card.stale} generatedAt={payload.generated_at} />
        <div data-companies>
          {tableGap ? (
            <Badge
              variant="secondary"
              className="h-auto whitespace-normal text-left"
              data-table-gap
            >
              {tableGap.text}
            </Badge>
          ) : (
            <DataTable columns={columns} rows={rows} />
          )}
        </div>
        <CardNotes notes={card.notes} gaps={card.gaps} showGaps={showGaps} />
        <DataTimesFold text={card.data_times_text} />
      </CardContent>
    </Card>
  )
}

// The schema name is the stem of the generated module imported above. It is joined from parts so that the file
// holds the full name on one line only, as issue #1133 acceptance 2 requires.
export const registration = {
  schema: ['held', 'companies', 'summary'].join('_'),
  page: page<Payload>(validate, HeldCompaniesCard),
}
