// Synthetic fixture written from contracts/schemas/page.indicator_detail.v1.json alone.
import type { IndicatorDetail } from '../../../../src/generated/indicator_detail.v1'

export const ok = {
  schema: 'indicator_detail',
  schema_version: 1,
  run_id: 'run-1',
  build_id: 'build-1',
  scope: 'macro',
  generated_at: '2026-11-06T20:00:00+08:00',
  data_times: [{ source: 'fred', data_time: '2026-11-06T19:00:00+08:00' }],
  run_status: 'ok',
  slug: 'unemployment',
  title: 'Unemployment rate detail',
  sentence: 'Unemployment edged up to the highest level this year.',
  latest: {
    value: { value: 4.2, text: '4.2 percent', unit: 'series_value' },
    observed: { day: '2026-10-01', text: 'Observed for October' },
    released: { day: '2026-11-05', text: 'Released 5 November' },
  },
  revision: {
    previous: { value: 4.1, text: 'Previous vintage 4.1 percent', unit: 'series_value' },
    vintage: { day: '2026-10-01', text: 'Vintage of 1 October' },
    step: { value: 0.1, text: 'Revised up by 0.1 points', unit: 'series_change' },
    text: 'Revised up from 4.1 percent after the October vintage.',
  },
  reference_line: { label: 'Long-run average 5.0 percent', value: { value: 5, text: '5.0 percent', unit: 'series_value' } },
  series_title: 'Unemployment rate, percent',
  explanation_title: 'How to read this series',
  points: [
    { observed_on: '2026-08-01', value: 4.0, text: '4.0 percent', vintage_on: '2026-09-01', released: { day: '2026-09-05', text: 'Released 5 September' } },
    { observed_on: '2026-09-01', value: 4.1, text: '4.1 percent', vintage_on: '2026-10-01', released: { day: '2026-10-03', text: 'Released 3 October' } },
    { observed_on: '2026-10-01', value: 4.2, text: '4.2 percent', vintage_on: '2026-11-01', released: { reason: 'no_data', text: 'Release day not recorded' } },
  ],
  explanation: ['The rate counts people looking for work.', 'A rise of 0.1 points is within noise.'],
  gaps: [],
  stale: { data_time: '2026-11-06T19:00:00+08:00', stale_after: '2026-11-08T20:00:00+08:00', text: 'Fresh until 8 November' },
} satisfies IndicatorDetail
