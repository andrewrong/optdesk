// One section of the ledger: lead, strategies table, open table, totals note and the folded More lines.
import { DataTable } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import type { Section } from '@/generated/ledger_page.v1'
import { columnsOf, figure, rowOf, withDetail } from './cells'

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
      {section.chart ? <p className="text-sm">{section.chart.summary}</p> : null}
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
