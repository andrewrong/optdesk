import type { HouseholdRisk } from '../../../../src/generated/household_risk.v1'

// Synthetic fixture written from contracts/schemas/page.household_risk.v1.json alone.
export const stale = {
  schema: 'household_risk',
  schema_version: 1,
  run_id: 'run-stale',
  build_id: 'build-stale',
  scope: 'household',
  generated_at: '2026-01-05T09:30:00+08:00',
  data_times: [{ source: 'positions', data_time: '2026-01-05T09:00:00+08:00' }],
  run_status: 'ok',
  title: 'Household risk stale',
  sentence: 'Sentence stale: the book leans long and short vega is small.',
  vega_title: 'Vega by expiry stale',
  delta: {
    title: 'Delta by underlying stale',
    meaning: 'Meaning of delta stale: dollars gained per one dollar move.',
    heads: ['Underlying', 'Delta'],
    rows: [
      { name: 'Alpha Corp stale', delta: { value: 1200.5, text: 'Delta plus 1,200 alpha stale', unit: 'usd_signed', currency: 'USD', tone: 'up' } },
      { name: 'Beta Corp stale', delta: { value: -300, text: 'Delta minus 300 beta stale', unit: 'usd_signed', currency: 'USD', tone: 'down', approx_usd: { value: 300, text: 'About 300 dollars stale', unit: 'USD' } } },
    ],
    total: { value: 900.5, text: 'Delta total 900 stale', unit: 'usd_signed', currency: 'USD', tone: 'up' },
    
  },
  beta_delta: {
    title: 'Beta weighted delta stale',
    meaning: 'Meaning of beta delta stale: delta scaled to the index.',
    benchmark_text: 'Benchmark text stale: SPY over one year.',
    heads: ['Name', 'Beta', 'Weighted delta'],
    rows: [
      { name: 'Alpha Corp beta row stale', beta: { value: 1.2, text: 'Beta 1.2 alpha stale', unit: 'beta' }, weighted_delta: { value: 1440, text: 'Weighted 1,440 alpha stale', unit: 'usd_signed', currency: 'USD', tone: 'up' } },
    ],
    total: { value: 1440, text: 'Weighted total 1,440 stale', unit: 'usd_signed', currency: 'USD', tone: 'up' },
  },
  sigma: {
    title: 'Sigma moves stale',
    meaning: 'Meaning of sigma stale: gain or loss for a move of n sigma.',
    columns: ['Minus two stale', 'Minus one stale', 'Plus one stale', 'Plus two stale'],
    aria_text: 'Aria text sigma stale',
    rows: [
      {
        name: 'Alpha Corp sigma row stale',
        cells: [
          { value: -500, text: 'Sigma cell a stale', unit: 'usd_signed', currency: 'USD', tone: 'down' },
          { value: -250, text: 'Sigma cell b stale', unit: 'usd_signed', currency: 'USD', tone: 'down' },
          { value: 250, text: 'Sigma cell c stale', unit: 'usd_signed', currency: 'USD', tone: 'up' },
          { value: 500, text: 'Sigma cell d stale', unit: 'usd_signed', currency: 'USD', tone: 'up' },
        ],
      },
    ],
    scale_low: { value: -500, text: 'Scale low stale', unit: 'usd_signed', currency: 'USD' },
    scale_high: { value: 500, text: 'Scale high stale', unit: 'usd_signed', currency: 'USD' },
    
  },
  vega: {
    meaning: 'Meaning of vega stale: dollars per vol point.',
    heads: ['Expiry', 'Vega'],
    rows: [{ expiry_text: 'Expiry March stale', vega: { value: -40, text: 'Vega minus 40 stale', unit: 'usd_per_vol_point' } }],
    total: { value: -40, text: 'Vega total minus 40 stale', unit: 'usd_per_vol_point' },
  },
  standard_metrics: {
    title: 'Standard metrics stale',
    metrics: [
      {
        label: 'Metric label one stale',
        value_text: 'Metric value one stale',
        explanation: 'Metric explanation one stale',
        table: {
          columns: ['Metric column a stale', 'Metric column b stale'],
          rows: [{ cells: ['Metric cell a stale', 'Metric cell b stale'] }],
        },
      },
      { label: 'Metric label two stale', value_text: 'Metric value two stale', explanation: 'Metric explanation two stale', table: null },
    ],
  },
  notes: ['Note one stale', 'Note two stale'],
  gaps: [],
  stale: {
    data_time: '2026-01-05T09:00:00+08:00',
    stale_after: '2026-01-05T15:00:00+08:00',
    text: 'Stale label stale: data is older than its limit.',
  },
} satisfies HouseholdRisk
