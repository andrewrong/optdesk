// Issue #1224 acceptance 6, 10 and 14: the browser hides the 盘中 section from `expires_at` with a clock it does not
// compute with, reads the page again every 60 s, and draws nothing when the payload has no section. Expected texts
// are read from the fixtures, not from the component.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { WeatherCard } from '@/pages/home/WeatherCard'
import { WeatherIntraday } from '@/pages/home/WeatherIntraday'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/market-weather', `${name}.json`),
      'utf8',
    ),
  )
let root: Root | null = null
let host: HTMLElement
const reload = vi.fn()
// The section alone, with the re-read injected so no page is really reloaded.
async function show(payload: { intraday?: unknown; closed?: unknown }) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(
      <WeatherIntraday
        intraday={payload.intraday as never}
        closed={payload.closed as never}
        reload={reload}
      />,
    )
  })
}
const section = () => host.querySelector('[data-intraday]')
const expired = () => host.querySelector('[data-intraday-expired]')

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
beforeEach(() => {
  vi.useFakeTimers()
})
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
  vi.useRealTimers()
  reload.mockReset()
})

// The fixture expires at 2026-10-22T22:43:00+08:00 (14:43 UTC); its reading was made 14:30 UTC.
const EXPIRES = Date.parse('2026-10-22T22:43:00+08:00')

describe('WeatherIntraday', () => {
  it('shows the section, rows and status line of the fixture before expires_at', async () => {
    const intraday = fixture('intraday')
    vi.setSystemTime(EXPIRES - 60_000)
    await show(intraday)
    expect(section()).not.toBeNull()
    expect(host.textContent).toContain(intraday.intraday.gauge.label)
    expect(host.textContent).toContain(intraday.intraday.gauge.headline)
    for (const row of intraday.intraday.rows) {
      expect(host.textContent).toContain(`${row.series} ${row.value}`)
      expect(host.textContent).toContain(row.as_of)
      expect(host.textContent).toContain(row.label)
    }
    for (const carried of intraday.intraday.gauge.carried) {
      expect(host.textContent).toContain(carried.vote)
      expect(host.textContent).toContain(carried.close_date.slice(5))
    }
    expect(host.querySelector('[data-intraday-as-of]')?.textContent).toBe(intraday.intraday.as_of)
    expect(host.querySelector('[data-intraday-status]')?.textContent).toBe(
      intraday.intraday.status_line,
    )
    expect(expired()).toBeNull()
  })

  it('shows the SPX row with its label, time zone and ratio text from the fixture (issue #1225)', async () => {
    const spx = fixture('intraday-spx')
    vi.setSystemTime(EXPIRES - 60_000)
    await show(spx)
    const row = spx.intraday.rows.find((r: { series: string }) => r.series === 'SPX')
    expect(row.label).toBe('盘中·延迟')
    expect(host.textContent).toContain(`SPX ${row.value}`)
    expect(host.textContent).toContain(row.label)
    expect(host.textContent).toContain(row.as_of)
    expect(
      host.querySelector('[data-intraday-row="SPX"] [data-intraday-detail]')?.textContent,
    ).toBe(row.detail)
    expect(host.querySelector('[data-intraday-trend-line]')).toBeNull()
  })

  it('shows the one persistent trend line while the SPX leg falls back to the close (issue #1225)', async () => {
    const gap = fixture('intraday-spx-fallback')
    vi.setSystemTime(EXPIRES - 60_000)
    await show(gap)
    expect(host.querySelector('[data-intraday-trend-line]')?.textContent).toBe(
      gap.intraday.trend_line,
    )
    expect(host.querySelector('[data-intraday-row="SPX"]')).toBeNull()
    expect(host.querySelector('[data-intraday-row="VIX"]')).not.toBeNull()
  })

  it('shows the valid and the stale quote without a gauge when a leg is stale', async () => {
    const partial = fixture('intraday-partial')
    vi.setSystemTime(EXPIRES - 60_000)
    await show(partial)
    expect(section()?.getAttribute('data-intraday')).toBe('stale')
    expect(host.querySelector('[data-intraday-carried]')).toBeNull()
    expect(host.querySelector('[data-intraday-stale]')?.textContent).toBe(
      partial.intraday.rows[1].stale.text,
    )
    expect(host.querySelector('[data-intraday-status]')?.textContent).toBe(
      partial.intraday.status_line,
    )
  })

  it('hides the section at expires_at on the timer and shows the expired text, with no new page', async () => {
    const intraday = fixture('intraday')
    vi.setSystemTime(EXPIRES - 30_000)
    await show(intraday)
    expect(section()).not.toBeNull()
    await act(async () => {
      vi.setSystemTime(EXPIRES) // now >= expires_at: the equal instant already hides
      vi.advanceTimersByTime(60_000)
    })
    expect(section()).toBeNull()
    expect(expired()?.textContent).toBe(intraday.intraday.expired_text)
    expect(host.querySelector('[data-intraday-status]')).toBeNull()
  })

  it('shows the section again when a newer page arrives with a later expires_at', async () => {
    vi.setSystemTime(EXPIRES + 1_000)
    const later = fixture('intraday')
    later.intraday.expires_at = '2026-10-22T22:48:00+08:00'
    await show(later)
    expect(section()).not.toBeNull()
    expect(expired()).toBeNull()
  })

  it('draws nothing of the section when the payload has none', async () => {
    vi.setSystemTime(EXPIRES)
    await show(fixture('ok'))
    expect(section()).toBeNull()
    expect(expired()).toBeNull()
    expect(host.querySelector('[data-closed]')).toBeNull()
  })

  it('reads the page again every 60 s while the feature is on, and not otherwise', async () => {
    vi.setSystemTime(EXPIRES - 600_000)
    await show(fixture('intraday'))
    await act(async () => {
      vi.advanceTimersByTime(180_000)
    })
    expect(reload).toHaveBeenCalledTimes(3)
    await act(async () => root?.unmount())
    document.body.innerHTML = ''
    reload.mockReset()
    await show(fixture('ok'))
    await act(async () => {
      vi.advanceTimersByTime(180_000)
    })
    expect(reload).not.toHaveBeenCalled()
  })
})

describe('WeatherIntraday in the card', () => {
  it('is drawn inside the weather card from the payload field', async () => {
    const intraday = fixture('intraday')
    vi.setSystemTime(EXPIRES - 60_000)
    host = document.createElement('div')
    document.body.append(host)
    await act(async () => {
      root = createRoot(host)
      root.render(<WeatherCard payload={intraday} />)
    })
    expect(host.querySelector('[data-card="market-weather"] [data-intraday]')).not.toBeNull()
    // The close gauge is still there beside it.
    expect(host.querySelector('[data-headline]')?.textContent).toBe(intraday.headline)
  })

  it('shows the closed text of the payload and still reads the page again', async () => {
    vi.setSystemTime(EXPIRES)
    const closed = { ...fixture('ok'), closed: '休市 · 显示 10-21 收盘天气' }
    await show(closed)
    expect(host.querySelector('[data-closed]')?.textContent).toBe(closed.closed)
    expect(section()).toBeNull()
    await act(async () => {
      vi.advanceTimersByTime(60_000)
    })
    expect(reload).toHaveBeenCalledTimes(1)
  })
})
