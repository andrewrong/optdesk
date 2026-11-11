// Synthetic core page payload with a stale label.
import type { CorePage } from '../../../../src/generated/core_page.v1'
import { ok } from './ok'

export const stale = {
  ...ok,
  stale_after: '2026-10-27T21:00:00Z',
  stale: { data_time: '2026-10-23T21:00:00Z', is_stale: false, text: 'Data as of 10-23 close' },
} satisfies CorePage
