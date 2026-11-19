// Issue #3286: the family page of docs/design/household-family-total.md prints `views` and `lead_notes` as sent,
// each only when present. The fixtures of fixtures/household-page/ are the reference; the browser computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { HouseholdPage, validateHouseholdPage } from '@/pages/household/HouseholdPage'

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
