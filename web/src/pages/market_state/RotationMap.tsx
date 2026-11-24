// The rotation map (docs/design/market-state.md Decision 15, mockup v7): one series per row with a rotation. By
// default a series is only today's point (the last of its payload rotation) large with the fund's symbol beside it;
// the fund the user clicked also gets its earlier points (faded toward the old end), the line through them and an
// arrowhead on the last segment; one fund at a time. A Gap rotation is not drawn. The four quadrants are
// named at the plot's corners by the fixed quadrant name, and listed in full under the map by id; the producer's quadrant sentences are not shown.
import { useState } from 'react'
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
  leading: { word: 'Leading — ahead of SPY over 3 months and still gaining', sx: 1, sy: 1, position: 'insideTopRight' },
  improving: { word: 'Improving — behind SPY over 3 months but gaining pace', sx: -1, sy: 1, position: 'insideTopLeft' },
  lagging: { word: 'Lagging — behind SPY over 3 months and still losing', sx: -1, sy: -1, position: 'insideBottomLeft' },
  weakening: { word: 'Weakening — ahead of SPY over 3 months but losing pace', sx: 1, sy: -1, position: 'insideBottomRight' },
} as const

// The end of a zero-centred axis: the largest absolute value rounded up to a multiple of 10 (at least 10), by remainder arithmetic: the web source calls no Math function.
export function axisEnd(values: number[]): number {
  const largest = values.reduce((m, v) => (v < 0 ? -v : v) > m ? (v < 0 ? -v : v) : m, 0)
  const rest = largest % 10
  const up = rest === 0 ? largest : largest - rest + 10
  return up < 10 ? 10 : up
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
// the first position whose box clears every label already placed, every dot (a box of LARGE around each anchor,
// its own included) and the axis lines x = zero.x and y = zero.y wins.
export function placeLabels(anchors: Anchor[], lineHeight: number, zero?: { x?: number; y?: number }): Placed[] {
  const placed: Placed[] = []
  const clear = (b: Placed) =>
    placed.every((o) => b.right <= o.left || o.right <= b.left || b.bottom <= o.top || o.bottom <= b.top) &&
    anchors.every((a) => b.right <= a.x - LARGE || a.x + LARGE <= b.left || b.bottom <= a.y - LARGE || a.y + LARGE <= b.top) &&
    !(zero?.x !== undefined && b.left < zero.x && zero.x < b.right) &&
    !(zero?.y !== undefined && b.top < zero.y && zero.y < b.bottom)
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
  const own = placeLabels(anchors, LINE_HEIGHT, { x: xScale?.(0), y: yScale?.(0) }).find((p) => p.id === symbol)
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
  // The fund whose tail is drawn (none at first): a click on its today dot clears it, a click on another moves it.
  const [selected, setSelected] = useState<string | null>(null)
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
              tickCount={5}
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
              tickCount={5}
              padding={{ top: 32, bottom: 16 }}
              tickLine={false}
              tickFormatter={(tick: number) => `${String(tick).replace('-', '−')}${unitSuffix('percentage_points')}`}
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
                label={{ value: QUADRANT[q.id].word.split(' — ')[0], position: QUADRANT[q.id].position, fontSize: 12, fill: 'var(--muted-foreground)' }}
              />
            ))}
            <ReferenceLine x={0} stroke="var(--foreground)" />
            <ReferenceLine y={0} stroke="var(--foreground)" />
            <ChartTooltip content={<Tip />} />
            {drawn.map((item) => {
              const open = item.symbol === selected
              const shown = open ? item.points : item.points.slice(-1)
              return (
                <Scatter
                  key={item.symbol}
                  name={item.symbol}
                  data={shown}
                  line={open ? { stroke: item.tone, strokeWidth: 1.5, ...(item.ring ? { strokeDasharray: '4 3' } : {}) } : undefined}
                  fill={item.tone}
                  stroke={item.tone}
                  shape={<TailPoint count={shown.length} tone={item.tone} ring={item.ring} />}
                  isAnimationActive={false}
                  cursor="pointer"
                  onClick={() => setSelected(open ? null : item.symbol)}
                >
                  <LabelList content={<TodayLabel count={shown.length} symbol={item.symbol} tone={item.tone} todays={todays} />} />
                </Scatter>
              )
            })}
            {drawn.filter((item) => item.symbol === selected).map((item) => (
              <TailArrow key={item.symbol} points={item.points} symbol={item.symbol} tone={item.tone} />
            ))}
          </ScatterChart>
        </ChartContainer>
      </div>
      <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-muted-foreground" data-quadrant-notes>
        {axes.quadrants.map((q) => (
          <li key={q.id}>{QUADRANT[q.id].word}</li>
        ))}
      </ul>
    </section>
  )
}
