import type { OptionNoticesPage } from '../../../../src/generated/option_notices_page.v1'
import { ok } from './ok'

// The main account has no capture: the page gap names the cause and there are no blocks.
export const gap = {
  ...ok,
  stale: null,
  run_status: 'partial',
  gaps: [{ kind: 'gap', reason: 'no_capture', text: 'gap The main account has no snapshot' }],
  accounts: [],
} satisfies OptionNoticesPage
