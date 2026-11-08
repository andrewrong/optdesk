// Synthetic home page payload written from contracts/schemas/page.home_page.v2.json: the five members are the
// ok payloads of the member fixtures.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import type { HomePage } from '../../../../../src/generated/home_page.v2'
import { ok as held } from '../../held-v2/fixtures/ok'
import { ok as household } from '../../household-v2/fixtures/ok'
import { ok as macro } from '../../macro-v2/fixtures/ok'

const member = (dir: string) =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, '../../../../../fixtures/v3', dir, 'ok.json'), 'utf8'))

export const ok = {
  schema: 'home_page',
  schema_version: 2,
  run_id: 'run-demo-0001',
  build_id: 'build-demo-0001',
  scope: 'home',
  generated_at: '2026-10-26T21:30:00Z',
  data_times: [],
  run_status: 'ok',
  title: '今日总览',
  say: '五张卡片，一页看完。',
  gap_line: '今天没有缺口',
  block_links: [
    { href: '/macro', text: '宏观页' },
    { href: '/market', text: '市场页' },
    { href: '/household', text: '家庭页' },
    { href: '/companies', text: '持仓公司页' },
    { href: '/options', text: '期权页' },
  ],
  report_links: [
    { href: '/report/weekly', text: '周报' },
    { href: '/report/monthly', text: '月报' },
    { href: '/report/quarterly', text: '季报' },
  ],
  household,
  companies: held,
  macro,
  market: member('market-summary'),
  options: member('options-summary'),
} satisfies HomePage
