// The indicator detail page (indicator_detail v2). Every visible string and figure is a field of the payload; this
// file formats, compares and computes nothing. The reference line is drawn as a second series of the same value.
import { Chart } from '@/charts/Chart'
import { DataTable, type Column } from '@/components/data-table/data-table'
import type { ChartSpec } from '@/charts/spec'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/indicator_detail'

function specOf(payload: Payload): ChartSpec {
  const series = [
    { name: payload.series_title, points: payload.points.map((point) => ({ value: point.value, text: point.text })) },
  ]
  const line = payload.reference_line
  if (line) {
    series.push({
      name: line.label,
      points: payload.points.map(() => ({ value: line.value.value, text: line.value.text })),
    })
  }
  return {
    kind: 'line',
    categories: payload.points.map((point) => ('day' in point.released ? point.released.day : point.observed_on)),
    series,
    legend: line !== null,
  }
}

function Recent({ recent }: { recent: Payload['recent'] }) {
  if ('reason' in recent) return <GapLine text={recent.text} reason={recent.reason} />
  const [period, released, value, change] = recent.columns
  const columns: Column[] = [
    { id: 'period', label: period },
    { id: 'released', label: released },
    { id: 'value', label: value },
    { id: 'change', label: change },
  ]
  return (
    <section data-recent>
      <h3 className="font-medium">{recent.title}</h3>
      <DataTable
        columns={columns}
        rows={recent.rows.map((row) => ({
          period: { text: row.period },
          released: { text: row.released },
          value: { text: row.value },
          change: { text: row.change },
        }))}
      />
    </section>
  )
}

function GapLine({ text, reason }: { text: string; reason: string }) {
  return (
    <Alert data-gap={reason}>
      <AlertDescription>{text}</AlertDescription>
    </Alert>
  )
}

function Latest({ latest }: { latest: Payload['latest'] }) {
  if ('reason' in latest) return <GapLine text={latest.text} reason={latest.reason} />
  const { value, observed, released } = latest
  return (
    <div data-latest className="flex flex-col gap-1">
      <p className="text-2xl font-semibold" data-latest-value>
        {value.text}
      </p>
      <p className="text-sm text-muted-foreground">{observed.text}</p>
      {'reason' in released ? (
        <GapLine text={released.text} reason={released.reason} />
      ) : (
        <p className="text-sm text-muted-foreground">{released.text}</p>
      )}
    </div>
  )
}

export function IndicatorDetailView({ payload }: { payload: Payload }) {
  const { revision } = payload
  return (
    <Card data-page="indicator_detail">
      <CardHeader>
        <CardTitle>{payload.title}</CardTitle>
        <CardDescription>{payload.sentence}</CardDescription>
        <Badge variant="outline" data-stale-label>
          {payload.stale.text}
        </Badge>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <Latest latest={payload.latest} />
        {payload.points.length > 0 ? (
          <Chart spec={specOf(payload)} ariaText={payload.series_title} name="indicator-detail" />
        ) : null}
        <p className="text-sm text-muted-foreground" data-meta-line>
          {payload.meta_line}
        </p>
        <Recent recent={payload.recent} />
        {revision ? (
          <p className="text-sm text-muted-foreground" data-revision>
            {revision.text}
          </p>
        ) : null}
        {payload.explanation.length > 0 ? (
          <section data-explanation>
            <h3 className="font-medium">{payload.explanation_title}</h3>
            <ul className="list-disc pl-5 text-sm">
              {payload.explanation.map((line, index) => (
                <li key={index}>{line}</li>
              ))}
            </ul>
          </section>
        ) : null}
        {payload.gaps.map((gap, index) => (
          <GapLine key={index} text={gap.text} reason={gap.reason} />
        ))}
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'indicator_detail',
  page: page<Payload>(validate, IndicatorDetailView),
}
