import type { OptionNoticesPage } from '../../../../src/generated/option_notices_page.v1'
import { ok } from './ok'

// The second alias has no stored capture: a no_capture block with its note and no table.
export const secondMissing = {
  ...ok,
  stale: null,
  accounts: [
    ok.accounts[0]!,
    {
      alias: 'miss-second',
      main: false,
      status: 'no_capture',
      data_day: null,
      notices: [],
      cash_by_date: [],
      cushion_text: null,
      note: 'miss no snapshot of this account in the last 7 days',
    },
  ],
} satisfies OptionNoticesPage
