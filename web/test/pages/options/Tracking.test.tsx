// The tracking page shows the strings of the v2 payload grouped by rule version; the synthetic fixtures of
// tracking-v2/fixtures are the reference.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { Tracking, registration } from '@/pages/options/Tracking'
import { gap } from './tracking-v2/fixtures/gap'
import { ok } from './tracking-v2/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'tracking-v2/fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<Tracking payload={payload as never} />)
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

it('registers the page, refuses bad-version and accepts ok', () => {
  expect(registration.schema).toBe('tracking')
  expect(registration.page.validate(badVersion).ok).toBe(false)
  expect(registration.page.validate(ok).ok).toBe(true)
})

it('shows the current version, every tile, the notes, the comparison and My trades', async () => {
  await show(ok)
  const text = host.textContent ?? ''
  const current = ok.versions[0]
  expect(text).toContain(current.label)
  expect(text).toContain(current.expired_text)
  for (const line of ok.how) expect(text).toContain(line)
  for (const group of current.groups) {
    expect(text).toContain(group.title)
    if (Array.isArray(group.summary)) {
      for (const tile of group.summary) {
        expect(text).toContain(tile.label)
        expect(text).toContain(tile.text)
      }
    }
  }
  for (const figure of ['31', '24', '3', '+0.9%', '+0.2%']) expect(text).toContain(figure)
  expect(text).toContain('Too early to conclude.')
  for (const cell of (current.covered_calls.comparison.rows as unknown as string[][]).flat()) expect(text).toContain(cell)
  for (const cell of (current.my_trades as unknown as { rows: string[][] }).rows.flat()) expect(text).toContain(cell)
  expect(text).not.toContain('OLD')
})

it('folds the group rows closed', async () => {
  await show(ok)
  expect(host.textContent).not.toContain('AAA')
  const trigger = host.querySelector('[data-tracking-group="sell-put"] [data-slot="accordion-trigger"]')
  expect(trigger?.getAttribute('aria-expanded')).toBe('false')
})

it('renders a my_trades gap as text with no My trades table and folds older versions', async () => {
  await show(gap)
  const text = host.textContent ?? ''
  expect(text).toContain('My trades have no data source yet')
  expect(text).toContain('This group has no results to sum up yet')
  expect(host.querySelectorAll('table')).toHaveLength(0)

  await act(async () => root?.unmount())
  host.remove()
  await show(ok)
  const old = [...host.querySelectorAll('[data-slot="accordion-item"]')].find((i) =>
    i.textContent?.includes('Earlier rule versions'),
  )
  expect(old?.querySelector('[data-slot="accordion-trigger"]')?.getAttribute('aria-expanded')).toBe('false')
  expect(host.textContent).not.toContain('Rule v2')
})
