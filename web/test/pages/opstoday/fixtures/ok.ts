// Synthetic ops today page payload written from contracts/schemas/page.ops_today_page.v1.json.
import type { OpsTodayPage } from '../../../../src/generated/ops_today_page.v1'

export const ok = {
  schema: 'ops_today_page',
  schema_version: 1,
  run_id: 'run-1',
  build_id: 'build-1',
  scope: 'ops',
  generated_at: '2026-11-12T01:00:00Z',
  data_times: [{ data_time: '2026-11-12T00:59:00Z', source: 'ops.capture_run' }],
  run_status: 'ok',
  stale: null,
  title: 'Runs today',
  notes: ['Rows are the newest run of each key.'],
  gaps: [],
  day: '2026-11-12',
  today: {
    rows: [
      {
        dataset: 'master.option_list',
        key: 'master.option_list/us',
        market: 'US',
        status: 'ok',
        error_code: null,
        reason_code: null,
        returned_n: 12,
        usable_n: 0,
        detail: null,
      },
      {
        dataset: 'plan.option_open_item',
        key: 'plan.option_open_item/jp',
        market: 'JP',
        status: 'failed',
        error_code: 'E_PLAN',
        reason_code: 'no_chain',
        returned_n: 0,
        usable_n: 0,
        detail: 'AAPL 2026-12-18 outside the range',
      },
      {
        dataset: 'ops.not_wired',
        key: 'ops.not_wired/job-1',
        market: 'US',
        status: 'not_wired',
        error_code: null,
        reason_code: 'not_wired',
        returned_n: 0,
        usable_n: 0,
        detail: 'OT_PG_DSN_OPTION_OPEN',
      },
    ],
  },
} satisfies OpsTodayPage

export const stale = {
  ...ok,
  stale: { data_time: '2026-11-11T00:00:00Z', is_stale: true, text: 'Runs are from yesterday.' },
} satisfies OpsTodayPage
