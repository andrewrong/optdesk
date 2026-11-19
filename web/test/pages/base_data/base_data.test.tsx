// The base data page prints the payload as sent: top verdict, one row per dataset, the checks of an opened row, gaps in words.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as pageFolder from '@/pages/base_data/BaseDataPage'
import type { Payload } from '@/types/base_data_page'
import { gap, viewGap } from './fixtures/gap'
import { ok } from './fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
  const View = pageFolder.registration.page.component as unknown as (props: { payload: Payload }) => React.ReactElement
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(<View payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
})
const rowOf = (h: HTMLElement, dataset: string) => h.querySelector(`tr[data-dataset="${dataset}"]`) as HTMLElement

describe('base data page', () => {
  it('is registered as base_data_page', () => {
    expect(pageFolder.registration.schema).toBe('base_data_page')
  })

  it('renders one row per dataset with its verdict', async () => {
    const h = await render(ok)
    expect([...(h.querySelector('table') as HTMLElement).querySelectorAll('th')].map((c) => c.textContent)).toEqual([
      'Dataset', 'Verdict', 'Newest value', 'Date', 'Age', 'Failed checks', 'Cross-check',
    ])
    expect([...h.querySelectorAll('tr[data-dataset]')].map((r) => r.getAttribute('data-dataset'))).toEqual(['quotes', 'option_chains', 'fx'])
    const chains = rowOf(h, 'option_chains')
    expect(chains.querySelector('[data-level]')?.getAttribute('data-level')).toBe('fail')
    expect(chains.textContent).toContain('rows 112,004')
    expect(chains.textContent).toContain('2026-11-16')
    expect(chains.textContent).toContain('1 s')
    expect(chains.textContent).toContain('age; crossed 2.1%')
    expect(h.querySelector('[data-top]')?.getAttribute('data-top')).toBe('fail')
    expect(h.textContent).toContain(ok.top.text_zh)
    expect(h.textContent).toContain('Market data')
    expect(h.textContent).toContain('Rates and events')
  })

  it('shows date, age and failed checks inside the dataset cell', async () => {
    const h = await render(ok)
    const cell = rowOf(h, 'option_chains').querySelector('td') as HTMLElement
    const block = cell.querySelector('.md\\:hidden') as HTMLElement
    expect(block).not.toBeNull()
    expect(block.textContent).toContain('1 s')
    expect(block.textContent).toContain('age; crossed 2.1%')
    const age = [...h.querySelectorAll('th')].find((c) => c.textContent === 'Age') as HTMLElement
    expect(age.classList.contains('hidden')).toBe(true)
  })

  it('opens a row to show its checks', async () => {
    const h = await render(ok)
    expect(h.querySelector('[data-checks]')).toBeNull()
    const toggle = rowOf(h, 'option_chains').querySelector('button[data-toggle]') as HTMLButtonElement
    await act(async () => toggle.click())
    const checks = h.querySelector('[data-checks="option_chains"]') as HTMLElement
    expect(checks).not.toBeNull()
    expect(checks.textContent).toContain('crossed')
    expect(checks.textContent).toContain('Crossed quotes')
    expect(checks.textContent).toContain('买价高于卖价的行占比过高。')
    expect(checks.textContent).toContain('NVDA 2026-12-18 C 200')
    expect(checks.textContent).toContain('0.02')
    expect(checks.textContent).toContain('trial value')
    await act(async () => toggle.click())
    expect(h.querySelector('[data-checks]')).toBeNull()
  })

  it('sorts by verdict worst first and shows no repeated group name', async () => {
    const h = await render(ok)
    const head = h.querySelector('th[aria-sort], th[tabindex="0"]') as HTMLElement
    await act(async () => head.click())
    expect([...h.querySelectorAll('tr[data-dataset]')].map((r) => r.getAttribute('data-dataset'))).toEqual(['option_chains', 'fx', 'quotes'])
    expect(h.querySelectorAll('tr[data-group]').length).toBe(0)
    expect(rowOf(h, 'fx').textContent).toContain('Rates and events')
  })

  it('renders a view gap and the other rows', async () => {
    const h = await render(viewGap)
    expect(h.querySelector('[data-gap="view_unavailable"]')?.textContent).toContain('The option view did not answer in time')
    expect(rowOf(h, 'option_chains').querySelector('[data-level]')?.getAttribute('data-level')).toBe('unknown')
    expect(rowOf(h, 'quotes').textContent).toContain('SPY last 590.10')
    expect(rowOf(h, 'fx').textContent).toContain('USDJPY 154.20')
  })

  it('renders the gap payload', async () => {
    const h = await render(gap)
    expect(h.querySelector('[data-gap="facts_unavailable"]')?.textContent).toContain('All four fact views failed to read.')
    expect(h.querySelector('[data-top]')?.getAttribute('data-top')).toBe('unknown')
    expect(h.querySelectorAll('tr[data-dataset]').length).toBe(0)
  })
})
