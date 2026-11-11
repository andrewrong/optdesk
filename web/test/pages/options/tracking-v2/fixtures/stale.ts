// Synthetic fixture written from contracts/schemas/page.tracking.v2.json alone.
import type { Tracking } from '../../../../../src/generated/tracking.v2'
import { ok } from './ok'

export const stale = {
  ...ok,
  stale: {
    data_time: '2026-10-01T19:00:00+08:00',
    stale_after: '2026-10-03T19:00:00+08:00',
    text: 'Data is stale, as of Oct 1',
  },
} satisfies Tracking
