import type { PriceLinesPage } from '../../../../src/generated/price_lines_page.v1'
import { ok } from './ok'

// Synthetic fixture: the producer marks the page stale and gives the time the label turns stale.
export const stale = {
  ...ok,
  run_id: 'run-stale',
  stale: {
    data_time: '2026-01-05T09:00:00+08:00',
    is_stale: true,
    text: 'Stale label text: data of 09:00',
  },
  stale_after: '2026-01-05T10:00:00+08:00',
} satisfies PriceLinesPage
