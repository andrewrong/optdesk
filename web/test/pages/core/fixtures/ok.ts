// Synthetic core page payload written from contracts/schemas/page.core_page.v1.json.
import type { CorePage } from '../../../../src/generated/core_page.v1'

export const ok = {
  schema: 'core_page',
  schema_version: 1,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'core',
  generated_at: '2026-10-26T21:30:00Z',
  data_times: [],
  run_status: 'ok',
  stale: null,
  title: 'Core holdings',
  sentence: 'Three companies, return at the current price under four assumptions.',
  notes: [],
  gaps: [],
  basis_note: 'No target price · not in any rule',
  scale: {
    low_text: '0%',
    high_text: '15%',
    treasury: { text: 'Treasury 4.2%', marker_percent: 28 },
  },
  rows: [
    {
      symbol: 'AAA',
      price_text: '$100.00',
      cyclical: false,
      flag: 'Above Treasury 10-14',
      returns: [
        { assumption: 'Growth 10%', text: '5.1%', marker_percent: 34 },
        { assumption: 'Growth 12%', text: '6.3%', marker_percent: 42 },
        { assumption: 'Growth 14%', text: '7.2%', marker_percent: 48 },
      ],
    },
    {
      symbol: 'BBB',
      price_text: '$50.00',
      cyclical: true,
      flag: null,
      returns: [{ assumption: 'Growth 8%', text: '3.0%', marker_percent: 20 }],
    },
    {
      symbol: 'CCC',
      price_text: '$75.00',
      cyclical: false,
      flag: null,
      returns: { kind: 'gap', reason: 'no_data', text: 'Valuation: not enough data, three years of reports missing' },
    },
  ],
  jp_note: 'Japan stocks not included yet, waiting for the Japan slice.',
  legend: ['Dot is the return at the current price', 'Line is the Treasury'],
  snapshot_text: 'Snapshot 2026-10-26',
} satisfies CorePage
