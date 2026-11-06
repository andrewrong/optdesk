// Issue #1131: the 宏观 card shows the text fields of the payload and nothing else. Fixtures are the invented
// payloads of fixtures/macro-summary/; the literals below are from its README.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MacroCard } from '@/pages/home/MacroCard'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/macro-summary', name),
      'utf8',
    ),
  )
let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MacroCard payload={payload as never} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})

describe('MacroCard', () => {
  it('keeps the standalone gap alert by default', async () => {
    // Issue #1233: omitting showGaps preserves the standalone card's gap Alert.
    const payload = fixture('ok.json')
    payload.card.gaps = [{ reason: 'no_data', text: '独立宏观卡片数据暂缺' }]
    await show(payload)
    expect(host.querySelector('[data-gaps]')?.textContent).toBe('独立宏观卡片数据暂缺')
  })

  it('shows the title, say, ease number, ticks, reading text and stale label as sent', async () => {
    await show(fixture('ok.json'))
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe('宏观')
    expect(host.querySelector('[data-slot=card-description]')?.textContent).toBe(
      '松紧度中性，较上周 −0.2。本周的事件是 CPI。',
    )
    expect(host.querySelector('[data-ease-figure]')?.textContent).toBe('56')
    expect([...host.querySelectorAll('[data-ticks] span')].map((s) => s.textContent)).toEqual([
      '0',
      '25',
      '50',
      '75',
      '100',
    ])
    expect(host.querySelector('[data-neutral-band]')?.textContent).toBe('中性区间 34% – 66%')
    expect(host.textContent).toContain('当前 56，中性；固定刻度 0 到 100。')
    expect(host.querySelector('[data-stale]')?.textContent).toBe('数据日 10-23')
    expect(host.querySelector('[data-marker]')?.getAttribute('aria-valuenow')).toBe('56')
  })

  it('shows the events table with the Gap text of 前值, never 0', async () => {
    await show(fixture('ok.json'))
    const headers = [...host.querySelectorAll('[data-events] th')].map((h) => h.textContent)
    expect(headers).toEqual(['日期', '事件', '前值'])
    const rows = [...host.querySelectorAll('[data-events] tbody tr')].map((r) =>
      [...r.querySelectorAll('td')].map((c) => c.textContent),
    )
    expect(rows).toEqual([
      ['周三 10-28', '美国 CPI', '前值暂缺'],
      ['周五 11-06', '美国非农就业', '前值暂缺'],
    ])
    expect(host.querySelector('[data-events]')?.textContent).not.toMatch(/0$|\b0\b/)
  })

  it('shows notes, the gaps list and the 数据时间 text', async () => {
    await show(fixture('ok.json'))
    expect(host.querySelector('[data-gaps]')?.textContent).toBe('前值暂缺')
    expect(host.textContent).toContain(
      '前值是序列最新存储的一期，标注所属月份；没有存储或没有对应序列的标为暂缺。',
    )
    expect(host.querySelector('[data-data-times]')?.textContent).toContain('数据时间')
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.textContent).toContain('松紧度数据日 10-23；事件日期按发布日历，无发布时间。')
  })

  it('is flagged stale only once the clock passes stale_after (ADR 0034)', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    try {
      const ok = fixture('ok.json')
      // stale_after 2026-10-27T09:30:00+08:00, generated_at 2026-10-26T09:30:00+08:00 (fixture README)
      vi.setSystemTime(new Date(Date.parse(ok.generated_at) + 60_000))
      await show(ok)
      expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('false')
      expect(host.querySelector('[data-clock-skew]')).toBeNull()
      await act(async () => root?.unmount())
      host.remove()

      vi.setSystemTime(new Date(Date.parse(ok.card.stale.stale_after) + 60_000))
      await show(ok)
      expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('true')
    } finally {
      vi.useRealTimers()
    }
  })

  it('flags a device clock more than 5 minutes behind generated_at', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    try {
      const ok = fixture('ok.json')
      vi.setSystemTime(new Date(Date.parse(ok.generated_at) - 6 * 60_000))
      await show(ok)
      expect(host.querySelector('[data-clock-skew]')).not.toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows the week change text', async () => {
    await show(fixture('ok.json'))
    expect(host.querySelector('[data-week-change]')?.textContent).toBe('−0.2')
  })

  it('shows a Gap ease as its text, with no marker', async () => {
    const gap = fixture('gap.json')
    expect('reason' in gap.card.ease).toBe(true)
    await show(gap)
    expect(host.querySelector('[data-ease-gap]')?.textContent).toBe(gap.card.ease.text)
    expect(host.querySelector('[data-marker]')).toBeNull()
  })

  it('escapes hostile event text', async () => {
    const ok = fixture('ok.json')
    ok.card.events[0].name_text = '<img src=x onerror=alert(1)>'
    await show(ok)
    expect(host.querySelector('img')).toBeNull()
    expect(host.textContent).toContain('<img src=x onerror=alert(1)>')
  })
})
