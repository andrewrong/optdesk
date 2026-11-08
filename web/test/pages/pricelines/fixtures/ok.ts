import type { PriceLinesPage } from '../../../../src/generated/price_lines_page.v1'

// Synthetic fixture written from contracts/schemas/page.price_lines_page.v1.json alone.
export const ok = {
  schema: 'price_lines_page',
  schema_version: 1,
  run_id: 'run-ok',
  build_id: 'build-ok',
  scope: 'plan',
  generated_at: '2026-01-05T09:30:00+08:00',
  data_times: [{ source: 'plan', data_time: '2026-01-05T09:00:00+08:00' }],
  run_status: 'ok',
  stale: null,
  title: 'Price lines title ok',
  sentence: 'Price lines sentence ok',
  notes: ['Price lines note ok'],
  gaps: [],
  intro: 'Intro ok: the lines the owner keeps.',
  saved: 'Saved message ok for Alpha.',
  error: null,
  form: {
    post_path: '/plan/price-lines',
    expires: '1767576000',
    mac: 'a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90',
  },
  rows: [
    { symbol: 'ALPHA', market: 'US', price: 'Price 130.00 alpha ok', note: 'Note alpha ok' },
    { symbol: 'BETA', market: 'JP', price: 'Price 2,400 beta ok', note: null },
  ],
  add: { markets: ['US', 'JP'], hint: 'Add hint ok' },
  unset: [
    { symbol: 'GAMMA', market: 'US' },
    { symbol: 'DELTA', market: 'JP' },
  ],
  footnote: 'Footnote ok',
} satisfies PriceLinesPage
