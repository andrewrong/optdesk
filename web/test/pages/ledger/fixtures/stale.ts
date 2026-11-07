import type { LedgerPage } from '../../../../src/generated/ledger_page.v1'

// Synthetic ledger_page fixture written from contracts/schemas/page.ledger_page.v1.json alone.
export const stale = {
  schema: 'ledger_page',
  schema_version: 1,
  build_id: 'build-demo-0001',
  run_id: 'run-demo-0001',
  run_status: 'ok',
  scope: 'demo',
  generated_at: '2026-10-23T09:00:00+08:00',
  data_times: [{ data_time: '2026-10-23T08:30:00+08:00', source: 'demo_ledger' }],
  title: 'stale 账本标题',
  stale: {
    data_time: '2026-10-23T08:30:00+08:00',
    stale_after: '2020-01-01T00:00:00+08:00',
    text: 'stale 数据时间标签',
  },
  header: { title: 'stale 页头标题', through: 'stale 截至 2026-10-22', fx_note: 'stale 汇率说明' },
  overview: [
    {
      key: 'acct_a_usd',
      title: 'stale 概览账户',
      total: { text: 'stale 已平合计 +120', tone: 'gain' },
      count: 'stale 2 笔未平',
    },
    { key: 'shares', title: 'stale 概览股票', total: null, count: 'stale 3 held' },
  ],
  sections: [
    {
      key: 'acct_a_usd',
      title: 'stale 分区标题',
      chart: null,
      lead: null,
      more: ['stale 更多一行'],
      open: [
        {
          label: 'stale 未平合约',
          small: 'stale 未平小字',
          premium: { text: 'stale 权利金 -30', tone: 'loss' },
          detail: ['stale 未平细节'],
        },
      ],
      open_columns: ['stale 合约列', 'stale 权利金列'],
      open_title: 'stale 未平表标题',
      strategies: [
        {
          cells: [
            { text: 'stale 策略名' },
            { text: 'stale 已平 5' },
            { text: 'stale 结果 +120', tone: 'gain' },
            { text: 'stale 备注' },
          ],
          closed: [
            {
              cells: [
                { text: 'stale 已平合约' },
                { text: 'stale 已实现 +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['stale 已平细节'],
            },
          ],
          dots: [{ text: 'stale 点', value: 120, delivered: false }],
          lines: ['stale 策略行'],
        },
      ],
      strategy_columns: ['stale 策略列', 'stale 笔数列', 'stale 结果列', 'stale 备注列'],
      totals_differ: null,
    },
  ],
  shares: [
    {
      key: 'acct_a_usd',
      title: 'stale 股票分区',
      lead: 'stale 股票引导',
      note: 'stale 股票说明',
      rows: [{ cells: [{ text: 'stale 股票代码' }, { text: 'stale 股数 100' }, { text: 'stale 成本 10' }] }],
      sales: [
        {
          cells: [
            { text: 'stale 卖出股数' },
            { text: 'stale 卖出价' },
            { text: 'stale 平均成本' },
            { text: 'stale 卖出结果', tone: 'gain' },
          ],
          detail: ['stale 卖出细节'],
        },
      ],
    },
  ],
  nothing_closed: 'stale 没有已平仓位',
  notes: [{ term: 'stale 术语', text: 'stale 术语解释' }],
  footer: ['stale 页脚一行'],
  gaps: [],
} satisfies LedgerPage
