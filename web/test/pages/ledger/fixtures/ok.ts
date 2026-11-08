import type { ChartPoint, LedgerPage } from '../../../../src/generated/ledger_page.v1'

// Twelve steps of a cumulative line, as the schema's two-or-more tuple of day, text and number.
const line = (): [ChartPoint, ChartPoint, ...ChartPoint[]] => {
  const steps = Array.from({ length: 12 }, (_, i) => ({
    day: `2026-${String(i + 1).padStart(2, '0')}-01`,
    text: `ok 累计 +${i * 10}`,
    value: i * 10,
  }))
  const [a, b, ...rest] = steps
  return [a as ChartPoint, b as ChartPoint, ...rest]
}

// Synthetic ledger_page fixture written from contracts/schemas/page.ledger_page.v1.json alone.
export const ok = {
  schema: 'ledger_page',
  schema_version: 1,
  build_id: 'build-demo-0001',
  run_id: 'run-demo-0001',
  run_status: 'ok',
  scope: 'demo',
  generated_at: '2026-10-23T09:00:00+08:00',
  data_times: [{ data_time: '2026-10-23T08:30:00+08:00', source: 'demo_ledger' }],
  title: 'ok 账本标题',
  stale: {
    data_time: '2026-10-23T08:30:00+08:00',
    stale_after: '2099-01-01T00:00:00+08:00',
    text: 'ok 数据时间标签',
  },
  header: { title: 'ok 页头标题', through: 'ok 截至 2026-10-22', fx_note: 'ok 汇率说明' },
  overview: [
    {
      key: 'acct_a_usd',
      title: 'ok 概览账户',
      total: { text: 'ok 已平合计 +120', tone: 'gain' },
      count: 'ok 2 笔未平',
    },
    { key: 'shares', title: 'ok 概览股票', total: null, count: 'ok 3 held' },
  ],
  sections: [
    {
      key: 'acct_a_usd',
      title: 'ok 分区标题',
      chart: { points: line(), summary: 'ok 曲线摘要' },
      lead: { caption: 'ok 引导', total: { text: 'ok 引导合计 +120', tone: 'gain' } },
      more: ['ok 更多一行'],
      open: [
        {
          label: 'ok 未平合约',
          small: 'ok 未平小字',
          premium: { text: 'ok 权利金 -30', tone: 'loss' },
          detail: ['ok 未平细节'],
        },
      ],
      open_columns: ['ok 合约列', 'ok 权利金列'],
      open_title: 'ok 未平表标题',
      strategies: [
        {
          cells: [
            { text: 'ok 策略名' },
            { text: 'ok 已平 5' },
            { text: 'ok 结果 +120', tone: 'gain' },
            { text: 'n<30' },
          ],
          closed: [
            {
              cells: [
                { text: 'ok 已平合约' },
                { text: 'ok 已实现 +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['ok 已平细节'],
            },
          ],
          dots: [{ text: 'ok 点', value: 120, delivered: false }],
          lines: ['ok 策略行'],
        },
      ],
      strategy_columns: ['Strategy', 'Closed', 'Realized', 'Ann.'],
      totals_differ: null,
    },
    {
      key: 'acct_b_usd',
      title: 'ok 第二美元分区',
      chart: { points: line(), summary: 'ok 第二曲线摘要' },
      lead: { caption: 'ok 引导', total: { text: 'ok 引导合计 +120', tone: 'gain' } },
      more: ['ok 更多一行'],
      open: [
        {
          label: 'ok 未平合约',
          small: 'ok 未平小字',
          premium: { text: 'ok 权利金 -30', tone: 'loss' },
          detail: ['ok 未平细节'],
        },
      ],
      open_columns: ['ok 合约列', 'ok 权利金列'],
      open_title: 'ok 未平表标题',
      strategies: [
        {
          cells: [
            { text: 'ok 策略名' },
            { text: 'ok 已平 5' },
            { text: 'ok 结果 +120', tone: 'gain' },
            { text: 'n<30' },
          ],
          closed: [
            {
              cells: [
                { text: 'ok 已平合约' },
                { text: 'ok 已实现 +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['ok 已平细节'],
            },
          ],
          dots: [{ text: 'ok 点', value: 120, delivered: false }],
          lines: ['ok 策略行'],
        },
      ],
      strategy_columns: ['Strategy', 'Closed', 'Realized', 'Ann.'],
      totals_differ: null,
    },
    {
      key: 'acct_c_jpy',
      title: 'ok 日元分区',
      chart: null,
      lead: { caption: 'ok 引导', total: { text: 'ok 引导合计 +120 约 ok approx_usd 800 美元', tone: 'gain' } },
      more: ['ok 更多一行'],
      open: [
        {
          label: 'ok 未平合约',
          small: 'ok 未平小字',
          premium: { text: 'ok 权利金 -30', tone: 'loss' },
          detail: ['ok 未平细节'],
        },
      ],
      open_columns: ['ok 合约列', 'ok 权利金列'],
      open_title: 'ok 未平表标题',
      strategies: [
        {
          cells: [
            { text: 'ok 策略名' },
            { text: 'ok 已平 5' },
            { text: 'ok 结果 +120', tone: 'gain' },
            { text: 'n<30' },
          ],
          closed: [
            {
              cells: [
                { text: 'ok 已平合约' },
                { text: 'ok 已实现 +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['ok 已平细节'],
            },
          ],
          dots: [{ text: 'ok 点', value: 120, delivered: false }],
          lines: ['ok 策略行'],
        },
      ],
      strategy_columns: ['Strategy', 'Closed', 'Realized', 'Ann.'],
      totals_differ: null,
    },
  ],
  shares: [
    {
      key: 'acct_a_usd',
      title: 'ok 股票分区',
      lead: 'ok 股票引导',
      note: 'ok 股票说明',
      rows: [{ cells: [{ text: 'ok 股票代码' }, { text: 'ok 股数 100' }, { text: 'ok 成本 10' }] }],
      sales: [
        {
          cells: [
            { text: 'ok 卖出股数' },
            { text: 'ok 卖出价' },
            { text: 'ok 平均成本' },
            { text: 'ok 卖出结果', tone: 'gain' },
          ],
          detail: ['ok 卖出细节'],
        },
      ],
    },
  ],
  nothing_closed: null,
  notes: [{ term: 'ok 术语', text: 'ok 术语解释' }],
  footer: ['ok 页脚一行'],
  gaps: [],
} satisfies LedgerPage
