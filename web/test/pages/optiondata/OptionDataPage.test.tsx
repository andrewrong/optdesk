// The fixtures of contracts/fixtures are the independent reference: the page shows their texts as written.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act, cloneElement, type ReactElement } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { OptionDataPage, validateOptionDataPage } from '@/pages/optiondata/OptionDataPage'
import type { OptionDataPagePayload } from '@/pages/optiondata/OptionDataPage'

const yAxisProps = vi.hoisted(() => [] as Record<string, unknown>[])
vi.mock('recharts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('recharts')>()),
  ResponsiveContainer: ({ children }: { children: ReactElement<{ width: number; height: number }> }) =>
    cloneElement(children, { width: 640, height: 288 }),
  YAxis: (props: Record<string, unknown>) => {
    yAxisProps.push(props)
    return null
  },
}))

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const dir = path.resolve(import.meta.dirname, '../../../../contracts/fixtures')
const fixture = (name: string) =>
  JSON.parse(readFileSync(path.join(dir, `page.option_data_page.v1.${name}.json`), 'utf8'))

let root: Root | undefined
let host: HTMLElement
async function show(payload: OptionDataPagePayload) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionDataPage payload={payload} />)
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
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const rowsOf = () => host.querySelectorAll('tbody tr')

describe('option data page', () => {
  it('ok shows 6 or more rows and 3 series', async () => {
    const payload = fixture('ok')
    await show(payload)
    expect(rowsOf().length).toBeGreaterThanOrEqual(6)
    expect(host.querySelectorAll('[role="tab"]')).toHaveLength(3)
    expect(host.textContent).toContain(payload.title)
    expect(host.textContent).toContain(payload.sentence)
    expect(host.textContent).toContain(payload.chain_title)
    expect(host.textContent).toContain(payload.underlying.price.text)
  })

  it('shows the gap text for the status-2 row and the null delta, never 0', async () => {
    const payload = fixture('ok')
    await show(payload)
    const rows: { status: { code: number }; delta: { kind: string; text: string } }[] =
      payload.chain.rows
    const noQuote = rows.findIndex((r) => r.status.code === 2)
    const nullDelta = rows.findIndex((r) => r.status.code !== 2 && r.delta.kind === 'gap')
    expect(noQuote).toBeGreaterThanOrEqual(0)
    expect(nullDelta).toBeGreaterThanOrEqual(0)
    const cells = (i: number) => [...rowsOf()[i]!.querySelectorAll('td')].map((c) => c.textContent)
    expect(cells(noQuote)).toContain('no quote')
    expect(cells(noQuote)).toContain(payload.chain.rows[noQuote].bid.text)
    expect(cells(nullDelta)).toContain(payload.chain.rows[nullDelta].delta.text)
    expect(cells(nullDelta)).not.toContain('0')
    expect(cells(noQuote)).not.toContain('0')
  })

  it('terms shows the strike, expiry and right text', async () => {
    const payload = fixture('terms')
    await show(payload)
    const row = payload.chain.rows[0]
    const text = rowsOf()[0]!.textContent
    expect(text).toContain(row.strike.text)
    expect(text).toContain(row.expiry)
    expect(text).toContain(row.right)
  })

  it('gap shows the empty-table message and no chart', async () => {
    const payload = fixture('gap')
    await show(payload)
    expect(rowsOf()).toHaveLength(0)
    expect(host.querySelector('[data-chain-empty]')?.textContent).toBe(payload.sentence)
    expect(host.textContent).toContain(payload.gaps[0].text)
    expect(host.querySelector('[data-slot="chart"]')).toBeNull()
  })

  it('stale shows the stale text', async () => {
    const payload = fixture('stale')
    await show(payload)
    expect(host.querySelector('[data-stale="true"]')?.textContent).toBe(payload.stale.text)
  })

  it('bad-version is rejected by validate and the others are accepted', () => {
    const bad = validateOptionDataPage(fixture('bad-version'))
    expect(bad.ok).toBe(false)
    for (const name of ['ok', 'terms', 'gap', 'stale']) {
      expect(validateOptionDataPage(fixture(name)).ok, name).toBe(true)
    }
  })

  it('choices shows three links of the fixture, the current one marked', async () => {
    const payload = fixture('choices')
    await show(payload)
    const nav = host.querySelectorAll('[data-underlying-links] a')
    expect([...nav].map((a) => a.getAttribute('href'))).toEqual(
      payload.underlyings.map((c: { href: string }) => c.href),
    )
    const marked = [...nav].filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked).toHaveLength(1)
    expect(marked[0]?.getAttribute('href')).toBe('/options/data/9000')
  })

  it('ok shows no choices block', async () => {
    await show(fixture('ok'))
    expect(host.querySelector('[data-underlying-links]')).toBeNull()
  })

  it('validate rejects a non-string href', () => {
    const bad = fixture('choices')
    bad.underlyings[0].href = 123
    expect(validateOptionDataPage(bad).ok).toBe(false)
  })

  it('validate still accepts ok and choices', () => {
    expect(validateOptionDataPage(fixture('ok')).ok).toBe(true)
    expect(validateOptionDataPage(fixture('choices')).ok).toBe(true)
  })

  it('jpy shows the USD approximation, the earnings tone and the vol line as written', async () => {
    const payload = fixture('jpy')
    await show(payload)
    const u = payload.underlying
    const price = host.querySelector('[data-underlying-price]')!
    expect(price.textContent).toContain(u.price.text)
    expect(price.textContent).toContain(u.price.approx_usd_text)
    expect(host.querySelector('[data-field="vol_line"]')?.textContent).toBe(u.vol_line)
    const earnings = host.querySelector('[data-field="earnings_text"]')!
    expect(earnings.textContent).toContain(u.earnings_text)
    const badge = earnings.querySelector('[data-earnings]')!
    expect(badge.getAttribute('data-earnings')).toBe('in_term')
    expect(badge.getAttribute('data-variant')).toBe('destructive')
    const bidRow = (payload.chain.rows as { bid: { kind: string; text: string } }[]).findIndex(
      (r) => r.bid.kind === 'value',
    )
    expect(rowsOf()[bidRow]!.textContent).toContain(payload.chain.rows[bidRow].bid.approx_usd_text)
  })

  it('earnings tone alone picks the variant', async () => {
    const payload = fixture('jpy')
    payload.underlying.earnings_tone = 'outside'
    payload.underlying.earnings_text = 'E 无'
    await show(payload)
    expect(host.querySelector('[data-earnings]')?.getAttribute('data-variant')).toBe('secondary')
  })

  it('terms shows the missing texts, every strike, the caption and the slots', async () => {
    const payload = fixture('terms')
    await show(payload)
    const count = (t: string, w: string) => t.split(w).length - 1
    const vol = host.querySelector('[data-field="vol_line"]')!
    expect(count(vol.textContent ?? '', '缺')).toBe(1)
    const earn = host.querySelector('[data-field="earnings_text"]')!
    expect(count(earn.textContent ?? '', '没有数据来源')).toBe(1)
    expect(earn.hasAttribute('data-earnings')).toBe(false)
    expect(earn.querySelector('[data-earnings]')).toBeNull()
    const rows: { strike: { text: string } }[] = payload.chain.rows
    rows.forEach((r, i) => expect(rowsOf()[i]!.textContent).toContain(r.strike.text))
    const series = payload.quote_series[0]
    expect(host.querySelector('[role="tab"]')?.textContent).toBe(series.caption)
    expect(host.querySelector('[data-slots]')?.getAttribute('data-slots')).toBe(
      series.points.map((p: { slot: string }) => p.slot).join(','),
    )
  })

  it('the chain header has Chinese shell labels and English terms', async () => {
    await show(fixture('terms'))
    const heads = [...host.querySelectorAll('thead th')].map((h) => h.textContent ?? '')
    const all = heads.join('|')
    for (const t of ['IV', 'Δ', 'Γ', 'Θ', 'Vega', 'OI']) expect(all).toContain(t)
    for (const t of ['到期', '类型', '行权价', '状态', '买价', '卖价', '成交价', '成交量']) {
      expect(all).toContain(t)
    }
    expect(all).not.toContain('Expiry')
    expect(all).not.toContain('Strike')
  })

  it('fits the quote y-axis to the data', async () => {
    yAxisProps.length = 0
    await show(fixture('ok'))
    expect(yAxisProps.length).toBeGreaterThan(0)
    expect(yAxisProps[0]!.domain).toEqual(['auto', 'auto'])
  })
})
