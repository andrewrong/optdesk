// The ease gauge: a grey track, a shaded neutral band and a dot at marker_percent. It draws payload fields only;
// the band is placed by CSS (`left`, `right: calc(100% - high%)`), so no number is computed here.
import { Slider } from '@/components/ui/slider'

type Figure = { value: number; text: string }
export type EaseGaugeData = {
  marker_percent: Figure
  neutral_band: { low: Figure; high: Figure }
  ticks: string[]
  aria_text: string
}

export function EaseGauge({ ease }: { ease: EaseGaugeData }) {
  return (
    <div className="space-y-1" data-ease-gauge>
      <Slider
        disabled
        value={[ease.marker_percent.value]}
        min={0}
        max={100}
        thumbLabel={ease.aria_text}
        data-marker
      >
        <div
          className="absolute inset-y-0 bg-foreground/15"
          style={{
            left: `${ease.neutral_band.low.value}%`,
            right: `calc(100% - ${ease.neutral_band.high.value}%)`,
          }}
          data-neutral-band-shade
        />
      </Slider>
      <div className="flex justify-between text-xs text-muted-foreground" data-ticks>
        {ease.ticks.map((tick, index) => (
          <span key={`${index}-${tick}`}>{tick}</span>
        ))}
      </div>
    </div>
  )
}
