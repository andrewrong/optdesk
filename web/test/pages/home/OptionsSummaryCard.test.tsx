// Issue #130: the options_summary v2 card shows the text fields of the payload and nothing else. Expected strings
// are read from the fixtures of fixtures/options-summary/, not from the component.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ErrorBoundary } from '@/shell/ErrorBoundary'
import { App } from '@/shell/App'
import { OptionsSummaryCard, registration } from '@/pages/home/OptionsSummaryCard'
import { validate } from '@/types/options_summary'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (dir: string, name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../fixtures', dir, name), 'utf8'),
  )
const os = (name: string) => fixture('options-summary', name)

let root: Root | null = null
let host: HTMLElement
async function show(node: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(node)
  })
}
const showCard = (payload: unknown) => show(<OptionsSummaryCard payload={payload as never} />)
beforeEach(() => vi.spyOn(console, 'error').mockImplementation(() => {}))
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

const KEYS = ['sold_count_month', 'premium_month_usd', 'open_premium_usd', 'max_loss_usd']
const tile = (key: string) => host.querySelector(`[data-tile=${key}]`)!

describe('OptionsSummaryCard', () => {
  it('rejects bad-version.json with unsupported_version', () => {
    const result = validate(os('bad-version.json'))
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.gap.reason).toBe('unsupported_version')
  })

  it('registers under the schema name options_summary', () => {
    expect(registration.schema).toBe('options_summary')
  })

  it('shows label, figure and caption of each of the four tiles as sent', async () => {
    const ok = os('ok.json')
    await showCard(ok)
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(ok.card.title)
    for (const key of KEYS) {
      const sent = ok.card.tiles[key]
      expect(tile(key).querySelector('[data-tile-label]')?.textContent).toBe(sent.label)
      expect(tile(key).querySelector('[data-tile-figure]')?.textContent).toBe(sent.figure.text)
      expect(tile(key).querySelector('[data-tile-caption]')?.textContent ?? null).toBe(
        sent.caption ?? null,
      )
    }
  })

  it('shows the premium series text, its caption and a chart', async () => {
    const ok = os('ok.json')
    await showCard(ok)
    expect(host.querySelector('[data-series-text]')?.textContent).toBe(ok.card.premium_series_text)
    expect(host.querySelector('[data-series-caption]')?.textContent).toBe(
      ok.card.premium_series_caption,
    )
    expect(host.querySelector('[data-chart]')?.getAttribute('aria-label')).toBe(
      ok.card.premium_series_text,
    )
  })

  it('shows a Gap tile as its text and not a zero', async () => {
    const gap = os('gap.json')
    await showCard(gap)
    for (const key of ['open_premium_usd', 'max_loss_usd']) {
      expect('reason' in gap.card.tiles[key].figure).toBe(true)
      expect(tile(key).querySelector('[data-tile-gap]')?.textContent).toBe(
        gap.card.tiles[key].figure.text,
      )
      expect(tile(key).querySelector('[data-tile-figure]')).toBeNull()
      expect(tile(key).textContent).not.toMatch(/\b0\b/)
    }
  })

  it('shows card.stale.text for stale.json', async () => {
    const stale = os('stale.json')
    await showCard(stale)
    expect(host.querySelector('[data-stale-label]')?.textContent).toBe(stale.card.stale.text)
  })
})

describe('App with options_summary', () => {
  it('shows the card for an options_summary payload and none for another schema', async () => {
    const ok = os('ok.json')
    await show(<App payload={ok} />)
    expect(host.textContent).toContain(ok.card.title)
    expect(host.querySelector('[data-card=options-summary]')).not.toBeNull()
    await act(async () => root?.unmount())
    host.remove()
    await show(<App payload={fixture('market-weather', 'ok.json')} />)
    expect(host.querySelector('[data-card=options-summary]')).toBeNull()
    expect(host.textContent).not.toContain(ok.card.title)
  })

  it('renders no card body for bad-version.json, and the gap text is what is thrown', async () => {
    const bad = os('bad-version.json')
    const gap = validate(bad)
    await show(
      <ErrorBoundary onError={() => {}}>
        <App payload={bad} />
      </ErrorBoundary>,
    )
    expect(host.querySelector('[data-card=options-summary]')).toBeNull()
    expect(host.textContent).toBe('')
    expect(gap.ok ? '' : gap.gap.text).toMatch(/Unsupported schema_version/)
  })
})

describe('OptionsSummaryCard and the clock', () => {
  it('renders the same stale tone whatever the device clock says', async () => {
    const ok = os('ok.json')
    vi.useFakeTimers({ toFake: ['Date'] })
    try {
      vi.setSystemTime(new Date(Date.parse(ok.card.stale.stale_after) - 1000))
      await showCard(ok)
      // the chart carries generated ids, so compare the stale label and the tiles
      const outside = () =>
        ['[data-stale-label]', '[data-tiles]'].map((sel) => host.querySelector(sel)?.outerHTML)
      const before = outside()
      await act(async () => root?.unmount())
      document.body.innerHTML = ''
      vi.setSystemTime(new Date(Date.parse(ok.card.stale.stale_after) + 1000))
      await showCard(ok)
      expect(outside()).toEqual(before)
      expect(host.querySelector('[data-clock-skew]')).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })
})
