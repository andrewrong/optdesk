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
  title: 'stale ledger title',
  stale: {
    data_time: '2026-10-23T08:30:00+08:00',
    stale_after: '2020-01-01T00:00:00+08:00',
    text: 'stale data time label',
  },
  header: { title: 'stale header title', through: 'stale through 2026-10-22', fx_note: 'stale fx note' },
  overview: [
    {
      key: 'acct_a_usd',
      title: 'stale overview account',
      total: { text: 'stale closed total +120', tone: 'gain' },
      count: 'stale 2 open',
    },
    { key: 'shares', title: 'stale overview stocks', total: null, count: 'stale 3 held' },
  ],
  sections: [
    {
      key: 'acct_a_usd',
      title: 'stale section title',
      chart: null,
      lead: null,
      more: ['stale more row'],
      open: [
        {
          label: 'stale open contract',
          small: 'stale open small text',
          premium: { text: 'stale premium -30', tone: 'loss' },
          detail: ['stale open detail'],
        },
      ],
      open_columns: ['stale contract column', 'stale premium column'],
      open_title: 'stale open table title',
      strategies: [
        {
          cells: [
            { text: 'stale strategy name' },
            { text: 'stale closed 5' },
            { text: 'stale result +120', tone: 'gain' },
            { text: 'stale note' },
          ],
          closed: [
            {
              cells: [
                { text: 'stale closed contract' },
                { text: 'stale realized +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['stale closed detail'],
            },
          ],
          dots: [{ text: 'stale dot', value: 120, delivered: false }],
          lines: ['stale strategy line'],
        },
      ],
      strategy_columns: ['stale strategy column', 'stale count column', 'stale result column', 'stale note column'],
      totals_differ: null,
    },
  ],
  shares: [
    {
      key: 'acct_a_usd',
      title: 'stale stock section',
      lead: 'stale stock lead',
      note: 'stale stock note',
      rows: [{ cells: [{ text: 'stale stock symbol' }, { text: 'stale shares 100' }, { text: 'stale cost 10' }] }],
      sales: [
        {
          cells: [
            { text: 'stale sold shares' },
            { text: 'stale sale price' },
            { text: 'stale average cost' },
            { text: 'stale sale result', tone: 'gain' },
          ],
          detail: ['stale sale detail'],
        },
      ],
    },
  ],
  nothing_closed: 'stale no closed positions',
  notes: [{ term: 'stale term', text: 'stale term explanation' }],
  footer: ['stale footer line'],
  gaps: [],
} satisfies LedgerPage
