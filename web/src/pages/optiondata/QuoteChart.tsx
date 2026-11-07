// The bid, ask and last lines of the quote series by slot; a gap point is a hole in the line. Tabs switch series.
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { Payload } from '@/types/option_data_page'

type Series = Payload['quote_series'][number]
type PayloadCell = Series['points'][number]['bid']

const CONFIG: ChartConfig = {
  bid: { label: 'Bid', color: 'var(--chart-1)' },
  ask: { label: 'Ask', color: 'var(--chart-2)' },
  last: { label: 'Last', color: 'var(--chart-3)' },
}

const valueOf = (cell: PayloadCell) => (cell.kind === 'value' ? cell.value : null)

function SeriesChart({ series }: { series: Series }) {
  const data = series.points.map((point) => ({
    slot: point.slot,
    bid: valueOf(point.bid),
    ask: valueOf(point.ask),
    last: valueOf(point.last),
  }))
  return (
    <ChartContainer config={CONFIG} className="aspect-[2/1] w-full min-h-56">
      <LineChart data={data}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="slot" />
        <YAxis />
        <ChartTooltip content={<ChartTooltipContent />} />
        {Object.keys(CONFIG).map((key) => (
          <Line
            key={key}
            dataKey={key}
            stroke={`var(--color-${key})`}
            dot={false}
            connectNulls={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ChartContainer>
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
