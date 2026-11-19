// Synthetic base data page payload with the version 2 slot: one bars row with an R6 cross_check, one row with none.
import type { BaseDataPage } from '../../../../src/generated/base_data_page.v1'

export const crossCheck = {
  schema: 'base_data_page',
  schema_version: 1,
  run_id: 'run-synthetic-0002',
  built_at: '2026-11-18T09:30:00Z',
  as_of: '2026-11-18',
  last_sessions: { XNYS: '2026-11-17', XTKS: '2026-11-18' },
  top: {
    level: 'fail',
    text_zh: '基础数据共 2 项：1 项正常，1 项失败。',
    counts: { pass: 1, warn: 0, fail: 1, unknown: 0 },
  },
  groups: [
    {
      name: 'Market data',
      rows: [
        {
          dataset: 'bars',
          label: 'Daily bars',
          label_zh: '日线',
          level: 'fail',
          newest_value: 'SPY close 590.10',
          newest_day: '2026-11-17',
          newest_subject: 'SPY',
          age_text: '0 s',
          failed_summary: 'R6',
          checks: [],
          cross_check: {
            pair_id: 'R6',
            level: 'fail',
            n: 5,
            matched: 3,
            diff: 2,
            missing: 0,
            skipped: 1,
            tolerance_text: 'close within 0.5 percent',
            source_id: 'yfinance',
            run_at: '2026-11-18T08:00:00Z',
            items: [
              { key: 'AAPL 2026-11-17', ours: '190.10', other: '191.90', diff: '0.9 percent', kind: 'diff', hint_zh: '两个来源不一致', company: 'Apple' },
              { key: 'AAPL 2026-11-16', ours: '189.00', other: '190.50', diff: '0.8 percent', kind: 'diff', hint_zh: '两个来源不一致', company: 'Apple' },
              { key: 'MSFT 2026-11-17', ours: '', other: '410.00', diff: '', kind: 'missing_ours', hint_zh: '我方缺少该日', company: 'Microsoft' },
              { key: 'ZZZ 2026-11-17', ours: '', other: '', diff: '', kind: 'skipped', hint_zh: '来源限流，未比较', company: 'Zeta' },
            ],
          },
        },
        {
          dataset: 'fx',
          label: 'FX USDJPY',
          label_zh: '汇率',
          level: 'pass',
          newest_value: 'USDJPY 154.20',
          newest_day: '2026-11-17',
          newest_subject: 'USDJPY',
          age_text: '1 s',
          failed_summary: '',
          checks: [],
        },
      ],
    },
  ],
  glossary: [{ term: 'pass', text: '所有检查通过。' }],
  derived_note: 'Derived data is not checked here.',
  repair_path: '/data-health',
  gaps: [],
} satisfies BaseDataPage

// A pair whose run failed: unknown, the reason line, no items.
export const failedRun = {
  ...crossCheck,
  groups: [
    {
      name: 'Market data',
      rows: [
        {
          dataset: 'bars',
          label: 'Daily bars',
          label_zh: '日线',
          level: 'unknown',
          newest_value: 'SPY close 590.10',
          newest_day: '2026-11-17',
          newest_subject: 'SPY',
          age_text: '0 s',
          failed_summary: 'R6',
          checks: [],
          cross_check: {
            pair_id: 'R6',
            level: 'unknown',
            n: 0,
            matched: 0,
            diff: 0,
            missing: 0,
            reason: 'rate_limited',
            reason_zh: '来源限流，今日未完成核对',
            tolerance_text: 'close within 0.5 percent',
            source_id: 'yfinance',
            run_at: '2026-11-18T08:00:00Z',
            items: [],
          },
        },
      ],
    },
  ],
} satisfies BaseDataPage
