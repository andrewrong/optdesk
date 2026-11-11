// The bid, ask and last lines of the quote series by slot; a gap point is a hole in the line, never a bridge and
// never zero. Tabs switch series; `Chart` draws each one.
import { Chart } from '@/charts/Chart'
import type { ChartPoint, ChartSpec } from '@/charts/spec'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { Payload } from '@/types/option_data_page'

type Series = Payload['quote_series'][number]
type PayloadCell = Series['points'][number]['bid']

const NAMES = { bid: 'Bid', ask: 'Ask', last: 'Last' } as const

const pointOf = (cell: PayloadCell): ChartPoint =>
  cell.kind === 'value' ? { value: cell.value, text: cell.text } : { value: null }

function specOf(series: Series): ChartSpec {
  return {
    kind: 'line',
    legend: true,
    categories: series.points.map((point) => point.slot),
    series: (['bid', 'ask', 'last'] as const).map((key) => ({
      name: NAMES[key],
      points: series.points.map((point) => pointOf(point[key])),
      hole: true,
    })),
  }
}

function SeriesChart({ series }: { series: Series }) {
  return (
    <div data-slots={series.points.map((point) => point.slot).join(',')}>
      <Chart spec={specOf(series)} ariaText={series.caption} name="option-quote-series" />
    </div>
  )
}

export function QuoteChart({ series }: { series: Series[] }) {
  const first = series[0]
  if (first === undefined) return null
  return (
    <Tabs defaultValue={String(first.instrument_id)}>
      <TabsList>
        {series.map((one) => (
          <TabsTrigger key={one.instrument_id} value={String(one.instrument_id)}>
            {one.caption}
          </TabsTrigger>
        ))}
      </TabsList>
      {series.map((one) => (
        <TabsContent key={one.instrument_id} value={String(one.instrument_id)}>
          <SeriesChart series={one} />
        </TabsContent>
      ))}
    </Tabs>
  )
}
