// The 持仓公司 card shows the text fields of the payload and nothing else. Fixtures are the synthetic payloads of
// held-v2/fixtures/, written from the v2 schema alone.
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { HeldCompaniesCard, registration } from '@/pages/home/HeldCompaniesCard'
import { validate } from '@/types/held_companies_summary'
import { gap } from './held-v2/fixtures/gap'
import { ok } from './held-v2/fixtures/ok'
import { stale } from './held-v2/fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'held-v2/fixtures/bad-version.json'), 'utf8'),
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
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-23T22:00:00+08:00'))
})
afterEach(async () => {
  await act(async () => root?.unmount())
  vi.useRealTimers()
  root = null
  document.body.innerHTML = ''
})

describe('HeldCompaniesCard', () => {
  it('is registered for the schema held_companies_summary', () => {
    expect(registration.schema).toBe('held_companies_summary')
  })

  it('refuses a payload of another schema version and takes version 2', () => {
    expect(validate(badVersion).ok).toBe(false)
    expect(validate(ok).ok).toBe(true)
  })

  it('shows the title, say, subtitle and the column labels of the ok fixture as sent', async () => {
    await show(ok)
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(ok.card.title)
    const descriptions = [...host.querySelectorAll('[data-slot=card-description]')].map(
      (d) => d.textContent,
    )
    expect(descriptions).toEqual([ok.card.say, ok.card.subtitle])
    expect([...host.querySelectorAll('[data-companies] th')].map((h) => h.textContent)).toEqual(
      ok.card.columns.map((c) => c.label),
    )
    expect(host.querySelector('[data-volatility]')).toBeNull()
  })

  it('shows the rows of the ok fixture as their text fields', async () => {
    await show(ok)
    const rows = bodyRows()
    expect(rows).toHaveLength(ok.card.rows.length)
    expect(rows[0]).toEqual(['AMD', '10-28 盘后', '+2.1%', '+5.0%'])
    expect(rows[1]).toEqual(['MSFT', '11-12', '−0.4% · 10-22', '−1.0%'])
    expect(rows[2]).toEqual(['NVDA', '11-19 盘前', '+0.8%', '+2.2%'])
  })

  it('shows 盘前 or 盘后 where the source states it, the date alone on the other row, named once in the notes', async () => {
    await show(ok)
    const rows = bodyRows()
    expect(rows[0]![1]).toMatch(/盘前|盘后/)
    expect(rows[2]![1]).toMatch(/盘前|盘后/)
    expect(rows[1]![1]).toBe('11-12')
    expect(rows[1]![1]).not.toMatch(/盘前|盘后/)
    const named = ok.card.notes.filter((n) => n.includes('MSFT'))
    expect(named).toHaveLength(1)
    expect(host.textContent!.split(named[0]!)).toHaveLength(2)
  })

  it('shows the news column as a gap that states the missing source', async () => {
    await show(ok)
    const gaps = [...host.querySelectorAll('[data-gaps] li')].map((l) => l.textContent)
    expect(gaps).toEqual(['近 5 日消息：暂无数据源'])
  })

  it('aligns the first column left and the others right', async () => {
    await show(ok)
    const cells = [...host.querySelectorAll('[data-companies] tbody tr:first-child td')]
    expect(cells.map((c) => c.classList.contains('text-right'))).toEqual([false, true, true, true])
  })

  it('colours a cell by the tone sent', async () => {
    await show(ok)
    const trs = host.querySelectorAll('[data-companies] tbody tr')
    expect(trs[0]!.querySelectorAll('td')[2]!.querySelector('.text-success')).not.toBeNull()
    expect(trs[1]!.querySelectorAll('td')[2]!.querySelector('.text-down')).not.toBeNull()
  })

  it('sorts the 财报日 column by the value (ISO date) of the cells', async () => {
    const sorted = structuredClone(ok)
    const rows = sorted.card.rows
    // texts that would sort differently from their dates
    rows[0]!.earnings = { date: '2026-12-01', text: 'ZZ' }
    rows[1]!.earnings = { date: '2026-11-12', text: 'AA' }
    await show(sorted)
    const header = [...host.querySelectorAll('[data-companies] th button')].find(
      (b) => b.textContent === ok.card.columns[1]!.label,
    ) as HTMLElement
    await act(async () => header.click())
    const names = bodyRows().map((r) => r[0])
    expect(names.indexOf('MSFT')).toBeLessThan(names.indexOf('AMD'))
  })

  it('shows the stale label, notes, gaps and data_times_text of the stale fixture', async () => {
    vi.setSystemTime(new Date('2026-10-25T21:30:00+08:00'))
    await show(stale)
    const label = host.querySelector('[data-stale]')
    expect(label?.textContent).toBe(stale.card.stale.text)
    expect(label?.getAttribute('data-stale')).toBe('true')
    for (const note of stale.card.notes) expect(host.textContent).toContain(note)
    expect([...host.querySelectorAll('[data-gaps] li')].map((l) => l.textContent)).toEqual(
      stale.card.gaps.map((g) => g.text),
    )
    await act(async () => {
      host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
    })
    expect(host.textContent).toContain(stale.card.data_times_text)
  })

  it('shows the ok fixture label as not stale, with its notes and gaps', async () => {
    await show(ok)
    expect(host.querySelector('[data-stale]')?.textContent).toBe(ok.card.stale.text)
    expect(host.querySelector('[data-stale]')?.getAttribute('data-stale')).toBe('false')
    expect(host.textContent).toContain(ok.card.notes[0])
    expect(host.textContent).toContain(ok.card.gaps[0]!.text)
  })

  it('shows a table-level Gap as its text, with no table', async () => {
    await show(gap)
    expect(host.querySelector('[data-table-gap]')?.textContent).toBe(gap.card.rows.text)
    expect(host.querySelector('table')).toBeNull()
    expect(host.textContent).toContain(gap.card.say)
    expect(host.textContent).toContain(gap.card.gaps[0]!.text)
  })

  it('escapes hostile company text', async () => {
    const hostile = structuredClone(ok)
    hostile.card.rows[0]!.symbol = '<img src=x onerror=alert(1)>'
    await show(hostile)
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
