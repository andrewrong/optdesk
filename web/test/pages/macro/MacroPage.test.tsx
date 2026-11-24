// Issue #1206 board 8 and fixture strings are the independent references.
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { MacroPage, validateMacroPage } from '@/pages/macro/MacroPage'
import { pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const dir = path.resolve(import.meta.dirname, '../../../fixtures/macro-page')
const fixture = (name: string) => JSON.parse(readFileSync(path.join(dir, name), 'utf8'))
let root: Root | undefined
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MacroPage payload={payload as never} />)
  })
}
async function openEase() {
  await act(async () => {
    host.querySelector<HTMLElement>('[data-ease-fold] [data-slot=accordion-trigger]')!.click()
  })
}
const v2dir = path.resolve(import.meta.dirname, '../../../../contracts/fixtures')
const v2 = (name: string) =>
  JSON.parse(readFileSync(path.join(v2dir, `page.macro_page.v2.${name}.json`), 'utf8'))
// jsdom has no ResizeObserver, which the Radix slider measures its thumb with.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('validates fixtures and registers the macro page', () => {
  expect(pages.macro_page).toBeDefined()
  for (const name of readdirSync(dir).filter(
    (name) => name.endsWith('.json') && (!name.startsWith('bad-') || name === 'bad-version.json'),
  )) {
    expect(validateMacroPage(fixture(name)).ok, name).toBe(!name.startsWith('bad-'))
  }
})

it('renders the Python gauge and every event and indicator with its label', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  await openEase()
  expect(host.querySelector('[role=slider]')?.getAttribute('aria-valuenow')).toBe('56')
  expect(host.textContent).toContain('Neutral, −0.2 from last week.')
  const events = host.querySelectorAll('[data-events] tbody tr')
  expect(events).toHaveLength(payload.events.length)
  payload.events.forEach((event: { name_text: string; when_text: string }, i: number) => {
    expect(events[i]!.textContent).toContain(event.name_text)
    expect(events[i]!.textContent).toContain(event.when_text)
  })
  const rows = host.querySelectorAll('[data-indicators] tbody tr')
  expect(rows).toHaveLength(payload.indicators.length)
  payload.indicators.forEach(
    (
      indicator: {
        name: string
        latest: { text: string }
        label: string
        source_text: string
        detail_href: string | null
      },
      i: number,
    ) => {
      expect(rows[i]!.textContent).toContain(indicator.name)
      expect(rows[i]!.textContent).toContain(indicator.latest.text)
      expect(rows[i]!.textContent).toContain(indicator.source_text)
      expect(rows[i]!.querySelector('[data-slot=badge]')?.textContent).toBe(indicator.label)
      // Issue #1456: the Detail link leads where Python's detail_href says; a row without one has no link.
      expect(rows[i]!.querySelector('a')?.getAttribute('href') ?? null).toBe(indicator.detail_href)
    },
  )
})

it('draws the 0 to 100 scale, every status text, the detail links and the missing cause once', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  await openEase()
  const ticks = [...host.querySelectorAll('[data-ticks] span')].map((n) => n.textContent)
  expect(ticks[0]).toBe('0')
  expect(ticks.at(-1)).toBe('100')
  expect(ticks).toEqual(['0', '25', '50', '75', '100'])
  expect(payload.ease.aria_text).toContain('0 to 100')
  expect(host.querySelector('[data-neutral-band]')?.textContent).toBe('33.5% – 66.5%')
  expect(host.querySelector('[role=slider]')?.getAttribute('aria-valuenow')).toBe(
    String(payload.ease.marker_percent.value),
  )
  for (const indicator of payload.indicators as { label: string; detail_href: string | null }[]) {
    expect(host.querySelector('[data-indicators]')?.textContent).toContain(indicator.label)
  }
  const hrefs = [...host.querySelectorAll('[data-indicators] a')].map((a) => a.getAttribute('href'))
  expect(hrefs).toEqual(
    payload.indicators.flatMap((i: { detail_href: string | null }) => (i.detail_href ? [i.detail_href] : [])),
  )
  // once in the dash's sr-only text and once printed beside it
  expect(host.textContent?.split('data source not connected')).toHaveLength(3)
  expect(host.textContent).toContain('No gaps.')
})

it('shows the ease gap once without a gauge and keeps the missing indicator reason', async () => {
  const payload = fixture('gap.json')
  await show(payload)
  await openEase()
  expect(host.querySelector('[role=slider]')).toBeNull()
  expect(host.textContent?.split(payload.ease.text)).toHaveLength(2)
  const row = [...host.querySelectorAll('[data-indicators] tbody tr')].find((row) =>
    row.textContent?.includes('China PMI'),
  )!
  expect(row.textContent).toContain('data source not connected')
  expect(row.querySelectorAll('td')[1]!.textContent).not.toBe('0')
})

it('shows Python stale and page gap labels', async () => {
  const payload = fixture('stale.json')
  payload.gaps = [{ reason: 'no_data', text: 'standalone page gap' }]
  await show(payload)
  const alerts = [...host.querySelectorAll('[role=alert]')].map((node) => node.textContent)
  expect(alerts).toContain(payload.stale.text)
  expect(host.querySelector('[data-gaps]')?.textContent).toContain('standalone page gap')
})

it('colours each indicator source badge by the status Python sent', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  const cls = { 'On time': 'text-success', 'n/a': 'text-destructive', Late: 'text-warning' } as const
  // The name cell is the first of a row; a gap cell elsewhere also shows a Badge.
  const badges = [
    ...host.querySelectorAll('[data-indicators] tbody td:first-child [data-slot=badge]'),
  ]
  const seen = badges.map((b) => b.textContent)
  expect(seen).toEqual(expect.arrayContaining(['On time', 'n/a', 'Late']))
  for (const b of badges) expect(b.classList).toContain(cls[b.textContent as keyof typeof cls])
})

it('uses the gauge', async () => {
  await show(fixture('ok.json'))
  await openEase()
  expect(host.querySelectorAll('[role=slider]')).toHaveLength(1)
  expect(host.querySelectorAll('[data-ease-gauge]')).toHaveLength(1)
  expect(host.querySelector('[role=progressbar]')).toBeNull()
})

it('shows the Ease only inside a closed fold: the closed page has no figure, the opened one has it', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  expect(host.querySelector('[data-ease-fold] [data-slot=accordion-trigger]')?.textContent).toBe(
    payload.ease_title,
  )
  expect(host.querySelector('[data-ease-figure]')).toBeNull()
  expect(host.querySelector('[data-ease-gauge]')).toBeNull()
  expect(host.querySelector('[data-neutral-band]')).toBeNull()
  await openEase()
  expect(host.querySelector('[data-ease-figure]')?.textContent).toBe(payload.ease.figure.text)
})

it('renders a version 1 payload without the liquidity block', async () => {
  await show(fixture('ok.json'))
  expect(host.querySelector('[data-liquidity]')).toBeNull()
})

it('renders the liquidity block of the v2 golden after the events', async () => {
  const payload = v2('golden')
  expect(validateMacroPage(payload).ok).toBe(true)
  await show(payload)
  const block = host.querySelector('[data-liquidity]')!
  expect(block.textContent).toContain(payload.liquidity_title)
  for (const line of payload.liquidity.lines) expect(block.textContent).toContain(line.sentence)
  expect(
    host.querySelector('[data-events]')!.compareDocumentPosition(block) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy()
  expect(host.querySelector('[data-ease-figure]')).toBeNull()
  await openEase()
  expect(host.querySelector('[data-ease-figure]')?.textContent).toBe(payload.ease.figure.text)
})

it('renders the v2 gap fixture and shows its gap text', async () => {
  const payload = v2('gap')
  expect(validateMacroPage(payload).ok).toBe(true)
  await show(payload)
  const text = 'reason' in payload.liquidity ? payload.liquidity.text : payload.ease.text
  if (!('reason' in payload.liquidity)) await openEase()
  expect(host.textContent).toContain(text)
})

it('shows the stale label of the v2 stale fixture', async () => {
  const payload = v2('stale')
  expect(validateMacroPage(payload).ok).toBe(true)
  await show(payload)
  const alerts = [...host.querySelectorAll('[role=alert]')].map((node) => node.textContent)
  expect(alerts).toContain(payload.stale.text)
})

it('refuses the v2 bad-version fixture', () => {
  expect(validateMacroPage(v2('bad-version')).ok).toBe(false)
})

// Decisions 2, 3 and 5 of docs/design/macro-page-v2.md: the optional fields are added to the golden here.
const lead = {
  title: 'What a put seller reads first',
  note: 'display only; no rule reads this',
  rows: [
    ['tbill', 'ok'],
    ['hy_spread', 'ok'],
    ['next_event', 'ok'],
    ['index_iv', 'ok'],
  ].map(([key, state]) => ({
    key,
    state,
    label: `label ${key}`,
    text: `text ${key}`,
    change_text: null,
    data_day: null,
    hint: `hint ${key}`,
  })),
}
function withV2Fields() {
  const payload = v2('golden')
  payload.lead = lead
  payload.events[0].days_to = 19
  payload.events[0].in_window = true
  payload.events[1].days_to = 50
  payload.events[1].in_window = false
  payload.indicators[0].group = 'rates'
  payload.indicators[0].change_text = '+0.10 pp over five sessions'
  payload.indicators[0].data_day = '2026-10-23'
  payload.indicators[0].hint = 'hint cpi'
  payload.indicators[1].group = 'credit'
  payload.indicators[2].group = 'rates'
  payload.indicators[2].latest = { text: '99.1', unit: 'index_points', value: 99.1 }
  return payload
}
const position = (a: Element, b: Element) => a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING

it('TestMacroPageOrder: lead, events, indicators, ease, liquidity, notes; no lead renders without it', async () => {
  const payload = withV2Fields()
  expect(validateMacroPage(payload).ok).toBe(true)
  await show(payload)
  const order = ['[data-lead]', '[data-events]', '[data-indicators]', '[data-ease-fold]', '[data-liquidity]', '[data-notes]']
  const nodes = order.map((sel) => host.querySelector(sel)!)
  nodes.forEach((n, i) => expect(n, order[i]).not.toBeNull())
  for (let i = 1; i < nodes.length; i++) expect(position(nodes[i - 1]!, nodes[i]!), order[i]).toBeTruthy()
  await act(async () => root?.unmount())
  host.remove()
  const plain = v2('golden')
  await show(plain)
  expect(host.querySelector('[data-lead]')).toBeNull()
  expect(host.querySelector('[data-events]')).not.toBeNull()
  expect(host.querySelector('[data-indicators]')).not.toBeNull()
})

it('TestEventWindowMarker: marker and day count only where the payload sends them', async () => {
  await show(withV2Fields())
  const rows = host.querySelectorAll('[data-events] tbody tr')
  expect(rows[0]!.textContent).toContain('in window')
  expect(rows[0]!.textContent).toContain('19')
  expect(rows[1]!.textContent).not.toContain('in window')
  expect(rows[1]!.textContent).toContain('50')
  await act(async () => root?.unmount())
  host.remove()
  await show(v2('golden'))
  expect(host.querySelector('[data-events]')!.textContent).not.toContain('in window')
})

it('TestGroupedIndicators: groups in payload order with change, day and hint', async () => {
  await show(withV2Fields())
  const groups = [...host.querySelectorAll('[data-indicator-group]')]
  expect(groups.map((g) => g.getAttribute('data-indicator-group'))).toEqual(['rates', 'credit'])
  const rates = groups[0]!.querySelectorAll('tbody tr')
  expect(rates).toHaveLength(2)
  expect(rates[0]!.textContent).toContain('+0.10 pp over five sessions')
  expect(rates[0]!.textContent).toContain('2026-10-23')
  expect(rates[0]!.querySelector('[aria-label="hint cpi"]')).not.toBeNull()
  expect(rates[1]!.textContent).toContain('99.1')
  expect(rates[1]!.querySelector('[data-hint]')).toBeNull()
  expect(groups[1]!.textContent).toContain('data source not connected')
})

it('prints the reason of an indicator gap beside the dash, as the payload sent it', async () => {
  const payload = v2('golden')
  payload.indicators[1].latest = { reason: 'no_data', text: 'no data yet' }
  await show(payload)
  const row = host.querySelectorAll('[data-indicators] tbody tr')[1]!
  expect(row.querySelector('[data-missing-reason]')?.textContent).toBe('no data yet')
  expect(row.querySelector('[data-missing]')).not.toBeNull()
})

it('prints no missing reason for an indicator with a value', async () => {
  await show(v2('golden'))
  const row = host.querySelectorAll('[data-indicators] tbody tr')[0]!
  expect(row.querySelector('[data-missing-reason]')).toBeNull()
})

it('renders actual column', async () => {
  await show(fixture('ok.json'))
  const heads = [...host.querySelectorAll('[data-events] thead th')].map((th) => th.textContent)
  expect(heads.indexOf('Actual')).toBe(heads.indexOf('Previous') + 1)
  const col = heads.indexOf('Actual')
  const rows = host.querySelectorAll('[data-events] tbody tr')
  expect(rows[0]!.querySelectorAll('td')[col]!.textContent).toBe('3.1%')
  expect(rows[1]!.querySelectorAll('td')[col]!.textContent).toContain('not yet published')
})

it('renders an event without actual as before', async () => {
  const payload = fixture('ok.json')
  delete payload.events[0].actual
  await show(payload)
  const col = [...host.querySelectorAll('[data-events] thead th')].findIndex((th) =>
    th.textContent?.includes('Actual'),
  )
  expect(host.querySelectorAll('[data-events] tbody tr')[0]!.querySelectorAll('td')[col]!.textContent).not.toContain('3.1%')
})
