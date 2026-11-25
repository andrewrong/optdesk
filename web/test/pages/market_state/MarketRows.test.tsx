// A value figure prints its payload text alone: the text already carries its unit, so nothing follows it.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import type { ChartSpec } from '@/charts/spec'
import { MarketRows } from '@/pages/market_state/MarketRows'
import type { Payload } from '@/types/market_state_page'

const drawn: { spec: ChartSpec; ariaText: string }[] = []
vi.mock('@/charts/Chart', () => ({
  Chart: (props: { spec: ChartSpec; ariaText: string }) => {
    drawn.push(props)
    return <div data-chart-stub />
  },
}))

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const ok = JSON.parse(
  readFileSync(
    path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.market_state_page.v1.ok.json'),
    'utf8',
  ),
) as Payload

type TestRow = Payload['market']['rows'][number]

async function render(patch: Partial<TestRow>): Promise<{ host: HTMLElement; done: () => Promise<void> }> {
  const base = ok.market.rows[0]!
  const block = { ...ok.market, rows: [{ ...base, ...patch }] } as unknown as Payload['market']
  const host = document.createElement('div')
  document.body.appendChild(host)
  const root = createRoot(host)
  await act(async () => {
    root.render(<MarketRows block={block} />)
  })
  return {
    host,
    done: async () => {
      await act(async () => root.unmount())
      host.remove()
    },
  }
}

async function figureOf(unit: string): Promise<{ text: string; span: Element | null }> {
  const { host, done } = await render({ figure: { kind: 'value', text: '7.2', unit } } as unknown as Partial<TestRow>)
  const row = host.querySelector('[data-market-row]')!
  const span = row.querySelector('span.ml-1')
  const text = row.textContent ?? ''
  await done()
  return { text, span }
}

describe('MarketRows figure unit', () => {
  it('prints the figure text alone for a percent unit, with no suffix or id after it', async () => {
    const { text, span } = await figureOf('percent')
    expect(span).toBeNull()
    expect(text).toContain('7.2')
    expect(text).not.toContain('7.2%')
    expect(text).not.toContain('percent')
  })

  it('prints nothing after the figure of a unit with no suffix', async () => {
    const { text, span } = await figureOf('index_points')
    expect(span).toBeNull()
    expect(text).not.toContain('index_points')
  })
})

describe('MarketRows range and hint', () => {
  // jsdom has no ResizeObserver, which the Radix tooltip measures with.
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  it('range badge shows state, cuts and notice', async () => {
    const { host, done } = await render({
      range: {
        state: 'caution',
        state_text: 'Caution (trial)',
        cuts: [
          { level: 'caution', text: 'Caution from 20', basis: 'published', source: 'A' },
          { level: 'alarm', text: 'Alarm from 30', basis: 'published', source: 'B' },
        ],
        notice: 'A notice only; nothing is blocked.',
      },
    } as Partial<TestRow>)
    const badge = host.querySelector('[data-range-state]')!
    expect(badge.textContent).toBe('Caution (trial)')
    expect(badge.getAttribute('data-range-state')).toBe('caution')
    const text = host.textContent ?? ''
    expect(text).toContain('Caution from 20')
    expect(text).toContain('Alarm from 30')
    expect(text).toContain('A notice only; nothing is blocked.')
    await done()
  })

  it('range gap prints its text', async () => {
    const { host, done } = await render({
      range: { kind: 'gap', reason: 'no_data', text: 'Range not available' },
    } as unknown as Partial<TestRow>)
    expect(host.querySelector('[data-range-state]')).toBeNull()
    expect(host.querySelector('[data-range-gap]')?.textContent).toBe('Range not available')
    await done()
  })

  it('hint opens as tooltip on the label', async () => {
    const { host, done } = await render({ hint: '这是提示文本' })
    expect(document.body.textContent).not.toContain('这是提示文本')
    const mark = host.querySelector('[data-hint-mark]') as HTMLElement
    expect(mark).not.toBeNull()
    await act(async () => {
      mark.click()
    })
    expect(document.body.textContent).toContain('这是提示文本')
    await done()
  })

  it('row without range is unchanged', async () => {
    const { host, done } = await render({ range: undefined, hint: undefined })
    expect(host.querySelector('[data-range-state]')).toBeNull()
    expect(host.querySelector('[data-range-gap]')).toBeNull()
    expect(host.querySelector('[data-hint-mark]')).toBeNull()
    expect(host.querySelector('[data-range-notice]')).toBeNull()
    await done()
  })
})

describe('MarketRows hint placement', () => {
  it('hint mark is not nested inside the row toggle button', async () => {
    const { host, done } = await render({ hint: '提示' })
    const mark = host.querySelector('[data-hint-mark]')!
    expect(mark.closest('button')).toBeNull()
    await done()
  })
})

describe('MarketRows price_chart', () => {
  const priceChart = {
    caption: 'caption of the price chart',
    aria_text: 'aria of the price chart',
    unit: 'index_points',
    points: [
      { day: '2026-11-10', close: 100, close_text: '100.00' },
      {
        day: '2026-11-11',
        close: 101,
        close_text: '101.00',
        ma200: 95,
        ma200_text: '95.00',
        band_low: 98,
        band_low_text: '98.00',
        band_high: 104,
        band_high_text: '104.00',
      },
    ],
  }
  const open = async (patch: Partial<TestRow>) => {
    drawn.length = 0
    const view = await render(patch)
    await act(async () => {
      view.host.querySelector('button')!.click()
    })
    return view
  }

  it('draws the average and the band from price_chart', async () => {
    const { host, done } = await open({ price_chart: priceChart } as unknown as Partial<TestRow>)
    const { spec, ariaText } = drawn.at(-1)!
    expect(spec.categories).toEqual(['2026-11-10', '2026-11-11'])
    expect(spec.series).toHaveLength(2)
    expect(spec.series[0]!.dashed).toBeUndefined()
    expect(spec.series[0]!.points).toEqual([
      { value: 100, text: '100.00' },
      { value: 101, text: '101.00' },
    ])
    expect(spec.series[1]!.dashed).toBe(true)
    expect(spec.series[1]!.points).toEqual([{ value: null }, { value: 95, text: '95.00' }])
    expect(spec.band!.low).toEqual([{ value: null }, { value: 98, text: '98.00' }])
    expect(spec.band!.high).toEqual([{ value: null }, { value: 104, text: '104.00' }])
    expect(spec.unit).toBe('index_points')
    expect(ariaText).toBe('aria of the price chart')
    expect(host.querySelector('[data-history-caption]')?.textContent).toBe('caption of the price chart')
    await done()
  })

  it('draws the history chart for a row without price_chart', async () => {
    const { done } = await open({
      history: {
        caption: 'old caption',
        aria_text: 'old aria',
        unit: 'x',
        points: [{ day: '2026-11-10', value: 1, text: '1.0' }],
      },
    } as unknown as Partial<TestRow>)
    const { spec } = drawn.at(-1)!
    expect(spec.band).toBeUndefined()
    expect(spec.series).toHaveLength(1)
    await done()
  })
})

describe('TestMarketRowsNasdaq', () => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  const ids = ['sp500_vs_200d', 'sp500_drawdown', 'nasdaq_drawdown', 'vix', 'vix_term', 'hy_spread']

  it('renders the Nasdaq-100 card right after the S&P 500 drawdown card, showing the payload text as given', async () => {
    const base = ok.market.rows[0]!
    const rows = ids.map((id) => ({ ...base, id, range: undefined, hint: undefined }) as unknown as TestRow)
    rows[2] = {
      ...base,
      id: 'nasdaq_drawdown',
      figure: { kind: 'value', text: '-7.31%' },
      range: {
        state: 'caution',
        state_text: 'Caution (trial)',
        cuts: [
          { level: 'caution', text: 'Caution from -8.20%', basis: 'percentile', source: 'own history' },
          { level: 'alarm', text: 'Alarm from -17.45%', basis: 'percentile', source: 'own history' },
        ],
        notice: 'A notice only; nothing is blocked.',
      },
      hint: '纳斯达克 100 距近 60 个交易日最高点的回撤。',
    } as unknown as TestRow
    const block = { ...ok.market, rows } as unknown as Payload['market']
    const host = document.createElement('div')
    document.body.appendChild(host)
    const root = createRoot(host)
    await act(async () => {
      root.render(<MarketRows block={block} />)
    })

    const found = [...host.querySelectorAll('[data-market-row]')].map((el) => el.getAttribute('data-market-row'))
    expect(found).toEqual(ids)
    const card = host.querySelector('[data-market-row="sp500_drawdown"]')!.nextElementSibling!
    expect(card.getAttribute('data-market-row')).toBe('nasdaq_drawdown')

    const text = card.textContent ?? ''
    expect(text).toContain('-7.31%')
    expect(card.querySelector('[data-range-state]')!.textContent).toBe('Caution (trial)')
    expect(card.querySelector('[data-range-state]')!.getAttribute('data-range-state')).toBe('caution')
    expect(text).toContain('Caution from -8.20%')
    expect(text).toContain('Alarm from -17.45%')
    expect(text).toContain('A notice only; nothing is blocked.')
    expect(card.querySelector('[data-hint-mark]')).not.toBeNull()
    await act(async () => {
      ;(card.querySelector('[data-hint-mark]') as HTMLElement).click()
    })
    expect(document.body.textContent).toContain('纳斯达克 100 距近 60 个交易日最高点的回撤。')

    await act(async () => root.unmount())
    host.remove()
  })
})
