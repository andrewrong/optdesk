import type { Company, ValueLine } from '../../../../src/generated/option_candidates_page.v1'

const base = (key: string, over: Partial<Company> = {}): Company => ({
  key,
  ticker: key,
  spot: '$96.50',
  spot_day: '2026-11-17',
  buy_price: '$90.00',
  buy_price_note: null,
  next_earnings: '2027-01-28',
  counts: '2 fit, 1 check, 1 collapsed',
  tier: { id: 'fits', word: 'valueline 适合', glyph: '✓' },
  count_parts: { fits: '2', check: '1', grey: '1' },
  spot_text: '$96.50',
  spot_note: 'valueline 存档 11-17',
  price_note: null,
  earnings_text: 'valueline 2027-01-28',
  source_badge: null,
  ...over,
})

export const valueLine: ValueLine = {
  label: 'Valuation buy line B_hi',
  text: '$91.20 (review 2026-11-10)',
  hint: 'valueline 提示：B_hi 只是展示',
  reason: null,
  gate_label: 'Buy price used by the gate (hand-typed)',
  gate_text: '$90.00',
  gate_hint: 'valueline 闸门提示：只用手工价',
  line_label: 'valueline 估值买入线',
  line_text: '$91.20',
  run_state: 'used',
  note: 'valueline 复核于 11-10，运行就用它',
}

export const notUsedLine: ValueLine = {
  ...valueLine,
  run_state: 'not_used',
  note: 'valueline 复核于 11-10，最近一次运行没用到',
}

export const noProfileLine: ValueLine = {
  ...valueLine,
  text: 'valueline old no profile',
  hint: null,
  reason: 'no_profile',
  line_text: 'valueline 无估值档案',
  run_state: 'none',
  note: 'valueline 导入估值，或手填买入价',
}

export const stripLabels = {
  spot: 'valueline 现价',
  buy_price: 'valueline 买入价',
  next_earnings: 'valueline 下次财报',
  contracts: 'valueline 合约',
  fits: 'valueline 适合',
  check: 'valueline 待查',
  grey: 'valueline 折叠',
  grey_hint: 'valueline 折叠 = 未判定',
}

export const noReviewLine: ValueLine = {
  ...valueLine,
  text: 'valueline 没有有效的估值复核',
  hint: null,
  reason: 'no_review',
  gate_text: 'not set',
  gate_hint: null,
  line_text: 'valueline 没有有效的估值复核',
  run_state: 'none',
  note: null,
}

export const withLine = base('AAA', { valuation: valueLine })
export const withReason = base('BBB', {
  buy_price: null,
  buy_price_note: 'not set',
  price_note: 'valueline 没有买入价',
  tier: { id: 'grey', word: 'valueline 未判定', glyph: '–' },
  valuation: noReviewLine,
})
export const withNull = base('CCC', { valuation: null })
export const withoutKey = base('CCC')

const badge = (code: string, label: string, gloss: string, tone: 'warn' | 'info' | 'none') => ({ code, label, gloss, tone })
export const valuationSource = base('AAA', {
  buy_price: '$91.20',
  buy_source: badge('valuation', 'Valuation buy zone', 'valueline 来源：估值买入区上沿', 'info'),
  source_badge: badge('valuation', 'Valuation buy zone', 'valueline 来源：估值买入区上沿', 'info'),
})
export const handSource = base('BBB', {
  buy_source: badge('hand', 'Hand price', 'valueline 来源：手工买入价', 'none'),
  source_badge: badge('hand', 'Hand price', 'valueline 来源：手工买入价', 'none'),
})
export const notJudgedSource = base('CCC', {
  buy_price: null,
  buy_price_note: 'not judged',
  price_note: 'valueline 没有买入价',
  buy_source: badge('none', 'Not judged', 'valueline 来源：没有买入价，不评判', 'none'),
  source_badge: null,
})
