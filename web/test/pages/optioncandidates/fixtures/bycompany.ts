import type { Company, TierRow, TierView, Verdict } from '../../../../src/generated/option_candidates_page.v1'

const company = (key: string, buy: string | null): Company => ({
  key,
  ticker: key,
  spot: '$96.50',
  spot_day: '2026-11-17',
  buy_price: buy,
  buy_price_note: null,
  next_earnings: '2027-01-28',
  counts: 'bc counts',
  spot_text: '$96.50',
  spot_note: `bc ${key} 存档`,
  price_note: buy === null ? 'bc 未设' : null,
  earnings_text: 'bc 2027-01-28',
})

const row = (id: string, co: string, over: Partial<TierRow> = {}): TierRow => ({
  id,
  company: co,
  contract: { name: `${co} put ${id}`, strike: '$85.00', expiry: '12-18', dte: '31d' },
  margin: { main: '6.0%', sub: null, gap: null },
  below_spot: { main: '12.4%', sub: null, gap: null },
  premium: { main: '$42.00', sub: '5.8%/yr', gap: null },
  badges: [],
  notes: [],
  fields: [],
  line: null,
  ...over,
})

const verdict = (id: Verdict['id'], word: string, glyph: string, tone: Verdict['tone']): Verdict => ({ id, word, glyph, tone })

// Synthetic by_company TierView written from the schema: four companies, one per verdict that matters.
export const bycompany: TierView = {
  chain_day: '2026-11-17',
  companies: [company('AAA', '$90.00'), company('BBB', '$80.00'), company('CCC', '$95.00'), company('DDD', null)],
  common: [],
  kept: null,
  kept_note: null,
  stored_title: 'bc 已存表格',
  idle_note: 'bc 空闲行：ZZA',
  buy_hint: 'bc 买入价提示',
  strip_labels: {
    spot: 'bc 现价',
    buy_price: 'bc 买入价',
    next_earnings: 'bc 下次财报',
    contracts: 'bc 合约',
    fits: 'bc 适合',
    check: 'bc 待查',
    grey: 'bc 折叠',
    grey_hint: 'bc 折叠提示',
  },
  card_note: 'bc 卡片说明',
  heads: { companies: 'bc 公司', companies_note: 'bc 旧公司说明', tiers: 'bc 合约分档' },
  legend: [{ tier: 'fits', title: 'bc old legend', text: 'bc 旧图例' }],
  columns: [{ id: 'contract', label: 'bc old col', hint: null }],
  tiers: [
    {
      id: 'fits',
      word: 'bc Fits',
      icon: 'check',
      title: 'bc fits title',
      explain: 'bc fits explain',
      collapsed: false,
      rows: [
        row('a2', 'AAA', { tier: { id: 'fits', word: 'bc 适合', glyph: '✓' } }),
        row('a1', 'AAA', { tier: { id: 'fits', word: 'bc 适合', glyph: '✓' } }),
      ],
      companies: [{ company: 'AAA', count: 2, row_ids: ['a2', 'a1'] }],
    },
    {
      id: 'check',
      word: 'bc Check',
      icon: 'alert',
      title: 'bc check title',
      explain: 'bc check explain',
      collapsed: false,
      rows: [row('a3', 'AAA', { tier: { id: 'check', word: 'bc 待查', glyph: '!' } })],
    },
    {
      id: 'grey_above_buy_price',
      word: 'bc Grey',
      icon: 'minus',
      title: 'bc grey title',
      explain: 'bc grey explain',
      collapsed: true,
      rows: [
        row('a4', 'AAA', { line: 'bc 折叠行 a4' }),
        row('b1', 'BBB', { line: 'bc 折叠行 b1' }),
        row('c1', 'CCC', { line: 'bc 折叠行 c1' }),
      ],
      companies: [
        { company: 'AAA', count: 1, row_ids: ['a4'] },
        { company: 'BBB', count: 1, row_ids: ['b1'] },
        { company: 'CCC', count: 1, row_ids: ['c1'] },
      ],
    },
  ],
  by_company: {
    heading_note: 'bc 新公司说明',
    legend: [
      { tier: 'fits', title: 'bc new legend', text: 'bc 年化 图例' },
      { tier: 'grey', title: 'bc new grey', text: 'bc 折叠 图例' },
    ],
    columns: [
      { id: 'arrow', label: 'bc new col arrow', hint: null },
      { id: 'tier', label: 'bc new col tier', hint: null },
      { id: 'contract', label: 'bc new col contract', hint: null },
      { id: 'margin', label: 'bc new col margin', hint: null },
      { id: 'below_spot', label: 'bc new col below', hint: null },
      { id: 'premium', label: 'bc new col premium', hint: null },
      { id: 'badges', label: 'bc new col badges', hint: null },
    ],
    blocks: [
      {
        company: 'AAA',
        verdict: verdict('sellable', 'bc 有可卖', '✓', 'success'),
        verdict_text: 'bc AAA 可卖两行',
        best_yield: 'bc 12.0%/yr',
        row_ids: ['a1', 'a2', 'a3'],
        folded: { title: 'bc 另有 1 个合约', summary: 'bc 价格过高 1', reasons: [{ code: 'out_band', label: 'bc 区间外', count: '1' }], row_ids: ['a4'] },
      },
      {
        company: 'BBB',
        verdict: verdict('above_buy', 'bc 价外高于买入价', '–', 'warn'),
        verdict_text: 'bc BBB 保本点高于买入价',
        best_yield: null,
        row_ids: [],
        folded: { title: 'bc 另有 1 个合约', summary: 'bc 高于买入价 1', reasons: [{ code: 'above_buy', label: 'bc 高于', count: '1' }], row_ids: ['b1'] },
      },
      {
        company: 'CCC',
        verdict: verdict('out_band', 'bc 区间外', '–', 'muted'),
        verdict_text: 'bc CCC 全部在区间外',
        best_yield: null,
        row_ids: [],
        folded: { title: 'bc 另有 1 个合约', summary: 'bc 区间外 1', reasons: [{ code: 'out_band', label: 'bc 区间外', count: '1' }], row_ids: ['c1'] },
      },
      {
        company: 'DDD',
        verdict: verdict('no_buy_price', '未判定', '–', 'muted'),
        verdict_text: 'bc DDD 未设买入价',
        best_yield: null,
        row_ids: [],
        folded: null,
      },
    ],
  },
}

// The same payload without the by_company layout: the tier sections draw, each with its companies lines.
export const tierCompanies: TierView = structuredClone(bycompany)
delete tierCompanies.by_company
