// The Macro card shows the fields of a macro_summary v2 payload and computes nothing. Fixtures are written from the
// schema alone in ./macro-v2/fixtures/; every expected text is read from them.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MacroCard, validateMacro } from '@/pages/home/MacroCard'
import { gap } from './macro-v2/fixtures/gap'
import { ok } from './macro-v2/fixtures/ok'
import { stale } from './macro-v2/fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const badVersion: unknown = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'macro-v2/fixtures/bad-version.json'), 'utf8'),
)
type Shown = Parameters<typeof MacroCard>[0]['payload']

let root: Root | null = null
let host: HTMLElement
async function show(payload: Shown, showGaps?: boolean) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MacroCard payload={payload} showGaps={showGaps} />)
  })
}
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
  root = null
  document.body.innerHTML = ''
})

describe('MacroCard (macro_summary v2)', () => {
  it('renders the title as a link to titleHref', async () => {
    host = document.createElement('div')
    document.body.append(host)
    await act(async () => {
      root = createRoot(host)
      root.render(<MacroCard payload={ok} titleHref="/macro" />)
    })
    const a = host.querySelector('[data-slot=card-title] a')
    expect(a?.getAttribute('href')).toBe('/macro')
    expect(a?.textContent).toBe(ok.card.title)
  })

  it('shows title, say, ease and stale text of ok as sent', async () => {
    await show(ok)
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(ok.card.title)
    expect(host.querySelector('[data-slot=card-description]')?.textContent).toBe(ok.card.say)
    expect(host.querySelector('[data-ease-figure]')?.textContent).toBe(ok.card.ease.figure.text)
    expect([...host.querySelectorAll('[data-ticks] span')].map((s) => s.textContent)).toEqual(
      ok.card.ease.ticks,
    )
    expect(host.querySelector('[data-neutral-band]')?.textContent).toBe(
      `Neutral band ${ok.card.ease.neutral_band.low.text} – ${ok.card.ease.neutral_band.high.text}`,
    )
    expect(host.textContent).toContain(ok.card.ease.reading_text)
    expect(host.querySelector('[data-week-change]')?.textContent).toContain(
      ok.card.ease.week_change.text,
    )
    expect(host.querySelector('[data-stale]')?.textContent).toBe(ok.card.stale.text)
    expect(host.querySelector('[role=slider]')?.getAttribute('aria-valuenow')).toBe(
      String(ok.card.ease.marker_percent.value),
    )
  })

  it('labels the week change', async () => {
    await show(ok)
    const text = host.querySelector('[data-week-change]')?.textContent ?? ''
    expect(text.startsWith('vs last week')).toBe(true)
    expect(text.indexOf('vs last week')).toBeLessThan(text.indexOf(ok.card.ease.week_change.text))
    expect(text.endsWith(ok.card.ease.week_change.text)).toBe(true)
  })

  it('shows the events of ok with the Gap text of Previous', async () => {
    await show(ok)
    const headers = [...host.querySelectorAll('[data-events] th')].map((h) => h.textContent)
    expect(headers).toEqual(['Date', 'Event', 'Previous'])
    const rows = [...host.querySelectorAll('[data-events] tbody tr')].map((r) =>
      [...r.querySelectorAll('td')].map((c) => c.textContent),
    )
    expect(rows).toEqual(
      ok.card.events.map((e) => [
        e.when_text,
        e.name_text,
        typeof e.previous === 'string' ? e.previous : expect.stringContaining(e.previous.text),
      ]),
    )
  })

  it('draws the fixed 0 to 100 scale with the neutral band texts and the stored marker', async () => {
    await show(ok)
    const ticks = [...host.querySelectorAll('[data-ticks] span')].map((s) => s.textContent)
    expect(ticks[0]).toBe('0')
    expect(ticks[ticks.length - 1]).toBe('100')
    expect(host.textContent).toContain('34%')
    expect(host.textContent).toContain('66%')
    expect(host.querySelector('[role=slider]')?.getAttribute('aria-valuenow')).toBe(
      String(ok.card.ease.marker_percent.value),
    )
  })

  it('shows the previous value of each event, and the missing text on the last one only', async () => {
    await show(ok)
    const cells = [...host.querySelectorAll('[data-events] tbody tr')].map(
      (r) => r.querySelectorAll('td')[2]?.textContent,
    )
    const last = ok.card.events.length - 1
    ok.card.events.forEach((event, index) => {
      if (index < last) {
        expect(typeof event.previous).toBe('string')
        expect(cells[index]).toBe(event.previous)
      } else {
        expect(typeof event.previous).toBe('object')
        expect(cells[index]).toContain('—')
        expect(cells[index]).toContain((event.previous as { text: string }).text)
      }
    })
  })

  it('shows the notes, the gaps and the data times text of ok', async () => {
    await show(ok)
    expect(host.querySelector('[data-gaps]')?.textContent).toBe(ok.card.gaps[0].text)
    for (const note of ok.card.notes) expect(host.textContent).toContain(note)
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.textContent).toContain(ok.card.data_times_text)
  })

  it('keeps the standalone gap alert by default and hides it when showGaps is false', async () => {
    await show(ok, false)
    expect(host.querySelector('[data-gaps]')).toBeNull()
  })

  it('shows the Gap ease of gap as its text, with no marker', async () => {
    expect('reason' in gap.card.ease).toBe(true)
    await show(gap)
    expect(host.querySelector('[data-ease-gap]')?.textContent).toBe(gap.card.ease.text)
    expect(host.textContent).toContain(gap.card.gaps[0].text)
    expect(host.querySelector('[role=slider]')).toBeNull()
  })

  it('shows the stale text of stale and flags it once the clock passes stale_after', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    try {
      vi.setSystemTime(new Date(Date.parse(stale.card.stale.stale_after) + 60_000))
      await show(stale)
      expect(host.querySelector('[data-stale]')?.textContent).toContain(stale.card.stale.text)
      expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('true')
      await act(async () => root?.unmount())
      host.remove()

      vi.setSystemTime(new Date(Date.parse(ok.generated_at) + 60_000))
      await show(ok)
      expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('false')
      expect(host.querySelector('[data-clock-skew]')).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('flags a device clock more than 5 minutes behind generated_at', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    try {
      vi.setSystemTime(new Date(Date.parse(ok.generated_at) - 6 * 60_000))
      await show(ok)
      expect(host.querySelector('[data-clock-skew]')).not.toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('escapes hostile event text', async () => {
    const hostile = '<img src=x onerror=alert(1)>'
    const events = [{ ...ok.card.events[0], name_text: hostile }] as unknown as Shown['card']['events']
    const payload: Shown = { ...ok, card: { ...ok.card, events } }
    await show(payload)
    expect(host.querySelector('img')).toBeNull()
    expect(host.textContent).toContain(hostile)
  })

  it('refuses bad-version and accepts the three v2 fixtures', () => {
    expect(validateMacro(badVersion).ok).toBe(false)
    for (const payload of [ok, gap, stale]) expect(validateMacro(payload).ok).toBe(true)
  })
})
