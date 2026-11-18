// Issues #1770, #1826: every open structure by days to expiry, in two groups. Python sends the groups, the
// order, the strings and the links; the table only sorts on demand.
import { CalendarClock, Info } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/structures_list'
import { StaleAlert } from '@/pages/home/StaleAlert'
export { validate as validateStructuresList } from '@/types/structures_list'
export type { Payload as StructuresListPayload } from '@/types/structures_list'

const COLUMNS: Column[] = [
  { id: 'symbol', label: 'Underlying' },
  { id: 'kind', label: 'Type', wrap: true },
  { id: 'expiry', label: 'Expiry' },
  { id: 'moneyness', label: 'In/out of the money', wrap: true },
  { id: 'distance', label: 'Distance to last', align: 'right' },
  { id: 'earned', label: 'Earned %', align: 'right' },
  { id: 'ex_dividend', label: 'Ex-dividend', wrap: true },
  { id: 'max_loss', label: 'Max loss', align: 'right' },
  { id: 'max_gain', label: 'Max profit', align: 'right' },
  { id: 'pnl', label: 'Floating P/L', align: 'right' },
]
const text = (value: string) => ({ text: value, value })
// A text of Python's that is a Badge, or its Gap as it is.
const badge = (cell: string | Gap): Cell =>
  typeof cell === 'string' ? { text: cell, value: cell, badge: true } : cell
const plain = (cell: string | Gap): Cell => (typeof cell === 'string' ? text(cell) : cell)

type Group = Payload['groups'][number]
type Row = Group['rows'][number]
type Gap = Extract<Row['distance'], object>

const toCells = (row: Row): Record<string, Cell> => ({
  symbol: { text: row.symbol, value: row.symbol, href: row.href },
  kind: text(row.kind_text),
  expiry: text(row.expiry_text),
  moneyness: badge(row.moneyness),
  distance: plain(row.distance),
  earned: plain(row.earned),
  ...(row.ex_dividend === null ? {} : { ex_dividend: badge(row.ex_dividend) }),
  max_loss: row.max_loss,
  max_gain: row.max_gain,
  pnl: row.pnl,
})

const NO_EX_DIVIDEND = '除息日列暂无数据：当前没有可用的除息日来源，已隐藏。'
const EMPTY_GROUP = ['本周没有需要决定的结构。', '没有其他结构。']

export function StructuresList({ payload }: { payload: Payload }) {
  const hasExDividend = payload.groups.some((group) => group.rows.some((row) => row.ex_dividend !== null))
  const columns = hasExDividend ? COLUMNS : COLUMNS.filter((column) => column.id !== 'ex_dividend')
  return (
    <div className="min-w-0 space-y-6" data-page="structures-list">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
        {hasExDividend ? null : (
          <p className="mt-1 text-sm text-muted-foreground" data-structures-no-ex-dividend>
            {NO_EX_DIVIDEND}
          </p>
        )}
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      {payload.groups.map((group, index) => (
        <Card key={group.title} className="min-w-0" data-structures-group={group.title}>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {index === 0 ? <CalendarClock aria-hidden /> : null}
              {group.title}
            </CardTitle>
          </CardHeader>
          <CardContent className="min-w-0">
            {group.rows.length === 0 ? (
              <Alert data-structures-empty>
                <Info aria-hidden />
                <AlertDescription>{EMPTY_GROUP[index] ?? EMPTY_GROUP[1]}</AlertDescription>
              </Alert>
            ) : (
              <DataTable columns={columns} rows={group.rows.map(toCells)} />
            )}
            {group.rows.some((row) => row.compare) ? (
              <ul className="mt-3 space-y-1 text-sm text-muted-foreground">
                {group.rows.map((row) =>
                  row.compare ? (
                    <li key={row.href} data-structures-compare>
                      {row.compare.sentence}
                    </li>
                  ) : null,
                )}
              </ul>
            ) : null}
          </CardContent>
        </Card>
      ))}
      {payload.note ? (
        <Alert data-structures-note>
          <Info aria-hidden />
          <AlertDescription>{payload.note}</AlertDescription>
        </Alert>
      ) : null}
    </div>
  )
}

export const registration = {
  schema: 'structures_list',
  page: page<Payload>(validate, StructuresList),
}
