// The advice page (issue #1603): yesterday's advice rows and their close-to-close delta. Every row text, the delta
// and the acted word are made by Python; the table only sorts on demand. A missing day is one gap, shown once.
import { TriangleAlert } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/advice_page'
export { validate as validateAdvicePage } from '@/types/advice_page'
export type { Payload as AdvicePagePayload } from '@/types/advice_page'

// Fixed column heads of the table: no payload field is read for them.
const COLUMNS: Column[] = [
  { id: 'sentence', label: '建议', wrap: true },
  { id: 'account', label: '账户' },
  { id: 'ref_value', label: '参考值', align: 'right' },
  { id: 'after_value', label: '收盘后值', align: 'right' },
  { id: 'delta', label: '差值', align: 'right' },
  { id: 'acted', label: '是否执行' },
]

type AdviceRow = Payload['rows'][number]
type TextCell = AdviceRow['sentence']

// A text cell as the data table reads it: the schema allows a null where the table wants it absent.
const text = (c: TextCell): Cell => ({
  ...c,
  badge: c.badge ?? undefined,
  source: c.source ?? undefined,
})

const toRow = (row: AdviceRow) => ({
  sentence: text(row.sentence),
  account: text(row.account),
  ref_value: row.ref_value,
  after_value: row.after_value,
  delta: row.delta,
  acted: text(row.acted),
})

export function AdvicePage({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="advice">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground">{payload.sentence}</p>
      </header>
      {payload.gap ? (
        <Alert>
          <TriangleAlert />
          <AlertDescription>{payload.gap.text}</AlertDescription>
        </Alert>
      ) : null}
      {payload.rows.length > 0 ? (
        <Card className="min-w-0">
          <CardHeader>
            <CardTitle>{payload.title}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <DataTable columns={COLUMNS} rows={payload.rows.map(toRow)} />
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}
