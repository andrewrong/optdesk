import type { OptionPnlPage } from '../../../../src/generated/option_pnl_page.v1'
import { ok } from './ok'

// A lot split on the ledger alone: no split lines, only the ledger and the commission.
export const noQuotes = {
  ...ok,
  run_id: 'run-no-quotes',
  accounts: [
    {
      ...ok.accounts[0],
      lots: [
        {
          id: 'lot-ddd',
          title: 'Synthetic DDD put 70.00',
          summary: 'Synthetic DDD summary',
          lines: [
            { id: 'ledger', label: 'Ledger says', text: '$12.00', hint: null, tone: 'none' },
            { id: 'commission', label: 'Commission', text: '$1.30', hint: null, tone: 'none' },
          ],
          days_line: { id: 'days', label: 'Days', text: 'Synthetic no quote days', hint: null, tone: 'none' },
          iv_line: null,
          end_line: null,
          recon: [],
          badges: [],
          gap: null,
        },
      ],
    },
  ],
} satisfies OptionPnlPage
