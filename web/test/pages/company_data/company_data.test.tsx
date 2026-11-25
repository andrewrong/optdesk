// The company data page prints the payload as sent and writes nothing (docs/design/data-collection.md#9).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as pageModule from '@/pages/company_data/CompanyDataPage'
import * as tableModule from '@/pages/company_data/CompanyTable'
import { validate, type Payload } from '@/types/company_data_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.company_data_page.v1.${name}.json`), 'utf8'))
const ok = () => fixture('ok')

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(<pageModule.CompanyDataPage payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
})
const order = (el: HTMLElement) => [...el.querySelectorAll('tbody tr')].map((r) => r.getAttribute('data-company'))

describe('company data page', () => {
  it('TestRowsPrintedAsGiven', async () => {
    const el = await render(ok())
    expect(pageModule.registration.schema).toBe('company_data_page')
    expect(order(el)).toEqual(['US:ACME', 'US:BOLT', 'US:CRUX'])
    const acme = el.querySelector('tr[data-company="US:ACME"]')?.textContent ?? ''
    for (const text of ['271', 'FY2026 Q3 (2026-06-27)', 'September', '2026-11-16', '2027-01-29', 'Acme Widgets Inc.']) expect(acme).toContain(text)
    const bolt = el.querySelector('tr[data-company="US:BOLT"]')
    expect(bolt?.querySelector('[data-reason="no_next_report"]')?.textContent).toBe('no_next_report' + 'No report date is listed.')
    expect(el.querySelector('tr[data-company="US:CRUX"]')?.textContent).toContain('—')
    expect(el.querySelector('[data-gloss="bar_count"] svg')).not.toBeNull()
    expect(el.querySelector('[data-term="no_cik"]')?.textContent).toContain('The company has no SEC number yet.')
  })

  it('fixes the company column and shows the scroll hint', async () => {
    const el = await render(ok())
    expect(el.querySelectorAll('[data-scroll-hint]')).toHaveLength(1)
    expect(el.querySelector('[data-scroll-hint]')?.compareDocumentPosition(el.querySelector('table') as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
    const head = el.querySelector('thead th') as HTMLElement
    const cell = el.querySelector('tbody tr td') as HTMLElement
    for (const c of [head, cell]) for (const k of ['sticky', 'left-0', 'bg-background']) expect(c.classList.contains(k)).toBe(true)
    expect(cell.classList.contains('whitespace-normal')).toBe(true)
    expect(el.querySelectorAll('thead th')[1]?.classList.contains('sticky')).toBe(false)
  })

  it('links the rows to their quarters page and the page to data health, as links', async () => {
    const el = await render(ok())
    const hrefs = [...el.querySelectorAll('a')].map((a) => a.getAttribute('href'))
    expect(hrefs).toEqual(['/data-health', '/company-quarters?c=US:ACME', '/company-quarters?c=US:BOLT'])
  })

  it('sorts on bar_count, eps_quarters and missing only', async () => {
    const el = await render(ok())
    const sortable = [...el.querySelectorAll('th[data-sort]')].map((h) => h.getAttribute('data-sort'))
    expect(sortable).toEqual(['bar_count', 'eps_quarters', 'missing'])
    const head = el.querySelector('th[data-sort="bar_count"]') as HTMLElement
    await act(async () => head.click())
    expect(order(el)).toEqual(['US:CRUX', 'US:BOLT', 'US:ACME'])
    await act(async () => head.click())
    expect(order(el)).toEqual(['US:ACME', 'US:BOLT', 'US:CRUX'])
    const missing = el.querySelector('th[data-sort="missing"]') as HTMLElement
    await act(async () => missing.click())
    const first = order(el)
    await act(async () => missing.click())
    expect([first, order(el)]).toEqual([
      ['US:BOLT', 'US:CRUX', 'US:ACME'],
      ['US:ACME', 'US:CRUX', 'US:BOLT'],
    ])
  })

  it('TestIfrsAndJpSayLeftOutOnPurpose', async () => {
    const p = ok()
    p.companies[2]!.missing = [{ code: 'jp', text: 'No statements source for Japan; left out on purpose.' }]
    const el = await render(p)
    expect(el.querySelector('tr[data-company="US:BOLT"] [data-reason="no_statements"]')?.textContent).toContain('left out on purpose')
    expect(el.querySelector('tr[data-company="US:CRUX"] [data-reason="jp"]')?.textContent).toContain('left out on purpose')
  })

  it('TestNoControlWritesAnything', async () => {
    const el = await render(ok())
    expect(el.querySelector('form')).toBeNull()
    expect(el.querySelector('button')).toBeNull()
    expect(el.querySelector('input, select, textarea')).toBeNull()
  })

  it('prints the gap line of the gap fixture', async () => {
    const el = await render(fixture('gap'))
    expect(el.querySelector('[data-gap="companies_unavailable"]')?.textContent).toBe('The list of companies could not be read.')
    expect(order(el)).toEqual([])
  })

  it('TestRejectsUnknownField', () => {
    expect(validate(ok()).ok).toBe(true)
    expect(validate({ ...ok(), extra: 1 }).ok).toBe(false)
    const nested = ok()
    ;(nested.companies[0]!.missing as unknown[]).push({ code: 'jp', text: 'x', extra: 1 })
    expect(validate(nested).ok).toBe(false)
  })

  it('TestNoLogicInWeb', () => {
    expect(Object.keys(pageModule).sort()).toEqual(['CompanyDataPage', 'registration'])
    expect(Object.keys(tableModule)).toEqual(['CompanyTable'])
  })
})
