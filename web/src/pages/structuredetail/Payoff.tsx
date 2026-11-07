// The payoff line chart: the points are Python's; the chart layer only lays them out.
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import { Badge } from '@/components/ui/badge'
import type { Payload } from '@/types/structure_detail'

type Payoff = Exclude<Payload['payoff'], { reason: string }>

export function PayoffChart({ payoff }: { payoff: Payoff }) {
  const spec: ChartSpec = {
    kind: 'line',
    categories: payoff.points.map((point) => `${point.price}`),
    series: [{ name: payoff.aria_text, points: payoff.points.map((point) => ({ value: point.pnl })) }],
  }
  return (
    <>
      <Chart spec={spec} ariaText={payoff.aria_text} name="payoff" />
      <div className="mt-3 flex flex-wrap gap-2">
        {[payoff.spot, payoff.break_even, payoff.max_gain, payoff.max_loss].map((f) => (
          <Badge key={f.text} variant="secondary">
            {f.text}
          </Badge>
        ))}
        <Badge variant="outline">{payoff.band.text}</Badge>
      </div>
    </>
  )
}
