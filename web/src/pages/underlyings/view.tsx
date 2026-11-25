// The underlyings page (docs/design/underlyings.md#11 web-underlyings-view): the table of names, the add form, one edit
// form per row, the confirm step and the history, as plain HTML forms that post to the route of the payload. Every
// visible value is a payload field; this file posts nothing by script and computes nothing.
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import type { Row as Name } from '@/generated/underlyings.v2'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollHint } from '@/components/ScrollHint'
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

// The gloss shown next to the field name in the label of an edit form, keyed by the field name.
const GLOSS: Record<string, string> = {
  buy_price: '你想买入的价格',
  sale_price: '你想卖出的价格',
  report_date_override: '你自己定的财报日，格式 YYYY-MM-DD',
  issuer: '用于财报与日历的公司编号',
  status: '是否关注这个标的',
  options: '对这个标的卖出看跌期权（sell put）',
  fundamentals: '读取它的财报',
}

function labelText(name: string, gloss: boolean) {
  const g = gloss ? GLOSS[name] : undefined
  return g ? `${name} (${g})` : name
}

function Token({ form }: { form: Form }) {
  return (
    <>
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
    </>
  )
}

function Flag({ on }: { on: boolean }) {
  return <span>{on ? '开' : '关'}</span>
}

// The anchor id of the edit form of a row: the key with every character outside letters, digits, "_" and "-" as "-".
function editId(key: string) {
  return `edit-${key.replace(/[^A-Za-z0-9_-]/g, '-')}`
}

// The columns of text that wrap on a narrow screen; the figure cells keep their no-wrap.
const WRAP = new Set(['name', 'status', 'note'])
const WRAP_CLASS = 'whitespace-normal break-words'
const STICKY_CLASS = 'sticky left-0 bg-background'

const baseColumns: ColumnDef<Name>[] = [
  {
    id: 'name',
    header: '名称',
    cell: ({ row: { original: r } }) => (
      <div>
        <span className="font-medium">{r.symbol}</span> <span className="text-muted-foreground">{r.market}</span>{' '}
        <span>{r.name}</span>
      </div>
    ),
  },
  { id: 'kind', header: '类型', cell: ({ row }) => row.original.kind },
  {
    id: 'status',
    header: '状态',
    cell: ({ row }) => <Badge variant="secondary">{row.original.status_text}</Badge>,
  },
  { id: 'options', header: '期权', cell: ({ row }) => <Flag on={row.original.options} /> },
  {
    id: 'fundamentals',
    header: '基本面',
    cell: ({ row }) => <Flag on={row.original.fundamentals} />,
  },
  { id: 'held', header: '持有', cell: ({ row }) => <Flag on={row.original.held} /> },
  { id: 'buy', header: '买入价', cell: ({ row }) => row.original.buy_price },
  { id: 'sale', header: '卖出价', cell: ({ row }) => row.original.sale_price },
  {
    id: 'followed',
    header: '关注来源',
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
  { id: 'note', header: '备注', cell: ({ row }) => row.original.note },
]

const editColumn: ColumnDef<Name> = {
  id: 'edit',
  header: '',
  cell: ({ row }) => (
    <Button asChild variant="link" size="sm">
      <a href={`#${editId(row.original.key)}`}>{'编辑'}</a>
    </Button>
  ),
}

function NamesTable({ rows, editable }: { rows: Name[]; editable: boolean }) {
  const columns = editable ? [...baseColumns, editColumn] : baseColumns
  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() })
  return (
    <>
      <ScrollHint />
      <Table data-names>
        <TableHeader>
          {table.getHeaderGroups().map((hg) => (
            <TableRow key={hg.id}>
              {hg.headers.map((h, i) => (
                <TableHead key={h.id} className={i === 0 ? STICKY_CLASS : undefined}>
                  {flexRender(h.column.columnDef.header, h.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map((row) => (
            <TableRow key={row.original.key} data-name={row.original.key}>
              {row.getVisibleCells().map((c, i) => (
                <TableCell
                  key={c.id}
                  className={[WRAP.has(c.column.id) ? WRAP_CLASS : '', i === 0 ? STICKY_CLASS : ''].filter(Boolean).join(' ') || undefined}
                >
                  {flexRender(c.column.columnDef.cell, c.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </>
  )
}

function Field({
  id,
  name,
  defaultValue,
  gloss = false,
}: {
  id: string
  name: string
  defaultValue: string
  gloss?: boolean
}) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{labelText(name, gloss)}</Label>
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
        <Button type="submit">{'添加'}</Button>
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
      id={editId(row.key)}
      tabIndex={-1}
      data-row={row.key}
      className="hidden gap-3 border-t pt-3 target:grid sm:grid-cols-3"
    >
      <Token form={form} />
      <input type="hidden" name="key" value={row.key} />
      <div className="sm:col-span-3 font-medium">{row.key}</div>
      {TEXT_FIELDS.filter(has).map((f) => (
        <Field key={f} id={`${row.key}-${f}`} name={f} defaultValue={values[f]} gloss />
      ))}
      {SWITCH_FIELDS.filter(has).map((f) => (
        <div key={f} className="flex items-center gap-2">
          <Switch id={`${row.key}-${f}`} name={f} defaultChecked={row[f]} />
          <Label htmlFor={`${row.key}-${f}`}>{labelText(f, true)}</Label>
        </div>
      ))}
      <div className="flex items-end">
        <Button type="submit">{'保存'}</Button>
      </div>
    </form>
  )
}

function ConfirmCard({ confirm, form }: { confirm: NonNullable<Payload['confirm']>; form: Form | null }) {
  return (
    <Dialog defaultOpen>
      <DialogContent data-confirm showCloseButton={false} aria-describedby={undefined} className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle asChild>
            <h2 className="text-lg font-semibold">{'确认修改'}</h2>
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
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
          {form ? (
            <form method="post" action={form.post_path} data-confirm-form className="flex gap-2">
              <Token form={form} />
              {confirm.fields.map((f, i) => (
                <input key={`${i}-${f.name}`} type="hidden" name={f.name} value={f.value} />
              ))}
              <input type="hidden" name="confirm" value="1" />
              <Button type="submit">{'确认并保存'}</Button>
              <DialogClose asChild>
                <Button type="button" variant="outline">
                  {'取消'}
                </Button>
              </DialogClose>
            </form>
          ) : (
            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="outline">
                  {'取消'}
                </Button>
              </DialogClose>
            </DialogFooter>
          )}
        </div>
      </DialogContent>
    </Dialog>
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
          <NamesTable rows={payload.rows} editable={form !== null} />
        </CardContent>
      </Card>
      {confirm ? <ConfirmCard confirm={confirm} form={form} /> : null}
      {form ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">{'添加与编辑'}</h2>
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
            <h2 className="text-lg font-semibold">{'历史'}</h2>
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
