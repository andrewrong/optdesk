// The report of the strategy replay page: counts, excluded rows, one table row per line with its lot count, the notes.
// Every text and figure is a payload field; a null figure shows n/a and nothing is computed here.
import type { Line, Report } from '@/generated/strategy_replay_page.v1'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const fig = (v: string | null) => v ?? 'n/a'

const COUNTS = ['lots', 'lots_with_quote', 'still_open', 'lot_days', 'quote_days'] as const

type ColumnKey = keyof NonNullable<Report['hints']>['columns']

// [hint key, English word, right aligned]
const COLUMNS: [ColumnKey, string, boolean][] = [
  ['set', 'set', false],
  ['variant', 'variant', false],
  ['currency', 'ccy', false],
  ['lots', 'lots', true],
  ['cycles', 'cycles', true],
  ['closed_early', 'closed early', true],
  ['held', 'held', true],
  ['assigned', 'assigned', true],
  ['rolled', 'rolled', true],
  ['in_the_money', 'ITM', true],
  ['kept_share', 'kept share', true],
  ['mean_gross', 'mean gross', true],
  ['worst_gross', 'worst gross', true],
  ['hold_kept_share', 'hold kept share', true],
  ['actual_kept_share', 'actual kept share', true],
]

// The payload's Chinese hint, muted; nothing when the payload has none.
function Hint({ text, block }: { text?: string; block?: boolean }) {
  if (!text) return null
  return <span className={block ? 'block font-normal text-muted-foreground' : 'ml-1 text-muted-foreground'}>{text}</span>
}

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
          {COUNTS.map((k, i) => (
            <span key={k}>
              {i > 0 ? ' · ' : ''}
              {`${k} ${report.counts[k]}`}
              <Hint text={report.hints?.counts[k]} />
            </span>
          ))}
        </p>
        <p className="text-sm text-muted-foreground">
          {report.variants.map((v, i) => (
            <span key={v.name}>
              {i > 0 ? ' · ' : ''}
              {`${v.name} ${v.params_hash}`}
              <Hint text={v.hint} />
            </span>
          ))}
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
                {COLUMNS.map(([key, word, right]) => (
                  <TableHead key={key} className={right ? 'text-right' : undefined}>
                    {word}
                    <Hint text={report.hints?.columns[key]} block />
                  </TableHead>
                ))}
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
