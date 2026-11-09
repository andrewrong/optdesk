import type { OptionTrackingPage } from '../../../../src/generated/option_tracking_page.v1'

export const noMarks = {
  schema: 'option_tracking_page',
  schema_version: 1,
  run_id: 'run-nm',
  build_id: 'build-nm',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [{ source: 'src-a', data_time: '2026-01-02T09:00:00+08:00' }],
  run_status: 'partial',
  stale: {
    data_time: '2026-01-01T09:00:00+08:00',
    is_stale: true,
    text: 'Synthetic stale label text',
  },
  title: 'Synthetic tracking no marks',
  notes: [],
  gaps: [],
  as_of: 'Synthetic as-of text',
  mark_note: 'Synthetic mark note',
  accounts: [
    {
      alias: 'acct-a',
      title: 'Synthetic account A',
      mark_day: null,
      guard_day: null,
      long_legs_left_out: 2,
      gap: null,
      open: {
        columns: [
          { id: 'contract', label: 'Contract col', align: 'left', hint: null },
          { id: 'mid', label: 'Mid col', align: 'right', hint: null },
        ],
        rows: [
          {
            id: 'pos-1',
            cells: {
              contract: { text: 'Unmarked contract', value: null, badges: [], tone: 'none' },
              mid: { kind: 'gap', reason: 'stale', text: 'Synthetic no-mark gap text' },
            },
          },
        ],
      },
      recent: null,
      recon: null,
    },
  ],
} satisfies OptionTrackingPage
