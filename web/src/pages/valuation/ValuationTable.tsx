// The table view: one row per company, the hand-typed buy price beside the buy zone, an entry button in the name
// cell of an unvalued row. Every cell is a payload text; the glosses of the heads are static text. A word cell wraps,
// a figure cell stays on one line.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Info, Plus } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import type { Payload } from '@/types/valuation_page'
import { FlagBadges, GLOSS } from './CompanyView'

type Row = NonNullable<Payload['rows']>[number]

// The cells that hold words wrap; the figure cells keep the whitespace-nowrap of the Table cell.
const WRAP_CELL = new Set(['version_date', 'status', 'zone', 'buy_zone'])
const STICKY = 'sticky left-0 bg-background'
const WRAP_HEAD = new Set(['buy_price', 'version_date'])

// The gloss reads "<term>: <text>"; the tooltip shows the text without the leading term.
const Head = ({ label, gloss }: { label: string; gloss?: string }) => {
  const explain = gloss ? gloss.replace(`${label}: `, '') : ''
  return (
    <>
      {label}
      {explain ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <button type="button" aria-label={explain} className="ml-1 inline-flex align-middle text-muted-foreground">
              <Info className="size-3" aria-hidden />
            </button>
          </TooltipTrigger>
          <TooltipContent>{explain}</TooltipContent>
        </Tooltip>
      ) : null}
    </>
  )
}

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
  text('close', 'Close'),
  text('v', 'V', GLOSS['V'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('p', 'P', GLOSS['P'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('version_date', 'Version date', GLOSS['Version date'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('r', 'R', GLOSS['R'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('c', 'C', GLOSS['C'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('heavy', 'Heavy', GLOSS['Heavy'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('buy_zone', 'Buy zone'),
  text('buy_price', 'Hand-typed buy price'),
  text('fair_zone', 'Fair zone'),
  text('trim', 'Trim', GLOSS['Trim'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('clear', 'Clear', GLOSS['Clear'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('core', 'Core', GLOSS['Core'] ?? ''), // gloss source: GLOSS in CompanyView.tsx
  text('status', 'Status'),
]

export function ValuationTable({ rows }: { rows: Row[] }) {
  const table = useReactTable({ data: rows, columns: COLUMNS, getCoreRowModel: getCoreRowModel() })
  return (
    <TooltipProvider>
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((group) => (
            <TableRow key={group.id}>
              {group.headers.map((h) => (
                <TableHead key={h.id} className={cn(WRAP_HEAD.has(h.column.id) && 'whitespace-normal', h.column.id === 'name' && STICKY)}>
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
                <TableCell key={cell.id} className={cn(
                    WRAP_CELL.has(cell.column.id) && 'whitespace-normal break-words',
                    cell.column.id === 'name' && STICKY,
                  )}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TooltipProvider>
  )
}
