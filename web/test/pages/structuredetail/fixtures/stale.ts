// Synthetic fixture: the same page as ok, with another stale label and run status.
import type { StructureDetail } from '../../../../src/generated/structure_detail.v3'
import { ok } from './ok'

export const stale = {
  ...ok,
  run_status: 'partial',
  sentence: 'Stale sentence: the numbers are older than the label says they should be.',
  stale: {
    data_time: '2029-12-30T09:55:00Z',
    stale_after: '2029-12-30T10:30:00Z',
    text: 'Stale label: the quotes are three days old.',
  },
} satisfies StructureDetail
