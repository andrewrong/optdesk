import type { HouseholdRisk } from '../../../../src/generated/household_risk.v1'

// Synthetic fixture written from contracts/schemas/page.household_risk.v1.json alone.
export const ok = {
  schema: 'household_risk',
  schema_version: 1,
  run_id: 'run-ok',
  build_id: 'build-ok',
  scope: 'household',
  generated_at: '2026-01-05T09:30:00+08:00',
  data_times: [{ source: 'positions', data_time: '2026-01-05T09:00:00+08:00' }],
  run_status: 'ok',
  title: 'Household risk ok',
  sentence: 'Sentence ok: the book leans long and short vega is small.',
  vega_title: 'Vega by expiry ok',
  delta: {
    title: 'Delta by underlying ok',
    meaning: 'Meaning of delta ok: dollars gained per one dollar move.',
    heads: ['Underlying', 'Delta'],
    rows: [
      { name: 'Alpha Corp ok', delta: { value: 1200.5, text: 'Delta plus 1,200 alpha ok', unit: 'usd_signed', currency: 'USD', tone: 'up' } },
      { name: 'Beta Corp ok', delta: { value: -300, text: 'Delta minus 300 beta ok', unit: 'usd_signed', currency: 'USD', tone: 'down', approx_usd: { value: 300, text: 'About 300 dollars ok', unit: 'USD' } } },
    ],
    total: { value: 900.5, text: 'Delta total 900 ok', unit: 'usd_signed', currency: 'USD', tone: 'up' },
    
  },
  beta_delta: {
    title: 'Beta weighted delta ok',
    meaning: 'Meaning of beta delta ok: delta scaled to the index.',
    benchmark_text: 'Benchmark text ok: SPY over one year.',
    heads: ['Name', 'Beta', 'Weighted delta'],
    rows: [
      { name: 'Alpha Corp beta row ok', beta: { value: 1.2, text: 'Beta 1.2 alpha ok', unit: 'beta' }, weighted_delta: { value: 1440, text: 'Weighted 1,440 alpha ok', unit: 'usd_signed', currency: 'USD', tone: 'up' } },
    ],
    total: { value: 1440, text: 'Weighted total 1,440 ok', unit: 'usd_signed', currency: 'USD', tone: 'up' },
  },
  sigma: {
    title: 'Sigma moves ok',
    meaning: 'Meaning of sigma ok: gain or loss for a move of n sigma.',
    columns: ['Minus two ok', 'Minus one ok', 'Plus one ok', 'Plus two ok'],
    aria_text: 'Aria text sigma ok',
    rows: [
      {
        name: 'Alpha Corp sigma row ok',
        cells: [
          { value: -500, text: 'Sigma cell a ok', unit: 'usd_signed', currency: 'USD', tone: 'down' },
          { value: -250, text: 'Sigma cell b ok', unit: 'usd_signed', currency: 'USD', tone: 'down' },
          { value: 250, text: 'Sigma cell c ok', unit: 'usd_signed', currency: 'USD', tone: 'up' },
          { value: 500, text: 'Sigma cell d ok', unit: 'usd_signed', currency: 'USD', tone: 'up' },
        ],
      },
    ],
    scale_low: { value: -500, text: 'Scale low ok', unit: 'usd_signed', currency: 'USD' },
    scale_high: { value: 500, text: 'Scale high ok', unit: 'usd_signed', currency: 'USD' },
    
  },
  vega: {
    meaning: 'Meaning of vega ok: dollars per vol point.',
    heads: ['Expiry', 'Vega'],
    rows: [{ expiry_text: 'Expiry March ok', vega: { value: -40, text: 'Vega minus 40 ok', unit: 'usd_per_vol_point' } }],
    total: { value: -40, text: 'Vega total minus 40 ok', unit: 'usd_per_vol_point' },
  },
  standard_metrics: {
    title: 'Standard metrics ok',
    metrics: [
      {
        label: 'Metric label one ok',
        value_text: 'Metric value one ok',
        explanation: 'Metric explanation one ok',
        table: {
          columns: ['Metric column a ok', 'Metric column b ok'],
          rows: [{ cells: ['Metric cell a ok', 'Metric cell b ok'] }],
        },
      },
      { label: 'Metric label two ok', value_text: 'Metric value two ok', explanation: 'Metric explanation two ok', table: null },
    ],
  },
  notes: ['Note one ok', 'Note two ok'],
  gaps: [],
  stale: {
    data_time: '2026-01-05T09:00:00+08:00',
    stale_after: '2026-01-05T15:00:00+08:00',
    text: 'Stale label ok: data is current.',
  },
} satisfies HouseholdRisk
