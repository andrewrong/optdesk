import type { OptionTrackingPage } from '../../../../src/generated/option_tracking_page.v1'

export const gap = {
  schema: 'option_tracking_page',
  schema_version: 1,
  run_id: 'run-gap',
  build_id: 'build-gap',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [],
  run_status: 'partial',
  stale: null,
  title: 'Synthetic tracking gap',
  notes: [],
  gaps: [],
  as_of: 'Synthetic as-of text',
  mark_note: 'Synthetic mark note',
  accounts: [
    {
      alias: 'acct-b',
      title: 'Synthetic account B',
      mark_day: null,
      guard_day: null,
      long_legs_left_out: 0,
      open: null,
      recent: null,
      recon: null,
      gap: { kind: 'gap', reason: 'no_data', text: 'Synthetic account gap cause' },
    },
  ],
} satisfies OptionTrackingPage
