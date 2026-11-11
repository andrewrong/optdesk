// Synthetic fixture written from contracts/schemas/page.indicator_detail.v2.json alone.
import type { IndicatorDetail } from '../../../../src/generated/indicator_detail.v2'

export const stale = {
  schema: 'indicator_detail',
  schema_version: 2,
  run_id: 'run-1',
  build_id: 'build-1',
  scope: 'macro',
  generated_at: '2026-11-06T20:00:00+08:00',
  data_times: [{ source: 'fred', data_time: '2026-11-06T19:00:00+08:00' }],
  run_status: 'partial',
  slug: 'unemployment',
  title: 'Unemployment rate detail',
  sentence: 'The unemployment rate rose slightly, the highest this year.',
  latest: {
    value: { value: 4.2, text: '4.2%', unit: 'series_value' },
    observed: { day: '2026-10-01', text: 'October data' },
    released: { day: '2026-11-05', text: 'Released 5 Nov' },
  },
  revision: {
    previous: { value: 4.1, text: 'Previous version 4.1%', unit: 'series_value' },
    vintage: { day: '2026-10-01', text: 'Version of 1 Oct' },
    step: { value: 0.1, text: 'Revised up 0.1 percentage points', unit: 'series_change' },
    text: 'Revised up from 4.1% after the October version.',
  },
  reference_line: { label: 'Long-run mean 5.0%', value: { value: 5, text: '5.0%', unit: 'series_value' } },
  series_title: 'Unemployment rate, %',
  explanation_title: 'How to read this indicator',
  meta_line: 'Source FRED · data to 2026-10-01 · on time',
  recent: {
    title: 'Last 12 periods',
    columns: ['Period', 'Released', 'Value', 'Change'],
    rows: [
      { period: '2026-10', released: '11-05', value: '4.2%', change: '+0.1' },
      { period: '2026-09', released: '10-03', value: '4.1%', change: '+0.1' },
      { period: '2026-08', released: '09-05', value: '4.0%', change: 'Flat' },
    ],
  },
  points: [
    { observed_on: '2026-08-01', value: 4.0, text: '4.0%', vintage_on: '2026-09-01', released: { day: '2026-09-05', text: 'Released 5 Sep' } },
    { observed_on: '2026-09-01', value: 4.1, text: '4.1%', vintage_on: '2026-10-01', released: { day: '2026-10-03', text: 'Released 3 Oct' } },
    { observed_on: '2026-10-01', value: 4.2, text: '4.2%', vintage_on: '2026-11-01', released: { reason: 'no_data', text: 'Release day not recorded' } },
  ],
  explanation: ['The unemployment rate counts people looking for work.', 'A rise of 0.1 percentage points is normal noise.'],
  gaps: [{ reason: 'stale', text: 'This indicator was not updated on time.' }],
  stale: { data_time: '2026-11-06T19:00:00+08:00', stale_after: '2026-11-01T20:00:00+08:00', text: 'Out of date: data past its limit' },
} satisfies IndicatorDetail
