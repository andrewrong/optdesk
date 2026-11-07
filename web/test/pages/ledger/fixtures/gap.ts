import type { LedgerPage } from '../../../../src/generated/ledger_page.v1'

// Synthetic ledger_page fixture written from contracts/schemas/page.ledger_page.v1.json alone.
export const gap = {
  schema: 'ledger_page',
  schema_version: 1,
  build_id: 'build-demo-0001',
  run_id: 'run-demo-0001',
  run_status: 'partial',
  scope: 'demo',
  generated_at: '2026-10-23T09:00:00+08:00',
  data_times: [{ data_time: '2026-10-23T08:30:00+08:00', source: 'demo_ledger' }],
  title: 'gap 账本标题',
  stale: {
    data_time: '2026-10-23T08:30:00+08:00',
    stale_after: '2099-01-01T00:00:00+08:00',
    text: 'gap 数据时间标签',
  },
  header: { title: 'gap 页头标题', through: 'gap 截至 2026-10-22', fx_note: 'gap 汇率说明' },
  overview: [
    {
      key: 'acct_a_usd',
      title: 'gap 概览账户',
      total: { text: 'gap 已平合计 +120', tone: 'gain' },
      count: 'gap 2 笔未平',
    },
    { key: 'shares', title: 'gap 概览股票', total: null, count: 'gap 3 held' },
  ],
  sections: [
    {
      key: 'acct_a_usd',
      title: 'gap 分区标题',
      chart: null,
      lead: { caption: 'gap 引导', total: null },
      more: ['gap 更多一行'],
      open: [
        {
          label: 'gap 未平合约',
          small: 'gap 未平小字',
          premium: { text: 'gap 权利金 -30', tone: 'loss' },
          detail: ['gap 未平细节'],
        },
      ],
      open_columns: ['gap 合约列', 'gap 权利金列'],
      open_title: 'gap 未平表标题',
      strategies: [
        {
          cells: [
            { text: 'gap 策略名' },
            { text: 'gap 已平 5' },
            { text: 'gap 结果 +120', tone: 'gain' },
            { text: 'gap 备注' },
          ],
          closed: [
            {
              cells: [
                { text: 'gap 已平合约' },
                { text: 'gap 已实现 +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['gap 已平细节'],
            },
          ],
          dots: [{ text: 'gap 点', value: 120, delivered: false }],
          lines: ['gap 策略行'],
        },
      ],
      strategy_columns: ['gap 策略列', 'gap 笔数列', 'gap 结果列', 'gap 备注列'],
      totals_differ: 'gap 合计不一致',
    },
  ],
  shares: [
    {
      key: 'acct_a_usd',
      title: 'gap 股票分区',
      lead: 'gap 股票引导',
      note: 'gap 股票说明',
      rows: [{ cells: [{ text: 'gap 股票代码' }, { text: 'gap 股数 100' }, { text: 'gap 成本 10' }] }],
      sales: [
        {
          cells: [
            { text: 'gap 卖出股数' },
            { text: 'gap 卖出价' },
            { text: 'gap 平均成本' },
            { text: 'gap 卖出结果', tone: 'gain' },
          ],
          detail: ['gap 卖出细节'],
        },
      ],
    },
  ],
  nothing_closed: null,
  notes: [{ term: 'gap 术语', text: 'gap 术语解释' }],
  footer: ['gap 页脚一行'],
  gaps: [{ reason: 'missing_quote', text: 'gap 缺少报价' }],
} satisfies LedgerPage
