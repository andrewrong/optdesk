// The rotation map: one series per row with a rotation, its points in payload order (today's last) joined by a line.
// A Gap rotation is not drawn. The quadrant texts sit at fixed corners by id; the texts are the payload's.
import { CartesianGrid, ReferenceLine, Scatter, ScatterChart, XAxis, YAxis } from 'recharts'
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, type ChartConfig } from '@/components/ui/chart'
import type { Payload } from '@/types/market_state_page'

type Sectors = Payload['sectors']
type Spot = { day: string; x: number; y: number; text: string }

const CORNER = {
  leading: 'right-2 top-2',
  weakening: 'right-2 bottom-8',
  lagging: 'left-12 bottom-8',
  improving: 'left-12 top-2',
} as const

function Tip({ active, payload }: { active?: boolean; payload?: { name?: string; payload?: Spot }[] }) {
  const item = payload?.[0]
  if (!active || !item?.payload) return null
  return (
    <div className="rounded-md border bg-background p-2 text-xs shadow" data-rotation-tip>
      <div className="font-medium">{item.name}</div>
      <div>{item.payload.text}</div>
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
            // The unranked row says so in its legend and tooltip name, with the producer's rank text.
            name: 'kind' in row.rank_63 ? `${row.symbol} (${row.rank_63.text})` : row.symbol,
            points: row.rotation as Spot[],
          },
        ],
  )
  const config: ChartConfig = Object.fromEntries(
    drawn.map((item, index) => [item.symbol, { label: item.name, color: `var(--chart-${(index % 5) + 1})` }]),
  )
  return (
    <section aria-label="Rotation map" data-rotation-map>
      <p className="text-sm text-muted-foreground" data-rotation-note>
        {axes.note}
      </p>
      <div className="relative mt-2">
        <ChartContainer config={config} className="h-[420px] w-full">
          <ScatterChart margin={{ left: 8, right: 16, top: 16, bottom: 24 }}>
            <CartesianGrid stroke="var(--border)" />
            <XAxis type="number" dataKey="x" name={axes.x_caption} domain={['auto', 'auto']} tickLine={false} />
            <YAxis type="number" dataKey="y" name={axes.y_caption} domain={['auto', 'auto']} tickLine={false} />
            <ReferenceLine x={0} stroke="var(--foreground)" />
            <ReferenceLine y={0} stroke="var(--foreground)" />
            <ChartTooltip content={<Tip />} />
            <ChartLegend content={<ChartLegendContent />} />
            {drawn.map((item) => (
              <Scatter
                key={item.symbol}
                name={item.name}
                data={item.points}
                line
                fill={`var(--color-${item.symbol})`}
                stroke={`var(--color-${item.symbol})`}
                isAnimationActive={false}
              />
            ))}
          </ScatterChart>
        </ChartContainer>
        {axes.quadrants.map((q) => (
          <span
            key={q.id}
            className={`pointer-events-none absolute text-xs text-muted-foreground ${CORNER[q.id]}`}
            data-quadrant={q.id}
          >
            {q.text}
          </span>
        ))}
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
