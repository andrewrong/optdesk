// Issue #1206 board 8 and fixture strings are the independent references.
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
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
  expect(host.querySelector('[role=progressbar]')?.getAttribute('aria-valuenow')).toBe('56')
  expect(host.textContent).toContain('中性，较上周 −0.2。')
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
      // Issue #1456: the 详情 link leads where Python's detail_href says; a row without one has no link.
      expect(rows[i]!.querySelector('a')?.getAttribute('href') ?? null).toBe(indicator.detail_href)
    },
  )
})

it('draws the -2 to +2 scale, every status text, the detail links and the missing cause once', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  const ticks = [...host.querySelectorAll('[data-ticks] span')].map((n) => n.textContent)
  expect(ticks[0]).toBe('-2')
  expect(ticks.at(-1)).toBe('+2')
  expect(ticks).toEqual(['-2', '-1', '0', '+1', '+2'])
  expect(payload.ease.aria_text).toContain('-2 到 +2')
  expect(host.querySelector('[data-neutral-band]')?.textContent).toBe('-0.5 – +0.5')
  expect(host.querySelector('[role=progressbar]')?.getAttribute('aria-valuenow')).toBe(
    String(payload.ease.marker_percent.value),
  )
  for (const indicator of payload.indicators as { label: string; detail_href: string | null }[]) {
    expect(host.querySelector('[data-indicators]')?.textContent).toContain(indicator.label)
  }
  const hrefs = [...host.querySelectorAll('[data-indicators] a')].map((a) => a.getAttribute('href'))
  expect(hrefs).toEqual(
    payload.indicators.flatMap((i: { detail_href: string | null }) => (i.detail_href ? [i.detail_href] : [])),
  )
  expect(host.textContent?.split('数据源未接入')).toHaveLength(2)
  expect(host.textContent).toContain('没有缺口。')
})

it('shows the ease gap once without a gauge and keeps the missing indicator reason', async () => {
  const payload = fixture('gap.json')
  await show(payload)
  expect(host.querySelector('[role=progressbar]')).toBeNull()
  expect(host.textContent?.split(payload.ease.text)).toHaveLength(2)
  const row = [...host.querySelectorAll('[data-indicators] tbody tr')].find((row) =>
    row.textContent?.includes('中国 PMI'),
  )!
  expect(row.textContent).toContain('数据源未接入')
  expect(row.querySelectorAll('td')[1]!.textContent).not.toBe('0')
})

it('shows Python stale and page gap labels as alerts', async () => {
  const payload = fixture('stale.json')
  payload.gaps = [{ reason: 'no_data', text: '独立的页面缺口' }]
  await show(payload)
  const alerts = [...host.querySelectorAll('[role=alert]')].map((node) => node.textContent)
  expect(alerts).toContain(payload.stale.text)
  expect(alerts).toContain('独立的页面缺口')
})

it('colours each indicator source badge by the status Python sent', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  const cls = { 及时: 'text-success', 缺: 'text-destructive', 迟到: 'text-warning' } as const
  // The name cell is the first of a row; a gap cell elsewhere also shows a Badge.
  const badges = [
    ...host.querySelectorAll('[data-indicators] tbody td:first-child [data-slot=badge]'),
  ]
  const seen = badges.map((b) => b.textContent)
  expect(seen).toEqual(expect.arrayContaining(['及时', '缺', '迟到']))
  for (const b of badges) expect(b.classList).toContain(cls[b.textContent as keyof typeof cls])
})
