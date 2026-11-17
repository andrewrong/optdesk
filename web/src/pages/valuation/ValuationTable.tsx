// The table view: one row per company, the hand-typed buy price beside the buy zone, an entry button on an
// unvalued row. Every cell is a payload text.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Plus } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/valuation_page'
import { FlagBadges } from './CompanyView'

type Row = NonNullable<Payload['rows']>[number]

const text = (key: keyof Row, header: string): ColumnDef<Row> => ({
  id: key,
  header,
  cell: ({ row }) => String(row.original[key]),
})

const COLUMNS: ColumnDef<Row>[] = [
  {
    id: 'name',
    header: 'Name',
    cell: ({ row: { original: r } }) => (
      <div>
        <a className="font-medium underline" href={`/valuation-company?company=${encodeURIComponent(r.company_key)}`}>
          {r.name}
        </a>{' '}
        <span className="text-muted-foreground">{r.company_key}</span>
        {r.not_entered !== '' ? <Badge variant="secondary">{r.not_entered}</Badge> : null}
      </div>
    ),
  },
  text('v', 'V'),
  text('p', 'P'),
  text('version_date', 'Version date'),
  text('r', 'R'),
  text('c', 'C'),
  text('heavy', 'Heavy'),
  text('buy_zone', 'Buy zone'),
  text('buy_price', 'Hand-typed buy price'),
  text('fair_zone', 'Fair zone'),
  text('trim', 'Trim'),
  text('clear', 'Clear'),
  text('core', 'Core'),
  text('close', 'Close'),
  {
    id: 'zone',
    header: 'Zone',
    cell: ({ row: { original: r } }) => (
      <div className="flex flex-wrap gap-1">
        {r.zone !== '' ? <Badge>{r.zone}</Badge> : null}
        <FlagBadges flags={r.flags} />
      </div>
    ),
  },
  text('status', 'Status'),
  {
    id: 'entry',
    header: '',
    cell: ({ row: { original: r } }) =>
      r.valued ? null : (
        <Button asChild size="sm" variant="outline">
          <a href={r.entry_link}>
            <Plus aria-hidden />
            Enter
          </a>
        </Button>
      ),
  },
]

export function ValuationTable({ rows }: { rows: Row[] }) {
  const table = useReactTable({ data: rows, columns: COLUMNS, getCoreRowModel: getCoreRowModel() })
  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id}>
            {group.headers.map((h) => (
              <TableHead key={h.id}>{flexRender(h.column.columnDef.header, h.getContext())}</TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id} data-company={row.original.company_key}>
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
