// The price band: recharts reference lines and one reference area over an axis that runs from `band.min` to
// `band.max`. The position of a mark is `band.marks[].pos` as the builder wrote it; nothing is computed here.
import { ComposedChart, ReferenceArea, ReferenceLine, Scatter, XAxis, YAxis } from 'recharts'
import { ChartContainer, type ChartConfig } from '@/components/ui/chart'
import type { Payload } from '@/types/valuation_page'

type Band = NonNullable<Payload['band']>

const CONFIG: ChartConfig = {}

// recharts 3 draws a ReferenceLine or ReferenceArea only when the chart has a graphical series. The series is a
// Scatter over the marks of the band, one point per mark; its shape renders nothing, so no dot reads as a price.
const NO_DOT = () => <g />

export function PriceBand({ band }: { band: Band }) {
  // A band without marks draws no chart; the page states why a line or zone is missing from `lines.zone_note`.
  if (band.marks.length === 0) return null
  const points = band.marks.map((m) => ({ pos: Number(m.pos) }))
  const fairLow = band.marks.find((m) => m.name === 'fair_low')
  const fairHigh = band.marks.find((m) => m.name === 'fair_high')
  return (
    <div data-chart="price-band" className="space-y-2">
      <div role="img" aria-label={`Price band from ${band.min} to ${band.max}`} className="h-[120px] w-full">
        <ChartContainer config={CONFIG} className="h-full w-full">
          <ComposedChart margin={{ left: 16, right: 16, top: 24, bottom: 8 }}>
            <XAxis
              type="number"
              dataKey="pos"
              domain={[0, 1]}
              ticks={[0, 1]}
              tickFormatter={(t: number) => (t === 0 ? band.min : band.max)}
              tickLine={false}
            />
            <YAxis hide type="number" dataKey="pos" domain={[0, 1]} />
            <Scatter data={points} dataKey="pos" shape={NO_DOT} isAnimationActive={false} legendType="none" />
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
