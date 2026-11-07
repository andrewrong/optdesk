import type { StockOptionPage } from '../../../../src/generated/stock_option_page.v1'
import { ok } from './ok'

export const stale = {
  ...ok,
  stale: {
    data_time: '2026-01-02T09:30:00+08:00',
    stale_after: '2026-01-03T09:30:00+08:00',
    text: 'Stale label quotes are three days old',
  },
} satisfies StockOptionPage
