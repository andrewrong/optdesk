// Synthetic diagnosis page with two companies that cannot be diagnosed: each is a Gap row with its cause.
import type { DiagnosisPage } from '../../../../src/generated/diagnosis_page.v1'
import { cyclical, financial, ok } from './ok'

export const gap = {
  ...ok,
  groups: [
    {
      ...cyclical,
      rows: [
        ...cyclical.rows.slice(0, 1),
        { kind: 'gap', reason: 'no_data', text: 'DDD has only 4 years, not enough for a 10-year check.' },
      ],
    },
    {
      ...financial,
      rows: [{ kind: 'gap', reason: 'no_reader', text: 'EEE has no US GAAP annual reports.' }],
    },
  ],
} satisfies DiagnosisPage
