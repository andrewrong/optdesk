// The underlyings page (docs/design/underlyings.md#11 web-underlyings-view): the table of names, the add form, one edit
// form per row, the confirm step and the history, as plain HTML forms that post to the route of the payload. Every
// visible value is a payload field; this file posts nothing by script and computes nothing.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import type { Row as Name } from '@/generated/underlyings.v1'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/underlyings'

type Form = NonNullable<Payload['form']>

const TEXT_FIELDS = [
  'status',
  'name',
  'issuer',
  'buy_price',
  'sale_price',
  'report_date_override',
  'note',
  'exchange',
  'currency',
] as const
const SWITCH_FIELDS = ['options', 'fundamentals'] as const

function Token({ form }: { form: Form }) {
  return (
    <>
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
    </>
  )
}

function Flag({ on, label }: { on: boolean; label: string }) {
  return <Switch checked={on} disabled aria-label={label} />
}

const columns: ColumnDef<Name>[] = [
  {
    id: 'name',
    header: 'Name',
    cell: ({ row: { original: r } }) => (
      <div>
        <span className="font-medium">{r.symbol}</span> <span className="text-muted-foreground">{r.market}</span>{' '}
        <span>{r.name}</span>
      </div>
    ),
  },
  { id: 'kind', header: 'Kind', cell: ({ row }) => row.original.kind },
  {
    id: 'status',
    header: 'Status',
    cell: ({ row }) => <Badge variant="secondary">{row.original.status_text}</Badge>,
  },
  { id: 'options', header: 'Options', cell: ({ row }) => <Flag on={row.original.options} label="options" /> },
  {
    id: 'fundamentals',
    header: 'Fundamentals',
    cell: ({ row }) => <Flag on={row.original.fundamentals} label="fundamentals" />,
  },
  { id: 'held', header: 'Held', cell: ({ row }) => <Flag on={row.original.held} label="held" /> },
  { id: 'buy', header: 'Buy', cell: ({ row }) => row.original.buy_price },
  { id: 'sale', header: 'Sale', cell: ({ row }) => row.original.sale_price },
  {
    id: 'followed',
    header: 'Followed by',
    cell: ({ row: { original: r } }) => (
      <div className="flex flex-wrap items-center gap-1">
        {r.followed_by.map((g) => (
          <Badge key={g} variant="outline">
            {g}
          </Badge>
        ))}
        <span className="text-xs text-muted-foreground">{r.followed_text}</span>
      </div>
    ),
  },
  { id: 'note', header: 'Note', cell: ({ row }) => row.original.note },
]

function NamesTable({ rows }: { rows: Name[] }) {
  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() })
  return (
    <Table data-names>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id}>
            {hg.headers.map((h) => (
              <TableHead key={h.id}>{flexRender(h.column.columnDef.header, h.getContext())}</TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.original.key} data-name={row.original.key}>
            {row.getVisibleCells().map((c) => (
              <TableCell key={c.id}>{flexRender(c.column.columnDef.cell, c.getContext())}</TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function Field({ id, name, defaultValue }: { id: string; name: string; defaultValue: string }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{name}</Label>
      <Input id={id} name={name} defaultValue={defaultValue} />
    </div>
  )
}

function AddForm({ form }: { form: Form }) {
  return (
    <form method="post" action="/owner-underlyings" data-add-form className="grid gap-3 sm:grid-cols-3">
      <Token form={form} />
      {['symbol', 'market', 'kind', 'name'].map((n) => (
        <Field key={n} id={`add-${n}`} name={n} defaultValue="" />
      ))}
      <div className="flex items-end">
        <Button type="submit">Add</Button>
      </div>
    </form>
  )
}

function EditForm({ row, form }: { row: Name; form: Form }) {
  const has = (f: string) => row.editable.includes(f)
  // exchange and currency are editable on a pending row but are not part of the payload: they start empty.
  const values: Record<(typeof TEXT_FIELDS)[number], string> = {
    status: row.status,
    name: row.name,
    issuer: row.issuer,
    buy_price: row.buy_price,
    sale_price: row.sale_price,
    report_date_override: row.report_date_override,
    note: row.note,
    exchange: '',
    currency: '',
  }
  return (
    <form
      method="post"
      action="/owner-underlyings"
      data-row={row.key}
      className="grid gap-3 border-t pt-3 sm:grid-cols-3"
    >
      <Token form={form} />
      <input type="hidden" name="key" value={row.key} />
      <div className="sm:col-span-3 font-medium">{row.key}</div>
      {TEXT_FIELDS.filter(has).map((f) => (
        <Field key={f} id={`${row.key}-${f}`} name={f} defaultValue={values[f]} />
      ))}
      {SWITCH_FIELDS.filter(has).map((f) => (
        <div key={f} className="flex items-center gap-2">
          <Switch id={`${row.key}-${f}`} name={f} defaultChecked={row[f]} />
          <Label htmlFor={`${row.key}-${f}`}>{f}</Label>
        </div>
      ))}
      <div className="flex items-end">
        <Button type="submit">Save</Button>
      </div>
    </form>
  )
}

function ConfirmCard({ confirm }: { confirm: NonNullable<Payload['confirm']> }) {
  return (
    <Card data-confirm>
      <CardHeader>
        <CardTitle>
          <h2 className="text-lg font-semibold">Confirm changes</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Table>
          <TableBody>
            {confirm.changes.map((c, i) => (
              <TableRow key={`${i}-${c.label}`} data-change>
                <TableCell>{c.label}</TableCell>
                <TableCell>{c.old}</TableCell>
                <TableCell>{c.new}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {confirm.stops.map((s, i) => (
          <Alert key={`${i}-${s}`} variant="warn" data-stop>
            <AlertDescription>{s}</AlertDescription>
          </Alert>
        ))}
      </CardContent>
    </Card>
  )
}

export function UnderlyingsView({ payload }: { payload: Payload }) {
  const { form, confirm } = payload
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">{payload.title}</h1>
          </CardTitle>
          <p className="text-sm text-muted-foreground">{payload.sentence}</p>
        </CardHeader>
        <CardContent className="space-y-3">
          {payload.notice !== '' ? (
            <Alert data-notice>
              <AlertDescription>{payload.notice}</AlertDescription>
            </Alert>
          ) : null}
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          <NamesTable rows={payload.rows} />
        </CardContent>
      </Card>
      {confirm ? <ConfirmCard confirm={confirm} /> : null}
      {form ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">Add and edit</h2>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <AddForm form={form} />
            {payload.rows.map((r) => (
              <EditForm key={r.key} row={r} form={form} />
            ))}
          </CardContent>
        </Card>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>
            <h2 className="text-lg font-semibold">History</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ul data-history className="space-y-1 text-sm">
            {payload.history.map((h, i) => (
              <li key={`${i}-${h.key}`}>
                <span className="text-muted-foreground">{h.at}</span> <span className="font-medium">{h.key}</span> {h.text}
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </div>
  )
}

export const registration = { schema: 'underlyings', page: page<Payload>(validate, UnderlyingsView) }
