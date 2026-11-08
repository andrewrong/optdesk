// Synthetic fixture written from contracts/schemas/page.tracking.v2.json alone.
import type { Tracking } from '../../../../../src/generated/tracking.v2'

const row = (symbol: string, outcome: string) => ({
  symbol,
  expiry_text: '2026-11-20',
  strike: '50',
  status: 'PASS',
  outcome,
  premium: { text: '+$150.00', unit: 'usd' as const, value: 150 },
  underlying_move: { reason: 'no_data' as const, text: '未到期' },
  assigned: null,
})

export const ok = {
  schema: 'tracking',
  schema_version: 2,
  run_id: 'run-1',
  build_id: 'build-1',
  scope: 'options',
  generated_at: '2026-11-06T20:00:00+08:00',
  data_times: [{ source: 'scan', data_time: '2026-11-06T19:00:00+08:00' }],
  run_status: 'ok',
  title: '系统期权跟踪',
  sentence: '按规则版本看系统给出的期权后来怎么样。',
  how: ['每一行是一次扫描里的一个候选。', '平均结果与比国债都是后台给出的文字。'],
  note: null,
  stale: {
    data_time: '2026-11-06T19:00:00+08:00',
    stale_after: '2026-11-08T20:00:00+08:00',
    text: '数据截至 11 月 6 日',
  },
  versions: [
    {
      label: '规则 v3（当前）',
      expired_text: '到 2026-11-30 前到期的已全部结算',
      groups: [
        {
          key: 'sell-put',
          title: '卖出看跌',
          summary: [
            { label: '给出', text: '31' },
            { label: '已到期', text: '24' },
            { label: '跌破', text: '3' },
            { label: '平均结果', text: '+0.9%' },
            { label: '比国债', text: '+0.2%' },
          ],
          note: null,
          rows: [row('AAA', '价外到期'), row('BBB', '价内到期')],
        },
        {
          key: 'sell-put-wide',
          title: '宽松条件',
          summary: [
            { label: '给出', text: '5' },
            { label: '已到期', text: '1' },
          ],
          note: '太早，不下结论。',
          rows: [row('CCC', '未到期')],
        },
      ],
      covered_calls: {
        title: '备兑看涨',
        sub: '持有正股时卖出看涨',
        groups: [
          {
            key: 'cc',
            title: '备兑组',
            summary: [{ label: '给出', text: '12' }],
            note: null,
            rows: [row('DDD', '价外到期')],
          },
        ],
        comparison: {
          columns: ['方案', '平均结果', '比国债'],
          rows: [['系统备兑', '+0.5%', '+0.1%']] as never,
        },
        note: '样本少。',
      },
      my_trades: {
        title: '我的交易',
        sub: '你自己下的单',
        columns: ['标的', '结果'],
        rows: [['EEE', '赚 $120']] as never,
        note: null,
      },
    },
    {
      label: '规则 v2',
      expired_text: '已停用',
      groups: [
        {
          key: 'old',
          title: '旧组',
          summary: [{ label: '给出', text: '77' }],
          note: null,
          rows: [row('OLD', '价外到期')],
        },
      ],
      covered_calls: null,
      my_trades: { reason: 'not_built', text: '旧版本没有我的交易' },
    },
  ],
} satisfies Tracking
