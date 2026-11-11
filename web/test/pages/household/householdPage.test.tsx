// Issue #1437: the /household page shows the text fields of `household_page` and nothing else. The fixtures of
// fixtures/household-page/ are the independent reference; the browser computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { HouseholdCard } from '@/pages/home/HouseholdCard'
import { HouseholdPage, validateHouseholdPage } from '@/pages/household/HouseholdPage'
import { monthChangeToSpec, trendToSpec } from '@/pages/household/householdToSpec'
import { pages } from '@/shell/pages'
import { ok } from '../home/household-v2/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const read = (dir: string, name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../fixtures', dir, name), 'utf8'),
  )
const fixture = (name: string) => read('household-page', name)
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
const BLOCKS = [
  'overview',
  'accounts',
  'composition',
  'movers',
  'trend',
  'stress',
  'income',
  'month_change',
  'cash_after',
  'obligations',
  'concentration',
  'allocation',
  'long_term',
] as const
beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('is registered and validates the fixtures', () => {
  expect(pages.household_page).toBeDefined()
  expect(validateHouseholdPage(fixture('ok.json')).ok).toBe(true)
  expect(validateHouseholdPage(fixture('bad-version.json')).ok).toBe(false)
})

it('shows the twelve blocks in schema order with the payload texts', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  const shown = [...host.querySelectorAll('[data-block]')]
  expect(shown.map((el) => el.getAttribute('data-block'))).toEqual([...BLOCKS])
  const text = host.textContent ?? ''
  expect(text).toContain(payload.sentence)
  expect(text).toContain(payload.stale.text)
  BLOCKS.forEach((name, i) => {
    expect(shown[i]!.textContent).toContain(payload[name].title)
    expect(shown[i]!.textContent).toContain(payload[name].meaning)
  })
  // a JPY cell keeps its ¥ amount and shows Python's ≈USD beside it
  const overview = host.querySelector('[data-block=overview]')!.textContent!
  expect(overview).toContain('¥3,000,000')
  expect(overview).toContain('≈$20,000')
  expect(overview).toContain(payload.overview.totals[0].text)
  // the Flex-only member's line: Python's cell texts and its own data day
  const accounts = host.querySelector('[data-block=accounts] table')!
  const row = payload.accounts.rows[0]
  expect(accounts.querySelector('tbody')!.textContent).toContain(row.net_liquidation.cell.text)
  expect(accounts.querySelector('tbody')!.textContent).toContain(row.cash.cell.text)
  expect(accounts.querySelector('tbody')!.textContent).toContain(row.data_day)
  expect(host.querySelector('[data-block=allocation]')!.textContent).toContain('+10.0')
  expect(host.querySelector('[data-block=month_change] [data-chart]')).not.toBeNull()
})

it('draws the scope headings, whose words, window sentences and the gap line as sent', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  const accounts = host.querySelector('[data-block=accounts]')!
  for (const row of payload.accounts.rows) {
    const section = accounts.querySelector(`[data-scope-rows=${row.scope}]`)!
    const scope = payload.scopes.find((s: { key: string }) => s.key === row.scope)
    expect(section.querySelector('h3')!.textContent).toBe(scope.label)
    expect(section.textContent).toContain(row.name)
    expect(section.textContent).toContain(row.whose)
  }
  const trend = host.querySelector('[data-block=trend]')!
  const windows = [...trend.querySelectorAll('[data-trend-window]')].map((p) => p.textContent)
  expect(windows).toEqual(payload.trend.series.map((s: { window: string }) => s.window))
  for (const series of payload.trend.series) expect(trend.textContent).toContain(series.change.text)
  const concentration = host.querySelector('[data-block=concentration]')!
  const gap = payload.concentration.scope_gaps[0]
  const line = concentration.querySelector(`[data-scope-rows=${gap.scope}] [data-scope-gap]`)!
  expect(line.textContent).toBe(gap.gap.text)
  expect(concentration.textContent).toContain(payload.concentration.rows[0].accounts[0].label)
})

it('draws the movers as two lists under their scope, in the order and with the texts sent', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  const movers = host.querySelector('[data-block=movers]')!
  const cells = (list: string) =>
    [...movers.querySelectorAll(`[data-mover-list=${list}] tbody tr`)].map((tr) =>
      [...tr.querySelectorAll('td')].map((td) => td.textContent),
    )
  const line = (row: {
    name: string
    account: { whose: string; label: string }
    previous: { text: string }
    current: { text: string }
    amount: { text: string }
  }) => [
    `${row.name} ${row.account.whose}${row.account.label}`,
    row.previous.text,
    row.current.text,
    row.amount.text,
  ]
  expect(cells('Stocks')).toEqual(payload.movers.stocks.map(line))
  expect(cells('Options')).toEqual(payload.movers.options.map(line))
  const daughter = movers.querySelector('[data-scope-rows=daughter]')!
  expect(daughter.textContent).toContain('BBB')
  expect(daughter.textContent).not.toContain(payload.movers.totals[0].text)
  const gap = payload.movers.scope_gaps[0]
  expect(movers.querySelector(`[data-scope-rows=${gap.scope}] [data-scope-gap]`)!.textContent).toBe(
    gap.gap.text,
  )
})

it('shows no table header that is not a payload text, and the obligations heads and total labels', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  const texts = new Set<string>()
  JSON.stringify(payload, (k, v) => {
    if (k === 'text' || k === 'title') texts.add(v)
    if (k === 'columns' || k === 'total_labels' || k === 'heads') for (const t of v) texts.add(t)
    return v
  })
  const headers = [...host.querySelectorAll('th')].map((th) => th.textContent?.trim() ?? '')
  for (const h of headers.filter(Boolean)) expect(texts.has(h)).toBe(true)

  const table = host.querySelector('[data-block=obligations] table')!
  const heads = [...table.querySelectorAll('th')].map((th) => th.textContent?.trim())
  expect(heads).toEqual(payload.obligations.columns)
  const names = [...table.querySelectorAll('tbody tr')].map(
    (tr) => tr.querySelector('td')?.textContent,
  )
  const labels = payload.obligations.total_labels
  expect(labels).toHaveLength(2)
  expect(new Set(labels).size).toBe(2)
  for (const l of labels) expect(names).toContain(l)
})

it('opens a month to its items as sent, without adding anything, and keeps the daughter sentence', async () => {
  // Issue #1682: the items, their summary line, the step sentences and the gap line are Python's texts.
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  const month = payload.income.rows.find((row: { items?: unknown[] | null }) => row.items)
  const node = host.querySelector(`[data-block=income] [data-month="${month.name}"]`)!
  expect(node.textContent).toContain(month.amount.text)
  const trigger = node.querySelector('button')!
  expect(node.textContent).not.toContain(month.items[0].item)
  await act(async () => trigger.click())
  expect(node.textContent).toContain(month.summary)
  const rows = [...node.querySelectorAll('tbody tr')].map((tr) =>
    [...tr.querySelectorAll('td')].map((td) => td.textContent),
  )
  expect(rows).toEqual(
    month.items.map(
      (i: { part: string; item: string; account: { label: string }; amount: { text: string } }) => [
        i.part,
        i.item,
        i.account.label,
        i.amount.text,
      ],
    ),
  )
  for (const gap of payload.income.scope_gaps ?? []) {
    expect(
      host.querySelector(`[data-block=income] [data-scope-gap=${gap.scope}]`)!.textContent,
    ).toBe(gap.gap.text)
  }
})

it('shows a gap as its text, with no figure', async () => {
  const payload = fixture('gap.json')
  await show(<HouseholdPage payload={payload} />)
  const trend = host.querySelector('[data-block=trend]')!
  // gap.json carries a gap on the block and on each of its two series: the series draw theirs, the block adds none
  expect(trend.querySelectorAll('[data-block-gap]')).toHaveLength(payload.trend.series.length)
  expect(payload.trend.series).toHaveLength(2)
  expect(trend.querySelector('[data-block-gap]')?.textContent).toBe(payload.trend.gap.text)
  expect(trend.querySelector('[data-block-gap]')?.getAttribute('data-reason')).toBe(
    payload.trend.gap.reason,
  )
  expect(trend.querySelector('[data-chart]')).toBeNull()
  const stress = host.querySelector('[data-block=stress]')!
  expect(stress.querySelector('[data-block-gap]')?.textContent).toBe(payload.stress.gap.text)
  expect(stress.querySelector('table')).toBeNull()
})

it('shows the trend gap sentence when the payload has no series', async () => {
  const payload = fixture('ok.json')
  payload.trend.rows = []
  payload.trend.series = []
  payload.trend.gap = { reason: 'no_data', text: 'No net liquidation history yet.' }
  await show(<HouseholdPage payload={payload} />)
  const trend = host.querySelector('[data-block=trend]')!
  const gaps = trend.querySelectorAll('[data-block-gap]')
  expect(gaps).toHaveLength(1)
  expect(gaps[0]!.textContent).toBe('No net liquidation history yet.')
  expect(gaps[0]!.getAttribute('data-reason')).toBe('no_data')
  expect(trend.querySelector('[data-chart]')).toBeNull()
})

it('links to /risk, and the Household overview card links to /household', async () => {
  await show(<HouseholdPage payload={fixture('ok.json')} />)
  const link = [...host.querySelectorAll('a')].find((a) => a.getAttribute('href') === '/risk')
  expect(link).toBeDefined()
  await act(async () => root?.unmount())
  host.remove()
  await show(<HouseholdCard payload={ok} />)
  const card = [...host.querySelectorAll('a')].find((a) => a.getAttribute('href') === '/household')
  expect(card?.textContent).toContain('Household page')
})

it('colours the Driver cell by the tone Python sent: up success, down down, flat muted, no amount none', async () => {
  const payload = fixture('ok.json')
  const money = (tone: string, text: string) => ({ ...payload.movers.stocks[0].amount, tone, text })
  const base = payload.movers.stocks[0]
  payload.movers.stocks = [
    { ...base, name: 'UP', amount: money('up', '+$50.00') },
    { ...base, name: 'DOWN', amount: money('down', '−$30.00') },
    { ...base, name: 'FLAT', amount: money('flat', '$0.00') },
    { ...base, name: 'NONE', amount: null, note: 'no amount' },
  ]
  payload.movers.options = []
  await show(<HouseholdPage payload={payload} />)
  const cell = (name: string) =>
    [...host.querySelectorAll('[data-mover-list=Stocks] tbody tr')]
      .find((tr) => tr.querySelector('td')!.textContent!.startsWith(name))!
      .querySelectorAll('td')[3]!
  expect(cell('UP').querySelector('.text-success')?.textContent).toBe('+$50.00')
  expect(cell('DOWN').querySelector('.text-down')?.textContent).toBe('−$30.00')
  expect(cell('FLAT').querySelector('.text-muted-foreground')?.textContent).toBe('$0.00')
  expect(cell('NONE').querySelector('.text-success, .text-down, .text-muted-foreground')).toBeNull()
  expect(cell('NONE').textContent).toBe('no amount')
})

it.each([
  ['up', 'text-success'],
  ['down', 'text-down'],
  ['flat', 'bg-secondary'],
  ['none', 'bg-secondary'],
])('draws the trend change Badge of tone %s with the class %s', async (tone, expected) => {
  const payload = fixture('ok.json')
  payload.trend.series[0].change.tone = tone
  await show(<HouseholdPage payload={payload} />)
  const badge = host.querySelector(
    `[data-block=trend] [data-scope-rows=${payload.trend.series[0].scope}] [data-slot=badge]`,
  )!
  expect(badge.textContent).toBe(payload.trend.series[0].change.text)
  expect(badge.className).toContain(expected)
})

// Issue #1749: one chart for the trend block, a line for each scope, y values are the payload's percent figures.
it('draws the trend as one chart with a line for each scope, as percent since the first point', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  expect(host.querySelectorAll('[data-block=trend] [data-chart]')).toHaveLength(1)
  const spec = trendToSpec(payload.trend.series)!
  const scopes = payload.trend.series
  expect(spec.kind).toBe('line')
  expect(spec.series.map((s) => s.name)).toEqual(scopes.map((s: { label: string }) => s.label))
  scopes.forEach((scope: { rows: { change: { value: number } }[] }, i: number) => {
    // a line has a point only on the days its scope has rows (no filling)
    expect(spec.series[i]!.points.flatMap((p) => (p.value === null ? [] : [p.value]))).toEqual(
      scope.rows.map((r) => r.change.value),
    )
  })
  const first = scopes[0].rows[1]
  const at = spec.categories.indexOf(first.day)
  expect(at).toBeGreaterThanOrEqual(0)
  expect(spec.series[0]!.points[at]!.text).toBe(`${first.change.text} ${first.value.text}`)
})

it('keeps the gap of a scope with no rows as a line and draws one line for the other', async () => {
  const payload = fixture('ok.json')
  const daughter = payload.trend.series[1]
  daughter.rows = []
  daughter.change = undefined
  daughter.window = undefined
  daughter.gap = { reason: 'missing_input', text: 'No data read for Trend' }
  await show(<HouseholdPage payload={payload} />)
  const trend = host.querySelector('[data-block=trend]')!
  expect(trend.querySelectorAll('[data-chart]')).toHaveLength(1)
  expect(trendToSpec(payload.trend.series)!.series).toHaveLength(1)
  expect(trend.querySelector('[data-scope-rows=daughter] [data-block-gap]')!.textContent).toBe(
    daughter.gap.text,
  )
})

it('draws no trend chart when no scope has rows', async () => {
  await show(<HouseholdPage payload={fixture('gap.json')} />)
  const trend = host.querySelector('[data-block=trend]')!
  expect(trend.querySelector('[data-chart]')).toBeNull()
  expect(trend.querySelectorAll('[data-scope-rows] [data-block-gap]')).toHaveLength(2)
})

// Issue #1759: the heads of Composition, Concentration and Allocation and the words of the header links are the payload's own texts.
it('prints the heads and the header link words the payload sends, and the route is no name', async () => {
  const payload = fixture('ok.json')
  payload.composition.heads = ['H1', 'H2', 'H3']
  payload.concentration.heads = ['C1', 'C2', 'C3']
  payload.allocation.heads = ['A1', 'A2', 'A3', 'A4', 'A5']
  payload.risk_link = 'RISK-WORDS'
  payload.positions_link = 'POSITION-WORDS'
  await show(<HouseholdPage payload={payload} />)
  const heads = (block: string) =>
    [...host.querySelectorAll(`[data-block=${block}] th`)].map((th) => th.textContent?.trim())
  expect(heads('composition')).toEqual(['H1', 'H2', 'H3'])
  expect(heads('concentration').slice(0, 3)).toEqual(['C1', 'C2', 'C3'])
  // Concentration draws name, share, amount (design order); Composition keeps name, amount, share.
  const first = (block: string) =>
    [...host.querySelector(`[data-block=${block}] tbody tr`)!.querySelectorAll('td')].map(
      (td) => td.textContent,
    )
  const row = payload.concentration.rows.find((r: { scope: string }) => r.scope === 'household')
  expect(first('concentration').slice(-2)).toEqual([row.share.text, row.amount.text])
  expect(first('composition').slice(1, 3)).toEqual([
    payload.composition.rows[0].amount.text,
    payload.composition.rows[0].share.text,
  ])
  expect(heads('allocation')).toEqual(['A1', 'A2', 'A3', 'A4', 'A5'])
  for (const [href, words] of [
    ['/risk', 'RISK-WORDS'],
    ['/positions', 'POSITION-WORDS'],
  ]) {
    const link = [...host.querySelectorAll('a')].find((a) => a.getAttribute('href') === href)
    expect(link?.textContent).toBe(words)
    expect(link?.getAttribute('aria-label')).toBe(words)
  }
})

// Issue #1760: the result sentences are Python's, shown under the title; none where the field is null.
it('shows the four result sentences as sent and none where the field is null', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  for (const name of ['concentration', 'cash_after', 'income', 'month_change'] as const) {
    expect(payload[name].result).toBeTruthy()
    expect(host.querySelector(`[data-block=${name}] [data-result]`)!.textContent).toBe(
      payload[name].result,
    )
  }
  await act(async () => root?.unmount())
  host.remove()
  const bare = structuredClone(payload)
  for (const name of ['concentration', 'cash_after', 'income', 'month_change'] as const)
    bare[name].result = null
  await show(<HouseholdPage payload={bare} />)
  expect(host.querySelectorAll('[data-result]')).toHaveLength(0)
})

it('draws the month change as a transparent base stacked under the visible height, as sent', async () => {
  const payload = fixture('ok.json')
  await show(<HouseholdPage payload={payload} />)
  expect(host.querySelectorAll('[data-block=month_change] [data-chart]')).toHaveLength(1)
  const spec = monthChangeToSpec(payload.month_change)
  const rows = payload.month_change.rows
  expect(spec.series).toHaveLength(2)
  expect(spec.series[0]!.stack).toBe(spec.series[1]!.stack)
  expect(spec.series[0]!.invisible).toBe(true)
  expect(spec.series[0]!.points.map((p) => p.value)).toEqual(
    rows.map((r: { base: { value: number } }) => r.base.value),
  )
  expect(spec.series[1]!.points.map((p) => p.value)).toEqual(
    rows.map((r: { height: { value: number } }) => r.height.value),
  )
})

// Issue #1444: the page keeps the payload order in a grid; seven blocks are wide, six are half from lg.
it('puts the blocks in cells in payload order, the seven wide ones spanning two columns from lg', async () => {
  await show(<HouseholdPage payload={fixture('ok.json')} />)
  const cells = [...host.querySelectorAll('[data-cell]')]
  expect(cells.map((el) => el.getAttribute('data-cell'))).toEqual([...BLOCKS])
  const wide = ['overview', 'accounts', 'composition', 'movers', 'income', 'obligations', 'concentration']
  cells.forEach((el) => {
    const name = el.getAttribute('data-cell')!
    expect(el.classList.contains('lg:col-span-2')).toBe(wide.includes(name))
  })
  const grid = cells[0]!.parentElement!
  expect(grid.className).toContain('grid-cols-1')
  expect(grid.className).toContain('lg:grid-cols-2')
})

it('leaves a gap in the trend line at a null point instead of connecting over it', () => {
  const payload = fixture('ok.json')
  const spec = trendToSpec(payload.trend.series)!
  expect(spec.series.every((s) => s.hole === true)).toBe(true)
  expect(spec.series.some((s) => s.points.some((p) => p.value === null))).toBe(true)
})
