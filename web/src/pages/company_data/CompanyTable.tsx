// The company grid, a @tanstack/react-table table: one row per `companies[]`, one column per `columns[]`. The only sorts are
// on bar_count, eps_quarters and missing; a sort changes the order of rows, never a figure. A header with a sort is a
// focusable cell (the page holds no button). Counts are payload text: they are ordered by their digits, not parsed.
import { flexRender, getCoreRowModel, getSortedRowModel, useReactTable, type ColumnDef, type Row } from '@tanstack/react-table'
import { Info } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/company_data_page'

type Company = Payload['companies'][number]
type Column = Payload['columns'][number]

const digits = (text: string) => text.replace(/\D/g, '').replace(/^0+(?=\d)/, '')
// Text of digits in the order of the numbers they spell: the shorter first, then as written.
const byDigits = (a: string, b: string) => (a.length !== b.length ? a.length - b.length : a < b ? -1 : a > b ? 1 : 0)
const SORTABLE = new Set(['bar_count', 'eps_quarters', 'missing'])

function sortingFor(key: string): ColumnDef<Company>['sortingFn'] {
  if (key === 'missing') return (a: Row<Company>, b: Row<Company>) => a.original.missing.length - b.original.missing.length
  return (a: Row<Company>, b: Row<Company>) =>
    byDigits(digits(String(a.original[key as 'bar_count'])), digits(String(b.original[key as 'bar_count'])))
}

function value(c: Company, key: string): React.ReactNode {
  switch (key) {
    case 'fiscal_year_end':
      return c.fiscal_year_end
    case 'price_to':
      return c.price_to ?? '—'
    case 'bar_count':
      return c.bar_count
    case 'statements':
      return c.statements ? 'yes' : 'no'
    case 'latest_quarter':
      return c.latest_quarter
    case 'eps_quarters':
      return c.eps_quarters
    case 'next_report':
      return c.next_report ?? '—'
    case 'missing':
      return (
        <div className="flex flex-col items-start gap-1">
          {c.missing.map((m) => (
            <span key={m.code} data-reason={m.code} className="flex items-start gap-1 text-xs">
              <Badge variant="secondary">{m.code}</Badge>
              <span>{m.text}</span>
            </span>
          ))}
        </div>
      )
    default:
      return null
  }
}

export function CompanyTable({ columns: spec, companies }: Pick<Payload, 'columns' | 'companies'>) {
  const columns: ColumnDef<Company>[] = [
    {
      id: 'company',
      enableSorting: false,
      header: 'Company',
      cell: ({ row }) => {
        const c = row.original
        const label = `${c.symbol} ${c.name}`
        return (
          <div>
            {c.quarters_path !== '' ? (
              <a href={c.quarters_path} className="underline underline-offset-4">
                {label}
              </a>
            ) : (
              <span>{label}</span>
            )}
            <div className="text-xs text-muted-foreground">
              {c.market} {c.issuer}
            </div>
          </div>
        )
      },
    },
    ...spec.map(
      (c: Column): ColumnDef<Company> => ({
        id: c.key,
        // react-table sorts only a column with an accessor; the value is the payload's own field, ordered by sortingFn.
        accessorFn: SORTABLE.has(c.key) ? (row: Company) => (c.key === 'missing' ? row.missing : row[c.key as 'bar_count']) : undefined,
        enableSorting: SORTABLE.has(c.key),
        sortingFn: SORTABLE.has(c.key) ? sortingFor(c.key) : undefined,
        header: () => (
          <div className="space-y-1">
            <div>{c.label}</div>
            {c.gloss !== '' ? (
              <div data-gloss={c.key} className="flex items-start gap-1 text-xs font-normal text-muted-foreground">
                <Info aria-hidden className="mt-0.5 size-3 shrink-0" />
                <span>{c.gloss}</span>
              </div>
            ) : null}
          </div>
        ),
        cell: ({ row }) => value(row.original, c.key),
      }),
    ),
  ]
  const table = useReactTable({ data: companies, columns, getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel() })
  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id}>
            {group.headers.map((h) => {
              const sorted = h.column.getIsSorted()
              const sortable = h.column.getCanSort()
              return (
                <TableHead
                  key={h.id}
                  data-sort={sortable ? h.column.id : undefined}
                  aria-sort={sorted === 'asc' ? 'ascending' : sorted === 'desc' ? 'descending' : undefined}
                  tabIndex={sortable ? 0 : undefined}
                  className={`align-top whitespace-normal ${sortable ? 'cursor-pointer' : ''}`}
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
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id} data-company={row.original.key}>
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
