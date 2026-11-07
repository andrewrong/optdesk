// The macro rates page: every string is a field of the payload and every figure is the text the builder wrote;
// this file computes nothing. The line is drawn from the numbers, the tooltip and the table read the texts.
import { Info, TriangleAlert } from 'lucide-react'
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import { DataTable, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/macro_rates'

type Series = Payload['series'][number]

const COLUMNS: Column[] = [
  { id: 'date', label: 'date' },
  { id: 'value', label: 'value', align: 'right' },
]

// The points by `date`; a null value stays null (the line joins its neighbours, it is never a zero).
export function chartSpecOf(series: Series): ChartSpec {
  return {
    kind: 'line',
    categories: series.points.map((point) => point.date),
    series: [
      {
        name: series.title,
        points: series.points.map((point) => ({ value: point.value, text: point.text })),
      },
    ],
  }
}

function SeriesCard({ series }: { series: Series }) {
  const rows = series.points.map((point) => ({
    date: { text: point.date, value: point.date },
    value: { text: point.text, value: point.value },
  }))
  return (
    <Card data-series={series.id}>
      <CardHeader>
        <CardTitle>{series.title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {series.latest.kind === 'value' ? (
          <p data-latest className="text-2xl font-semibold">
            {series.latest.text}
          </p>
        ) : (
          <Alert data-latest-gap={series.latest.reason}>
            <Info aria-hidden />
            <AlertDescription>{series.latest.text}</AlertDescription>
          </Alert>
        )}
        <Chart spec={chartSpecOf(series)} ariaText={series.title} name={series.id} />
        <DataTable columns={COLUMNS} rows={rows} />
      </CardContent>
    </Card>
  )
}

export function MacroRatesView({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="macro-rates">
      <Card>
        <CardHeader>
          <CardTitle>{payload.title}</CardTitle>
          <p className="text-muted-foreground">{payload.sentence}</p>
        </CardHeader>
      </Card>
      {payload.gaps.map((gap, index) => (
        <Alert key={`${index}-${gap.text}`} data-gap={gap.reason}>
          <TriangleAlert aria-hidden />
          <AlertDescription>
            <Badge variant="outline">{gap.reason}</Badge> {gap.text}
          </AlertDescription>
        </Alert>
      ))}
      {payload.series.map((series) => (
        <SeriesCard key={series.id} series={series} />
      ))}
    </div>
  )
}

export const registration = {
  schema: 'macro_rates',
  page: page<Payload>(validate, MacroRatesView),
}
