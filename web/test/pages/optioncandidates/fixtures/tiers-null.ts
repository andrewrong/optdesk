import type { TierView } from '../../../../src/generated/option_candidates_page.v1'
import { ok } from './ok'
import { tiers } from './tiers'

// The same page with the tier field null.
export const tiersNull = { ...ok, short_put_tiers: null }

// The tier view with the three optional texts null: no buy hint, no idle line, no spot hint.
export const tiersTextsNull: TierView = { ...tiers, idle_note: null, buy_hint: null, spot_hint: null }

// The tier view of an older stored page: no `heads` key.
export const tiersNoHeads: TierView = { ...tiers, heads: undefined }
