// Synthetic base data page payloads with gaps: one view failed (the other rows stay), and every view failed.
import type { BaseDataPage } from '../../../../src/generated/base_data_page.v1'
import { ok } from './ok'

// The option view failed: its row is not judged, the other rows are as in ok.
export const viewGap = {
  ...ok,
  top: {
    level: 'unknown',
    text_zh: '基础数据共 3 项：1 项正常，1 项警告，1 项未判定。',
    counts: { pass: 1, warn: 1, fail: 0, unknown: 1 },
  },
  groups: [
    {
      name: 'Market data',
      rows: [
        ok.groups[0]!.rows[0]!,
        {
          dataset: 'option_chains',
          label: 'Option chains end of day',
          label_zh: '期权链',
          level: 'unknown',
          newest_value: '',
          newest_day: null,
          newest_subject: '',
          age_text: '',
          failed_summary: 'not_read',
          checks: [],
        },
      ],
    },
    ok.groups[1]!,
  ],
  gaps: [{ kind: 'gap', reason: 'view_unavailable', text: 'The option view did not answer in time; its rows are not judged.' }],
} satisfies BaseDataPage

// Every view failed: no rows at all.
export const gap = {
  ...ok,
  top: { level: 'unknown', text_zh: '基础数据暂时无法读取。', counts: { pass: 0, warn: 0, fail: 0, unknown: 12 } },
  groups: [],
  glossary: [],
  derived_note: '',
  gaps: [{ kind: 'gap', reason: 'facts_unavailable', text: 'All four fact views failed to read.' }],
} satisfies BaseDataPage
