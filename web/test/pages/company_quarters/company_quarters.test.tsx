// The company quarters page prints the payload as sent. Payloads are the contract fixtures with one change each
// (docs/design/data-collection.md#9, Decisions 5 to 12).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as pageModule from '@/pages/company_quarters/CompanyQuartersPage'
import * as tableModule from '@/pages/company_quarters/QuartersTable'
import { validate, type Payload } from '@/types/company_quarters_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.company_quarters_page.v1.${name}.json`), 'utf8'))
const ok = () => fixture('ok')

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(<pageModule.CompanyQuartersPage payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
})
const row = (el: HTMLElement, periodEnd: string) => el.querySelector(`tr[data-period-end="${periodEnd}"]`) as HTMLElement

describe('company quarters page', () => {
  it('TestQuartersFixtureOkRenders', async () => {
    const p = ok()
    p.columns.unshift({ key: 'filing_date', label: 'Filing date', gloss: 'the day the report was filed with the SEC' })
    const el = await render(p)
    expect(pageModule.registration.schema).toBe('company_quarters_page')
    const rows = [...el.querySelectorAll('tbody tr')]
    expect(rows.map((r) => r.getAttribute('data-period-end'))).toEqual(['2025-09-27', '2025-06-28'])
    expect(row(el, '2025-09-27').textContent).toContain('1,234,500,000')
    expect(row(el, '2025-09-27').textContent).toContain('2025-11-01')
    expect(el.textContent).toContain('Every number on this page is invented for the contract fixture.')
    expect([...el.querySelectorAll('[data-choices] a')].map((a) => a.getAttribute('href'))).toEqual([
      '/company-quarters?c=US:ACME',
      '/company-quarters?c=US:BOLT',
    ])
    expect(el.querySelector('[data-gloss="revenue"]')?.querySelector('svg')).not.toBeNull()
    expect(el.textContent).toContain('Revenue of the quarter, in USD.')
  })

  it('prints Filing date once, filled from the row, when columns[] carries filing_date as the builder sends it', async () => {
    const p = ok()
    p.columns.unshift({ key: 'filing_date', label: 'Filing date', gloss: 'the day the report was filed with the SEC' })
    p.quarters[0]!.filing_date = '2025-11-01'
    const el = await render(p)
    const heads = [...el.querySelectorAll('thead th')].filter((th) => th.textContent?.startsWith('Filing date'))
    expect(heads).toHaveLength(1)
    expect(el.querySelector('[data-gloss="filing_date"]')).not.toBeNull()
    expect(row(el, p.quarters[0]!.period_end).textContent).toContain('2025-11-01')
  })

  it('TestQuartersFixedTextsChinese', async () => {
    const el = await render(ok())
    expect(el.querySelector('h1')?.textContent).toBe('公司季度')
    expect(el.querySelector('thead th')?.textContent?.startsWith('季度')).toBe(true)
  })

  it('TestQuartersGapFixtureRenders', async () => {
    const el = await render(fixture('gap'))
    expect(el.querySelector('[data-gap="unknown_company"]')?.textContent).toBe('The company in the address is not on the list; pick one.')
    expect(el.querySelector('table')).toBeNull()
    expect(el.querySelectorAll('[data-choices] a')).toHaveLength(1)
  })

  it('TestMissingCellPrintsWhyText', async () => {
    const el = await render(ok())
    const cell = row(el, '2025-09-27').querySelector('[data-missing]')
    expect(cell?.textContent).toBe('No filing reported it.')
    const p = ok()
    p.quarters[0]!.cells.buybacks!.why_text = 'Share counts changed.'
    p.quarters[0]!.cells.buybacks!.text = '999'
    await act(async () => root?.unmount())
    host?.remove()
    const again = await render(p)
    const text = row(again, '2025-09-27').textContent ?? ''
    expect(text).toContain('Share counts changed.')
    expect(text).not.toContain('999')
  })

  it('TestDerivedAndRestatedBadges', async () => {
    const el = await render(ok())
    const first = row(el, '2025-09-27').textContent ?? ''
    expect(first).toContain('derived')
    expect(first).not.toContain('restated')
    const second = row(el, '2025-06-28').textContent ?? ''
    expect(second).toContain('restated')
    expect(second).not.toContain('derived')
  })

  it('TestFiscalYearEndInHeader', async () => {
    const el = await render(ok())
    expect(el.querySelector('thead [data-fiscal-year-end]')?.textContent).toBe('September')
  })

  it('TestEmptyLabelNotePrinted', async () => {
    const p = ok()
    p.quarters[0]!.label = 'Quarter ending 2025-09-27'
    p.notes = ['The label appears after the next daily run.']
    const el = await render(p)
    expect(row(el, '2025-09-27').textContent).toContain('Quarter ending 2025-09-27')
    expect(el.querySelector('[data-note]')?.textContent).toBe('The label appears after the next daily run.')
  })

  it('TestRejectsUnknownField', () => {
    expect(validate(ok()).ok).toBe(true)
    expect(validate({ ...ok(), extra: 1 }).ok).toBe(false)
    const nested = ok()
    ;(nested.quarters[0]!.cells.revenue as unknown as Record<string, unknown>).extra = 1
    expect(validate(nested).ok).toBe(false)
    expect(validate(fixture('gap')).ok).toBe(true)
  })

  it('TestNoLogicInWeb', () => {
    expect(Object.keys(pageModule).sort()).toEqual(['CompanyQuartersPage', 'registration'])
    expect(Object.keys(tableModule)).toEqual(['QuartersTable'])
  })
})
