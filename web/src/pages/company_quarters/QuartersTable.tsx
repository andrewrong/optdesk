// The quarterly table of one company, a @tanstack/react-table table: one row per `quarters[]` as given (newest first, no
// sort), one column per `columns[]`. A missing cell prints its `why_text` and no number; every text is the payload's.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { GlossHeader } from '@/components/gloss-header'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/company_quarters_page'

type Quarter = Payload['quarters'][number]
type Cell = Quarter['cells'][string]

function FigureCell({ cell }: { cell: Cell | undefined }) {
  if (cell === undefined) return null
  if (cell.missing) return <span data-missing className="text-muted-foreground">{cell.why_text}</span>
  return (
    <span className="inline-flex flex-wrap items-center gap-1" title={cell.concept}>
      <span className="tabular-nums">{cell.text}</span>
      {cell.derived ? <Badge variant="secondary">derived</Badge> : null}
      {cell.restated ? <Badge variant="warning">restated</Badge> : null}
    </span>
  )
}

export function QuartersTable({ company, columns: spec, quarters }: Pick<Payload, 'columns' | 'quarters'> & { company: NonNullable<Payload['company']> }) {
  const columns: ColumnDef<Quarter>[] = [
    {
      id: 'quarter',
      header: () => (
        <div>
          <div>Quarter</div>
          {company.fiscal_year_end !== '' ? <div data-fiscal-year-end className="text-xs font-normal">{company.fiscal_year_end}</div> : null}
        </div>
      ),
      cell: ({ row }) => (
        <div>
          <div>{row.original.label}</div>
          <div className="text-xs text-muted-foreground">{row.original.period_end}</div>
        </div>
      ),
    },
    ...spec.map(
      (c): ColumnDef<Quarter> => ({
        id: c.key,
        header: () => <GlossHeader id={c.key} label={c.label} gloss={c.gloss} />,
        cell: ({ row }) =>
          // The builder sends the filing date as the column `filing_date`; it has no cell, the date is on the quarter.
          c.key === 'filing_date' ? row.original.filing_date : <FigureCell cell={row.original.cells[c.key]} />,
      }),
    ),
  ]
  const table = useReactTable({ data: quarters, columns, getCoreRowModel: getCoreRowModel() })
  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id}>
            {group.headers.map((h) => (
              <TableHead key={h.id} className="align-top whitespace-normal">
                {flexRender(h.column.columnDef.header, h.getContext())}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id} data-period-end={row.original.period_end}>
            {row.getVisibleCells().map((c) => (
              <TableCell key={c.id} className="align-top whitespace-normal">
                {flexRender(c.column.columnDef.cell, c.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
