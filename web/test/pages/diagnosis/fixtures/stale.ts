// Synthetic diagnosis page carrying a stale label.
import type { DiagnosisPage } from '../../../../src/generated/diagnosis_page.v1'
import { ok } from './ok'

export const stale = {
  ...ok,
  stale: { data_time: '2026-10-20T00:00:00Z', is_stale: true, text: 'Annual report data stops at 20 Oct.' },
  stale_after: '2026-10-25T00:00:00Z',
} satisfies DiagnosisPage
