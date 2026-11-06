// Issue #1133: the 持仓公司 card shows the text fields of the payload and nothing else. Fixtures are the invented
// payloads of fixtures/held-companies-summary/; the literals below are from its README and files.
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { HeldCompaniesCard, registration } from '@/pages/home/HeldCompaniesCard'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/held-companies-summary', name),
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
    root.render(<HeldCompaniesCard payload={payload as never} />)
  })
}
const bodyRows = () =>
  [...host.querySelectorAll('[data-companies] tbody tr')].map((r) =>
    [...r.querySelectorAll('td')].map((c) => c.textContent),
  )
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})

describe('HeldCompaniesCard', () => {
  it('is registered for the schema held_companies_summary', () => {
    expect(registration.schema).toBe('held_companies_summary')
  })

  it('shows the title, say, subtitle and the four column labels as sent', async () => {
    const ok = fixture('ok.json')
    await show(ok)
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe('持仓公司')
    const descriptions = [...host.querySelectorAll('[data-slot=card-description]')].map(
      (d) => d.textContent,
    )
    expect(descriptions).toEqual([ok.card.say, ok.card.subtitle])
    expect(descriptions[0]).toBe('持有 9 家 · 2 家 21 日内财报 · 近 5 日消息：暂无数据源。')
    expect([...host.querySelectorAll('[data-companies] th')].map((h) => h.textContent)).toEqual([
      '公司',
      '财报日',
      '日涨跌',
      '5日涨跌',
    ])
    expect(host.querySelector('[data-volatility]')).toBeNull()
  })

  it('shows the short list of the ok fixture: the two names with a dated event', async () => {
    await show(fixture('ok.json'))
    const rows = bodyRows()
    expect(rows).toHaveLength(2)
    // fixtures/held-companies-summary/README.md rows 1 and 2
    expect(rows[0]).toEqual(['AMD', '10-28', '+2.1%', '+5.0%'])
    expect(rows[1]).toEqual(['MSFT', '11-12', '−0.4% · 10-22', '−1.0%'])
  })

  it('aligns the first and last columns left and the others right', async () => {
    await show(fixture('ok.json'))
    const cells = [...host.querySelectorAll('[data-companies] tbody tr:first-child td')]
    expect(cells.map((c) => c.classList.contains('text-right'))).toEqual([false, true, true, true])
  })

  it('colours a cell by the tone Python sent', async () => {
    await show(fixture('ok.json'))
    const first = host.querySelectorAll('[data-companies] tbody tr')[0]!
    expect(first.querySelectorAll('td')[2]!.querySelector('.text-success')).not.toBeNull()
    const second = host.querySelectorAll('[data-companies] tbody tr')[1]!
    expect(second.querySelectorAll('td')[2]!.querySelector('.text-down')).not.toBeNull()
  })

  it('sorts the 财报日 column by the value (ISO date) of the cells', async () => {
    const ok = fixture('ok.json')
    // texts that would sort differently from their dates
    ok.card.rows[0].earnings = { date: '2026-12-01', text: 'ZZ' }
    ok.card.rows[1].earnings = { date: '2026-11-12', text: 'AA' }
    await show(ok)
    const header = [...host.querySelectorAll('[data-companies] th button')].find(
      (b) => b.textContent === '财报日',
    ) as HTMLElement
    await act(async () => header.click())
    const names = bodyRows().map((r) => r[0])
    expect(names.indexOf('MSFT')).toBeLessThan(names.indexOf('AMD'))
  })

  it('shows the stale label, notes, gaps and the 数据时间 text of the stale fixture', async () => {
    const stale = fixture('stale.json')
    await show(stale)
    expect(host.querySelector('[data-stale]')?.textContent).toBe('数据已过期：数据日 10-23')
    expect(host.textContent).toContain('财报日只显示日期：盘前、盘后时间暂无数据源。')
    expect([...host.querySelectorAll('[data-gaps] li')].map((l) => l.textContent)).toEqual([
      '近 5 日消息：暂无数据源',
    ])
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.textContent).toContain('美股数据日 10-23 · 日股数据日 10-23')
  })

  it('shows a table-level Gap as its text, with no table', async () => {
    const gap = fixture('gap.json')
    await show(gap)
    expect(host.querySelector('[data-table-gap]')?.textContent).toBe(gap.card.rows.text)
    expect(host.querySelector('table')).toBeNull()
  })

  it('escapes hostile company text', async () => {
    const ok = fixture('ok.json')
    ok.card.rows[0].symbol = '<img src=x onerror=alert(1)>'
    await show(ok)
    expect(host.querySelector('img')).toBeNull()
    expect(host.textContent).toContain('<img src=x onerror=alert(1)>')
  })
})

describe('source of the home cards', () => {
  it('formats and compares nothing: no toLocaleString, Intl, toFixed, sign comparison or Math', () => {
    const dir = path.resolve(import.meta.dirname, '../../../src/pages/home')
    for (const file of readdirSync(dir).filter((f) => /\.tsx?$/.test(f))) {
      const source = readFileSync(path.join(dir, file), 'utf8')
      expect(source, file).not.toMatch(/toLocaleString|Intl\.|toFixed|Math\./)
      expect(source, file).not.toMatch(/\.(value|day_change|five_day_change)\s*[<>]=?\s*0\b/)
    }
  })
})
