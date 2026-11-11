// Synthetic diagnosis page payload written from contracts/schemas/page.diagnosis_page.v1.json.
import type { DiagnosisPage, DiagGroup, DiagRow } from '../../../../src/generated/diagnosis_page.v1'

const years = (prefix: string): DiagRow['years'] =>
  [
    { fy: 'FY2023', revenue: '$12.0B', margin: '18.0%', extra: prefix + '0.20', bar_percent: 80 },
    { fy: 'FY2024', revenue: '$13.0B', margin: '19.5%', extra: prefix + '0.22', bar_percent: 90 },
    { fy: 'FY2025', revenue: '$11.0B', margin: '12.0%', extra: prefix + '0.31', bar_percent: 45 },
  ]

export const cyclical: DiagGroup = {
  key: 'cyclical',
  title: 'Cyclicals',
  columns: ['Year', 'Revenue', 'Gross margin', 'Inventory ÷ revenue'],
  legend: ['The longer the bar, the higher inventory is against revenue'],
  rows: [
    {
      symbol: 'AAA',
      latest_fy: 'FY2025',
      latest: 'Inventory ÷ revenue 0.31',
      rank_text: '9th highest in 10 years',
      extra: 'Above normal by 0.09',
      usual: 'Usual 0.22',
      years: years(''),
    },
    {
      symbol: 'BBB',
      latest_fy: 'FY2025',
      latest: 'Inventory ÷ revenue 0.18',
      rank_text: '3rd highest in 10 years',
      extra: 'Below normal by 0.02',
      usual: 'Usual 0.20',
      years: years(''),
    },
  ],
}
export const financial: DiagGroup = {
  key: 'financial',
  title: 'Financials',
  columns: ['Year', 'Revenue', 'Net margin', 'ROA'],
  legend: [],
  rows: [
    {
      symbol: 'CCC',
      latest_fy: 'FY2025',
      latest: 'ROA 1.1%',
      rank_text: '5th highest in 10 years',
      extra: 'Above normal by 0.1',
      usual: 'Usual 1.0%',
      years: years('ROA '),
    },
  ],
}

export const ok = {
  schema: 'diagnosis_page',
  schema_version: 1,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'diagnosis',
  generated_at: '2026-10-26T21:30:00Z',
  data_times: [],
  run_status: 'ok',
  stale: null,
  title: 'Health check · 10 years',
  sentence: 'When inventory grows faster than revenue in cyclicals, take a second look.',
  notes: [],
  gaps: [],
  basis_note: 'Health only · no price · not in any rule',
  groups: [cyclical, financial],
} satisfies DiagnosisPage
