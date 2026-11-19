import type { OptionCandidatesPage, ValueCell } from '../../../../src/generated/option_candidates_page.v1'
import { ok } from './ok'
import { shortlist } from './shortlist'
import { tiers } from './tiers'

const cell = (text: string, value: number | null = null): ValueCell => ({ text, value, badges: [], tone: 'none' })
const columns = [
  { id: 'company', label: 'jp Company', align: 'left' as const, hint: null },
  { id: 'yield', label: 'jp annualised', align: 'right' as const, hint: null },
]

// Synthetic document with a JPY short-put group after the US one; every text is a field of the document.
export const jp = {
  ...ok,
  groups: [
    ok.groups[0],
    {
      kind: 'short_put',
      title: 'jp Sell put JPY',
      day: '2026-11-10',
      stale: false,
      counts: { ok: 1, blocked: 0, unknown: 0 },
      main: {
        columns,
        rows: [{ id: 'j1', cells: { company: cell('jp Company X'), yield: cell('4.0000%', 0.04) }, reasons: [] }],
      },
      other: null,
      gap: null,
      currency: 'JPY',
      note: 'jp 日本：金额为日元的说明',
      limit_counts: [
        { limit: 'jp_limit_a', count: 3, text: 'jp 涨跌幅限制 A 共 3 条' },
        { limit: 'jp_limit_b', count: 1, text: 'jp 涨跌幅限制 B 共 1 条' },
      ],
    },
    ok.groups[1],
  ],
  short_put_list_jpy: { ...shortlist, counts: 'jp yen list counts', run_day: 'jp run day' },
  short_put_tiers_jpy: { ...tiers, kept: 'jp yen tiers kept' },
} as unknown as OptionCandidatesPage
