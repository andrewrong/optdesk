// Issue #1132: the 家庭概览 card shows the text fields of the payload and nothing else. Fixtures are the merged
// payloads of fixtures/household-summary/; the literals below are from its README.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { HouseholdCard, registration } from '@/pages/home/HouseholdCard'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/household-summary', name),
      'utf8',
    ),
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
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})
const tile = (id: string) => host.querySelector<HTMLElement>(`[data-tile=${id}]`)!

describe('HouseholdCard', () => {
  it('is registered for the schema household_summary', () => {
    expect(registration.schema).toBe('household_summary')
  })

  it('shows the title, subtitle and four tiles as sent', async () => {
    await show(fixture('ok.json'))
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe('家庭概览')
    expect(host.textContent).toContain('两户合计 · 数据日 10-21')
    expect(host.querySelectorAll('[data-tile]')).toHaveLength(4)
    expect(tile('net_worth').textContent).toContain('净值')
    expect(tile('net_worth').textContent).toContain('≈$412,300')
    expect(tile('day_change').textContent).toContain('+$1,860')
    expect(tile('day_change').textContent).toContain('+0.45%')
    expect(tile('cash').textContent).toContain('38%')
    expect(tile('cash').textContent).toContain('+$156,700')
  })

  it('explains a tile on focus of its info mark, and only when the payload sends explain', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    for (const id of ['net_worth', 'day_change', 'month_to_date', 'cash']) {
      expect(tile(id).querySelectorAll('[data-explain]')).toHaveLength(1)
    }
    const trigger = tile('month_to_date').querySelector<HTMLElement>('[data-explain]')!
    expect(trigger.getAttribute('aria-label')).toBe('本月至今（不含出入金）')
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
    expect(tile('month_to_date').textContent).toContain('本月至今（不含出入金）')
    expect(tile('month_to_date').textContent).toContain('+$11,700')
    expect(tile('month_to_date').textContent).toContain('+2.94%')

    await act(async () => root?.unmount())
    host.remove()
    const month = payload.card.tiles.month_to_date
    month.caption = { text: '无基数' }
    const cash = payload.card.tiles.cash
    cash.label = '融资借款'
    cash.figure = { text: '融资中' }
    cash.caption = { value: -5000, text: '−$5,000', unit: 'usd_signed', tone: 'down' }
    await show(payload)
    expect(tile('month_to_date').textContent).toContain('无基数')
    expect(tile('cash').textContent).toContain('融资借款')
    expect(tile('cash').textContent).toContain('融资中')
    expect(tile('cash').textContent).toContain('−$5,000')
    expect(tile('cash').querySelector('[data-slot=badge]')).toBeNull()
    expect(tile('month_to_date').querySelector('[data-slot=badge]')).toBeNull()
  })

  it('shows a Gap figure as its text, never 0', async () => {
    await show(fixture('gap.json'))
    // gap.json README: the allocation rows are missing, so the cash share is a gap
    expect(tile('cash').querySelector('[data-slot=badge]')?.textContent).toBe(
      '缺国家分布，现金占比暂不显示',
    )
    expect(tile('month_to_date').querySelector('[data-slot=badge]')?.textContent).toBe(
      '本月还没有日结单',
    )
    expect(tile('cash').textContent).not.toMatch(/\b0(\.0+)?%?\b|\$0\b/)
  })

  it('shows the stale label as sent', async () => {
    // stale.json README: the label reads 数据已过期 · 数据日 10-21
    await show(fixture('stale.json'))
    expect(host.querySelector('[data-stale]')?.textContent).toBe('数据已过期 · 数据日 10-21')
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

  it('shows notes, the gaps list and the 数据时间 text', async () => {
    await show(fixture('ok.json'))
    expect(host.textContent).toContain('本月至今已扣除入金和出金，按简单法计算。')
    expect(host.querySelector('[data-gaps]')).toBeNull()
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.textContent).toContain('净值 10-21 · 现金 10-21')
  })
})
