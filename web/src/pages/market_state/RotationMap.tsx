// The rotation map (docs/design/market-state.md Decision 15, mockup v7): one series per row with a rotation, its
// points in payload order (today's last) joined by a line, faded toward the old end, today's point large with the
// fund's symbol beside it and an arrowhead on the last segment. A Gap rotation is not drawn. The four quadrants are
// named by fixed words at the plot's corners by id; the producer's quadrant sentences are not shown.
import {
  CartesianGrid,
  LabelList,
  ReferenceArea,
  ReferenceLine,
  Scatter,
  ScatterChart,
  XAxis,
  YAxis,
  useCartesianScale,
} from 'recharts'
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, type ChartConfig } from '@/components/ui/chart'
import type { Payload } from '@/types/market_state_page'

type Sectors = Payload['sectors']
type Spot = { day: string; x: number; y: number; text: string }
type Point = Spot & { fund: string; note: string }

// Eleven sector funds, eleven hues; SMH is a hollow ring in a neutral colour.
export const FUND_PALETTE = [
  '#4e79a7',
  '#f28e2b',
  '#59a14f',
  '#b07aa1',
  '#3d9a94',
  '#c9a227',
  '#9c755f',
  '#e377a0',
  '#e15759',
  '#1f4e79',
  '#38b6e8',
] as const
const RING_COLOR = '#6b7280'
const RING_SYMBOL = 'SMH'

const QUADRANT = {
  leading: { word: 'Leading', area: { x1: 0, y1: 0 }, position: 'insideTopRight' },
  improving: { word: 'Improving', area: { x2: 0, y1: 0 }, position: 'insideTopLeft' },
  lagging: { word: 'Lagging', area: { x2: 0, y2: 0 }, position: 'insideBottomLeft' },
  weakening: { word: 'Weakening', area: { x1: 0, y2: 0 }, position: 'insideBottomRight' },
} as const

const SMALL = 3
const LARGE = 7

type ShapeProps = { cx?: number; cy?: number; index?: number; count: number; tone: string; ring: boolean }

// One point of a tail: small and fading toward the old end; today's (the last) large and solid.
function TailPoint({ cx, cy, index = 0, count, tone, ring }: ShapeProps) {
  if (cx === undefined || cy === undefined) return null
  const today = index === count - 1
  const fade = count > 1 ? 0.25 + (0.5 * index) / (count - 1) : 1
  return (
    <circle
      cx={cx}
      cy={cy}
      r={today ? LARGE : SMALL}
      fill={ring ? 'var(--background)' : tone}
      stroke={tone}
      strokeWidth={ring ? 2 : 1}
      opacity={today ? 1 : fade}
      data-tail-point={index}
    />
  )
}

// The fund's symbol beside today's point.
function TodayLabel({ x, y, index, count, symbol, tone }: { x?: number; y?: number; index?: number; count: number; symbol: string; tone: string }) {
  if (index !== count - 1 || typeof x !== 'number' || typeof y !== 'number') return null
  return (
    <text x={x + LARGE + 3} y={y} dy="0.35em" fontSize={11} fontWeight={600} fill={tone} data-today-label={symbol}>
      {symbol}
    </text>
  )
}

// The last segment ends in an arrowhead at today's point; the marker orients itself along the segment.
function TailArrow({ points, symbol, tone }: { points: Point[]; symbol: string; tone: string }) {
  const today = points[points.length - 1]
  const before = points[points.length - 2]
  const to = useCartesianScale(today ?? { x: 0, y: 0 })
  const from = useCartesianScale(before ?? { x: 0, y: 0 })
  if (!today || !before || !to || !from) return null
  const id = `arrow-${symbol}`
  return (
    <g data-arrow={symbol}>
      <defs>
        <marker id={id} markerUnits="userSpaceOnUse" markerWidth={10} markerHeight={10} refX={10 + LARGE} refY={5} orient="auto">
          <path d="M0,0 L10,5 L0,10 z" fill={tone} />
        </marker>
      </defs>
      <line x1={from.x} y1={from.y} x2={to.x} y2={to.y} stroke={tone} strokeWidth={1.5} markerEnd={`url(#${id})`} />
    </g>
  )
}

function Tip({ active, payload }: { active?: boolean; payload?: { payload?: Point }[] }) {
  const point = payload?.[0]?.payload
  if (!active || !point) return null
  return (
    <div className="rounded-md border bg-background p-2 text-xs shadow" data-rotation-tip>
      <div className="font-medium">{point.note === '' ? point.fund : `${point.fund} (${point.note})`}</div>
      <div>{point.day}</div>
      <div>{point.text}</div>
    </div>
  )
}

export function RotationMap({ block }: { block: Sectors }) {
  const { rotation_axes: axes } = block
  const drawn = block.rows.flatMap((row, index) =>
    'kind' in row.rotation
      ? []
      : [
          {
            symbol: row.symbol,
            tone: row.symbol === RING_SYMBOL ? RING_COLOR : (FUND_PALETTE[index % FUND_PALETTE.length] ?? RING_COLOR),
            ring: row.symbol === RING_SYMBOL,
            // The unranked row says so in its tooltip, with the producer's rank text.
            points: (row.rotation as Spot[]).map((p): Point => ({
              ...p,
              fund: row.symbol,
              note: 'kind' in row.rank_63 ? row.rank_63.text : '',
            })),
          },
        ],
  )
  const config: ChartConfig = Object.fromEntries(drawn.map((item) => [item.symbol, { label: item.symbol, color: item.tone }]))
  return (
    <section aria-label="Rotation map" data-rotation-map>
      <p className="text-sm text-muted-foreground" data-rotation-note>
        {axes.note}
      </p>
      <div className="mt-2">
        <ChartContainer config={config} className="h-[460px] w-full">
          <ScatterChart margin={{ left: 16, right: 24, top: 16, bottom: 28 }}>
            <CartesianGrid stroke="var(--border)" />
            <XAxis
              type="number"
              dataKey="x"
              name={axes.x_caption}
              domain={['auto', 'auto']}
              padding={{ left: 16, right: 16 }}
              tickLine={false}
              label={{ value: axes.x_caption, position: 'insideBottom', offset: -16, fontSize: 12 }}
            />
            <YAxis
              type="number"
              dataKey="y"
              name={axes.y_caption}
              domain={['auto', 'auto']}
              padding={{ top: 32, bottom: 16 }}
              tickLine={false}
              label={{ value: axes.y_caption, angle: -90, position: 'insideLeft', offset: 0, fontSize: 12, style: { textAnchor: 'middle' } }}
            />
            {axes.quadrants.map((q) => (
              <ReferenceArea
                key={q.id}
                {...QUADRANT[q.id].area}
                ifOverflow="extendDomain"
                fill="none"
                stroke="none"
                label={{ value: QUADRANT[q.id].word, position: QUADRANT[q.id].position, fontSize: 12, fill: 'var(--muted-foreground)' }}
              />
            ))}
            <ReferenceLine x={0} stroke="var(--foreground)" />
            <ReferenceLine y={0} stroke="var(--foreground)" />
            <ChartTooltip content={<Tip />} />
            <ChartLegend content={<ChartLegendContent />} />
            {drawn.map((item) => (
              <Scatter
                key={item.symbol}
                name={item.symbol}
                data={item.points}
                line={{ stroke: item.tone, strokeWidth: 1.5 }}
                fill={item.tone}
                stroke={item.tone}
                shape={<TailPoint count={item.points.length} tone={item.tone} ring={item.ring} />}
                isAnimationActive={false}
              >
                <LabelList content={<TodayLabel count={item.points.length} symbol={item.symbol} tone={item.tone} />} />
              </Scatter>
            ))}
            {drawn.map((item) => (
              <TailArrow key={item.symbol} points={item.points} symbol={item.symbol} tone={item.tone} />
            ))}
          </ScatterChart>
        </ChartContainer>
      </div>
      <p className="text-sm text-muted-foreground" data-x-caption>
        {axes.x_caption}
      </p>
      <p className="text-sm text-muted-foreground" data-y-caption>
        {axes.y_caption}
      </p>
    </section>
  )
}
