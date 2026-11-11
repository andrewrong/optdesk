// Synthetic home page payload written from contracts/schemas/page.home_page.v3.json: the five members are the
// ok payloads of the member fixtures.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import type { HomePage } from '../../../../../src/generated/home_page.v3'
import { ok as held } from '../../held-v2/fixtures/ok'
import { ok as household } from '../../household-v2/fixtures/ok'
import { ok as macro } from '../../macro-v2/fixtures/ok'

const read = (...file: string[]) =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, '../../../../../', ...file), 'utf8'))

export const ok = {
  schema: 'home_page',
  schema_version: 3,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'home',
  generated_at: '2026-10-26T21:30:00Z',
  data_times: [],
  run_status: 'ok',
  title: 'Today',
  say: 'Five cards, one page.',
  gap_line: '2 gaps, see below',
  overview: {
    gaps: [
      { card: 'Macro', text: 'Previous: no data yet' },
      { card: 'Held companies', text: 'News, last 5 days: no data source yet' },
    ],
    items: [
      { href: '/macro', label: 'Macro', text: 'Ease is neutral, −0.2 vs last week. The event this week is CPI.' },
      { href: '/macro', label: 'Market overview', text: 'Implied volatility is above realised, diff +2.0 vol points.' },
      { href: '/household', label: 'Household overview', text: 'Two accounts total · data day 10-21' },
      {
        href: '/positions',
        label: 'Held companies',
        text: 'Holding 9 · 3 with earnings within 28 days · News, last 5 days: no data source yet.',
      },
      { href: '/options', label: 'Options overview', text: 'Sold 5 groups this month, premium collected +$310.' },
    ],
  },
  report_links: [
    { href: '/report/weekly', text: 'Weekly report' },
    { href: '/report/monthly', text: 'Monthly report' },
    { href: '/report/quarterly', text: 'Quarterly report' },
  ],
  household,
  companies: held,
  macro,
  market: read('fixtures/v3/market-summary/ok.json'),
  options: read('../contracts/fixtures/page.options_summary.v4.ok.json'),
  stale_after: '2026-10-23T05:35:00+08:00',
} satisfies HomePage
