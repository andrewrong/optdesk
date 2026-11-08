// Board 16, Diagnosis over ten years (docs/design/missing-pages.md#Decisions). Every text is the producer's: the view
// ranks nothing, sums nothing and formats nothing. A Gap row shows its cause and has no year table.
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { DataTable, type Column } from '@/components/data-table/data-table'
import type { DiagGroup, DiagRow, Gap } from '@/generated/diagnosis_page.v1'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/diagnosis_page'

const isGap = (row: DiagRow | Gap): row is Gap => 'kind' in row

// The year table takes its four headers from the group's `columns`, as sent.
function YearTable({ columns, row }: { columns: string[]; row: DiagRow }) {
  const ids = ['fy', 'revenue', 'margin', 'extra'] as const
  const cols: Column[] = ids.map((id, index) => ({
    id,
    label: columns[index] ?? '',
    align: index === 0 ? 'left' : 'right',
  }))
  const rows = row.years.map((year) => ({
    fy: { text: year.fy },
    revenue: { text: year.revenue },
    margin: { text: year.margin },
    extra: { text: year.extra },
  }))
  return (
    <>
      <DataTable columns={cols} rows={rows} />
      <ul className="mt-2 space-y-1" aria-hidden>
        {row.years.map((year) => (
          <li key={year.fy} className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="w-16">{year.fy}</span>
            <div className="h-2 flex-1 rounded bg-muted">
              <div
                data-mini-bar
                className="h-2 rounded bg-primary"
                style={{ width: `${year.bar_percent}%` }}
              />
            </div>
          </li>
        ))}
      </ul>
    </>
  )
}

function CompanyFold({ group, row }: { group: DiagGroup; row: DiagRow | Gap }) {
  if (isGap(row)) {
    return (
      <div data-gap-row className="py-2">
        <Badge variant="secondary" className="h-auto whitespace-normal text-left">
          {row.text}
        </Badge>
      </div>
    )
  }
  return (
    <AccordionItem value={row.symbol}>
      <AccordionTrigger>
        <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
          <span className="font-medium">{row.symbol}</span>
          <span>{row.latest_fy}</span>
          <span>{row.latest}</span>
          <Badge variant="secondary">{row.rank_text}</Badge>
          <span>{row.extra}</span>
          <span className="text-muted-foreground">{row.usual}</span>
        </span>
      </AccordionTrigger>
      <AccordionContent>
        <YearTable columns={group.columns} row={row} />
      </AccordionContent>
    </AccordionItem>
  )
}

export function DiagnosisPage({ payload }: { payload: Payload }) {
  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">{payload.title}</h1>
      <p>{payload.sentence}</p>
      <p className="text-sm text-muted-foreground">{payload.basis_note}</p>
      {payload.stale ? (
        <StaleAlert
          stale={{ text: payload.stale.text, stale_after: payload.stale_after ?? '' }}
          generatedAt={payload.generated_at}
        />
      ) : null}
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
      {payload.groups.map((group) => (
        <Card key={group.key}>
          <CardHeader>
            <CardTitle>{group.title}</CardTitle>
          </CardHeader>
          <CardContent>
            <Accordion type="multiple">
              {group.rows.map((row, index) => (
                <CompanyFold key={isGap(row) ? `gap-${index}` : row.symbol} group={group} row={row} />
              ))}
            </Accordion>
            {group.legend.map((line, index) => (
              <p key={`${index}-${line}`} className="text-xs text-muted-foreground">
                {line}
              </p>
            ))}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

export const registration = { schema: 'diagnosis_page', page: page<Payload>(validate, DiagnosisPage) }
