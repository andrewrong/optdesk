import type { Ledger } from '../../../../src/generated/option_pnl_page.v1'

// Invented numbers. The month group has 5 split lots; the company group has 20 split lots and no Sample line.
export const ledger = {
  title: 'Ledger',
  note: 'Synthetic ledger note, 2 open lots',
  views: [
    {
      id: 'by_month',
      label: 'By month',
      groups: [
        {
          id: 'grp-2026-11',
          label: '2026-11',
          summary: 'USD, 5 split lots, 1 of 6 ended lots left out of the sums',
          books: [
            {
              id: 'book-lg-m-0-0',
              currency: 'Synthetic month USD',
              counts: 'USD, 5 split lots of 6 ended',
              left_out: 'Synthetic left out: 1 with no stored quotes (ledger $12.00)',
              headline: {
                id: 'lgm0-0-rent',
                label: 'Rent minus move cost',
                text: '$226.00 · 5 lots',
                hint: 'Synthetic rent hint',
                tone: 'muted',
              },
              lines: [
                { id: 'lgm0-0-sample', label: 'Sample', text: 'Synthetic no conclusion (5 split lots, fewer than 20)', hint: null, tone: 'muted' },
                { id: 'lgm0-0-time', label: 'Time (theta)', text: '$412.00 · 5 lots', hint: 'Synthetic time hint', tone: 'muted' },
                { id: 'lgm0-0-move', label: 'Move cost (gamma)', text: '−$186.00 · 5 lots', hint: null, tone: 'muted' },
              ],
            },
          ],
        },
        {
          id: 'grp-2026-10',
          label: '2026-10',
          summary: 'Synthetic no ended lots this month',
          books: [],
        },
      ],
    },
    {
      id: 'by_company',
      label: 'By company',
      groups: [
        {
          id: 'grp-AAA',
          label: 'AAA',
          summary: 'USD, 20 split lots, lots ended 2026-08-14 to 2026-11-17',
          books: [
            {
              id: 'book-lg-c-0-0',
              currency: 'Synthetic company USD',
              counts: 'USD, 20 split lots of 20 ended',
              left_out: null,
              headline: {
                id: 'lgc0-0-rent',
                label: 'Rent minus move cost',
                text: '$310.00 · 20 lots',
                hint: null,
                tone: 'none',
              },
              lines: [
                { id: 'lgc0-0-time', label: 'Time (theta)', text: '$500.00 · 20 lots', hint: null, tone: 'none' },
              ],
            },
          ],
        },
      ],
    },
  ],
} satisfies Ledger
