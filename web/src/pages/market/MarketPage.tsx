// Board 7, the market page: the weather sentence and chart, the index table, the VIX term block and the breadth
// table. Every text is a field of the payload; nothing is compared, summed or formatted here. A missing figure is
// a Gap with its cause (Decisions 2 and 3 of docs/design/missing-pages.md).
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { FigureOrGap, Gap } from '@/generated/market_page.v1'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/market_page'

type Weather = Payload['weather']

const cellOf = (figure: FigureOrGap): Cell =>
  figure.kind === 'gap'
    ? { reason: figure.reason, text: figure.text }
    : { text: figure.text, value: figure.value }

const columnsOf = (labels: string[], count: number): Column[] =>
  Array.from({ length: count }, (_, index) => ({
    id: `c${index}`,
    label: labels[index] ?? '',
    align: index === 0 ? 'left' : 'right',
  }))

const rowOf = (cells: Cell[]): Record<string, Cell> =>
  Object.fromEntries(cells.map((cell, index) => [`c${index}`, cell]))

// A missing block: the producer's text, the reason in data-gap.
function MissingBlock({ gap }: { gap: Gap }) {
  return (
    <Alert data-gap={gap.reason}>
      <AlertDescription>{gap.text}</AlertDescription>
    </Alert>
  )
}

function closeLine(chart: Exclude<Weather['chart'], Gap>): ChartSpec {
  return {
    kind: 'line',
    categories: chart.points.map((point) => point.label),
    legend: true,
    series: [
      { name: chart.series_names[0], points: chart.points.map((point) => ({ value: point.close })) },
      ...(chart.series_names[1]
        ? [{ name: chart.series_names[1], points: chart.points.map((point) => ({ value: point.ma200 })) }]
        : []),
    ],
  }
}

function WeatherBlock({ weather }: { weather: Weather }) {
  const { chart } = weather
  return (
    <Card data-block="weather">
      <CardHeader>
        <CardTitle>
          <Badge>{weather.headline}</Badge>
        </CardTitle>
        <CardDescription>{weather.sentence}</CardDescription>
      </CardHeader>
      <CardContent>
        {'kind' in chart ? (
          <MissingBlock gap={chart} />
        ) : (
          <Chart spec={closeLine(chart)} ariaText={chart.aria_text} name="market-weather" />
        )}
      </CardContent>
    </Card>
  )
}

function IndicesBlock({ indices }: { indices: Payload['indices'] }) {
  const rows = indices.rows.map((row) =>
    rowOf([{ text: row.name }, cellOf(row.iv), cellOf(row.rv20), cellOf(row.iv_minus_rv20), cellOf(row.rank_3y)]),
  )
  return (
    <Card data-block="indices">
      <CardHeader>
        <CardTitle>{indices.title}</CardTitle>
        <CardDescription>{indices.sentence}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <DataTable columns={columnsOf(indices.columns, 5)} rows={rows} />
        <p className="text-xs text-muted-foreground">{indices.note}</p>
      </CardContent>
    </Card>
  )
}

function termLines(term: Exclude<Payload['term'], Gap>): ChartSpec {
  const series = (pick: (point: (typeof term.points)[number]) => FigureOrGap, name: string) => ({
    name,
    hole: true,
    points: term.points.map((point) => {
      const figure = pick(point)
      return figure.kind === 'gap' ? { value: null, text: figure.text } : { value: figure.value, text: figure.text }
    }),
  })
  return {
    kind: 'line',
    categories: term.points.map((point) => point.label),
    legend: true,
    series: [series((point) => point.today, 'today'), series((point) => point.five_days_ago, '5d ago')],
  }
}

function TermBlock({ term }: { term: Payload['term'] }) {
  if ('kind' in term) {
    return (
      <Card data-block="term">
        <CardContent>
          <MissingBlock gap={term} />
        </CardContent>
      </Card>
    )
  }
  return (
    <Card data-block="term">
      <CardHeader>
        <CardTitle>{term.title}</CardTitle>
        <CardDescription>{term.sentence}</CardDescription>
      </CardHeader>
      <CardContent>
        <Chart spec={termLines(term)} ariaText={term.sentence} name="market-term" />
      </CardContent>
    </Card>
  )
}

function BreadthBlock({ breadth }: { breadth: Payload['breadth'] }) {
  if ('kind' in breadth) {
    return (
      <Card data-block="breadth">
        <CardContent>
          <MissingBlock gap={breadth} />
        </CardContent>
      </Card>
    )
  }
  const rows = breadth.rows.map((row) => rowOf([{ text: row.label }, cellOf(row.figure)]))
  return (
    <Card data-block="breadth">
      <CardHeader>
        <CardTitle>{breadth.title}</CardTitle>
      </CardHeader>
      <CardContent>
        <DataTable columns={columnsOf([], 2)} rows={rows} />
      </CardContent>
    </Card>
  )
}

export function MarketPage({ payload }: { payload: Payload }) {
  const { stale } = payload
  return (
    <div className="flex min-w-0 flex-col gap-6" data-page="market-page">
      <header>
        <h1 className="text-xl font-semibold">{payload.title}</h1>
        <p className="text-sm text-muted-foreground">{payload.sentence}</p>
      </header>
      {stale ? (
        <StaleAlert
          stale={{ text: stale.text, stale_after: stale.is_stale ? stale.data_time : (payload.stale_after ?? '') }}
          generatedAt={payload.generated_at}
        />
      ) : null}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="min-w-0 lg:col-span-2" data-cell="weather">
          <WeatherBlock weather={payload.weather} />
        </div>
        <div className="min-w-0" data-cell="indices">
          <IndicesBlock indices={payload.indices} />
        </div>
        <div className="min-w-0" data-cell="term">
          <TermBlock term={payload.term} />
        </div>
        <div className="min-w-0 lg:col-span-2" data-cell="breadth">
          <BreadthBlock breadth={payload.breadth} />
        </div>
      </div>
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
    </div>
  )
}

export const registration = { schema: 'market_page', page: page<Payload>(validate, MarketPage) }
