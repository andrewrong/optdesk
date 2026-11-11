import type { ChartPoint, LedgerPage } from '../../../../src/generated/ledger_page.v1'

// Twelve steps of a cumulative line, as the schema's two-or-more tuple of day, text and number.
const line = (): [ChartPoint, ChartPoint, ...ChartPoint[]] => {
  const steps = Array.from({ length: 12 }, (_, i) => ({
    day: `2026-${String(i + 1).padStart(2, '0')}-01`,
    text: `ok cumulative +${i * 10}`,
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
  title: 'ok ledger title',
  stale: {
    data_time: '2026-10-23T08:30:00+08:00',
    stale_after: '2099-01-01T00:00:00+08:00',
    text: 'ok data time label',
  },
  header: { title: 'ok header title', through: 'ok through 2026-10-22', fx_note: 'ok fx note' },
  overview: [
    {
      key: 'acct_a_usd',
      title: 'ok overview account',
      total: { text: 'ok closed total +120', tone: 'gain' },
      count: 'ok 2 open',
    },
    { key: 'shares', title: 'ok overview stocks', total: null, count: 'ok 3 held' },
  ],
  sections: [
    {
      key: 'acct_a_usd',
      title: 'ok section title',
      chart: { points: line(), summary: 'ok curve summary' },
      lead: { caption: 'ok lead', total: { text: 'ok lead total +120', tone: 'gain' } },
      more: ['ok more row'],
      open: [
        {
          label: 'ok open contract',
          small: 'ok open small text',
          premium: { text: 'ok premium -30', tone: 'loss' },
          detail: ['ok open detail'],
        },
      ],
      open_columns: ['ok contract column', 'ok premium column'],
      open_title: 'ok open table title',
      strategies: [
        {
          cells: [
            { text: 'ok strategy name' },
            { text: 'ok closed 5' },
            { text: 'ok result +120', tone: 'gain' },
            { text: 'n<30' },
          ],
          closed: [
            {
              cells: [
                { text: 'ok closed contract' },
                { text: 'ok realized +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['ok closed detail'],
            },
          ],
          dots: [{ text: 'ok dot', value: 120, delivered: false }],
          lines: ['ok strategy line'],
        },
      ],
      strategy_columns: ['Strategy', 'Closed', 'Realized', 'Ann.'],
      totals_differ: null,
    },
    {
      key: 'acct_b_usd',
      title: 'ok second USD section',
      chart: { points: line(), summary: 'ok second curve summary' },
      lead: { caption: 'ok lead', total: { text: 'ok lead total +120', tone: 'gain' } },
      more: ['ok more row'],
      open: [
        {
          label: 'ok open contract',
          small: 'ok open small text',
          premium: { text: 'ok premium -30', tone: 'loss' },
          detail: ['ok open detail'],
        },
      ],
      open_columns: ['ok contract column', 'ok premium column'],
      open_title: 'ok open table title',
      strategies: [
        {
          cells: [
            { text: 'ok strategy name' },
            { text: 'ok closed 5' },
            { text: 'ok result +120', tone: 'gain' },
            { text: 'n<30' },
          ],
          closed: [
            {
              cells: [
                { text: 'ok closed contract' },
                { text: 'ok realized +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['ok closed detail'],
            },
          ],
          dots: [{ text: 'ok dot', value: 120, delivered: false }],
          lines: ['ok strategy line'],
        },
      ],
      strategy_columns: ['Strategy', 'Closed', 'Realized', 'Ann.'],
      totals_differ: null,
    },
    {
      key: 'acct_c_jpy',
      title: 'ok JPY section',
      chart: null,
      lead: { caption: 'ok lead', total: { text: 'ok lead total +120 approx ok approx_usd 800 USD', tone: 'gain' } },
      more: ['ok more row'],
      open: [
        {
          label: 'ok open contract',
          small: 'ok open small text',
          premium: { text: 'ok premium -30', tone: 'loss' },
          detail: ['ok open detail'],
        },
      ],
      open_columns: ['ok contract column', 'ok premium column'],
      open_title: 'ok open table title',
      strategies: [
        {
          cells: [
            { text: 'ok strategy name' },
            { text: 'ok closed 5' },
            { text: 'ok result +120', tone: 'gain' },
            { text: 'n<30' },
          ],
          closed: [
            {
              cells: [
                { text: 'ok closed contract' },
                { text: 'ok realized +120', tone: 'gain' },
                { text: '' },
              ],
              detail: ['ok closed detail'],
            },
          ],
          dots: [{ text: 'ok dot', value: 120, delivered: false }],
          lines: ['ok strategy line'],
        },
      ],
      strategy_columns: ['Strategy', 'Closed', 'Realized', 'Ann.'],
      totals_differ: null,
    },
  ],
  shares: [
    {
      key: 'acct_a_usd',
      title: 'ok stock section',
      lead: 'ok stock lead',
      note: 'ok stock note',
      rows: [{ cells: [{ text: 'ok stock symbol' }, { text: 'ok shares 100' }, { text: 'ok cost 10' }] }],
      sales: [
        {
          cells: [
            { text: 'ok sold shares' },
            { text: 'ok sale price' },
            { text: 'ok average cost' },
            { text: 'ok sale result', tone: 'gain' },
          ],
          detail: ['ok sale detail'],
        },
      ],
    },
  ],
  nothing_closed: null,
  notes: [{ term: 'ok term', text: 'ok term explanation' }],
  footer: ['ok footer line'],
  gaps: [],
} satisfies LedgerPage
