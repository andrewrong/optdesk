// The market state page: title, freshness text, the five rows, the sector table, the rotation map and the bars.
// Python supplies every figure and sentence; the browser shows them and computes nothing.
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/market_state_page'
import { ExcessBars } from './ExcessBars'
import { MarketRows } from './MarketRows'
import { RotationMap } from './RotationMap'
import { SectorTable } from './SectorTable'
export { validate as validateMarketStatePage } from '@/types/market_state_page'
export type { Payload as MarketStatePagePayload } from '@/types/market_state_page'

export function MarketStatePage({ payload }: { payload: Payload }) {
  return (
    <div className="min-w-0 space-y-6" data-page="market-state-page">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2 text-muted-foreground" data-freshness>
          {payload.freshness_text}
        </p>
        {payload.stale ? <p data-stale>{payload.stale.text}</p> : null}
      </header>
      <MarketRows block={payload.market} />
      <SectorTable block={payload.sectors} />
      <RotationMap block={payload.sectors} />
      <ExcessBars block={payload.sectors} />
    </div>
  )
}

export const registration = {
  schema: 'market_state_page',
  page: page<Payload>(validate, MarketStatePage),
}
