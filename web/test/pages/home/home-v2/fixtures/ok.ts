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
  title: '今日总览',
  say: '五张卡片，一页看完。',
  gap_line: '2 处缺口，见下',
  overview: {
    gaps: [
      { card: '宏观', text: '前值暂缺' },
      { card: '持仓公司', text: '近 5 日消息：暂无数据源' },
    ],
    items: [
      { href: '/macro', label: '宏观', text: '松紧度中性，较上周 −0.2。本周的事件是 CPI。' },
      { href: '/macro', label: '市场概览', text: '隐含波动率高于已实现，差 +2.0 个波动点。' },
      { href: '/household', label: '家庭概览', text: '两户合计 · 数据日 10-21' },
      {
        href: '/positions',
        label: '持仓公司',
        text: '持有 9 家 · 3 家 28 日内财报 · 近 5 日消息：暂无数据源。',
      },
      { href: '/options', label: '期权概览', text: '本月卖出 5 组，收取权利金 +$310。' },
    ],
  },
  report_links: [
    { href: '/report/weekly', text: '周报' },
    { href: '/report/monthly', text: '月报' },
    { href: '/report/quarterly', text: '季报' },
  ],
  household,
  companies: held,
  macro,
  market: read('fixtures/v3/market-summary/ok.json'),
  options: read('../contracts/fixtures/page.options_summary.v4.ok.json'),
  stale_after: '2026-10-23T05:35:00+08:00',
} satisfies HomePage
