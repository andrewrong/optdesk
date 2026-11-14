// The one place that draws a chart (ADR 0034, Charts): recharts through the shadcn chart component. The spec, its
// ARIA text and every figure in it come from Python; this file computes nothing, it lays the spec's points out as
// the rows recharts wants and draws them, without animation. The row of a point carries its text (`<key>_text`),
// colour and label, which the tooltip and the bar cells read.
import {
  Area,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ComposedChart,
  Line,
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
import { unitSuffix } from './unitSuffix'

type Row = Record<string, string | number | number[] | null>

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
    // The band is one range `[low, high]` per category; a missing end leaves the band out there.
    if (spec.band) {
      const low = spec.band.low[at]?.value
      const high = spec.band.high[at]?.value
      row.band = low != null && high != null ? [low, high] : null
    }
    return row
  })
}

// The last point of a line carries its text at the right edge (`<key>_end`): Python's text, else the value.
function addEnds(spec: ChartSpec, rows: Row[]) {
  spec.series.forEach((series, index) => {
    const at = series.points.findLastIndex((point) => point.value !== null)
    const point = series.points[at]
    const row = rows[at]
    if (point && row) row[`${keyOf(index)}_end`] = point.text ?? String(point.value)
  })
}

export function Chart({
  spec,
  ariaText,
  name,
  className = 'h-[180px] md:h-[220px] w-full',
}: {
  spec: ChartSpec
  ariaText: string
  name: string
  className?: string
}) {
  const rows = rowsOf(spec)
  if (spec.kind === 'line') addEnds(spec, rows)
  const config: ChartConfig = Object.fromEntries([
    ...spec.series.map((series, index) => [
      keyOf(index),
      { label: series.name, color: series.color ?? `var(--chart-${(index % 5) + 1})` },
    ]),
    ...(spec.band ? [['band', { label: spec.band.name, color: 'var(--chart-2)' }]] : []),
  ])
  const axis = {
    tickLine: false,
    axisLine: false,
    tick: { fontSize: 12, fill: 'var(--muted-foreground)' },
  } as const
  const unit = unitSuffix(spec.unit ?? '')
  const yAxis = { tickFormatter: (tick: string | number) => `${tick}${unit}` }
  const common = {
    data: rows,
    margin: { left: 8, right: spec.kind === 'line' ? 56 : 8, top: 8, bottom: 8 },
  }
  const parts = [
    <CartesianGrid key="grid" vertical={false} stroke="var(--border)" strokeWidth={1} />,
    <XAxis
      key="x"
      dataKey="category"
      {...axis}
      interval="preserveStartEnd"
      minTickGap={24}
      padding={{ left: 24, right: 24 }}
    />,
    spec.kind === 'line' ? (
      <YAxis key="y" {...axis} {...yAxis} tickCount={5} domain={['auto', 'auto']} />
    ) : (
      <YAxis key="y" {...axis} {...yAxis} tickCount={5} />
    ),
    <ChartTooltip key="tip" content={<ChartTooltipContent />} />,
    spec.legend ? (
      <ChartLegend
        key="legend"
        verticalAlign="top"
        wrapperStyle={{ fontSize: 12 }}
        content={({ payload }) => (
          // A series with `legendType: 'none'` shares the entry of its neighbour.
          <ChartLegendContent
            payload={payload
              ?.filter((item) => item.type !== 'none')
              .map(({ value, color, dataKey }) => ({
                value,
                color,
                dataKey: typeof dataKey === 'function' ? undefined : dataKey,
              }))}
          />
        )}
      />
    ) : null,
  ]
  return (
    <div role="img" aria-label={ariaText} data-chart={name} className={cn(className, 'overflow-hidden')}>
      <ChartContainer config={config} className="h-full w-full">
        {spec.kind === 'line' ? (
          <ComposedChart {...common}>
            {parts}
            {spec.band ? (
              <Area
                dataKey="band"
                name={spec.band.name}
                type="linear"
                stroke="none"
                fill="var(--color-band)"
                fillOpacity={0.12}
                tooltipType="none"
                isAnimationActive={false}
              />
            ) : null}
            {spec.series.map((series, index) => (
              <Line
                key={keyOf(index)}
                dataKey={keyOf(index)}
                name={series.name}
                type="linear"
                stroke={`var(--color-${keyOf(index)})`}
                strokeWidth={series.dashed ? 1 : 2}
                {...(series.dashed ? { strokeDasharray: '4 4' } : {})}
                {...(series.legendHidden ? { legendType: 'none' as const } : {})}
                dot={false}
                connectNulls={!series.hole}
                isAnimationActive={false}
              >
                <LabelList dataKey={`${keyOf(index)}_end`} position="right" fontSize={12} />
              </Line>
            ))}
          </ComposedChart>
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
                {...(series.legendHidden ? { legendType: 'none' as const } : {})}
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
