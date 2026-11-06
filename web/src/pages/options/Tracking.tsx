// Issue #1827: one row per scanned candidate with its outcome and the P&L split. Python sends the rows, the
// order and every string; the table only sorts on demand and the browser computes nothing.
import { ClipboardList, Info } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/generated/tracking'
import { StaleAlert } from '@/pages/home/StaleAlert'
export { validate as validateTracking } from '@/generated/tracking'
export type { Payload as TrackingPayload } from '@/generated/tracking'

const COLUMNS: Column[] = [
  { id: 'symbol', label: '标的' },
  { id: 'expiry', label: '到期' },
  { id: 'strike', label: '行权价', align: 'right' },
  { id: 'status', label: '状态' },
  { id: 'outcome', label: '结果', wrap: true },
  { id: 'premium', label: '权利金', align: 'right' },
  { id: 'underlying_move', label: '标的变动', align: 'right' },
  { id: 'assigned', label: '指派' },
]
const text = (value: string) => ({ text: value, value })

type Row = Payload['rows'][number]

const toCells = (row: Row): Record<string, Cell> => ({
  symbol: text(row.symbol),
  expiry: text(row.expiry_text),
  strike: text(row.strike),
  status: { text: row.status, value: row.status, badge: true },
  outcome: text(row.outcome),
  premium: row.premium,
  underlying_move: row.underlying_move,
  ...(row.assigned === null
    ? {}
    : { assigned: { text: row.assigned, value: row.assigned, badge: true } }),
})

export function Tracking({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="tracking">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
      </header>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      {payload.note ? (
        <Alert data-tracking-note>
          <Info aria-hidden />
          <AlertDescription>{payload.note}</AlertDescription>
        </Alert>
      ) : null}
      <Card className="min-w-0">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ClipboardList aria-hidden />
            系统期权跟踪
          </CardTitle>
        </CardHeader>
        <CardContent className="min-w-0">
          <DataTable columns={COLUMNS} rows={payload.rows.map(toCells)} />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>怎么算的</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          {payload.how.map((line) => (
            <p key={line}>{line}</p>
          ))}
        </CardContent>
      </Card>
      <Alert data-tracking-pending>
        <Info aria-hidden />
        <AlertDescription>{payload.pending}</AlertDescription>
      </Alert>
    </div>
  )
}
