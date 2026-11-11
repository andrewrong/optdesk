// Synthetic fixture written from contracts/schemas/page.tracking.v2.json alone.
import type { Tracking } from '../../../../../src/generated/tracking.v2'

const row = (symbol: string, outcome: string) => ({
  symbol,
  expiry_text: '2026-11-20',
  strike: '50',
  status: 'PASS',
  outcome,
  premium: { text: '+$150.00', unit: 'usd' as const, value: 150 },
  underlying_move: { reason: 'no_data' as const, text: 'Not expired' },
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
  title: 'System option tracking',
  sentence: 'How the options the system gave turned out, by rule version.',
  how: ['Each row is one candidate of one scan.', 'Average result and vs treasuries are both texts given by the back end.'],
  note: null,
  stale: {
    data_time: '2026-11-06T19:00:00+08:00',
    stale_after: '2026-11-08T20:00:00+08:00',
    text: 'Data as of Nov 6',
  },
  versions: [
    {
      label: 'Rule v3 (current)',
      expired_text: 'All expired up to 2026-11-30 are settled',
      groups: [
        {
          key: 'sell-put',
          title: 'Sell put',
          summary: [
            { label: 'Given', text: '31' },
            { label: 'Expired', text: '24' },
            { label: 'Breaks below', text: '3' },
            { label: 'Average result', text: '+0.9%' },
            { label: 'vs treasuries', text: '+0.2%' },
          ],
          note: null,
          rows: [row('AAA', 'Expires out of the money'), row('BBB', 'Expires in the money')],
        },
        {
          key: 'sell-put-wide',
          title: 'Loose conditions',
          summary: [
            { label: 'Given', text: '5' },
            { label: 'Expired', text: '1' },
          ],
          note: 'Too early to conclude.',
          rows: [row('CCC', 'Not expired')],
        },
      ],
      covered_calls: {
        title: 'Covered call',
        sub: 'Sell calls while holding the stock',
        groups: [
          {
            key: 'cc',
            title: 'Covered group',
            summary: [{ label: 'Given', text: '12' }],
            note: null,
            rows: [row('DDD', 'Expires out of the money')],
          },
        ],
        comparison: {
          columns: ['Plan', 'Average result', 'vs treasuries'],
          rows: [['System covered', '+0.5%', '+0.1%']] as never,
        },
        note: 'Few samples.',
      },
      my_trades: {
        title: 'My trades',
        sub: 'Orders you placed yourself',
        columns: ['Underlying', 'Result'],
        rows: [['EEE', 'Earned $120']] as never,
        note: null,
      },
    },
    {
      label: 'Rule v2',
      expired_text: 'Disabled',
      groups: [
        {
          key: 'old',
          title: 'Old group',
          summary: [{ label: 'Given', text: '77' }],
          note: null,
          rows: [row('OLD', 'Expires out of the money')],
        },
      ],
      covered_calls: null,
      my_trades: { reason: 'not_built', text: 'The old version has no my trades' },
    },
  ],
} satisfies Tracking
