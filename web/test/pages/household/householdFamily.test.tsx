// Issue #3286: the family page of docs/design/household-family-total.md prints `views` and `lead_notes` as sent,
// each only when present. The fixtures of fixtures/household-page/ are the reference; the browser computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { HouseholdPage, validateHouseholdPage } from '@/pages/household/HouseholdPage'
import { trendToSpec } from '@/pages/household/householdToSpec'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/household-page', name),
      'utf8',
    ),
  )
let root: Root | undefined
let host: HTMLElement
async function show(element: React.ReactElement) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(element)
  })
}
beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('prints the family page: the links, the lead note, three account rows, the count row and the cash note', async () => {
  const payload = fixture('family.json')
  expect(validateHouseholdPage(payload).ok).toBe(true)
  await show(<HouseholdPage payload={payload} />)
  const links = host.querySelector('[data-household-views]')!
  const items = [...links.querySelectorAll('[data-view]')]
  expect(items.map((el) => el.textContent)).toEqual(
    payload.views.map((v: { label: string }) => v.label),
  )
  payload.views.forEach((v: { href: string; current: boolean }, i: number) => {
    const anchor = items[i]!.closest('a') ?? items[i]!.querySelector('a')
    if (v.current) {
      expect(anchor).toBeNull()
      expect(items[i]!.getAttribute('aria-current')).toBe('page')
    } else {
      expect(anchor!.getAttribute('href')).toBe(v.href)
    }
  })
  const lead = [...host.querySelectorAll('[data-lead-note]')]
  expect(lead.map((p) => p.textContent)).toEqual(payload.lead_notes)
  expect(lead[0]!.className).toContain('text-muted-foreground')
  const rows = host.querySelectorAll('[data-block=accounts] tbody tr')
  expect(rows).toHaveLength(3)
  expect(rows[2]!.textContent).toContain(payload.accounts.rows[2].name)
  expect(rows[2]!.textContent).toContain(payload.accounts.rows[2].net_liquidation.gap)
  expect(host.querySelector('[data-block=overview]')!.textContent).toContain('计入 2 / 3')
  expect(host.textContent).toContain(payload.notes[0])
})

it('prints no row of links, no lead note and no error for a page without them', async () => {
  const payload = fixture('ok.json')
  expect(payload.views).toBeUndefined()
  expect(payload.lead_notes).toBeUndefined()
  await show(<HouseholdPage payload={payload} />)
  expect(host.querySelector('[data-household-views]')).toBeNull()
  expect(host.querySelector('[data-view]')).toBeNull()
  expect(host.querySelector('[data-lead-note]')).toBeNull()
  expect(host.querySelector('[data-block-gap]')).toBeNull()
  expect(host.querySelector('[data-block=overview]')).not.toBeNull()
})

// Issue #3589: the family trend of docs/design/net-value-trend.md, entry 3: the page draws `trend.series` as sent.
it('draws the family line and one line per account', async () => {
  const payload = fixture('family.json')
  expect(validateHouseholdPage(payload).ok).toBe(true)
  const series = payload.trend.series as {
    label: string
    scope: string
    window: string
    rows: { day: string; value: { value: number } }[]
  }[]
  expect(series.map((s) => s.scope)[0]).toBe('household')
  expect(series).toHaveLength(3)
  await show(<HouseholdPage payload={payload} />)
  const trend = host.querySelector('[data-block=trend]')!
  expect(trend.querySelectorAll('[data-chart]')).toHaveLength(1)
  expect(trend.querySelector('[data-block-gap]')).toBeNull()
  const spec = trendToSpec(payload.trend.series)!
  expect(spec.legend).toBe(true)
  expect(spec.series.map((s) => s.name)).toEqual(['全家合计', '账户（main）', '账户（daughter）'])
  expect(spec.series[0]!.points.map((p) => p.value)).toEqual([100000, 100750, 101500])
  expect(trend.querySelector('[data-scope-rows=household] [data-trend-window]')!.textContent).toBe(
    payload.trend.window,
  )
})

it('shows the gap when the trend has no series', async () => {
  const payload = fixture('family.json')
  payload.trend.series = []
  payload.trend.rows = []
  payload.trend.gap = { reason: 'no_data', text: '全家净值记录不足 2 天，画不出走势' }
  await show(<HouseholdPage payload={payload} />)
  const trend = host.querySelector('[data-block=trend]')!
  const gaps = trend.querySelectorAll('[data-block-gap]')
  expect(gaps).toHaveLength(1)
  expect(gaps[0]!.textContent).toBe('全家净值记录不足 2 天，画不出走势')
  expect(gaps[0]!.getAttribute('data-reason')).toBe('no_data')
  expect(trend.querySelector('[data-chart]')).toBeNull()
})

// Issue #3613: the family risk blocks of docs/design/household-risk.md, entry 3: rows, notes and totals as sent.
type PayloadRow = { name: string; amount: { text: string }; share: { text: string } }
// Each table row holds its payload row's name and the one text given, as sent.
function expectRowsShow(rows: Element[], given: PayloadRow[], text: (r: PayloadRow) => string) {
  given.forEach((r, i) => {
    expect(rows[i]!.textContent).toContain(r.name)
    expect(rows[i]!.textContent).toContain(text(r))
  })
}
const RISK_BLOCKS = ['obligations', 'cash_after', 'stress', 'concentration'] as const

it('shows the family risk blocks', async () => {
  const payload = fixture('family.json')
  expect(validateHouseholdPage(payload).ok).toBe(true)
  await show(<HouseholdPage payload={payload} />)
  for (const name of RISK_BLOCKS) {
    const block = host.querySelector(`[data-block=${name}]`)!
    expect(payload[name].title).toMatch(/^家庭风险/)
    expect(block.textContent).toContain(payload[name].title)
    expect(block.querySelector('[data-block-gap]')).toBeNull()
  }
  const cash = host.querySelector('[data-block=cash_after]')!
  const rows = [...cash.querySelectorAll('tbody tr')]
  expect(rows).toHaveLength(3)
  expectRowsShow(rows, payload.cash_after.rows, (r) => r.amount.text)
  expect(rows[1]!.textContent).toContain('缺口：现金不够')
  expect(rows[0]!.textContent).not.toContain('缺口')
  expect(rows[2]!.textContent).toContain(payload.cash_after.totals[0].text)
  const stress = [...host.querySelectorAll('[data-block=stress] tbody tr')]
  expect(stress).toHaveLength(payload.stress.rows.length)
  expectRowsShow(stress, payload.stress.rows, (r) => r.share.text)
  expect(host.querySelector('[data-block=obligations]')!.textContent).toContain(
    payload.obligations.rows[0].note,
  )
})

it('shows the concentration rows of the family scope', async () => {
  const payload = fixture('family.json')
  expect(validateHouseholdPage(payload).ok).toBe(true)
  await show(<HouseholdPage payload={payload} />)
  const scope = host.querySelector('[data-block=concentration] [data-scope-rows=household]')!
  const rows = [...scope.querySelectorAll('tbody tr')]
  // the household total row stays under the household scope, after the rows
  expect(rows).toHaveLength(payload.concentration.rows.length + 1)
  expect(rows[rows.length - 1]!.textContent).toContain(payload.concentration.totals[0].text)
  payload.concentration.rows.forEach((r: { scope: string }) => expect(r.scope).toBe('household'))
  expectRowsShow(rows, payload.concentration.rows, (r) => r.share.text)
})

it('shows the gap of a risk block', async () => {
  const payload = fixture('family.json')
  payload.concentration.rows = []
  payload.concentration.totals = []
  payload.concentration.gap = { reason: 'no_reader', text: '此处暂时读不到持仓，集中度算不出' }
  await show(<HouseholdPage payload={payload} />)
  const gaps = host.querySelectorAll('[data-block=concentration] [data-block-gap]')
  expect(gaps).toHaveLength(1)
  expect(gaps[0]!.textContent).toBe('此处暂时读不到持仓，集中度算不出')
  expect(gaps[0]!.getAttribute('data-reason')).toBe('no_reader')
  for (const name of ['obligations', 'cash_after', 'stress'] as const) {
    const block = host.querySelector(`[data-block=${name}]`)!
    expect(block.querySelector('[data-block-gap]')).toBeNull()
    expect(block.querySelectorAll('tbody tr').length).toBeGreaterThan(payload[name].rows.length - 1)
  }
})
