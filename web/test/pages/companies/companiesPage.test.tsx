// Issue #1643: the companies page shows the text fields of `companies_page` and nothing else. The fixtures of
// fixtures/companies-page/ are the independent reference; the browser computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { CompaniesPage, validateCompaniesPage } from '@/pages/companies/CompaniesPage'
import { pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/companies-page', name),
      'utf8',
    ),
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

it('is registered, validates the good fixtures and rejects the bad ones', () => {
  expect(pages.companies_page).toBeDefined()
  for (const name of ['ok.json', 'stale.json', 'gap.json']) {
    expect(validateCompaniesPage(fixture(name)).ok).toBe(true)
  }
  for (const name of ['bad-version']) {
    expect(validateCompaniesPage(fixture(`${name}.json`)).ok).toBe(false)
  }
})

it.each(['ok.json', 'stale.json'])(
  'shows every text of %s as the payload sent it',
  async (name) => {
    const payload = fixture(name)
    await show(payload)
    // the moat lines are folded: open every fold of a company row
    for (const trigger of host.querySelectorAll<HTMLElement>('[data-moat] button')) {
      await act(async () => trigger.click())
    }
    const text = host.textContent ?? ''
    const expected: string[] = [
      payload.title,
      payload.header.line,
      payload.stale.text,
      ...payload.notes,
      ...payload.gaps.map((g: { text: string }) => g.text),
      payload.unvalued,
    ].filter(Boolean)
    const { changes, glance, holdings, watch_list: watch } = payload
    if (changes) expected.push(changes.title, ...changes.lines, changes.sentence, changes.note)
    if (glance) {
      expected.push(
        glance.title,
        ...glance.tiles.flatMap((t: { label: string; text: string }) => [t.label, t.text]),
      )
    }
    if (holdings) {
      expected.push(holdings.volatility_note)
      expected.push(...holdings.columns.map((c: { text: string }) => c.text))
      for (const row of holdings.rows) {
        expected.push(...row.cells.map((c: { text: string }) => c.text), ...row.moat)
      }
    }
    expected.push(
      watch.title,
      watch.sentence,
      ...watch.columns.map((c: { text: string }) => c.text),
    )
    for (const row of watch.rows) expected.push(...row.cells.map((c: { text: string }) => c.text))
    for (const one of expected.filter((s) => typeof s === 'string' && s !== '')) {
      expect(text).toContain(one)
    }
    expect(host.querySelectorAll('[data-tile]').length).toBe(glance?.tiles.length ?? 0)
    expect(host.querySelectorAll('tbody tr').length).toBe(
      (holdings?.rows.length ?? 0) + watch.rows.length,
    )
  },
)

it('shows the gap sentence and the watch-list sentence, and no holdings, for gap.json', async () => {
  const payload = fixture('gap.json')
  await show(payload)
  const text = host.textContent ?? ''
  expect(text).toContain('Positions: not enough data')
  expect(text).toContain(payload.watch_list.sentence)
  expect(text).toContain(payload.stale.text)
  expect(host.querySelector('[data-block=holdings]')).toBeNull()
  expect(host.querySelector('table')).toBeNull()
})

it('shows a hostile company name as text', async () => {
  const payload = fixture('ok.json')
  const hostile = '<img src=x onerror=alert(1)>'
  payload.holdings.rows[0].cells[0].text = hostile
  await show(payload)
  expect(host.textContent).toContain(hostile)
  expect(host.querySelector('img')).toBeNull()
})
