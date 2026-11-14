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
  useXAxisScale,
  useYAxisScale,
} from 'recharts'
import { ChartContainer, ChartTooltip, type ChartConfig } from '@/components/ui/chart'
import { unitSuffix } from '@/charts/unitSuffix'
import type { Payload } from '@/types/market_state_page'

type Sectors = Payload['sectors']
type Spot = { day: string; x: number; y: number; text: string }
type Point = Spot & { fund: string; note: string }

// One fixed hue per sector fund (the mockup's ten plus a light blue for XLC), whatever the fund's rank today.
// SMH is a hollow ring in a neutral colour.
export const FUND_COLOR: Record<string, string> = {
  XLK: '#4e79a7',
  XLY: '#f28e2b',
  XLV: '#59a14f',
  XLI: '#b07aa1',
  XLB: '#3d9a94',
  XLP: '#c9a227',
  XLU: '#9c755f',
  XLRE: '#e377a0',
  XLE: '#e15759',
  XLF: '#1f4e79',
  XLC: '#38b6e8',
}
const RING_COLOR = '#6b7280'
const RING_SYMBOL = 'SMH'

// Each quadrant is bounded on both axes (0 to the axis end) so its name is drawn in its own plot corner.
const QUADRANT = {
  leading: { word: 'Leading', sx: 1, sy: 1, position: 'insideTopRight' },
  improving: { word: 'Improving', sx: -1, sy: 1, position: 'insideTopLeft' },
  lagging: { word: 'Lagging', sx: -1, sy: -1, position: 'insideBottomLeft' },
  weakening: { word: 'Weakening', sx: 1, sy: -1, position: 'insideBottomRight' },
} as const

// The end of a zero-centred axis: the largest absolute value, rounded up to a whole number (at least 1), by remainder arithmetic: the web source calls no Math function.
function axisEnd(values: number[]): number {
  const largest = values.reduce((m, v) => (v < 0 ? -v : v) > m ? (v < 0 ? -v : v) : m, 0)
  const whole = largest - (largest % 1)
  const up = whole < largest ? whole + 1 : whole
  return up < 1 ? 1 : up
}
// Recharts' function form of `domain`: symmetric about 0; display only, no payload value changes.
const symmetric = ([lo, hi]: readonly [number, number]): [number, number] => {
  const end = axisEnd([lo, hi])
  return [-end, end]
}

const LABEL_GAP = 10
const LINE_HEIGHT = 13
const CHAR_WIDTH = 7

type Anchor = { id: string; x: number; y: number; width: number }
export type Placed = { id: string; x: number; y: number; anchor: 'start' | 'end'; left: number; right: number; top: number; bottom: number }

// Places each label beside its point in order: right of it, else left of it, else shifted by whole line heights;
// the first position whose box clears every label already placed wins.
export function placeLabels(anchors: Anchor[], lineHeight: number): Placed[] {
  const placed: Placed[] = []
  const clear = (b: Placed) =>
    placed.every((o) => b.right <= o.left || o.right <= b.left || b.bottom <= o.top || o.bottom <= b.top)
  for (const a of anchors) {
    let pick: Placed | undefined
    const shifts = [0, 1, -1, 2, -2, 3, -3]
    for (const shift of shifts) {
      for (const side of ['start', 'end'] as const) {
        const x = side === 'start' ? a.x + LABEL_GAP : a.x - LABEL_GAP
        const y = a.y + shift * lineHeight
        const left = side === 'start' ? x : x - a.width
        const box: Placed = { id: a.id, x, y, anchor: side, left, right: left + a.width, top: y - lineHeight / 2, bottom: y + lineHeight / 2 }
        if (!pick && clear(box)) pick = box
      }
      if (pick) break
    }
    // Every position taken: fall back to the plain right-hand one.
    pick ??= { id: a.id, x: a.x + LABEL_GAP, y: a.y, anchor: 'start', left: a.x + LABEL_GAP, right: a.x + LABEL_GAP + a.width, top: a.y - lineHeight / 2, bottom: a.y + lineHeight / 2 }
    placed.push(pick)
  }
  return placed
}

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

type Today = { symbol: string; x: number; y: number }
type LabelProps = { x?: number; y?: number; index?: number; count: number; symbol: string; tone: string; todays: Today[] }

// The fund's symbol beside today's point; a label that would overlap an earlier one moves to the left or up/down a line.
function TodayLabel({ x, y, index, count, symbol, tone, todays }: LabelProps) {
  const xScale = useXAxisScale()
  const yScale = useYAxisScale()
  if (index !== count - 1 || typeof x !== 'number' || typeof y !== 'number') return null
  const anchors: Anchor[] = todays.map((t) => ({
    id: t.symbol,
    x: t.symbol === symbol || !xScale ? x : (xScale(t.x) ?? x),
    y: t.symbol === symbol || !yScale ? y : (yScale(t.y) ?? y),
    width: CHAR_WIDTH * t.symbol.length,
  }))
  const own = placeLabels(anchors, LINE_HEIGHT).find((p) => p.id === symbol)
  const at = own ?? { x: x + LABEL_GAP, y, anchor: 'start' as const }
  return (
    <text x={at.x} y={at.y} dy="0.35em" textAnchor={at.anchor} fontSize={11} fontWeight={600} fill={tone} data-today-label={symbol}>
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
  const drawn = block.rows.flatMap((row) =>
    'kind' in row.rotation
      ? []
      : [
          {
            symbol: row.symbol,
            tone: FUND_COLOR[row.symbol] ?? RING_COLOR,
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
  const todays: Today[] = drawn.flatMap((item) => {
    const last = item.points[item.points.length - 1]
    return last ? [{ symbol: item.symbol, x: last.x, y: last.y }] : []
  })
  const all = drawn.flatMap((item) => item.points)
  const endX = axisEnd(all.map((p) => p.x))
  const endY = axisEnd(all.map((p) => p.y))
  const config: ChartConfig = Object.fromEntries(drawn.map((item) => [item.symbol, { label: item.symbol, color: item.tone }]))
  return (
    <section aria-label="Rotation map" data-rotation-map>
      <p className="text-sm text-muted-foreground" data-rotation-note>
        {axes.note}
      </p>
      <div className="mt-2">
        <ChartContainer config={config} className="h-[460px] w-full">
          <ScatterChart margin={{ left: 16, right: 24, top: 16, bottom: 8 }}>
            <CartesianGrid stroke="var(--border)" />
            <XAxis
              type="number"
              dataKey="x"
              name={axes.x_caption}
              domain={symmetric}
              padding={{ left: 16, right: 16 }}
              tickLine={false}
              // The axis band is tall enough for its tick row and the title on its own line.
              height={52}
              label={{ value: axes.x_caption, position: 'insideBottom', offset: 0, fontSize: 12 }}
            />
            <YAxis
              type="number"
              dataKey="y"
              name={axes.y_caption}
              domain={symmetric}
              padding={{ top: 32, bottom: 16 }}
              tickLine={false}
              tickFormatter={(tick: number) => `${tick}${unitSuffix('percentage_points')}`}
              label={{ value: axes.y_caption, angle: -90, position: 'insideLeft', offset: 0, fontSize: 12, style: { textAnchor: 'middle' } }}
            />
            {axes.quadrants.map((q) => (
              <ReferenceArea
                key={q.id}
                x1={QUADRANT[q.id].sx > 0 ? 0 : -endX}
                x2={QUADRANT[q.id].sx > 0 ? endX : 0}
                y1={QUADRANT[q.id].sy > 0 ? 0 : -endY}
                y2={QUADRANT[q.id].sy > 0 ? endY : 0}
                ifOverflow="extendDomain"
                fill="none"
                stroke="none"
                label={{ value: QUADRANT[q.id].word, position: QUADRANT[q.id].position, fontSize: 12, fill: 'var(--muted-foreground)' }}
              />
            ))}
            <ReferenceLine x={0} stroke="var(--foreground)" />
            <ReferenceLine y={0} stroke="var(--foreground)" />
            <ChartTooltip content={<Tip />} />
            {drawn.map((item) => (
              <Scatter
                key={item.symbol}
                name={item.symbol}
                data={item.points}
                line={{ stroke: item.tone, strokeWidth: 1.5, ...(item.ring ? { strokeDasharray: '4 3' } : {}) }}
                fill={item.tone}
                stroke={item.tone}
                shape={<TailPoint count={item.points.length} tone={item.tone} ring={item.ring} />}
                isAnimationActive={false}
              >
                <LabelList content={<TodayLabel count={item.points.length} symbol={item.symbol} tone={item.tone} todays={todays} />} />
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
