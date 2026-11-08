// The one place that draws a chart (ADR 0034, Charts): recharts through the shadcn chart component. The spec, its
// ARIA text and every figure in it come from Python; this file computes nothing, it lays the spec's points out as
// the rows recharts wants and draws them, without animation. The row of a point carries its text (`<key>_text`),
// colour and label, which the tooltip and the bar cells read.
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  LabelList,
  XAxis,
  YAxis,
} from 'recharts'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { cn } from '@/lib/utils'
import type { ChartSpec } from './spec'

type Row = Record<string, string | number | null>

const keyOf = (index: number) => `s${index}`

function rowsOf(spec: ChartSpec): Row[] {
  return spec.categories.map((category, at) => {
    const row: Row = { category }
    spec.series.forEach((series, index) => {
      const point = series.points[at]
      const key = keyOf(index)
      row[key] = point?.value ?? null
      if (point?.text !== undefined) row[`${key}_text`] = point.text
      if (point?.label !== undefined) row[`${key}_label`] = point.label
      if (point?.color !== undefined) row[`${key}_color`] = point.color
    })
    return row
  })
}

export function Chart({
  spec,
  ariaText,
  name,
  className = 'aspect-[2/1] w-full min-h-56',
}: {
  spec: ChartSpec
  ariaText: string
  name: string
  className?: string
}) {
  const rows = rowsOf(spec)
  const config: ChartConfig = Object.fromEntries(
    spec.series.map((series, index) => [
      keyOf(index),
      { label: series.name, color: series.color ?? `var(--chart-${(index % 5) + 1})` },
    ]),
  )
  const axis = { tickLine: false, axisLine: false, tick: { fontSize: 16 } } as const
  const common = { data: rows, margin: { left: 8, right: 8, top: 16, bottom: 8 } }
  const parts = [
    <CartesianGrid key="grid" vertical={false} />,
    <XAxis key="x" dataKey="category" {...axis} interval="preserveStartEnd" minTickGap={24} />,
    <YAxis key="y" hide domain={['auto', 'auto']} />,
    <ChartTooltip key="tip" content={<ChartTooltipContent />} />,
    spec.legend ? <ChartLegend key="legend" content={<ChartLegendContent />} /> : null,
  ]
  return (
    <div role="img" aria-label={ariaText} data-chart={name} className={cn(className, 'overflow-hidden')}>
      <ChartContainer config={config} className="h-full w-full">
        {spec.kind === 'line' ? (
          <LineChart {...common}>
            {parts}
            {spec.series.map((series, index) => (
              <Line
                key={keyOf(index)}
                dataKey={keyOf(index)}
                name={series.name}
                type="monotone"
                stroke={`var(--color-${keyOf(index)})`}
                strokeWidth={2}
                dot={false}
                connectNulls
                isAnimationActive={false}
              />
            ))}
          </LineChart>
        ) : (
          <BarChart {...common}>
            {parts}
            {spec.series.map((series, index) => (
              <Bar
                key={keyOf(index)}
                dataKey={keyOf(index)}
                name={series.name}
                stackId={series.stack}
                fill={series.invisible ? 'transparent' : `var(--color-${keyOf(index)})`}
                hide={false}
                isAnimationActive={false}
                {...(series.invisible ? { tooltipType: 'none' as const } : {})}
              >
                {series.points.some((point) => point.color !== undefined)
                  ? rows.map((row, at) => {
                      const color = row[`${keyOf(index)}_color`]
                      return (
                        <Cell
                          key={at}
                          fill={typeof color === 'string' ? color : `var(--color-${keyOf(index)})`}
                        />
                      )
                    })
                  : null}
                {series.points.some((point) => point.label !== undefined) ? (
                  <LabelList dataKey={`${keyOf(index)}_label`} position="top" />
                ) : null}
              </Bar>
            ))}
          </BarChart>
        )}
      </ChartContainer>
    </div>
  )
}
