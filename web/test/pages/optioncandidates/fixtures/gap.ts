import type { OptionCandidatesPage } from '../../../../src/generated/option_candidates_page.v1'
import { ok } from './ok'

// The first group has no rows in its lookback: its cause is shown once and no table.
export const gap = {
  ...ok,
  groups: [
    {
      kind: 'short_put',
      title: 'gap 卖 put',
      day: null,
      stale: false,
      counts: { ok: 0, blocked: 0, unknown: 0 },
      main: null,
      other: null,
      gap: { kind: 'gap' as const, reason: 'no_rows', text: 'gap 七天内没有卖 put 候选行' },
    },
    ok.groups[1],
  ],
} satisfies OptionCandidatesPage
