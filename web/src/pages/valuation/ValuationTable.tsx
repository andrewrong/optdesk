// The table view: one row per company, the hand-typed buy price beside the buy zone, an entry button in the name
// cell of an unvalued row. Every cell is a payload text; the glosses of the heads are static text. A word cell wraps,
// a figure cell stays on one line.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Plus } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/valuation_page'
import { FlagBadges } from './CompanyView'

type Row = NonNullable<Payload['rows']>[number]

// The cells that hold words wrap; the figure cells keep the whitespace-nowrap of the Table cell.
const WRAP_CELL = new Set(['version_date', 'status', 'zone'])
const WRAP_HEAD = new Set(['buy_price', 'version_date'])

const Head = ({ label, gloss }: { label: string; gloss?: string }) => (
  <>
    {label}
    {gloss ? <span className="block max-w-40 text-xs font-normal whitespace-normal text-muted-foreground">{gloss}</span> : null}
  </>
)

const text = (key: keyof Row, header: string, gloss?: string): ColumnDef<Row> => ({
  id: key,
  header: () => <Head label={header} gloss={gloss} />,
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
        {r.valued ? null : (
          <Button asChild size="sm" variant="outline" className="ml-2">
            <a href={r.entry_link}>
              <Plus aria-hidden />
              Enter
            </a>
          </Button>
        )}
      </div>
    ),
  },
  // V: valuation-structure.md line 9
  text('v', 'V', "V: the owner's valuation of one share"),
  // P: lines 9 and 99
  text('p', 'P', 'P: predictability, 1 to 5'),
  // Version date: the date this valuation is valid from
  text('version_date', 'Version date', 'Version date: the date this valuation is valid from'),
  // R: line 75
  text('r', 'R', 'R: the multiple of the valuation that sets the fair price (C = V x R)'),
  // C: line 75
  text('c', 'C', 'C: fair price, V x R'),
  // Heavy: line 99
  text('heavy', 'Heavy', 'Heavy: the heavy point, C x (1 - H)'),
  text('buy_zone', 'Buy zone'),
  text('buy_price', 'Hand-typed buy price'),
  text('fair_zone', 'Fair zone'),
  // Trim, Clear, Core: line 78
  text('trim', 'Trim', 'Trim: C x (1 + S)'),
  text('clear', 'Clear', 'Clear: V x 1.2'),
  text('core', 'Core', 'Core: V x 1.5'),
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
]

export function ValuationTable({ rows }: { rows: Row[] }) {
  const table = useReactTable({ data: rows, columns: COLUMNS, getCoreRowModel: getCoreRowModel() })
  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id}>
            {group.headers.map((h) => (
              <TableHead key={h.id} className={WRAP_HEAD.has(h.column.id) ? 'whitespace-normal' : undefined}>
                {flexRender(h.column.columnDef.header, h.getContext())}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id} data-company={row.original.company_key}>
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id} className={WRAP_CELL.has(cell.column.id) ? 'whitespace-normal' : undefined}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
