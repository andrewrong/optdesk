import type { OptionCandidatesPage } from '../../../../src/generated/option_candidates_page.v1'
import { ok } from './ok'

const mainRows = ok.groups[0].main.rows.map((row) => ({
  ...row,
  cells: { ...row.cells, company: { ...row.cells.company, badges: ['Stale'] } },
}))

// The rows are of an earlier day: the group is stale, the first cell of each row carries `Stale`, the page has a label.
export const stale = {
  ...ok,
  stale: { data_time: '2026-11-06T16:45:00-05:00', is_stale: true, text: 'stale Data is from 2026-11-06, stale' },
  groups: [
    {
      ...ok.groups[0],
      day: '2026-11-06',
      stale: true,
      main: { ...ok.groups[0].main, rows: mainRows },
    },
  ],
} satisfies OptionCandidatesPage
