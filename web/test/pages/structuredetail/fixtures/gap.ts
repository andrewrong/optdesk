// Synthetic fixture: the figures that cannot be computed are named gaps, never zeros.
import type { StructureDetail } from '../../../../src/generated/structure_detail.v3'
import { ok } from './ok'

export const gap = {
  ...ok,
  run_status: 'partial',
  tiles: {
    ...ok.tiles,
    pnl: { label: 'Gap tile label', caption: 'Gap tile caption', figure: { reason: 'missing_quote', text: 'Gap tile figure: no quote' } },
  },
  legs: [{ reason: 'missing_quote', text: 'Gap leg: no quote for this leg' }],
  legs_total: { label: 'Gap total label', pnl: { reason: 'missing_quote', text: 'Gap total: a leg has no quote' } },
  greeks: { reason: 'no_data', text: 'Gap greeks: no model inputs' },
  payoff: { reason: 'missing_input', text: 'Gap payoff: no spot price' },
  gaps: [
    { reason: 'missing_quote', text: 'Gap line one: the short leg has no quote' },
    { reason: 'stale', text: 'Gap line two: the spot is stale' },
  ],
} satisfies StructureDetail
