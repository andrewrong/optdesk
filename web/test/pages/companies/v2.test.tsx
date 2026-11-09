// Issue #691: the companies page v2 shows the core, diagnosis and questions blocks and the price-line label as
// the payload wrote them. The contract fixtures are the reference; the browser computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { CompaniesPage, validateCompaniesPage } from '@/pages/companies/CompaniesPage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const fixture = (dir: string, name: string) =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, '../../../', dir, name), 'utf8'))
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
const block = (name: string) => host.querySelector(`[data-block="${name}"]`)?.textContent ?? ''

it('ok fixture renders the three blocks', async () => {
  const payload = fixture('../contracts/fixtures', 'page.companies_page.v2.ok.json')
  expect(validateCompaniesPage(payload).ok).toBe(true)
  await show(payload)
  const core = block('core')
  for (const text of ['6.2% a year', 'Spending earns', 'US 10-year 4.15%', 'Quotes of 11-06']) {
    expect(core).toContain(text)
  }
  expect(block('diagnosis')).toContain('inventory 18% of revenue')
  expect(block('questions')).toContain('18.2%')
  expect(block('questions')).toContain('Management has repaid debt every year since 2019.')
  expect(block('watch_list')).toContain('band')
  expect(block('watch_list')).toContain('150')
})

it('gap fixture renders gaps', async () => {
  await show(fixture('../contracts/fixtures', 'page.companies_page.v2.gap.json'))
  expect(block('core')).toContain('core block not built')
  expect(block('diagnosis')).toContain('diagnosis block not built')
  expect(block('questions')).toContain('no filings reader')
})

it('v1 document still renders', async () => {
  const v1 = fixture('fixtures/companies-page', 'ok.json')
  expect(validateCompaniesPage(v1).ok).toBe(true)
  await show(v1)
  expect(host.querySelector('[data-block="core"]')).toBeNull()
  expect(host.querySelector('[data-page="companies"]')).not.toBeNull()
})
