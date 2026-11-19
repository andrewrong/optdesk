// The report of the strategy replay page: counts, excluded rows, one table row per line with its lot count, the notes.
// Every text and figure is a payload field; a null figure shows n/a and nothing is computed here.
import type { Line, Report } from '@/generated/strategy_replay_page.v1'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const fig = (v: string | null) => v ?? 'n/a'

const COUNTS = ['lots', 'lots_with_quote', 'still_open', 'lot_days', 'quote_days'] as const

function LineRow({ line }: { line: Line }) {
  return (
    <TableRow data-set={line.set} data-variant={line.variant}>
      <TableCell>{line.set}</TableCell>
      <TableCell>{line.variant}</TableCell>
      <TableCell>{line.currency}</TableCell>
      <TableCell className="text-right">{line.lots}</TableCell>
      <TableCell className="text-right">{line.cycles}</TableCell>
      <TableCell className="text-right">{line.closed_early}</TableCell>
      <TableCell className="text-right">{line.held}</TableCell>
      <TableCell className="text-right">{line.assigned}</TableCell>
      <TableCell className="text-right">{line.rolled}</TableCell>
      <TableCell className="text-right">{line.in_the_money}</TableCell>
      <TableCell className="text-right">{fig(line.kept_share)}</TableCell>
      <TableCell className="text-right">{fig(line.mean_gross)}</TableCell>
      <TableCell className="text-right">{fig(line.worst_gross)}</TableCell>
      <TableCell className="text-right">{fig(line.hold_kept_share)}</TableCell>
      <TableCell className="text-right">{fig(line.actual_kept_share)}</TableCell>
      <TableCell>
        {line.minimum_text ? <Badge variant="secondary">{line.minimum_text}</Badge> : null}
      </TableCell>
    </TableRow>
  )
}

export function ReplayReport({ report }: { report: Report }) {
  return (
    <Card data-section="report">
      <CardHeader>
        <CardTitle>
          {report.lots_label} · {report.as_of}
          {report.first_quote_day ? ` · ${report.first_quote_day}` : ''}
          {report.last_quote_day ? ` – ${report.last_quote_day}` : ''}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm">
          {COUNTS.map((k) => `${k} ${report.counts[k]}`).join(' · ')}
        </p>
        <p className="text-sm text-muted-foreground">
          {report.variants.map((v) => `${v.name} ${v.params_hash}`).join(' · ')}
        </p>
        {report.excluded.map((x) => (
          <p key={x.reason} className="text-sm" data-excluded={x.reason}>
            {x.text} {x.lots}
          </p>
        ))}
        {report.lines.length === 0 ? (
          report.empty_text ? <p className="text-sm">{report.empty_text}</p> : null
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>set</TableHead>
                <TableHead>variant</TableHead>
                <TableHead>ccy</TableHead>
                <TableHead className="text-right">lots</TableHead>
                <TableHead className="text-right">cycles</TableHead>
                <TableHead className="text-right">closed early</TableHead>
                <TableHead className="text-right">held</TableHead>
                <TableHead className="text-right">assigned</TableHead>
                <TableHead className="text-right">rolled</TableHead>
                <TableHead className="text-right">ITM</TableHead>
                <TableHead className="text-right">kept share</TableHead>
                <TableHead className="text-right">mean gross</TableHead>
                <TableHead className="text-right">worst gross</TableHead>
                <TableHead className="text-right">hold kept share</TableHead>
                <TableHead className="text-right">actual kept share</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {report.lines.map((l) => (
                <LineRow key={`${l.set}-${l.variant}-${l.currency}`} line={l} />
              ))}
            </TableBody>
          </Table>
        )}
        {report.notes.map((n) => (
          <p key={n} className="text-xs text-muted-foreground">
            {n}
          </p>
        ))}
      </CardContent>
    </Card>
  )
}
