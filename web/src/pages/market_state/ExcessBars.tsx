// The 63-session gaps as bars, one per row whose figure is a value; a Gap row has no bar. The texts are the payload's.
import { Bar, BarChart, CartesianGrid, LabelList, XAxis, YAxis } from 'recharts'
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart'
import { unitSuffix } from '@/charts/unitSuffix'
import type { Payload } from '@/types/market_state_page'

const STEP = 5
const ROOM = 8

// A fixed step: the axis runs from the extreme bars plus room, outward to whole steps, ticks one step apart.
function axisTicks(values: number[]): number[] | undefined {
  if (values.length === 0) return undefined
  const lo = Math.floor((Math.min(...values) - ROOM) / STEP) * STEP
  const hi = Math.ceil((Math.max(...values) + ROOM) / STEP) * STEP
  return Array.from({ length: (hi - lo) / STEP + 1 }, (_, i) => lo + i * STEP)
}

const config: ChartConfig = { value: { label: 'Gap', color: 'var(--chart-1)' } }

export function ExcessBars({ block }: { block: Payload['sectors'] }) {
  const data = block.rows.flatMap((row) =>
    row.gap_63.kind === 'value' ? [{ label: row.symbol, value: row.gap_63.value, text: row.gap_63.text }] : [],
  )
  const ticks = axisTicks(data.map((d) => d.value))
  return (
    <section aria-label="Gap bars" data-excess-bars>
      <p className="mb-2 text-sm text-muted-foreground" data-bars-caption>
        {block.bars_caption}
      </p>
      <ChartContainer config={config} className="h-[360px] w-full">
        <BarChart data={data} layout="vertical" margin={{ left: 8, right: 56, top: 8, bottom: 8 }}>
          <CartesianGrid horizontal={false} stroke="var(--border)" />
          {/* 8 points beyond the extreme bars, rounded out to whole steps of 5: room for the label at the end of every bar. */}
          <XAxis
            type="number"
            domain={ticks ? [ticks[0]!, ticks[ticks.length - 1]!] : undefined}
            ticks={ticks}
            allowDecimals={false}
            tickFormatter={(tick: number) => `${tick}${unitSuffix('percentage_points')}`}
            tickLine={false}
            axisLine={false}
          />
          <YAxis type="category" dataKey="label" tickLine={false} axisLine={false} interval={0} />
          <ChartTooltip content={<ChartTooltipContent />} />
          <Bar dataKey="value" fill="var(--color-value)" isAnimationActive={false}>
            <LabelList dataKey="text" position="right" fontSize={12} />
          </Bar>
        </BarChart>
      </ChartContainer>
    </section>
  )
}
