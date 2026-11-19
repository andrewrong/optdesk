// The zero-lot payload: a report with no line and the empty text of the builder.
import type { Payload } from '@/types/strategy_replay_page'
import { report } from './report'

export const empty = (): Payload => {
  const base = report()
  return {
    ...base,
    report: {
      ...(base.report as NonNullable<Payload['report']>),
      first_quote_day: null,
      last_quote_day: null,
      counts: { lots: 0, lots_with_quote: 0, still_open: 0, lot_days: 0, quote_days: 0 },
      excluded: [],
      lines: [],
      empty_text: '数据不足，无结论：0 笔，存的报价 0 天，首末日 无',
    },
  }
}
