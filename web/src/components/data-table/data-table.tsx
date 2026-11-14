import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import { ArrowDown, ArrowUp, ArrowUpDown, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { MissingValue } from './missing-value'

// A figure cell is Python's `{text, value?, tone?}`; a gap cell is `{reason, text}` and has no value.
export interface FigureCell {
  text: string
  value?: number | string | null
  tone?: 'up' | 'down' | 'flat' | 'none' | null
  /** A line of text shown under `text` (for example the source of a row). */
  note?: string | null
  /** The dated source of the figure (Python's text), shown in a Badge next to it. */
  source?: string
  /** The status Python sent for the source (good / late / missing): colours the source Badge. */
  source_status?: keyof typeof SOURCE_STATUS_CLASS
  /** Show the text itself in a Badge (for example a status word written by Python). */
  badge?: boolean
  /** Where the text leads: shown as a link (Button link variant). */
  href?: string
  /** A cell the user can choose: the text is a toggle button; choosing is the page's state, not a figure. */
  select?: { onSelect: () => void; selected: boolean }
  /** A sharp-move mark written by Python: its text is shown in a destructive Badge beside the figure. */
  sharp?: { text: string } | null
  /** A display-only ≈USD figure beside a JPY amount (Python's text, for example `≈$82`). */
  approx_usd?: { text: string } | null
}
export interface GapCell {
  reason: string
  text: string
}
export type Cell = FigureCell | GapCell
export interface Column {
  id: string
  label: string
  align?: 'left' | 'right'
  /** Let the cells of this column wrap (overrides the nowrap of TableCell). */
  wrap?: boolean
  /** One sentence written by Python, shown as a tooltip on the header. */
  hint?: string | null
}
type Row = Record<string, Cell>

// The one tone-to-class map for the Tone Python sends (up / down / flat); none has no class.
export const TONE_CLASS = {
  up: 'text-success',
  down: 'text-down',
  flat: 'text-muted-foreground',
} as const
export const SOURCE_STATUS_CLASS = {
  good: 'text-success',
  late: 'text-warning',
  missing: 'text-destructive',
} as const
const isGap = (cell: Cell): cell is GapCell => 'reason' in cell
const SORT_LABEL = { asc: 'ascending', desc: 'descending' } as const

// shadcn Data Table: the user sorts on `value`, the text is shown as sent. A gap cell is a MissingValue dash
// and has no value; a column with no value in any cell cannot be sorted; the tone of a figure cell is written by Python. No grouping, no totals.
export function DataTable({
  columns,
  rows,
  dim,
}: {
  columns: Column[]
  rows: Row[]
  /** Python's flag per row, by its index in `rows`: a dimmed row (for example a skipped candidate). */
  dim?: boolean[]
}) {
  const wraps = (id: string) => columns.find((c) => c.id === id)?.wrap === true
  const alignRight = (id: string) => columns.find((c) => c.id === id)?.align === 'right'
  const [sorting, setSorting] = useState<SortingState>([])
  const hintOf = (id: string) => columns.find((c) => c.id === id)?.hint
  const defs: ColumnDef<Row>[] = columns.map(({ id, label }) => ({
    id,
    enableSorting:
      label !== '' &&
      rows.some((row) => {
      const cell = row[id]
      return cell !== undefined && !isGap(cell) && cell.value != null
    }),
    accessorFn: (row) => {
      const cell = row[id]
      return cell === undefined || isGap(cell) ? undefined : cell.value
    },
    header: label,
    cell: ({ row }) => {
      const cell = row.original[id]
      if (cell === undefined) return null
      if (isGap(cell)) {
        return <MissingValue text={cell.text} />
      }
      if (cell.select) {
        const { onSelect, selected } = cell.select
        return (
          <>
            <Button
              variant={selected ? 'secondary' : 'ghost'}
              size="sm"
              aria-pressed={selected}
              onClick={onSelect}
            >
              {cell.text}
            </Button>
            {cell.note ? <div className="text-xs text-muted-foreground">{cell.note}</div> : null}
          </>
        )
      }
      if (cell.href) {
        return (
          <Button asChild variant="link" size="sm">
            <a href={cell.href}>{cell.text}</a>
          </Button>
        )
      }
      const text = cell.badge ? (
        <Badge variant="secondary">{cell.text}</Badge>
      ) : cell.tone && cell.tone !== 'none' ? (
        <span className={TONE_CLASS[cell.tone]}>{cell.text}</span>
      ) : (
        cell.text
      )
      const marked = cell.sharp ? (
        <>
          {text}{' '}
          <Badge variant="destructive" data-sharp>
            <TriangleAlert aria-hidden />
            {cell.sharp.text}
          </Badge>
        </>
      ) : (
        text
      )
      const approx = cell.approx_usd ? (
        <>
          {marked} <span className="text-muted-foreground">{cell.approx_usd.text}</span>
        </>
      ) : (
        marked
      )
      const shown = cell.source ? (
        <>
          {approx}{' '}
          <Badge
            variant="secondary"
            className={cell.source_status && SOURCE_STATUS_CLASS[cell.source_status]}
          >
            {cell.source}
          </Badge>
        </>
      ) : (
        approx
      )
      return cell.note ? (
        <>
          {shown}
          <div className="text-xs text-muted-foreground">{cell.note}</div>
        </>
      ) : (
        shown
      )
    },
  }))
  const table = useReactTable({
    data: rows,
    columns: defs,
    state: { sorting },
    onSortingChange: setSorting,
    sortDescFirst: false,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })
  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id}>
            {group.headers.map((header) => {
              const sorted = header.column.getIsSorted()
              return (
                <TableHead
                  key={header.id}
                  className={cn(alignRight(header.column.id) && 'text-right')}
                  aria-sort={sorted ? `${SORT_LABEL[sorted]}` : 'none'}
                >
                  {(() => {
                    const hint = hintOf(header.column.id)
                    const label = flexRender(header.column.columnDef.header, header.getContext())
                    const SortIcon =
                      sorted === 'asc' ? ArrowUp : sorted === 'desc' ? ArrowDown : ArrowUpDown
                    const sortButton = header.column.getCanSort() ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={header.column.getToggleSortingHandler()}
                      >
                        {label}
                        <SortIcon aria-hidden className={cn(!sorted && 'opacity-40')} />
                      </Button>
                    ) : (
                      <span className="px-2.5 text-sm font-medium">{label}</span>
                    )
                    return hint ? (
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>{sortButton}</TooltipTrigger>
                          <TooltipContent>{hint}</TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    ) : (
                      sortButton
                    )
                  })()}
                </TableHead>
              )
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id} className={cn(dim?.[row.index] && 'opacity-55')}>
            {row.getVisibleCells().map((cell) => (
              <TableCell
                key={cell.id}
                className={cn(
                  'py-1.5',
                  alignRight(cell.column.id) && 'text-right tabular-nums',
                  wraps(cell.column.id) && 'whitespace-normal break-words',
                )}
              >
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
