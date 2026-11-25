// The spread legs page: one card per structure, in the order sent, group by group. Each card shows the symbol (a link to
// the detail page), the kind with its gloss, the expiry, the legs in the shared Data Table and the four figures. The
// producer wrote every text; the browser shows them and computes, parses and sorts nothing. No chart.
import { DataTable, type Column } from '@/components/data-table/data-table'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/spread_legs_page'
export { validate as validateSpreadLegsPage } from '@/types/spread_legs_page'
export type { Payload as SpreadLegsPagePayload } from '@/types/spread_legs_page'

type Structure = Payload['groups'][number]['structures'][number]

const LEG_COLUMNS: Column[] = [
  { id: 'side', label: '方向' },
  { id: 'quantity', label: '数量', align: 'right' },
  { id: 'strike', label: '行权价', align: 'right' },
  { id: 'open_price', label: '开仓价', align: 'right' },
]

// A gap shows its text as it is; a value shows its text. Never a number of our own.
function Figure({ name, label, cell }: { name: string; label: string; cell: Structure['net_credit'] }) {
  return (
    <div data-figure={name} data-kind={cell.kind}>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={cell.kind === 'gap' ? 'text-muted-foreground' : 'tabular-nums'}>
        {cell.kind === 'gap' ? <span data-gap={cell.reason}>{cell.text}</span> : cell.text}
      </dd>
    </div>
  )
}

function BreakEven({ cell }: { cell: Structure['break_even'] }) {
  return (
    <div data-figure="break_even" data-kind={cell.kind}>
      <dt className="text-xs text-muted-foreground">盈亏平衡点</dt>
      <dd className={cell.kind === 'gap' ? 'text-muted-foreground' : 'tabular-nums'}>
        {cell.kind === 'gap' ? <span data-gap={cell.reason}>{cell.text}</span> : cell.text}
      </dd>
    </div>
  )
}

function StructureCard({ s }: { s: Structure }) {
  const rows = s.legs.map((l) => ({
    side: { text: l.side_text },
    quantity: { text: l.quantity },
    strike: { text: l.strike },
    open_price: { text: l.open_price },
  }))
  return (
    <section className="min-w-0 space-y-3 rounded-lg border p-4" data-structure={s.href}>
      <header>
        <h3 className="text-lg font-semibold">
          <a href={s.href} className="underline-offset-4 hover:underline">
            {s.symbol}
          </a>
        </h3>
        <p className="text-sm">
          <span data-kind-text>{s.kind_text}</span>{' '}
          <span className="text-muted-foreground" data-kind-gloss>
            {s.kind_gloss}
          </span>
        </p>
        <p className="text-sm text-muted-foreground" data-expiry>
          {s.expiry_text}
        </p>
        {s.reason ? (
          <p className="text-sm text-muted-foreground" data-reason>
            {s.reason}
          </p>
        ) : null}
      </header>
      <div className="min-w-0 overflow-x-auto" data-legs>
        <DataTable columns={LEG_COLUMNS} rows={rows} />
      </div>
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Figure name="net_credit" label="净权利金收入" cell={s.net_credit} />
        <Figure name="max_loss" label="最大亏损" cell={s.max_loss} />
        <Figure name="max_gain" label="最大盈利" cell={s.max_gain} />
        <BreakEven cell={s.break_even} />
      </dl>
    </section>
  )
}

export function SpreadLegsPage({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="spread-legs-page">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground" data-sentence>
          {payload.sentence}
        </p>
        {payload.note ? (
          <p className="mt-1 text-sm text-muted-foreground" data-note>
            {payload.note}
          </p>
        ) : null}
        {payload.stale ? <p data-stale>{payload.stale.text}</p> : null}
      </header>
      {payload.groups.map((g, i) => (
        <div key={i} className="space-y-3" data-group={i}>
          <h2 className="text-xl font-semibold">{g.title}</h2>
          {g.structures.map((s) => (
            <StructureCard key={s.href} s={s} />
          ))}
        </div>
      ))}
    </div>
  )
}

export const registration = {
  schema: 'spread_legs_page',
  page: page<Payload>(validate, SpreadLegsPage),
}
