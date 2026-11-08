import type { PriceLinesPage } from '../../../../src/generated/price_lines_page.v1'
import { ok } from './ok'

// Synthetic fixture: the envelope names its causes and one saved line has no price.
export const gap = {
  ...ok,
  run_id: 'run-gap',
  run_status: 'partial',
  gaps: [
    { kind: 'gap', reason: 'missing_quote', text: 'Gap cause one: no quote for Beta' },
    { kind: 'gap', reason: 'no_data', text: 'Gap cause two: no data for Gamma' },
  ],
  rows: [
    { symbol: 'ALPHA', market: 'US', price: 'Price 130.00 alpha gap', note: 'Note alpha gap' },
    { symbol: 'BETA', market: 'JP', price: null, note: 'Note beta gap' },
  ],
} satisfies PriceLinesPage
