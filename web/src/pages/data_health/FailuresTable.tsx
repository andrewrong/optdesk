// The keys that keep failing or coming back empty, one table per dataset (docs/design/repeated-failures.md#5):
// a @tanstack/react-table table on the shadcn Table. The rows, the texts and the hidden-keys sentence are printed
// as the builder sent them; nothing here is counted, sorted or filtered.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/data_health_page'

type Dataset = Payload['datasets'][number]
type Failure = NonNullable<Dataset['repeated_failures']>[number]

const columns = (job: string): ColumnDef<Failure>[] => [
  { id: 'job', header: '任务', cell: () => job },
  { id: 'key', header: '键', cell: ({ row }) => row.original.key },
  {
    id: 'kind',
    header: '类型',
    cell: ({ row }) => (
      <Badge variant={row.original.kind === 'failed' ? 'destructive' : 'secondary'}>{row.original.kind}</Badge>
    ),
  },
  { id: 'days', header: '天数', cell: ({ row }) => row.original.days },
  { id: 'runs', header: '运行次数', cell: ({ row }) => row.original.runs },
  { id: 'first_day', header: '首日', cell: ({ row }) => row.original.first_day },
  { id: 'last_day', header: '末日', cell: ({ row }) => row.original.last_day },
  {
    id: 'last_class',
    header: '最近分类',
    cell: ({ row }) => (
      <div>
        <div>{row.original.last_class}</div>
        <div className="text-xs text-muted-foreground">{row.original.class_text}</div>
      </div>
    ),
  },
]

function Rows({ rows, job }: { rows: Failure[]; job: string }) {
  const table = useReactTable({ data: rows, columns: columns(job), getCoreRowModel: getCoreRowModel() })
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
          <TableRow key={row.id} data-kind={row.original.kind}>
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

// One dataset's block; a dataset whose payload has no `repeated_failures` shows nothing.
export function FailuresTable({ dataset, noneText }: { dataset: Dataset; noneText: string | undefined }) {
  const rows = dataset.repeated_failures
  if (rows === undefined) return null
  const hidden = dataset.failures_hidden_text
  return (
    <section data-failures={dataset.dataset_id} className="space-y-2">
      <h3 className="text-sm font-medium">{dataset.label}</h3>
      {rows.length === 0 ? (
        noneText ? <p className="text-sm text-muted-foreground">{noneText}</p> : null
      ) : (
        <Rows rows={rows} job={dataset.job_id ?? '-'} />
      )}
      {hidden ? <p className="text-sm text-muted-foreground">{hidden}</p> : null}
    </section>
  )
}
