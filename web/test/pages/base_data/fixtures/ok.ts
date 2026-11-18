// Synthetic base data page payload written from contracts/schemas/page.base_data_page.v1.json: two groups, three datasets.
import type { BaseDataPage } from '../../../../src/generated/base_data_page.v1'

export const ok = {
  schema: 'base_data_page',
  schema_version: 1,
  run_id: 'run-synthetic-0001',
  built_at: '2026-11-18T09:30:00Z',
  as_of: '2026-11-18',
  last_sessions: { XNYS: '2026-11-17', XTKS: '2026-11-18' },
  top: {
    level: 'fail',
    text_zh: '基础数据共 3 项：1 项正常，1 项警告，1 项失败。',
    counts: { pass: 1, warn: 1, fail: 1, unknown: 0 },
  },
  groups: [
    {
      name: 'Market data',
      rows: [
        {
          dataset: 'quotes',
          label: 'Quotes',
          label_zh: '报价快照',
          level: 'pass',
          newest_value: 'SPY last 590.10',
          newest_day: '2026-11-17',
          newest_subject: 'SPY',
          age_text: '0 s',
          failed_summary: '',
          checks: [
            {
              rule: 'fresh',
              label: 'Fresh',
              hint_zh: '最新值的日期不早于上一个收盘交易日。',
              level: 'pass',
              n: 40,
              bad: 0,
              share: '0',
              warn_at: '1',
              fail_at: '3',
              threshold_source: 'trial value',
              example: '',
            },
          ],
        },
        {
          dataset: 'option_chains',
          label: 'Option chains end of day',
          label_zh: '期权链',
          level: 'fail',
          newest_value: 'rows 112,004',
          newest_day: '2026-11-16',
          newest_subject: '',
          age_text: '1 s',
          failed_summary: 'age; crossed 2.1%',
          checks: [
            {
              rule: 'crossed',
              label: 'Crossed quotes',
              hint_zh: '买价高于卖价的行占比过高。',
              level: 'fail',
              n: 112004,
              bad: 2352,
              share: '0.021',
              warn_at: '0.005',
              fail_at: '0.02',
              threshold_source: 'trial value',
              example: 'NVDA 2026-12-18 C 200',
            },
          ],
        },
      ],
    },
    {
      name: 'Rates and events',
      rows: [
        {
          dataset: 'fx',
          label: 'FX USDJPY',
          label_zh: '汇率',
          level: 'warn',
          newest_value: 'USDJPY 154.20',
          newest_day: '2026-11-17',
          newest_subject: 'USDJPY',
          age_text: '1 s',
          failed_summary: 'age',
          checks: [],
        },
      ],
    },
  ],
  glossary: [
    { term: 'pass', text: '所有检查通过。' },
    { term: 'fail', text: '至少一项检查失败，请到数据健康页修复。' },
  ],
  derived_note: 'Derived data is not checked here.',
  repair_path: '/data-health',
  gaps: [],
} satisfies BaseDataPage
