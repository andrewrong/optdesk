import type { TierView, Badge, Field, Pair, TierRow } from '../../../../src/generated/option_candidates_page.v1'

const badge = (code: string, tone: Badge['tone'] = 'warn'): Badge => ({
  code,
  label: `tiers ${code} label`,
  gloss: `tiers ${code} 中文注释`,
  tone,
})
const pair = (main: string, sub: string | null = null): Pair => ({ main, sub, gap: null })
const field = (id: string, over: Partial<Field> = {}): Field => ({
  id,
  label: `tiers field ${id}`,
  hint: `tiers hint ${id} 中文`,
  value: `tiers value ${id}`,
  gap: null,
  badges: [],
  ...over,
})
const row = (id: string, over: Partial<TierRow> = {}): TierRow => ({
  id,
  company: 'AAA',
  contract: { name: `AAA put ${id}`, strike: '$85.00', expiry: '12-18', dte: '31d' },
  margin: pair('6.0%', 'BE $84.58 vs $90.00'),
  below_spot: pair('12.4%', '1.3 moves*'),
  premium: pair('$42.00', '5.8%/yr'),
  badges: [],
  notes: [],
  fields: [],
  line: null,
  ...over,
})

// Synthetic TierView written from the schema alone: fits, check and one grey group.
export const tiers: TierView = {
  chain_day: '2026-11-17',
  companies: [
    {
      key: 'AAA',
      ticker: 'AAA',
      spot: '$96.50',
      spot_day: '2026-11-17',
      buy_price: '$90.00',
      buy_price_note: null,
      buy_source: { code: 'hand', label: 'Hand price', gloss: 'tiers 来源：手工买入价', tone: 'none' },
      next_earnings: '2027-01-28',
      counts: '2 fit, 1 check, 1 collapsed',
    },
  ],
  common: [{ code: 'earnings_unconfirmed', label: 'tiers Earnings unconfirmed', gloss: 'tiers 财报日没有人工核实' }],
  kept: '未进入 Fits 的原因（行数）：没有买入价 1',
  kept_note: 'tiers 一行可以有几个原因',
  stored_title: 'tiers 原始表格',
  idle_note: 'tiers 2 家公司没有买入价和估值档案：ZZA, ZZB',
  buy_hint: 'tiers 未设买入价，请到 Companies > Watch list 的 Buy 列填写',
  spot_hint: 'tiers 现价是任务运行时取的价格，不是当日收盘',
  legend: [
    { tier: 'fits', title: 'tiers legend Fits', text: 'tiers 满足你的全部条件' },
    { tier: 'check', title: 'tiers legend Check', text: 'tiers 有需要再看一眼的徽标' },
    { tier: 'grey', title: 'tiers legend Not judged', text: 'tiers 未评判' },
    { tier: 'cash', title: 'tiers legend Cash', text: 'tiers 现金：每一行单独对照' },
  ],
  columns: [
    { id: 'arrow', label: 'tiers col arrow', hint: null },
    { id: 'tier', label: 'tiers col tier', hint: 'tiers col tier hint' },
    { id: 'contract', label: 'tiers col contract', hint: null },
    { id: 'margin', label: 'tiers col margin', hint: 'tiers col margin hint' },
    { id: 'below_spot', label: 'tiers col below spot', hint: null },
    { id: 'premium', label: 'tiers col premium', hint: null },
    { id: 'badges', label: 'tiers col badges', hint: null },
  ],
  tiers: [
    {
      id: 'fits',
      word: 'tiers Fits',
      icon: 'check',
      title: '2 tiers fit your conditions',
      explain: 'tiers 满足你的全部条件',
      collapsed: false,
      rows: [
        row('f1', {
          badges: [badge('thin_premium', 'info')],
          premium: pair('−$1,234.50', '5.8%/yr'),
          fields: [
            field('iv'),
            field('delta', { value: null, gap: { kind: 'gap', reason: 'no_data', text: 'tiers gap no delta' } }),
            field('evidence', { badges: [badge('as_of', 'info')] }),
          ],
        }),
        row('f2', { margin: { main: 'tiers gap margin text', sub: null, gap: { kind: 'gap', reason: 'no_data', text: 'tiers gap margin text' } } }),
      ],
    },
    {
      id: 'check',
      word: 'tiers Check',
      icon: 'alert',
      title: '1 tiers to check',
      explain: 'tiers 有需要再看一眼的徽标',
      collapsed: false,
      rows: [row('c1', { badges: [badge('no_open_interest')], fields: [field('iv')] })],
    },
    {
      id: 'grey_in_the_money',
      word: 'tiers Not judged',
      icon: 'minus',
      title: '1 tiers in the money',
      explain: 'tiers 价内',
      collapsed: true,
      rows: [row('g1', { below_spot: { main: 'tiers gap no iv', sub: null, gap: { kind: 'gap', reason: 'no_data', text: 'tiers gap no iv' } }, line: 'tiers grey line AAA $100.00 12-18' })],
    },
  ],
}
