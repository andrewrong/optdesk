// The market_summary v3 card shows the text fields of the payload and nothing else. Expected strings are read
// from the fixtures of fixtures/v3/market-summary, not written here.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToString } from 'react-dom/server'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MarketSummaryCard, registration } from '@/pages/home/MarketSummaryCard'
import type { ChartSpec } from '@/charts/spec'
import { App } from '@/shell/App'
import { validate } from '@/types/market_summary'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const read = (dir: string, name: string) =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, '../../../fixtures', dir, name), 'utf8'))
const fixture = (name: string) => read('v3/market-summary', name)

const drawn: ChartSpec[] = []
vi.mock('@/charts/Chart', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/charts/Chart')>()
  return {
    ...actual,
    Chart: (props: Parameters<typeof actual.Chart>[0]) => {
      drawn.push(props.spec)
      return actual.Chart(props)
    },
  }
})

const xAxes: Record<string, unknown>[] = []
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>()
  return {
    ...actual,
    XAxis: (props: Record<string, unknown>) => {
      xAxes.push(props)
      return null
    },
    LineChart: ({ children }: { children?: React.ReactNode }) => <svg>{children}</svg>,
    BarChart: ({ children }: { children?: React.ReactNode }) => <svg>{children}</svg>,
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
beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('MarketSummaryCard', () => {
  it('shows title, say, table rows and data times as sent', async () => {
    const payload = fixture('ok.json')
    await show(<MarketSummaryCard payload={payload} />)
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(payload.card.title)
    expect(host.querySelector('[data-slot=card-description]')?.textContent).toBe(payload.card.say)
    const headers = [...host.querySelectorAll('[data-index-table] th')].map((h) => h.textContent)
    for (const name of payload.card.index_table.columns) expect(headers).toContain(name)
    const body = host.querySelector('[data-index-table] tbody')?.textContent ?? ''
    for (const row of payload.card.index_table.rows) {
      expect(body).toContain(row.name)
      expect(body).toContain(row.iv.text)
      expect(body).toContain(row.rv20.text)
      expect(body).toContain(row.iv_minus_rv20.text)
    }
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.querySelector('[data-slot=accordion-content]')?.textContent).toBe(
      payload.card.data_times_text,
    )
    expect(host.querySelector('[role=img]')?.getAttribute('aria-label')).toBe(
      payload.card.chart.aria_text,
    )
    for (const label of payload.card.chart.end_labels) {
      expect(host.querySelector('[data-end-labels]')?.textContent).toContain(label)
    }
  })

  it('shows the text of a Gap figure and invents no digit', async () => {
    const payload = fixture('gap.json')
    await show(<MarketSummaryCard payload={payload} />)
    const at = payload.card.index_table.rows.findIndex((r: { name: string }) => r.name === 'N225')
    const gapRow = payload.card.index_table.rows[at]
    const cells = [...host.querySelectorAll('[data-index-table] tbody tr')][at]?.textContent ?? ''
    expect(cells).toContain(gapRow.iv.text)
    expect(cells).toContain(gapRow.iv_minus_rv20.text)
    // The row's own text, minus the texts of its figures, holds no digit.
    const rest = cells
      .replace(gapRow.name, '')
      .replace(gapRow.iv.text, '')
      .replace(gapRow.iv_minus_rv20.text, '')
      .replace(gapRow.rv20.text, '')
    expect(rest).not.toMatch(/\d/)
    const level = payload.card.vol_indices[2].level
    expect(host.querySelector('[data-vol-indices]')?.textContent).toContain(level.text)
  })

  it('draws the three named series over at least 30 points', async () => {
    const payload = fixture('ok.json')
    drawn.length = 0
    await show(<MarketSummaryCard payload={payload} />)
    const spec = drawn[drawn.length - 1]!
    expect(new Set(spec.series.map((s) => s.name))).toEqual(new Set(payload.card.chart.series_names))
    expect(payload.card.chart.series_names).toHaveLength(3)
    expect(spec.categories.length).toBeGreaterThanOrEqual(30)
  })

  it('thins the x-axis labels', async () => {
    const payload = fixture('ok.json')
    xAxes.length = 0
    drawn.length = 0
    await show(<MarketSummaryCard payload={payload} />)
    expect(xAxes.length).toBeGreaterThan(0)
    const axis = xAxes[xAxes.length - 1]!
    expect(axis.interval).toBe('preserveStartEnd')
    expect(axis.minTickGap).toBe(24)
    expect(drawn[drawn.length - 1]!.categories).toEqual(
      payload.card.chart.points.map((point: { label: string }) => point.label),
    )
  })

  it('shows the SPX, NDX and RUT rows of ok', async () => {
    await show(<MarketSummaryCard payload={fixture('ok.json')} />)
    const rows = [...host.querySelectorAll('[data-index-table] tbody tr')].map((r) => r.textContent ?? '')
    for (const name of ['SPX', 'NDX', 'RUT']) {
      expect(rows.some((text) => text.includes(name))).toBe(true)
    }
  })

  it('shows the N225 gap text of gap', async () => {
    const payload = fixture('gap.json')
    await show(<MarketSummaryCard payload={payload} />)
    const row = payload.card.index_table.rows.find((r: { name: string }) => r.name === 'N225')
    const shown = [...host.querySelectorAll('[data-index-table] tbody tr')].find((r) =>
      r.textContent?.includes('N225'),
    )
    expect(shown?.textContent).toContain(row.iv.text)
  })

  it('shows the stale label', async () => {
    const payload = fixture('stale.json')
    await show(<MarketSummaryCard payload={payload} />)
    expect(host.querySelector('[data-stale]')?.textContent).toBe(payload.card.stale.text)
    expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('true')
  })

  it('shows data-stale false for a fresh payload', async () => {
    await show(<MarketSummaryCard payload={fixture('ok.json')} />)
    expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('false')
  })
})

describe('validate', () => {
  it('rejects schema_version 2 and shows the gap text, no card body', () => {
    const result = validate(fixture('bad-version.json'))
    expect(result.ok).toBe(false)
    if (result.ok) return
    expect(result.gap.reason).toBe('unsupported_version')
    vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => renderToString(<App payload={fixture('bad-version.json')} />)).toThrow(
      result.gap.text,
    )
  })

  it('registers under market_summary', () => {
    expect(registration.schema).toBe('market_summary')
  })
})

describe('shell', () => {
  it('shows no market-summary card for another schema', async () => {
    const title = fixture('ok.json').card.title
    await show(<App payload={read('market-weather', 'ok.json')} />)
    expect(host.querySelector('[data-card=market-summary]')).toBeNull()
    expect(host.textContent).not.toContain(title)
  })

  it('shows the card for a market_summary payload', async () => {
    const payload = fixture('ok.json')
    await show(<App payload={payload} />)
    expect(host.querySelector('[data-card=market-summary]')).not.toBeNull()
    expect(host.textContent).toContain(payload.card.title)
  })
})
