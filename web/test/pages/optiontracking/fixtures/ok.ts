import type { OptionTrackingPage } from '../../../../src/generated/option_tracking_page.v1'

export const ok = {
  schema: 'option_tracking_page',
  schema_version: 1,
  run_id: 'run-ok',
  build_id: 'build-ok',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [{ source: 'src-a', data_time: '2026-01-02T09:00:00+08:00' }],
  run_status: 'ok',
  stale: null,
  title: 'Synthetic tracking title',
  notes: ['Synthetic note one', 'Synthetic note two'],
  gaps: [],
  as_of: 'Synthetic as-of text',
  mark_note: 'Synthetic mark note',
  accounts: [
    {
      alias: 'acct-a',
      title: 'Synthetic account A',
      mark_day: 'Synthetic mark day A',
      guard_day: 'Synthetic guard day A',
      long_legs_left_out: 3,
      gap: null,
      open: {
        columns: [
          { id: 'contract', label: 'Contract col', align: 'left', hint: null },
          { id: 'moneyness', label: 'Moneyness col', align: 'left', hint: null },
          { id: 'dte', label: 'Dte col', align: 'right', hint: null },
          { id: 'guard', label: 'Guard col', align: 'left', hint: null },
        ],
        rows: [
          {
            id: 'pos-2',
            cells: {
              contract: { text: 'Row two contract', value: null, badges: [], tone: 'none' },
              moneyness: { text: 'Row two money', value: null, badges: ['价外'], tone: 'none' },
              dte: { text: '45 days', value: 45, badges: [], tone: 'none' },
              guard: { text: 'Row two guard', value: null, badges: ['未确认'], tone: 'none' },
            },
          },
          {
            id: 'pos-1',
            cells: {
              contract: { text: 'Row one contract', value: null, badges: [], tone: 'none' },
              moneyness: { text: 'Row one money', value: null, badges: ['价内', '在存续期内'], tone: 'down' },
              dte: { text: '9 days', value: 9, badges: [], tone: 'none' },
              guard: { text: 'Row one guard', value: null, badges: ['阻止'], tone: 'none' },
            },
          },
        ],
      },
      recent: {
        columns: [{ id: 'contract', label: 'Recent contract col', align: 'left', hint: null }],
        rows: [
          {
            id: 'pos-9',
            cells: {
              contract: { text: 'Recent row contract', value: null, badges: ['提前'], tone: 'none' },
            },
          },
        ],
      },
      recon: {
        columns: [{ id: 'code', label: 'Check code col', align: 'left', hint: null }],
        rows: [
          {
            id: 'chk-1',
            cells: { code: { text: 'Check row text', value: null, badges: [], tone: 'none' } },
          },
        ],
      },
    },
  ],
} satisfies OptionTrackingPage
