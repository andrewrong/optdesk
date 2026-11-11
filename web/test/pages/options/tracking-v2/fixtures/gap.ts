// Synthetic fixture written from contracts/schemas/page.tracking.v2.json alone.
import type { Tracking } from '../../../../../src/generated/tracking.v2'
import { ok } from './ok'

export const gap = {
  ...ok,
  run_status: 'partial',
  versions: [
    {
      ...ok.versions[0],
      groups: [{ ...ok.versions[0].groups[0], summary: { reason: 'no_data', text: 'This group has no results to sum up yet' } }],
      covered_calls: null,
      my_trades: { reason: 'not_built', text: 'My trades have no data source yet' },
    },
  ],
} satisfies Tracking
