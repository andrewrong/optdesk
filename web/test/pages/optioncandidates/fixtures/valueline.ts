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
}

export const noReviewLine: ValueLine = {
  ...valueLine,
  text: 'valueline 没有有效的估值复核',
  hint: null,
  reason: 'no_review',
  gate_text: 'not set',
  gate_hint: null,
}

export const withLine = base('AAA', { valuation: valueLine })
export const withReason = base('BBB', {
  buy_price: null,
  buy_price_note: 'not set',
  valuation: noReviewLine,
})
export const withNull = base('CCC', { valuation: null })
export const withoutKey = base('CCC')

const badge = (code: string, label: string, gloss: string, tone: 'warn' | 'info' | 'none') => ({ code, label, gloss, tone })
export const valuationSource = base('AAA', {
  buy_price: '$91.20',
  buy_source: badge('valuation', 'Valuation buy zone', 'valueline 来源：估值买入区上沿', 'info'),
})
export const handSource = base('BBB', {
  buy_source: badge('hand', 'Hand price', 'valueline 来源：手工买入价', 'none'),
})
export const notJudgedSource = base('CCC', {
  buy_price: null,
  buy_price_note: 'not judged',
  buy_source: badge('none', 'Not judged', 'valueline 来源：没有买入价，不评判', 'none'),
})
