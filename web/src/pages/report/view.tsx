// Read-only view of page.report_page.v2: every string, figure, tone and state is a field of the payload.
// The open slot adds six blocks, drawn first; the five close blocks follow.
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/report_page'
import { AccountMoves, Actions, MarketWatch, Risks } from './close-blocks'
import { Header } from './frame'
import { GlobalAssets } from './global-assets'
import {
  AccountsBlock,
  CandidatesBlock,
  MarketLineBlock,
  RemindersBlock,
  SystemStatusBlock,
  TilesBlock,
} from './open-blocks'

export function View({ payload }: { payload: Payload }) {
  const { blocks } = payload
  return (
    <div className="min-w-0 space-y-6" data-page="report-page" data-slot={payload.slot}>
      <Header payload={payload} />
      {blocks.market_line ? <MarketLineBlock block={blocks.market_line} /> : null}
      {blocks.tiles ? <TilesBlock block={blocks.tiles} /> : null}
      {blocks.accounts ? <AccountsBlock block={blocks.accounts} /> : null}
      {blocks.candidates ? <CandidatesBlock block={blocks.candidates} /> : null}
      {blocks.reminders ? <RemindersBlock block={blocks.reminders} /> : null}
      {blocks.system_status ? <SystemStatusBlock block={blocks.system_status} /> : null}
      <Actions block={blocks.actions} />
      <AccountMoves block={blocks.account_moves} />
      <Risks block={blocks.risks} />
      <MarketWatch block={blocks.market_watch} />
      <GlobalAssets block={blocks.global_assets} />
    </div>
  )
}

export const registration = { schema: 'report_page', page: page(validate, View) }
