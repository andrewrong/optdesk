import type { HouseholdRisk } from '../../../../src/generated/household_risk.v1'

// Synthetic fixture written from contracts/schemas/page.household_risk.v1.json alone.
export const gap = {
  schema: 'household_risk',
  schema_version: 1,
  run_id: 'run-gap',
  build_id: 'build-gap',
  scope: 'household',
  generated_at: '2026-01-05T09:30:00+08:00',
  data_times: [{ source: 'positions', data_time: '2026-01-05T09:00:00+08:00' }],
  run_status: 'ok',
  title: 'Household risk gap',
  sentence: 'Sentence gap: the book leans long and short vega is small.',
  vega_title: 'Vega by expiry gap',
  delta: {
    title: 'Delta by underlying gap',
    meaning: 'Meaning of delta gap: dollars gained per one dollar move.',
    heads: ['Underlying', 'Delta'],
    rows: [
      { name: 'Alpha Corp gap', delta: { value: 1200.5, text: 'Delta plus 1,200 alpha gap', unit: 'usd_signed', currency: 'USD', tone: 'up' } },
      { name: 'Beta Corp gap', delta: { value: -300, text: 'Delta minus 300 beta gap', unit: 'usd_signed', currency: 'USD', tone: 'down', approx_usd: { value: 300, text: 'About 300 dollars gap', unit: 'USD' } } },
    ],
    total: { value: 900.5, text: 'Delta total 900 gap', unit: 'usd_signed', currency: 'USD', tone: 'up' },
    gap: { reason: 'missing_quote', text: 'Delta gap text: no quote for Beta Corp.' },
  },
  beta_delta: {
    title: 'Beta weighted delta gap',
    meaning: 'Meaning of beta delta gap: delta scaled to the index.',
    benchmark_text: 'Benchmark text gap: SPY over one year.',
    heads: ['Name', 'Beta', 'Weighted delta'],
    rows: [
      { name: 'Alpha Corp beta row gap', beta: { value: 1.2, text: 'Beta 1.2 alpha gap', unit: 'beta' }, weighted_delta: { value: 1440, text: 'Weighted 1,440 alpha gap', unit: 'usd_signed', currency: 'USD', tone: 'up' } },
    ],
    total: { value: 1440, text: 'Weighted total 1,440 gap', unit: 'usd_signed', currency: 'USD', tone: 'up' },
  },
  sigma: {
    title: 'Sigma moves gap',
    meaning: 'Meaning of sigma gap: gain or loss for a move of n sigma.',
    columns: ['Minus two gap', 'Minus one gap', 'Plus one gap', 'Plus two gap'],
    aria_text: 'Aria text sigma gap',
    rows: [
      {
        name: 'Alpha Corp sigma row gap',
        cells: [
          { value: -500, text: 'Sigma cell a gap', unit: 'usd_signed', currency: 'USD', tone: 'down' },
          { value: -250, text: 'Sigma cell b gap', unit: 'usd_signed', currency: 'USD', tone: 'down' },
          { value: 250, text: 'Sigma cell c gap', unit: 'usd_signed', currency: 'USD', tone: 'up' },
          { value: 500, text: 'Sigma cell d gap', unit: 'usd_signed', currency: 'USD', tone: 'up' },
        ],
      },
    ],
    scale_low: { value: -500, text: 'Scale low gap', unit: 'usd_signed', currency: 'USD' },
    scale_high: { value: 500, text: 'Scale high gap', unit: 'usd_signed', currency: 'USD' },
    gap: { reason: 'no_data', text: 'Sigma gap text: no history to size a move.' },
  },
  vega: {
    meaning: 'Meaning of vega gap: dollars per vol point.',
    heads: ['Expiry', 'Vega'],
    rows: [{ expiry_text: 'Expiry March gap', vega: { value: -40, text: 'Vega minus 40 gap', unit: 'usd_per_vol_point' } }],
    total: { value: -40, text: 'Vega total minus 40 gap', unit: 'usd_per_vol_point' },
  },
  standard_metrics: {
    title: 'Standard metrics gap',
    metrics: [
      {
        label: 'Metric label one gap',
        value_text: 'Metric value one gap',
        explanation: 'Metric explanation one gap',
        table: {
          columns: ['Metric column a gap', 'Metric column b gap'],
          rows: [{ cells: ['Metric cell a gap', 'Metric cell b gap'] }],
        },
      },
      { label: 'Metric label two gap', value_text: 'Metric value two gap', explanation: 'Metric explanation two gap', table: null },
    ],
  },
  notes: ['Note one gap', 'Note two gap'],
  gaps: [{ reason: 'not_built', text: 'Card gap text: the sigma table is not built.' }],
  stale: {
    data_time: '2026-01-05T09:00:00+08:00',
    stale_after: '2026-01-05T15:00:00+08:00',
    text: 'Stale label gap: data is current.',
  },
} satisfies HouseholdRisk
