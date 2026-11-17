// The review history and the detail of one review: a tanstack table whose row expands to `detail` and the
// report text. Every cell is a payload text.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/valuation_page'

type HistoryRow = NonNullable<Payload['history']>[number]
type Detail = HistoryRow['detail']

export function DetailView({ detail }: { detail: Detail }) {
  const facts: [string, string][] = [
    ['Source', detail.source],
    ['Fiscal period end', detail.fiscal_period_end],
    ['Report date', detail.report_date],
    ['Earnings base', detail.earnings_base],
    ['Base basis', detail.base_basis],
    ['Forward factor', detail.forward_factor],
    ['Locked multiple', detail.locked_multiple],
    ['Method results', detail.method_results.join(', ')],
    ['Reaction', detail.reaction],
    ['Moat', detail.moat],
    ['Understand', detail.understand],
    ['Management', detail.management],
    ['Financial', detail.financial],
    ['Reason P', detail.reasons.p],
    ['Reason moat', detail.reasons.moat],
    ['Reason understand', detail.reasons.understand],
    ['Reason management', detail.reasons.management],
    ['Reason financial', detail.reasons.financial],
    ['Entered at', detail.entered_at],
    ['Effective at', detail.effective_at ?? ''],
  ]
  return (
    <div className="space-y-2 text-sm" data-detail>
      <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1">
        {facts
          .filter(([, value]) => value !== '')
          .map(([label, value]) => (
            <div key={label} className="contents">
              <dt className="text-muted-foreground">{label}</dt>
              <dd>{value}</dd>
            </div>
          ))}
      </dl>
      {detail.adjustments.map((a, at) => (
        <p key={`a${at}`}>
          Adjustment {a.percent} {a.reason}
        </p>
      ))}
      {detail.scenarios.map((s, at) => (
        <p key={`s${at}`}>
          Scenario {s.label}: probability {s.probability}, EPS {s.eps}, multiple {s.multiple}
        </p>
      ))}
      {detail.report_text !== '' ? <p className="whitespace-pre-wrap">{detail.report_text}</p> : null}
    </div>
  )
}

const COLUMNS: ColumnDef<HistoryRow>[] = [
  { id: 'date', header: 'Date', cell: ({ row }) => row.original.date },
  { id: 'v_from', header: 'V from', cell: ({ row }) => row.original.v_from },
  { id: 'v_to', header: 'V to', cell: ({ row }) => row.original.v_to },
  { id: 'change', header: 'Change %', cell: ({ row }) => row.original.change_percent },
  { id: 'type', header: 'Type', cell: ({ row }) => row.original.type },
  { id: 'trigger', header: 'Trigger', cell: ({ row }) => row.original.trigger },
  { id: 'memo', header: 'Memo', cell: ({ row }) => row.original.memo },
  { id: 'status', header: 'Status', cell: ({ row }) => <Badge variant="secondary">{row.original.status}</Badge> },
  { id: 'void_reason', header: 'Void reason', cell: ({ row }) => row.original.void_reason },
  {
    id: 'expand',
    header: '',
    cell: ({ row }) => (
      <Button
        type="button"
        variant="ghost"
        size="sm"
        aria-expanded={row.getIsExpanded()}
        onClick={() => row.toggleExpanded()}
      >
        {row.getIsExpanded() ? 'Hide detail' : 'Detail'}
      </Button>
    ),
  },
]

export function ReviewHistory({ history }: { history: HistoryRow[] }) {
  const table = useReactTable({
    data: history,
    columns: COLUMNS,
    getCoreRowModel: getCoreRowModel(),
    getRowCanExpand: () => true,
  })
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
          <FragmentRows key={row.id} row={row} />
        ))}
      </TableBody>
    </Table>
  )
}

function FragmentRows({ row }: { row: ReturnType<ReturnType<typeof useReactTable<HistoryRow>>['getRowModel']>['rows'][number] }) {
  return (
    <>
      <TableRow data-review={row.original.review_id}>
        {row.getVisibleCells().map((cell) => (
          <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
        ))}
      </TableRow>
      {row.getIsExpanded() ? (
        <TableRow>
          <TableCell colSpan={COLUMNS.length}>
            <DetailView detail={row.original.detail} />
          </TableCell>
        </TableRow>
      ) : null}
    </>
  )
}
