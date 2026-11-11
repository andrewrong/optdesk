// The household risk page (household_risk v1), read-only. Every visible string and figure is a field of the payload;
// this file formats, compares and computes nothing. `value` is used only by the table's user-started sort.
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { CardNotes } from '@/pages/home/CardNotes'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/household_risk'

type Money = Payload['delta']['total']
type GapField = Payload['delta']['gap']

const moneyCell = (money: NonNullable<Money>): Cell => ({
  text: money.text,
  value: money.value,
  tone: money.tone,
  approx_usd: money.approx_usd ? { text: money.approx_usd.text } : null,
})
const figure = (text: string, value: number | string): Cell => ({ text, value })
const labelColumn = (id: string, label: string, align?: 'right'): Column => ({ id, label, align })

function Block({
  title,
  meaning,
  gap,
  total,
  children,
  name,
  ariaText,
}: {
  title: string
  meaning: string
  gap: GapField
  total?: string | null
  children: React.ReactNode
  name: string
  ariaText?: string
}) {
  return (
    <Card data-block={name}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{meaning}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2" role={ariaText ? 'group' : undefined} aria-label={ariaText}>
        {gap ? (
          <Alert data-gap>
            <AlertDescription>{gap.text}</AlertDescription>
          </Alert>
        ) : (
          children
        )}
        {total ? (
          <Badge variant="secondary" data-total>
            {total}
          </Badge>
        ) : null}
      </CardContent>
    </Card>
  )
}

function DeltaBlock({ block }: { block: Payload['delta'] }) {
  const [name, delta] = block.heads
  const rows = block.rows.map((row) => ({
    name: figure(row.name, row.name),
    delta: moneyCell(row.delta),
  }))
  return (
    <Block name="delta" title={block.title} meaning={block.meaning} gap={block.gap} total={block.total?.text}>
      <DataTable columns={[labelColumn('name', name), labelColumn('delta', delta, 'right')]} rows={rows} />
    </Block>
  )
}

function BetaBlock({ block }: { block: Payload['beta_delta'] }) {
  const [name, beta, weighted] = block.heads
  const rows = block.rows.map((row) => ({
    name: figure(row.name, row.name),
    beta: figure(row.beta.text, row.beta.value),
    weighted: moneyCell(row.weighted_delta),
  }))
  return (
    <Block name="beta-delta" title={block.title} meaning={block.meaning} gap={block.gap} total={block.total?.text}>
      <p className="text-xs text-muted-foreground" data-benchmark>
        {block.benchmark_text}
      </p>
      <DataTable
        columns={[labelColumn('name', name), labelColumn('beta', beta, 'right'), labelColumn('weighted', weighted, 'right')]}
        rows={rows}
      />
    </Block>
  )
}

function SigmaBlock({ block }: { block: Payload['sigma'] }) {
  const columns = [
    labelColumn('name', ''),
    ...block.columns.map((label, index) => labelColumn(`c${index}`, label, 'right')),
  ]
  const rowOf = (row: Payload['sigma']['rows'][number]) => ({
    name: figure(row.name, row.name),
    ...Object.fromEntries(row.cells.map((cell, index) => [`c${index}`, moneyCell(cell)])),
  })
  const rows = [...block.rows, ...(block.total ? [block.total] : [])].map(rowOf)
  return (
    <Block name="sigma" title={block.title} meaning={block.meaning} gap={block.gap} ariaText={block.aria_text}>
      <DataTable columns={columns} rows={rows} />
      {block.scale_low || block.scale_high ? (
        <p className="flex justify-between text-xs text-muted-foreground" data-scale>
          <span>{block.scale_low?.text}</span>
          <span>{block.scale_high?.text}</span>
        </p>
      ) : null}
    </Block>
  )
}

function VegaBlock({ title, block }: { title: string; block: Payload['vega'] }) {
  const [expiry, vega] = block.heads
  const rows = block.rows.map((row) => ({
    expiry: figure(row.expiry_text, row.expiry_text),
    vega: figure(row.vega.text, row.vega.value),
  }))
  return (
    <Block name="vega" title={title} meaning={block.meaning} gap={block.gap} total={block.total?.text}>
      <DataTable columns={[labelColumn('expiry', expiry), labelColumn('vega', vega, 'right')]} rows={rows} />
    </Block>
  )
}

function MetricsBlock({ block }: { block: Payload['standard_metrics'] }) {
  return (
    <Card data-block="standard-metrics">
      <CardHeader>
        <CardTitle>{block.title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {block.metrics.map((metric) => (
          <section key={metric.label} className="space-y-1" data-metric>
            <h3 className="text-sm font-medium">
              {metric.label} <Badge variant="secondary">{metric.value_text}</Badge>
            </h3>
            <p className="text-xs text-muted-foreground">{metric.explanation}</p>
            {metric.table ? (
              <DataTable
                columns={metric.table.columns.map((label, index) => labelColumn(`c${index}`, label))}
                rows={metric.table.rows.map((row) =>
                  Object.fromEntries(row.cells.map((text, index) => [`c${index}`, figure(text, text)])),
                )}
              />
            ) : null}
          </section>
        ))}
      </CardContent>
    </Card>
  )
}

export function HouseholdRiskView({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-4" data-page="household-risk">
      <Card>
        <CardHeader>
          <CardTitle>{payload.title}</CardTitle>
          <CardDescription data-sentence>{payload.sentence}</CardDescription>
        </CardHeader>
        <CardContent>
          <Alert data-stale>
            <AlertDescription>{payload.stale.text}</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="min-w-0" data-cell="delta">
          <DeltaBlock block={payload.delta} />
        </div>
        <div className="min-w-0" data-cell="vega">
          <VegaBlock title={payload.vega_title} block={payload.vega} />
        </div>
        <div className="min-w-0" data-cell="beta">
          <BetaBlock block={payload.beta_delta} />
        </div>
        <div className="min-w-0" data-cell="sigma">
          <SigmaBlock block={payload.sigma} />
        </div>
        <div className="min-w-0 lg:col-span-2" data-cell="standard-metrics">
          <MetricsBlock block={payload.standard_metrics} />
        </div>
      </div>
      <CardNotes notes={[...payload.notes]} gaps={[...payload.gaps]} />
    </div>
  )
}

export const registration = {
  schema: 'household_risk',
  page: page<Payload>(validate, HouseholdRiskView),
}
