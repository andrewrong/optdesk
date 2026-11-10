import type { OptionCandidatesPage } from '../../../../src/generated/option_candidates_page.v1'
import { ok } from './ok'

// A page whose only group has nothing that passed: the second table alone, a block and an unknown reason.
export const blocked = {
  ...ok,
  groups: [{ ...ok.groups[0], counts: { ok: 0, blocked: 1, unknown: 1 }, main: null }],
} satisfies OptionCandidatesPage
