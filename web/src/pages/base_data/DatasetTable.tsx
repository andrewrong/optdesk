// The dataset table, a @tanstack/react-table table: one row per dataset in the order of the payload groups, a group name
// row where the group changes, a sort on the verdict (worst first, a lookup of the four levels), and a row that opens
// to show its checks. Every text is a payload field.
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type Row as TableRowModel,
  type Table as ReactTable,
} from '@tanstack/react-table'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Fragment, useMemo } from 'react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/base_data_page'
import { CheckList } from './CheckList'
import { CrossCheck } from './CrossCheck'
import { LevelBadge, type Level } from './LevelBadge'

type Group = Payload['groups'][number]
type Dataset = Group['rows'][number] & { group: string }

const WORST_FIRST: Record<Level, number> = { fail: 0, unknown: 1, warn: 2, pass: 3 }
const DASH = '—'

// Whether the user has sorted: the one test the group rows and the group name inside a row both follow.
const isSorted = (table: ReactTable<Dataset>) => table.getState().sorting.length > 0

const columns: ColumnDef<Dataset>[] = [
  {
    id: 'dataset',
    header: '数据集',
    enableSorting: false,
    cell: ({ row, table }) => (
      <button
        type="button"
        data-toggle
        aria-expanded={row.getIsExpanded()}
        onClick={row.getToggleExpandedHandler()}
        className="flex items-start gap-1 text-left"
      >
        {row.getIsExpanded() ? <ChevronDown aria-hidden className="mt-0.5 size-4 shrink-0" /> : <ChevronRight aria-hidden className="mt-0.5 size-4 shrink-0" />}
        <span>
          {row.original.label}
          <span className="block text-xs text-muted-foreground">{row.original.label_zh}</span>
          {isSorted(table) ? <span className="block text-xs text-muted-foreground">{row.original.group}</span> : null}
          <span className="mt-1 block space-y-0.5 text-xs md:hidden">
            <span className="block">{row.original.newest_day ?? DASH}</span>
            <span className="block">{row.original.age_text || DASH}</span>
            <span className="block">{row.original.failed_summary || DASH}</span>
          </span>
        </span>
      </button>
    ),
  },
  {
    id: 'verdict',
    header: '结论',
    accessorFn: (r) => r.level,
    sortingFn: (a: TableRowModel<Dataset>, b: TableRowModel<Dataset>) => WORST_FIRST[a.original.level] - WORST_FIRST[b.original.level],
    cell: ({ row }) => <LevelBadge level={row.original.level} />,
  },
  { id: 'newest_value', header: '最新值', enableSorting: false, cell: ({ row }) => row.original.newest_value || DASH },
  { id: 'date', header: '日期', enableSorting: false, cell: ({ row }) => row.original.newest_day ?? DASH },
  { id: 'age', header: '时长', enableSorting: false, cell: ({ row }) => row.original.age_text || DASH },
  { id: 'failed', header: '失败检查', enableSorting: false, cell: ({ row }) => row.original.failed_summary || DASH },
  {
    id: 'cross_check',
    header: '交叉核对',
    enableSorting: false,
    cell: ({ row }) => {
      const cc = row.original.cross_check
      return (
        <span data-cross-check-cell className="flex items-center gap-2">
          {cc ? (
            <>
              <LevelBadge level={cc.level} />
              <span className="font-mono text-xs">{cc.pair_id}</span>
            </>
          ) : null}
        </span>
      )
    },
  },
]

// Below md these columns show inside the dataset cell instead.
const NARROW_HIDDEN = new Set(['date', 'age', 'failed'])
// The first column stays in view while the table scrolls sideways; numbers and dates keep one line, words wrap.
const NO_WRAP = new Set(['newest_value', 'date', 'age'])
const narrowClass = (id: string) => (NARROW_HIDDEN.has(id) ? 'hidden md:table-cell' : '')
const stickyClass = (id: string) => (id === 'dataset' ? 'sticky left-0 bg-background' : '')
const wrapClass = (id: string) => (NO_WRAP.has(id) ? 'whitespace-nowrap' : 'whitespace-normal break-words')

export function DatasetTable({ groups }: { groups: readonly Group[] }) {
  // Memoised: a new `data` array on every render would reset the opened rows.
  const data = useMemo<Dataset[]>(() => groups.flatMap((g) => g.rows.map((r) => ({ ...r, group: g.name }))), [groups])
  const table = useReactTable({
    data,
    columns,
    getRowId: (r) => r.dataset,
    getRowCanExpand: () => true,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })
  const rows = table.getRowModel().rows
  // A group name row stands only while the rows are in payload order; once sorted, a group's rows are apart, so the
  // name moves into the row itself and no group label repeats.
  const sorted = isSorted(table)
  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id}>
            {hg.headers.map((h) => {
              const sortable = h.column.getCanSort()
              const sorted = h.column.getIsSorted()
              return (
                <TableHead
                  key={h.id}
                  aria-sort={sorted === 'asc' ? 'ascending' : sorted === 'desc' ? 'descending' : undefined}
                  tabIndex={sortable ? 0 : undefined}
                  className={[sortable ? 'cursor-pointer' : '', narrowClass(h.column.id), stickyClass(h.column.id)].filter(Boolean).join(' ') || undefined}
                  onClick={sortable ? h.column.getToggleSortingHandler() : undefined}
                  onKeyDown={
                    sortable
                      ? (e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault()
                            h.column.toggleSorting()
                          }
                        }
                      : undefined
                  }
                >
                  {flexRender(h.column.columnDef.header, h.getContext())}
                </TableHead>
              )
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {rows.map((row, i) => (
          <Fragment key={row.id}>
            {!sorted && (i === 0 || rows[i - 1]!.original.group !== row.original.group) ? (
              <TableRow data-group={row.original.group} className="bg-muted/40">
                <TableCell colSpan={columns.length} className="font-medium">
                  {row.original.group}
                </TableCell>
              </TableRow>
            ) : null}
            <TableRow data-dataset={row.original.dataset}>
              {row.getVisibleCells().map((c) => (
                <TableCell key={c.id} className={`align-top ${wrapClass(c.column.id)} ${narrowClass(c.column.id)} ${stickyClass(c.column.id)}`.trim().replace(/ +/g, ' ')}>
                  {flexRender(c.column.columnDef.cell, c.getContext())}
                </TableCell>
              ))}
            </TableRow>
            {row.getIsExpanded() ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="whitespace-normal">
                  <CheckList dataset={row.original.dataset} checks={row.original.checks} />
                  {row.original.cross_check ? <CrossCheck pair={row.original.cross_check} /> : null}
                </TableCell>
              </TableRow>
            ) : null}
          </Fragment>
        ))}
      </TableBody>
    </Table>
  )
}
