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
const Head = ({ label, term, gloss }: { label: string; term: string; gloss?: string }) => {
  const explain = gloss ? gloss.replace(`${term}: `, '') : ''
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

// `term` is the English key into GLOSS; `header` is the Chinese head.
const text = (key: keyof Row, header: string, term?: string): ColumnDef<Row> => ({
  id: key,
  header: () => <Head label={header} term={term} gloss={term ? (GLOSS[term] ?? '') : ''} />,
  cell: ({ row }) => String(row.original[key]),
})

const COLUMNS: ColumnDef<Row>[] = [
  {
    id: 'name',
    header: '名称',
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
              录入
            </a>
          </Button>
        )}
      </div>
    ),
  },
  {
    id: 'zone',
    header: '区间',
    cell: ({ row: { original: r } }) => (
      <div className="flex flex-wrap gap-1">
        {r.zone !== '' ? <Badge>{r.zone}</Badge> : null}
        <FlagBadges flags={r.flags} />
      </div>
    ),
  },
  text('close', '收盘价'),
  text('v', 'V', 'V'), // gloss source: GLOSS in CompanyView.tsx
  text('p', 'P', 'P'), // gloss source: GLOSS in CompanyView.tsx
  text('version_date', '版本日期', 'Version date'), // gloss source: GLOSS in CompanyView.tsx
  text('r', 'R', 'R'), // gloss source: GLOSS in CompanyView.tsx
  text('c', 'C', 'C'), // gloss source: GLOSS in CompanyView.tsx
  text('heavy', '重仓点', 'Heavy'), // gloss source: GLOSS in CompanyView.tsx
  text('buy_zone', '买入区间'),
  text('buy_price', '手填买入价'),
  text('fair_zone', '合理区间'),
  text('trim', '减仓线', 'Trim'), // gloss source: GLOSS in CompanyView.tsx
  text('clear', '清仓线', 'Clear'), // gloss source: GLOSS in CompanyView.tsx
  text('core', '核心线', 'Core'), // gloss source: GLOSS in CompanyView.tsx
  text('status', '状态'),
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
