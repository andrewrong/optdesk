import type { OptionPnlPage } from '../../../../src/generated/option_pnl_page.v1'
import { ok } from './ok'

// One lot that could not be split: a gap and no lines.
export const lotGap = {
  ...ok,
  run_id: 'run-lot-gap',
  accounts: [
    {
      ...ok.accounts[0],
      lots: [
        {
          id: 'lot-eee',
          title: 'Synthetic EEE put 50.00',
          summary: 'Synthetic EEE summary',
          lines: [],
          days_line: { id: 'days', label: 'Days', text: 'Synthetic gap days', hint: null, tone: 'none' },
          iv_line: null,
          end_line: null,
          recon: [],
          badges: [],
          gap: { kind: 'gap', reason: 'no_data', text: 'Synthetic lot gap reason' },
        },
      ],
    },
  ],
} satisfies OptionPnlPage
