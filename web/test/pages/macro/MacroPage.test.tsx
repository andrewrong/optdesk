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
  expect(host.textContent?.split('data source not connected')).toHaveLength(2)
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

it('renders the liquidity block of the v2 golden before the events', async () => {
  const payload = v2('golden')
  expect(validateMacroPage(payload).ok).toBe(true)
  await show(payload)
  const block = host.querySelector('[data-liquidity]')!
  expect(block.textContent).toContain(payload.liquidity_title)
  for (const line of payload.liquidity.lines) expect(block.textContent).toContain(line.sentence)
  expect(
    block.compareDocumentPosition(host.querySelector('[data-events]')!) &
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
