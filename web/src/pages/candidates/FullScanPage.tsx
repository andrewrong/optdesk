// The full-scan page (issue #1801): every stored candidate, one table per strategy. Every visible string is a `text`,
// `label`, `sub`, `liquidity` or `note` of the payload; this file counts, sorts, compares and words nothing.
import { ChevronLeft, Info } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/full_scan'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'

export { validate as validateFullScan } from '@/types/full_scan'
export type { Payload as FullScanPayload } from '@/types/full_scan'

type Section = Payload['sections'][number]
type PayloadTable = NonNullable<Section['table']>

function PayloadTableView({ table, name }: { table: PayloadTable; name: string }) {
  const columns: Column[] = table.columns.map(({ id, label, align }) => ({ id, label, align }))
  const rows = table.rows.map((row) =>
    Object.fromEntries(row.cells.map(({ column, cell }) => [column, cell as Cell])),
  )
  return (
    <div className="min-w-0" data-table={name}>
      <DataTable columns={columns} rows={rows} />
    </div>
  )
}

function SectionCard({ section }: { section: Section }) {
  return (
    <Card data-section={section.strategy} className="min-w-0">
      <CardHeader>
        <CardTitle>{section.title}</CardTitle>
        {section.sub ? <CardDescription>{section.sub}</CardDescription> : null}
      </CardHeader>
      <CardContent className="space-y-2">
        {section.liquidity ? (
          <p className="text-sm text-muted-foreground" data-liquidity>
            {section.liquidity}
          </p>
        ) : null}
        {section.gap ? (
          <Alert data-section-gap>
            <Info aria-hidden />
            <AlertDescription>{section.gap.text}</AlertDescription>
          </Alert>
        ) : null}
        {section.table ? <PayloadTableView table={section.table} name={section.strategy} /> : null}
        {section.note ? <p className="text-xs text-muted-foreground">{section.note}</p> : null}
      </CardContent>
    </Card>
  )
}

export function FullScanPage({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-4" data-page="full-scan">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <Button asChild variant="link">
          <a href={payload.back.href} data-back>
            <ChevronLeft aria-hidden />
            {payload.back.label}
          </a>
        </Button>
      </div>
      {payload.stale ? (
        <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      ) : null}
      {payload.sections.map((section) => (
        <SectionCard key={section.strategy} section={section} />
      ))}
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
    </div>
  )
}

export const registration = {
  schema: 'full_scan',
  page: page<Payload>(validate, FullScanPage),
}
