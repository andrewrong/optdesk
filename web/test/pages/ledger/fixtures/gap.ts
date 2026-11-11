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
  title: 'gap ledger title',
  stale: {
    data_time: '2026-10-23T08:30:00+08:00',
    stale_after: '2099-01-01T00:00:00+08:00',
    text: 'gap data time label',
  },
  header: { title: 'gap header title', through: 'gap through 2026-10-22', fx_note: 'gap fx note' },
  overview: [
    {
      key: 'acct_a_usd',
      title: 'gap overview account',
      total: { text: 'gap closed total +120', tone: 'gain' },
      count: 'gap 2 open',
    },
    { key: 'shares', title: 'gap overview stocks', total: null, count: 'gap 3 held' },
  ],
  sections: [
    {
      key: 'acct_a_usd',
      title: 'gap section title',
      chart: null,
      lead: { caption: 'gap lead', total: null },
      more: ['gap more row'],
      open: [
        {
          label: 'gap open contract',
          small: 'gap open small text',
          premium: { text: 'gap premium -30', tone: 'loss' },
          detail: ['gap open detail'],
        },
      ],
      open_columns: ['gap contract column', 'gap premium column'],
      open_title: 'gap open table title',
      strategies: [
        {
          cells: [
            { text: 'gap strategy name' },
            { text: 'gap closed 5' },
            { text: 'gap result +120', tone: 'gain' },
            { text: 'gap note' },
          ],
          closed: [
            {
              cells: [
                { text: 'gap closed contract' },
                { text: 'gap realized +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['gap closed detail'],
            },
          ],
          dots: [{ text: 'gap dot', value: 120, delivered: false }],
          lines: ['gap strategy line'],
        },
      ],
      strategy_columns: ['gap strategy column', 'gap count column', 'gap result column', 'gap note column'],
      totals_differ: 'gap totals differ',
    },
  ],
  shares: [
    {
      key: 'acct_a_usd',
      title: 'gap stock section',
      lead: 'gap stock lead',
      note: 'gap stock note',
      rows: [{ cells: [{ text: 'gap stock symbol' }, { text: 'gap shares 100' }, { text: 'gap cost 10' }] }],
      sales: [
        {
          cells: [
            { text: 'gap sold shares' },
            { text: 'gap sale price' },
            { text: 'gap average cost' },
            { text: 'gap sale result', tone: 'gain' },
          ],
          detail: ['gap sale detail'],
        },
      ],
    },
  ],
  nothing_closed: null,
  notes: [{ term: 'gap term', text: 'gap term explanation' }],
  footer: ['gap footer line'],
  gaps: [{ reason: 'missing_quote', text: 'gap missing quote' }],
} satisfies LedgerPage
