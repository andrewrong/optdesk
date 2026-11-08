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
  title: '核心持仓',
  sentence: '三家公司，按四种假设看今天价格的回报。',
  notes: [],
  gaps: [],
  basis_note: '不给目标价 · 不进规则',
  scale: {
    low_text: '0%',
    high_text: '15%',
    treasury: { text: '国债 4.2%', marker_percent: 28 },
  },
  rows: [
    {
      symbol: 'AAA',
      price_text: '$100.00',
      cyclical: false,
      flag: '高过国债 10-14',
      returns: [
        { assumption: '增长 10%', text: '5.1%', marker_percent: 34 },
        { assumption: '增长 12%', text: '6.3%', marker_percent: 42 },
        { assumption: '增长 14%', text: '7.2%', marker_percent: 48 },
      ],
    },
    {
      symbol: 'BBB',
      price_text: '$50.00',
      cyclical: true,
      flag: null,
      returns: [{ assumption: '增长 8%', text: '3.0%', marker_percent: 20 }],
    },
    {
      symbol: 'CCC',
      price_text: '$75.00',
      cyclical: false,
      flag: null,
      returns: { kind: 'gap', reason: 'no_data', text: '估值数据不足：缺少三年财报' },
    },
  ],
  jp_note: '日股暂未纳入，等日股切片。',
  legend: ['点是今天价格的回报', '竖线是国债'],
  snapshot_text: '快照 2026-10-26',
} satisfies CorePage
