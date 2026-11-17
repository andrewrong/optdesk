import type { OptionPnlPage } from '../../../../src/generated/option_pnl_page.v1'
import { ok } from './ok'

// The page level gaps and the stale label, and an account that could not be built.
export const gap = {
  ...ok,
  run_id: 'run-gap',
  run_status: 'partial',
  stale: { data_time: '2026-01-01T09:00:00+08:00', is_stale: true, text: 'Synthetic stale label text' },
  gaps: [{ kind: 'gap', reason: 'stale_input', text: 'Synthetic page gap text' }],
  accounts: [
    {
      alias: 'acct-b',
      sample_note: 'Synthetic gap sample note',
      books: [],
      lots: [],
      gap: { kind: 'gap', reason: 'no_data', text: 'Synthetic account gap cause' },
    },
  ],
} satisfies OptionPnlPage
