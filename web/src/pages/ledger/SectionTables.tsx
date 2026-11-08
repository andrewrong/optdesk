// One section of the ledger: lead, strategies table, open table, totals note and the folded More lines.
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import { DataTable } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import type { ChartPoint, Section } from '@/generated/ledger_page.v1'
import { columnsOf, figure, rowOf, withDetail } from './cells'

// The cumulative realized line: category the day, value the number, tooltip the text, all from the payload.
function cumulativeLine(points: ChartPoint[]): ChartSpec {
  return {
    kind: 'line',
    categories: points.map((point) => point.day),
    series: [{ name: 'Realized', points: points.map((point) => ({ value: point.value, text: point.text })) }],
  }
}

export function SectionTables({ section }: { section: Section }) {
  const strategyRows = section.strategies.map((strategy) => rowOf(withDetail(strategy.cells, strategy.lines)))
  const closedRows = section.strategies.flatMap((strategy) =>
    strategy.closed.map((closed) => rowOf(withDetail(closed.cells, closed.detail))),
  )
  const openRows = section.open.map((row) =>
    rowOf([
      { text: row.label, note: [row.small, ...row.detail].filter(Boolean).join(' · ') },
      figure(row.premium),
    ]),
  )
  return (
    <div className="space-y-4" data-section={section.key}>
      {section.lead ? (
        <p>
          {section.lead.caption}
          {section.lead.total ? <> {figure(section.lead.total).text}</> : null}
        </p>
      ) : null}
      {section.totals_differ ? (
        <Alert data-totals-differ>
          <AlertDescription>{section.totals_differ}</AlertDescription>
        </Alert>
      ) : null}
      {section.chart ? (
        <div className="space-y-1" data-points={section.chart.points.length}>
          <Chart spec={cumulativeLine(section.chart.points)} ariaText={section.chart.summary} name="ledger-cumulative" />
          <p className="text-sm">{section.chart.summary}</p>
        </div>
      ) : null}
      {strategyRows.length > 0 ? (
        <DataTable columns={columnsOf(section.strategy_columns, 4)} rows={strategyRows} />
      ) : null}
      {closedRows.length > 0 ? (
        <DataTable columns={columnsOf([], 3)} rows={closedRows} />
      ) : null}
      {openRows.length > 0 ? (
        <div>
          {section.open_title ? <h4 className="font-medium">{section.open_title}</h4> : null}
          <DataTable columns={columnsOf(section.open_columns, 2)} rows={openRows} />
        </div>
      ) : null}
      {section.more.map((line, index) => (
        <p key={`${index}-${line}`} className="text-xs text-muted-foreground">
          {line}
        </p>
      ))}
    </div>
  )
}
