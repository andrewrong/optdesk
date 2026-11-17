import type { OptionPnlPage } from '../../../../src/generated/option_pnl_page.v1'

export const ok = {
  schema: 'option_pnl_page',
  schema_version: 1,
  run_id: 'run-ok',
  build_id: 'build-ok',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [{ source: 'src-a', data_time: '2026-01-02T09:00:00+08:00' }],
  run_status: 'ok',
  stale: null,
  title: 'Synthetic P&L split title',
  notes: ['Synthetic pnl note one'],
  gaps: [],
  as_of: 'Synthetic as-of text',
  accounts: [
    {
      alias: 'acct-a',
      sample_note: 'Synthetic sample note',
      gap: null,
      books: [
        {
          id: 'book-acct-a:USD',
          currency: 'Synthetic USD book',
          counts: 'Synthetic counts text',
          left_out: 'Synthetic left out: 1 with no stored close (ZZZ, ledger $12.00)',
          headline: {
            id: 'rent',
            label: 'Rent minus move cost',
            text: '−$1,234.50',
            hint: 'Synthetic rent hint',
            tone: 'good',
          },
          lines: [
            { id: 'time', label: 'Time (theta)', text: '$412.00', hint: 'Synthetic time hint', tone: 'none' },
            { id: 'move', label: 'Move cost (gamma)', text: '−$186.00', hint: null, tone: 'bad' },
          ],
        },
      ],
      lots: [
        {
          id: 'lot-aaa',
          title: 'Synthetic AAA put 90.00',
          summary: 'Synthetic AAA summary',
          lines: [
            { id: 'lot-time', label: 'Lot time', text: '$147.40', hint: 'Synthetic lot time hint', tone: 'none' },
            { id: 'lot-ledger', label: 'Lot ledger says', text: '$147.40', hint: null, tone: 'muted' },
          ],
          days_line: { id: 'days', label: 'Days', text: 'Synthetic days text', hint: null, tone: 'none' },
          iv_line: { id: 'iv', label: 'IV line', text: 'Synthetic IV text', hint: null, tone: 'none' },
          end_line: null,
          recon: [
            { id: 'recon-1', label: 'Reconciliation', text: 'Synthetic recon one', hint: null, tone: 'none' },
            { id: 'recon-2', label: 'Broker check', text: 'Synthetic recon two', hint: null, tone: 'bad' },
          ],
          badges: ['2-day step', 'wide quote'],
          gap: null,
        },
      ],
    },
  ],
} satisfies OptionPnlPage
