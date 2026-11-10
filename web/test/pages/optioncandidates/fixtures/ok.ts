import type { OptionCandidatesPage, ValueCell } from '../../../../src/generated/option_candidates_page.v1'

const cell = (text: string, value: number | null = null, badges: string[] = []): ValueCell => ({
  text,
  value,
  badges,
  tone: 'none',
})
const columns = [
  { id: 'company', label: 'ok 公司', align: 'left' as const, hint: null },
  { id: 'yield', label: 'ok 年化', align: 'right' as const, hint: 'ok 年化提示' },
]

// Synthetic option_candidates_page fixture written from contracts/schemas/page.option_candidates_page.v1.json alone.
export const ok = {
  schema: 'option_candidates_page',
  schema_version: 1,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'demo',
  generated_at: '2026-11-10T09:00:00+08:00',
  data_times: [{ data_time: '2026-11-09T16:45:00-05:00', source: 'demo_candidates' }],
  run_status: 'ok',
  stale: null,
  title: 'ok 期权候选标题',
  chain_day: '2026-11-09',
  notes: ['ok 日本期权候选备注', 'ok 第二条备注'],
  gaps: [],
  computed_at: 'ok 计算于 2026-11-09 17:00',
  rule_version: 'ok-rule-v1',
  estimate_note: 'ok 估价说明: 收盘 bid 不是成交价',
  groups: [
    {
      kind: 'short_put',
      title: 'ok 卖 put',
      day: '2026-11-09',
      stale: false,
      counts: { ok: 2, blocked: 1, unknown: 1 },
      main: {
        columns,
        rows: [
          { id: 'p1', cells: { company: cell('ok 乙公司'), yield: cell('12.3456%', 0.123456) }, reasons: [] },
          { id: 'p2', cells: { company: cell('ok 甲公司'), yield: cell('5.0000%', 0.05) }, reasons: [] },
        ],
      },
      other: {
        columns,
        rows: [
          {
            id: 'b1',
            cells: { company: cell('ok 被拦公司'), yield: cell('3.0000%', 0.03) },
            reasons: [{ gate: 'events', status: 'block', code: 'earnings_before_expiry', text: 'ok 到期前有财报' }],
          },
          {
            id: 'u1',
            cells: { company: cell('ok 未知公司'), yield: cell('2.0000%', 0.02) },
            reasons: [{ gate: 'chain_fresh', status: 'unknown', code: 'stale_chain', text: 'ok 期权链过旧无法确认' }],
          },
        ],
      },
      gap: null,
    },
    {
      kind: 'covered_call',
      title: 'ok 备兑 call',
      day: '2026-11-09',
      stale: false,
      counts: { ok: 1, blocked: 0, unknown: 0 },
      main: {
        columns,
        rows: [{ id: 'c1', cells: { company: cell('ok 备兑公司'), yield: cell('4.0000%', 0.04) }, reasons: [] }],
      },
      other: null,
      gap: null,
    },
  ],
} satisfies OptionCandidatesPage
