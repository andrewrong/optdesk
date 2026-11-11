// The options_summary v4 card shows the text fields of the payload and nothing else. Expected strings are read from
// the v4 fixtures of contracts/fixtures, not from the component.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ErrorBoundary } from '@/shell/ErrorBoundary'
import { App } from '@/shell/App'
import { OptionsSummaryCard, registration } from '@/pages/home/OptionsSummaryCard'
import type { ChartSpec } from '@/charts/spec'
import { validate } from '@/types/options_summary'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (dir: string, name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../fixtures', dir, name), 'utf8'),
  )
const os = (name: string) =>
  name === 'bad-version.json'
    ? fixture('v3/options-summary', name)
    : JSON.parse(
        readFileSync(
          path.resolve(import.meta.dirname, '../../../../contracts/fixtures', `page.options_summary.v4.${name}`),
          'utf8',
        ),
      )

const drawn: { spec: ChartSpec }[] = []
vi.mock('@/charts/Chart', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/charts/Chart')>()
  return {
    Chart: (props: { spec: ChartSpec; ariaText: string; name: string }) => {
      drawn.push(props)
      return actual.Chart(props)
    },
  }
})

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
  it('draws two series, premium and closed, with a label per point and the caption', async () => {
    const ok = os('ok.json')
    await showCard(ok)
    const spec = drawn.at(-1)!.spec
    expect(spec.series.map((s) => s.name)).toEqual([
      ok.card.premium_series_text,
      ok.card.closed_series_text,
    ])
    expect(spec.legend).toBe(true)
    const sent = ok.card.premium_series_6m
    expect(spec.categories).toEqual(sent.map((p: { label: string }) => p.label))
    expect(spec.series[0]!.points.map((p) => p.label)).toEqual(
      sent.map((p: { premium: { text: string } }) => p.premium.text),
    )
    expect(spec.series[1]!.points.map((p) => p.label)).toEqual(
      sent.map((p: { closed: { text: string } }) => p.closed.text),
    )
    expect(host.textContent).toContain(ok.card.premium_series_caption)
  })

  it('gives a gap point a null value with its text', async () => {
    const ok = os('ok.json')
    ok.card.premium_series_6m[0].closed = { kind: 'gap', reason: 'no_data', text: 'No data' }
    await showCard(ok)
    const point = drawn.at(-1)!.spec.series[1]!.points[0]
    expect(point).toEqual({ value: null, text: 'No data', label: 'No data' })
  })

  it('counts the sold tile as sold, not trades', async () => {
    const ok = os('ok.json')
    await showCard(ok)
    const text = tile('sold_count_month').textContent ?? ''
    expect(text).toContain('sold')
    expect(text).not.toContain('trades')
    expect(tile('sold_count_month').querySelector('[data-tile-figure]')?.textContent).toBe(
      ok.card.tiles.sold_count_month.figure.text,
    )
  })

  it('renders the title as a link to titleHref', async () => {
    const ok = os('ok.json')
    await show(<OptionsSummaryCard payload={ok} titleHref="/options" />)
    const a = host.querySelector('[data-slot=card-title] a')
    expect(a?.getAttribute('href')).toBe('/options')
    expect(a?.textContent).toBe(ok.card.title)
  })

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

  it('shows card.stale.text and data-stale true for stale.json', async () => {
    const stale = os('stale.json')
    await showCard(stale)
    const notice = host.querySelector('[data-stale]')!
    expect(notice.getAttribute('data-stale')).toBe('true')
    expect(notice.textContent).toBe(stale.card.stale.text)
  })

  it('gives data-stale false for ok.json', async () => {
    const ok = os('ok.json')
    await showCard(ok)
    expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('false')
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
