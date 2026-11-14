// Issue #2428: the Companies page shows the pepb block as the payload wrote it (card, gap line, history chart).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { CompaniesPage, validateCompaniesPage } from '@/pages/companies/CompaniesPage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const fixture = (name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../../contracts/fixtures', name), 'utf8'),
  )
let root: Root | undefined
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<CompaniesPage payload={payload as never} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const pepb = () => host.querySelector('[data-block="pepb"]')

it('a page without pepb renders as before', async () => {
  const payload = fixture('page.companies_page.v2.ok.json')
  expect(payload.pepb).toBeUndefined()
  await show(payload)
  expect(pepb()).toBeNull()
  expect(host.querySelector('[data-block="core"]')?.textContent).toContain('6.2% a year')
  expect(host.querySelector('[data-block="watch_list"]')?.textContent).toContain('150')
})

it('the fixture with pepb shows the sentence, the loss text and the rest as given', async () => {
  const payload = fixture('page.companies_page.v2.pepb.json')
  expect(validateCompaniesPage(payload).ok).toBe(true)
  await show(payload)
  const text = pepb()?.textContent ?? ''
  expect(text).toContain('34th percentile of 300 stored days')
  expect(text).toContain('loss, no PE')
  expect(text).toContain('20th percentile of 300 stored days since 2025-08-20: PE 21.0 is a price of 103.70')
  expect(text).toContain('15,200,000,000')
  expect(text).toContain('2026-10-30')
  expect(text).toContain('no filings source for Japanese names')
  expect(text).toContain('forward PE: no estimates source')
  expect(host.querySelectorAll('[data-company]')).toHaveLength(3)
  expect(host.querySelectorAll('[data-chart]').length).toBeGreaterThan(0)
})

it('a Gap block shows its text and no table', async () => {
  const payload = fixture('page.companies_page.v2.pepb.json')
  payload.pepb = { reason: 'not_built', text: 'pepb block not built' }
  await show(payload)
  expect(pepb()?.textContent).toContain('pepb block not built')
  expect(pepb()?.querySelector('table')).toBeNull()
  expect(host.querySelectorAll('[data-company]')).toHaveLength(0)
})
