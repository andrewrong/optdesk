// The price band: recharts reference lines and one reference area over an axis that runs from `band.min` to
// `band.max`. The position of a mark is `band.marks[].pos` as the builder wrote it; nothing is computed here.
import { ComposedChart, ReferenceArea, ReferenceLine, XAxis, YAxis } from 'recharts'
import { ChartContainer, type ChartConfig } from '@/components/ui/chart'
import type { Payload } from '@/types/valuation_page'

type Band = NonNullable<Payload['band']>

const CONFIG: ChartConfig = {}

// The two end points of the axis: without a data point recharts draws no plot area for the marks.
const AXIS_POINTS = [{ pos: 0 }, { pos: 1 }]

export function PriceBand({ band }: { band: Band }) {
  const fairLow = band.marks.find((m) => m.name === 'fair_low')
  const fairHigh = band.marks.find((m) => m.name === 'fair_high')
  return (
    <div data-chart="price-band" className="space-y-2">
      <div role="img" aria-label={`Price band from ${band.min} to ${band.max}`} className="h-[120px] w-full">
        <ChartContainer config={CONFIG} className="h-full w-full">
          <ComposedChart data={AXIS_POINTS} margin={{ left: 16, right: 16, top: 24, bottom: 8 }}>
            <XAxis
              type="number"
              dataKey="pos"
              domain={[0, 1]}
              ticks={[0, 1]}
              tickFormatter={(t: number) => (t === 0 ? band.min : band.max)}
              tickLine={false}
            />
            <YAxis hide domain={[0, 1]} />
            {fairLow && fairHigh ? (
              <ReferenceArea x1={Number(fairLow.pos)} x2={Number(fairHigh.pos)} fill="var(--chart-2)" fillOpacity={0.15} />
            ) : null}
            {band.marks.map((m) => (
              <ReferenceLine
                key={m.name}
                x={Number(m.pos)}
                stroke={m.name === 'close' ? 'var(--foreground)' : 'var(--muted-foreground)'}
                strokeDasharray={m.name === 'close' ? undefined : '4 4'}
                label={{ value: m.name, position: 'top', fontSize: 11 }}
                ifOverflow="visible"
              />
            ))}
          </ComposedChart>
        </ChartContainer>
      </div>
      <ul className="flex flex-wrap gap-x-4 text-sm text-muted-foreground">
        {band.marks.map((m) => (
          <li key={m.name}>
            {m.name} {m.value}
          </li>
        ))}
      </ul>
    </div>
  )
}
