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
        { kind: 'gap', reason: 'no_data', text: 'DDD 只有 4 年，不够 10 年诊断。' },
      ],
    },
    {
      ...financial,
      rows: [{ kind: 'gap', reason: 'no_reader', text: 'EEE 没有 US GAAP 年报。' }],
    },
  ],
} satisfies DiagnosisPage
