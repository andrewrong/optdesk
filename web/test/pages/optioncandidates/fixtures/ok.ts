import type { OptionCandidatesPage, ValueCell } from '../../../../src/generated/option_candidates_page.v1'

const cell = (text: string, value: number | null = null, badges: string[] = []): ValueCell => ({
  text,
  value,
  badges,
  tone: 'none',
})
const columns = [
  { id: 'company', label: 'ok Company', align: 'left' as const, hint: null },
  { id: 'yield', label: 'ok annualised', align: 'right' as const, hint: 'ok Annualised hint' },
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
  title: 'ok Option candidates title',
  chain_day: '2026-11-09',
  notes: ['ok Japan option candidates note', 'ok Second note'],
  gaps: [],
  computed_at: 'ok Computed at 2026-11-09 17:00',
  rule_version: 'ok-rule-v1',
  estimate_note: 'ok Valuation note: the closing bid is not a traded price',
  groups: [
    {
      kind: 'short_put',
      title: 'ok Sell put',
      day: '2026-11-09',
      stale: false,
      counts: { ok: 2, blocked: 1, unknown: 1 },
      main: {
        columns,
        rows: [
          { id: 'p1', cells: { company: cell('ok Company B'), yield: cell('12.3456%', 0.123456) }, reasons: [] },
          { id: 'p2', cells: { company: cell('ok Company A'), yield: cell('5.0000%', 0.05) }, reasons: [] },
        ],
      },
      other: {
        columns,
        rows: [
          {
            id: 'b1',
            cells: { company: cell('ok Blocked company'), yield: cell('3.0000%', 0.03) },
            reasons: [{ gate: 'events', status: 'block', code: 'earnings_before_expiry', text: 'ok Earnings before expiry' }],
          },
          {
            id: 'u1',
            cells: { company: cell('ok Unknown company'), yield: cell('2.0000%', 0.02) },
            reasons: [{ gate: 'chain_fresh', status: 'unknown', code: 'stale_chain', text: 'ok Option chain too old to confirm' }],
          },
        ],
      },
      gap: null,
    },
    {
      kind: 'covered_call',
      title: 'ok Covered call',
      day: '2026-11-09',
      stale: false,
      counts: { ok: 1, blocked: 0, unknown: 0 },
      main: {
        columns,
        rows: [{ id: 'c1', cells: { company: cell('ok Covered company'), yield: cell('4.0000%', 0.04) }, reasons: [] }],
      },
      other: null,
      gap: null,
    },
  ],
} satisfies OptionCandidatesPage
