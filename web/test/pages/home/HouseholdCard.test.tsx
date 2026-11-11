// Issue #1132: the Household overview card shows the text fields of the payload and nothing else. Fixtures are the
// household_summary v2 payloads of ./household-v2/fixtures/, written from the schema alone.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { HouseholdCard, registration } from '@/pages/home/HouseholdCard'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

import { validate } from '@/types/household_summary'
import { gap } from './household-v2/fixtures/gap'
import { ok } from './household-v2/fixtures/ok'
import { stale } from './household-v2/fixtures/stale'

const fixtures = { 'ok.json': ok, 'gap.json': gap, 'stale.json': stale }
// A fresh copy, so a test may edit it.
const fixture = (name: keyof typeof fixtures) => structuredClone(fixtures[name]) as any // eslint-disable-line @typescript-eslint/no-explicit-any
const badVersion: unknown = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'household-v2/fixtures/bad-version.json'), 'utf8'),
)
// jsdom has no ResizeObserver, which the Radix tooltip popper needs once it opens.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<HouseholdCard payload={payload as never} />)
  })
}
afterEach(async () => {
  vi.useRealTimers()
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})
const tile = (id: string) => host.querySelector<HTMLElement>(`[data-tile=${id}]`)!

describe('HouseholdCard', () => {
  it('renders the title as a link to titleHref', async () => {
    host = document.createElement('div')
    document.body.append(host)
    await act(async () => {
      root = createRoot(host)
      root.render(<HouseholdCard payload={fixture('ok.json')} titleHref="/household" />)
    })
    const a = host.querySelector('[data-slot=card-title] a')
    expect(a?.getAttribute('href')).toBe('/household')
    expect(a?.textContent).toBe(ok.card.title)
  })

  it('is registered for the schema household_summary', () => {
    expect(registration.schema).toBe('household_summary')
  })

  it('shows the title, subtitle and four tiles as sent', async () => {
    await show(fixture('ok.json'))
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(ok.card.title)
    expect(host.textContent).toContain(ok.card.subtitle)
    expect(host.querySelectorAll('[data-tile]')).toHaveLength(4)
    expect(tile('net_worth').textContent).toContain('Net value')
    expect(tile('net_worth').textContent).toContain(ok.card.tiles.net_worth.figure.text)
    expect(tile('day_change').textContent).toContain(ok.card.tiles.day_change.figure.text)
    expect(tile('day_change').textContent).toContain(ok.card.tiles.day_change.caption.text)
    expect(tile('cash').textContent).toContain(ok.card.tiles.cash.figure.text)
    expect(tile('cash').textContent).toContain(ok.card.tiles.cash.caption.text)
  })

  it('shows the net-worth caption with the family and daughter parts as text, the total from figure.text', async () => {
    await show(fixture('ok.json'))
    const net = tile('net_worth').textContent!
    const caption = ok.card.tiles.net_worth.caption.text
    expect(net).toContain(caption)
    expect(net).toContain('Household (me + spouse)')
    expect(net).toContain('Daughter (counted separately)')
    expect(net).toContain('already in the total')
    expect(net).toContain(ok.card.tiles.net_worth.figure.text)
  })

  it('explains a tile on focus of its info mark, and only when the payload sends explain', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    for (const id of ['net_worth', 'day_change', 'month_to_date', 'cash']) {
      expect(tile(id).querySelectorAll('[data-explain]')).toHaveLength(1)
    }
    const trigger = tile('month_to_date').querySelector<HTMLElement>('[data-explain]')!
    expect(trigger.getAttribute('aria-label')).toBe('Month to date (excl. deposits and withdrawals)')
    await act(async () => trigger.focus())
    const text = payload.card.tiles.month_to_date.explain
    expect(document.body.querySelector('[data-slot=tooltip-content]')?.textContent).toContain(text)

    await act(async () => root?.unmount())
    document.body.innerHTML = ''
    delete payload.card.tiles.month_to_date.explain
    await show(payload)
    expect(tile('month_to_date').querySelector('[data-explain]')).toBeNull()
    expect(tile('cash').querySelectorAll('[data-explain]')).toHaveLength(1)
  })

  it('prints the month and cash tiles as sent, and a negative cash has no pill', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    expect(tile('month_to_date').textContent).toContain('Month to date (excl. deposits and withdrawals)')
    expect(tile('month_to_date').textContent).toContain('+$11,700')
    expect(tile('month_to_date').textContent).toContain('+2.94%')

    await act(async () => root?.unmount())
    host.remove()
    const month = payload.card.tiles.month_to_date
    month.caption = { text: 'No base' }
    const cash = payload.card.tiles.cash
    cash.label = 'Margin loan'
    cash.figure = { text: 'On margin' }
    cash.caption = { value: -5000, text: '−$5,000', unit: 'usd_signed', tone: 'down' }
    await show(payload)
    expect(tile('month_to_date').textContent).toContain('No base')
    expect(tile('cash').textContent).toContain('Margin loan')
    expect(tile('cash').textContent).toContain('On margin')
    expect(tile('cash').textContent).toContain('−$5,000')
    expect(tile('cash').querySelector('[data-slot=badge]')).toBeNull()
    expect(tile('month_to_date').querySelector('[data-slot=badge]')).toBeNull()
  })

  it('shows a Gap figure as its text, never 0', async () => {
    await show(fixture('gap.json'))
    expect(tile('cash').querySelector('[data-slot=badge]')?.textContent).toBe(gap.card.gaps[1].text)
    expect(tile('month_to_date').querySelector('[data-slot=badge]')?.textContent).toBe(
      gap.card.gaps[0].text,
    )
    expect(tile('cash').textContent).not.toMatch(/\b0(\.0+)?%?\b|\$0\b/)
  })

  it('shows the stale label as sent', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(stale.generated_at))
    await show(fixture('stale.json'))
    expect(host.querySelector('[data-stale]')?.textContent).toBe(stale.card.stale.text)
    expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('true')
  })

  it('colours a figure by its tone only', async () => {
    const ok = fixture('ok.json')
    await show(ok)
    const figure = tile('day_change').querySelector('[data-figure]')!
    expect(figure.className).toContain('text-success')
    await act(async () => root?.unmount())
    host.remove()
    ok.card.tiles.day_change.figure.tone = 'down'
    await show(ok)
    const down = tile('day_change').querySelector('[data-figure]')!
    expect(down.className).toContain('text-down')
    expect(down.className).not.toContain('text-success')
    expect(tile('net_worth').querySelector('[data-figure]')!.className).not.toMatch(
      /text-success|text-down/,
    )
  })

  it('colours a down caption down only, and flat, none, null or missing tones not at all', async () => {
    const ok = fixture('ok.json')
    ok.card.tiles.day_change.caption.tone = 'down'
    await show(ok)
    const caption = tile('day_change').querySelectorAll<HTMLElement>('[data-figure]')[1]!
    expect(caption.className).toContain('text-down')
    expect(caption.className).not.toContain('text-muted-foreground')
    for (const tone of ['flat', 'none', null]) {
      await act(async () => root?.unmount())
      host.remove()
      ok.card.tiles.day_change.figure.tone = tone
      await show(ok)
      const figure = tile('day_change').querySelector('[data-figure]')!
      expect(figure.className).not.toMatch(/text-success|text-down/)
    }
  })

  it('shows notes, the gaps list and the data times text', async () => {
    await show(fixture('ok.json'))
    expect(host.textContent).toContain(ok.card.notes[1])
    expect(host.querySelector('[data-gaps]')).toBeNull()
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.textContent).toContain(ok.card.data_times_text)
  })

  it('shows the stale label of the ok fixture, not yet stale at its generation time', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(ok.generated_at))
    await show(fixture('ok.json'))
    expect(host.querySelector('[data-stale]')?.textContent).toBe(ok.card.stale.text)
    expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('false')
  })

  it('lists the gaps of the gap fixture and refuses a schema_version it does not read', async () => {
    await show(fixture('gap.json'))
    expect(host.querySelector('[data-gaps]')?.textContent).toContain(gap.card.gaps[0].text)
    expect(validate(badVersion).ok).toBe(false)
    expect(validate(ok).ok).toBe(true)
  })
})
