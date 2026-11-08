// Tracking page v2: grouped by rule version. Python sends the versions, groups, tiles, rows and every string; the
// view prints them in payload order and computes, averages and compares nothing.
import { ClipboardList, Info } from 'lucide-react'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/tracking'
import { StaleAlert } from '@/pages/home/StaleAlert'
export { validate as validateTracking } from '@/types/tracking'
export type { Payload as TrackingPayload } from '@/types/tracking'

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

type Version = Payload['versions'][number]
type Group = Version['groups'][number]
type Row = Group['rows'][number]
// The generated `rows` types of the two string tables are a union of flat tuples (a generator quirk); on the wire a
// row is a list of strings, so the view reads them as a list of rows.
type StringTable = { columns: readonly string[]; rows: readonly unknown[] }

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

function StringsTable({ table }: { table: StringTable }) {
  const columns: Column[] = table.columns.map((label, i) => ({ id: `c${i}`, label, wrap: true }))
  const rows = (table.rows as readonly (readonly string[])[]).map((row) =>
    Object.fromEntries(row.map((cell, i) => [`c${i}`, text(cell)])),
  )
  return <DataTable columns={columns} rows={rows} />
}

function GroupBlock({ group }: { group: Group }) {
  return (
    <Card className="min-w-0" data-tracking-group={group.key}>
      <CardHeader>
        <CardTitle>{group.title}</CardTitle>
        {group.note ? <p className="text-sm text-muted-foreground">{group.note}</p> : null}
      </CardHeader>
      <CardContent className="min-w-0 space-y-3">
        {Array.isArray(group.summary) ? (
          <div className="flex flex-wrap gap-2">
            {group.summary.map((tile) => (
              <Badge key={tile.label} variant="outline" className="h-auto gap-2 px-3 py-1">
                <span className="text-muted-foreground">{tile.label}</span>
                <span className="font-semibold">{tile.text}</span>
              </Badge>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">{group.summary.text}</p>
        )}
        <Accordion type="single" collapsible>
          <AccordionItem value="rows">
            <AccordionTrigger>评估明细</AccordionTrigger>
            <AccordionContent>
              <DataTable columns={COLUMNS} rows={group.rows.map(toCells)} />
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </CardContent>
    </Card>
  )
}

function VersionBody({ version }: { version: Version }) {
  const calls = version.covered_calls
  const mine = version.my_trades
  return (
    <div className="min-w-0 space-y-4">
      {version.groups.map((group) => (
        <GroupBlock key={group.key} group={group} />
      ))}
      {calls ? (
        <Card className="min-w-0" data-tracking-covered-calls>
          <CardHeader>
            <CardTitle>{calls.title}</CardTitle>
            <p className="text-sm text-muted-foreground">{calls.sub}</p>
          </CardHeader>
          <CardContent className="min-w-0 space-y-4">
            {calls.groups.map((group) => (
              <GroupBlock key={group.key} group={group} />
            ))}
            <StringsTable table={calls.comparison} />
            {calls.note ? <p className="text-sm text-muted-foreground">{calls.note}</p> : null}
          </CardContent>
        </Card>
      ) : null}
      <Card className="min-w-0" data-tracking-my-trades>
        {'columns' in mine ? (
          <>
            <CardHeader>
              <CardTitle>{mine.title}</CardTitle>
              <p className="text-sm text-muted-foreground">{mine.sub}</p>
            </CardHeader>
            <CardContent className="min-w-0 space-y-2">
              <StringsTable table={mine} />
              {mine.note ? <p className="text-sm text-muted-foreground">{mine.note}</p> : null}
            </CardContent>
          </>
        ) : (
          <CardContent>
            <p className="text-sm text-muted-foreground">{mine.text}</p>
          </CardContent>
        )}
      </Card>
    </div>
  )
}

export function Tracking({ payload }: { payload: Payload }) {
  const [current, ...older] = payload.versions
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
      <section className="min-w-0 space-y-4" data-tracking-version="current">
        <div className="flex flex-wrap items-center gap-2">
          <ClipboardList aria-hidden className="size-5" />
          <h2 className="text-lg font-semibold">{current.label}</h2>
          <Badge variant="secondary">{current.expired_text}</Badge>
        </div>
        <VersionBody version={current} />
      </section>
      {older.length > 0 ? (
        <Accordion type="single" collapsible>
          <AccordionItem value="older">
            <AccordionTrigger>更早的规则版本</AccordionTrigger>
            <AccordionContent className="space-y-6">
              {older.map((version) => (
                <section key={version.label} className="min-w-0 space-y-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="font-semibold">{version.label}</h3>
                    <Badge variant="secondary">{version.expired_text}</Badge>
                  </div>
                  <VersionBody version={version} />
                </section>
              ))}
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : null}
    </div>
  )
}

export const registration = {
  schema: 'tracking',
  page: page<Payload>(validate, Tracking),
}
