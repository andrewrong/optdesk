// The grid of datasets (rows) and session days (columns) of the data health page, a @tanstack/react-table table.
// The state, counts, texts and verdicts are printed as the builder sent them; nothing here is computed.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/data_health_page'
import { CellRepair, RepairAllForm } from './RepairButton'

type Form = Payload['form']
type Dataset = Payload['datasets'][number]
type Cell = Dataset['cells'][number]

function CellView({ cell, dataset, form }: { cell: Cell; dataset: Dataset; form: Form }) {
  return (
    <div data-cell={cell.day} data-state={cell.state} className="min-w-20 space-y-1 text-xs">
      <Badge variant={cell.state === 'missing' || cell.state === 'expired' ? 'destructive' : 'secondary'}>{cell.state}</Badge>
      {cell.state !== 'ok' && cell.state !== 'not_run' ? (
        <div>
          {cell.ok} of {cell.keys}
        </div>
      ) : null}
      {cell.missing_keys.length > 0 ? <div className="break-words text-muted-foreground">{cell.missing_keys.join(', ')}</div> : null}
      {cell.due_text !== '' ? <div>{cell.due_text}</div> : null}
      {cell.repair === 'not_refetchable' ? <div className="text-muted-foreground">{cell.why_text}</div> : null}
      {cell.repair === 'button' && form !== null && dataset.job_id !== null ? (
        <CellRepair form={form} job={dataset.job_id} day={cell.day} />
      ) : null}
    </div>
  )
}

export function CoverageGrid({ datasets, form }: { datasets: Dataset[]; form: Form }) {
  // The days of the grid: the days the builder sent, each once, as written (ISO dates order as text).
  const days = [...new Set(datasets.flatMap((d) => d.cells.map((c) => c.day)))].sort()
  const columns: ColumnDef<Dataset>[] = [
    {
      id: 'dataset',
      header: 'Dataset',
      cell: ({ row }) => {
        const d = row.original
        return (
          <div className="min-w-40 space-y-1">
            <div className="font-medium">{d.label}</div>
            <div className="text-xs text-muted-foreground">{d.dataset_id}</div>
            {d.newest.map((n) => (
              <div key={n.series_id} data-newest={n.series_id} className="text-xs">
                {n.series_id}: {n.day}, {n.age_days} days old
              </div>
            ))}
            {d.repair_all === 'button' && form !== null ? <RepairAllForm form={form} dataset={d.dataset_id} gapDays={d.gap_days} /> : null}
          </div>
        )
      },
    },
    ...days.map(
      (day): ColumnDef<Dataset> => ({
        id: day,
        header: day,
        cell: ({ row }) => {
          const cell = row.original.cells.find((c) => c.day === day)
          return cell === undefined ? null : <CellView cell={cell} dataset={row.original} form={form} />
        },
      }),
    ),
  ]
  const table = useReactTable({ data: datasets, columns, getCoreRowModel: getCoreRowModel() })
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
          <TableRow key={row.id} data-dataset={row.original.dataset_id}>
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
