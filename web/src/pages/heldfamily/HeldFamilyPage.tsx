// The held companies of the family as a table: the columns with their hints, the rows in payload order, the sum,
// the lead notes and the gaps. The page builder supplies every figure and sentence; the browser shows them and computes nothing.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/held_family_page'
export { validate as validateHeldFamilyPage } from '@/types/held_family_page'
export type { Payload as HeldFamilyPagePayload } from '@/types/held_family_page'

type Row = Payload['rows'][number]
type Column = Payload['columns'][number]

// A gap cell shows its reason in words (the producer's text); a value cell shows its text.
function CellText({ cell }: { cell: Row['value'] | Row['share'] | Row['earnings'] }) {
  if (cell.kind === 'gap') {
    return (
      <span data-gap={cell.reason} className="text-muted-foreground">
        {cell.text}
      </span>
    )
  }
  return <span className="tabular-nums">{cell.text}</span>
}

function cellOf(id: string, row: Row) {
  switch (id) {
    case 'company':
      return <span className="font-medium">{row.company}</span>
    case 'accounts':
      return row.accounts.map((a) => a.label).join(', ')
    case 'value':
      return <CellText cell={row.value} />
    case 'share':
      return <CellText cell={row.share} />
    case 'earnings':
      return <CellText cell={row.earnings} />
    default:
      return null
  }
}

export function HeldFamilyPage({ payload }: { payload: Payload }) {
  const columns: ColumnDef<Row>[] = payload.columns.map((c: Column) => ({
    id: c.id,
    header: () => (
      <div>
        <div>{c.label}</div>
        <div className="text-xs font-normal text-muted-foreground" data-hint={c.id}>
          {c.hint}
        </div>
      </div>
    ),
    cell: ({ row }) => cellOf(c.id, row.original),
  }))
  const table = useReactTable({ data: payload.rows, columns, getCoreRowModel: getCoreRowModel() })
  return (
    <div className="min-w-0 space-y-6" data-page="held-family-page">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground" data-total>
          {payload.total.text} {payload.total.currency} · {payload.total.day} · {payload.total.accounts_text}
        </p>
        {payload.stale ? <p data-stale>{payload.stale.text}</p> : null}
      </header>
      {payload.lead_notes.length > 0 ? (
        <ul className="space-y-1 text-sm text-muted-foreground" data-lead-notes>
          {payload.lead_notes.map((n, i) => (
            <li key={i}>{n}</li>
          ))}
        </ul>
      ) : null}
      <div className="overflow-x-auto">
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
              <TableRow key={row.id} data-company={row.original.company}>
                {row.getVisibleCells().map((c) => (
                  <TableCell key={c.id} className="align-top whitespace-normal">
                    {flexRender(c.column.columnDef.cell, c.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <p className="text-sm" data-rows-sum>
        <CellText cell={payload.rows_sum} />
      </p>
      {payload.notes.length > 0 ? (
        <ul className="space-y-1 text-sm text-muted-foreground" data-notes>
          {payload.notes.map((n, i) => (
            <li key={i}>{n}</li>
          ))}
        </ul>
      ) : null}
      {payload.gaps.length > 0 ? (
        <ul className="space-y-1 text-sm text-muted-foreground" data-gaps>
          {payload.gaps.map((g, i) => (
            <li key={i} data-gap={g.reason}>
              {g.text}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}

export const registration = {
  schema: 'held_family_page',
  page: page<Payload>(validate, HeldFamilyPage),
}
