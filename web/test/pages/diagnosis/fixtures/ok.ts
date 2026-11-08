// Synthetic diagnosis page payload written from contracts/schemas/page.diagnosis_page.v1.json.
import type { DiagnosisPage, DiagGroup, DiagRow } from '../../../../src/generated/diagnosis_page.v1'

const years = (prefix: string): DiagRow['years'] =>
  [
    { fy: 'FY2023', revenue: '$120亿', margin: '18.0%', extra: prefix + '0.20', bar_percent: 80 },
    { fy: 'FY2024', revenue: '$130亿', margin: '19.5%', extra: prefix + '0.22', bar_percent: 90 },
    { fy: 'FY2025', revenue: '$110亿', margin: '12.0%', extra: prefix + '0.31', bar_percent: 45 },
  ]

export const cyclical: DiagGroup = {
  key: 'cyclical',
  title: '周期股',
  columns: ['年', '收入', '毛利率', '库存 ÷ 收入'],
  legend: ['条形越长，库存占收入越高'],
  rows: [
    {
      symbol: 'AAA',
      latest_fy: 'FY2025',
      latest: '库存 ÷ 收入 0.31',
      rank_text: '10 年里第 9 高',
      extra: '高出常态 0.09',
      usual: '常值 0.22',
      years: years(''),
    },
    {
      symbol: 'BBB',
      latest_fy: 'FY2025',
      latest: '库存 ÷ 收入 0.18',
      rank_text: '10 年里第 3 高',
      extra: '低于常态 0.02',
      usual: '常值 0.20',
      years: years(''),
    },
  ],
}
export const financial: DiagGroup = {
  key: 'financial',
  title: '金融股',
  columns: ['年', '收入', '净利率', 'ROA'],
  legend: [],
  rows: [
    {
      symbol: 'CCC',
      latest_fy: 'FY2025',
      latest: 'ROA 1.1%',
      rank_text: '10 年里第 5 高',
      extra: '高出常态 0.1',
      usual: '常值 1.0%',
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
  title: '体质诊断 · 10 年',
  sentence: '周期股的库存比收入涨得快时，要多看一眼。',
  notes: [],
  gaps: [],
  basis_note: '只看体质 · 不给价格 · 不进任何规则',
  groups: [cyclical, financial],
} satisfies DiagnosisPage
