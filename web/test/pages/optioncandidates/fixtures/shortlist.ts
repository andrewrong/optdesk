import type { Badge, Pair, ShortList, ShortRow } from '../../../../src/generated/option_candidates_page.v1'

const pair = (main: string, sub: string | null = null): Pair => ({ main, sub, gap: null })
const gapPair = (text: string): Pair => ({ main: text, sub: null, gap: { kind: 'gap', reason: 'no_data', text } })
const badge = (code: string, tone: Badge['tone'] = 'info'): Badge => ({
  code,
  label: `shortlist ${code} label`,
  gloss: `shortlist ${code} 中文注释`,
  tone,
})
const row = (key: string, over: Partial<ShortRow> = {}): ShortRow => ({
  key,
  strike: pair(`$${key}`),
  expiry: pair('12-11', '24d'),
  otm: pair('6.1%'),
  delta: pair('0.22'),
  premium: pair('$37.10', '$3,710'),
  annualized: pair('87.5%'),
  buy: pair('$36.90', 'BE $608 / 5.1%'),
  buy_source: null,
  buy_link: null,
  badges: [],
  ...over,
})

// Synthetic ShortList written from the schema alone: two groups, a gap delta, a buy source and a buy link.
export const shortlist: ShortList = {
  run_day: 'Run day 2026-11-18',
  counts: 'Shown 3 rows, 2 underlyings',
  caveat: 'shortlist caveat 中文风险',
  filters: [
    { id: 'expiry', label: 'Expiry', value: '21-45 d', hint: 'shortlist hint expiry 中文' },
    { id: 'delta', label: '|Delta|', value: '0.15-0.30', hint: 'shortlist hint delta 中文' },
  ],
  dropped: 'Not shown 5',
  dropped_items: [{ code: 'no_bid', label: 'No bid', count: '5', hint: 'shortlist hint no_bid 中文' }],
  columns: [
    { id: 'strike', label: 'Strike', hint: 'shortlist col strike 中文' },
    { id: 'expiry', label: 'Expiry', hint: null },
    { id: 'otm', label: 'OTM %', hint: null },
    { id: 'delta', label: 'Delta', hint: null },
    { id: 'premium', label: 'Premium (bid)', hint: null },
    { id: 'annualized', label: 'Annualized', hint: null },
    { id: 'buy', label: 'Buy price', hint: null },
    { id: 'liquidity', label: 'Liquidity', hint: null },
  ],
  groups: [
    {
      ticker: 'ZZZ',
      spot: 'spot $686.97',
      summary: '2 rows, best 87.5%',
      rows: [
        row('645', { buy_source: badge('hand'), badges: [badge('wide_spread', 'warn')] }),
        row('640', { delta: gapPair('no delta'), buy: gapPair('No buy price'), buy_link: '/plan/price-lines' }),
      ],
    },
    { ticker: 'AAA', spot: 'spot $90.00', summary: '1 row, best 40.0%', rows: [row('85')] },
  ],
  folded_title: 'All 2,343 puts',
}
