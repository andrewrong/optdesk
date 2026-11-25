// The cross-check column and the opened pair print the payload fields as sent.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as pageFolder from '@/pages/base_data/BaseDataPage'
import type { Payload } from '@/types/base_data_page'
import { crossCheck, failedRun } from './fixtures/cross_check'

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
async function open(h: HTMLElement, dataset: string) {
  await act(async () => (rowOf(h, dataset).querySelector('[data-toggle]') as HTMLElement).click())
}
const cellOf = (h: HTMLElement, dataset: string) => rowOf(h, dataset).querySelector('[data-cross-check-cell]') as HTMLElement

describe('cross-check column', () => {
  it('renders the column with the pair verdict', async () => {
    const h = await render(crossCheck)
    expect([...h.querySelectorAll('th')].map((c) => c.textContent)).toContain('交叉核对')
    const cell = cellOf(h, 'bars')
    expect(cell.textContent).toContain('fail')
    expect(cell.textContent).toContain('R6')
    expect(cell.querySelector('[data-level="fail"]')).not.toBeNull()
    await open(h, 'bars')
    const pair = h.querySelector('[data-cross-check="R6"]') as HTMLElement
    expect(pair.textContent).toContain('compared 5')
    expect(pair.textContent).toContain('matched 3')
    expect(pair.textContent).toContain('differences 2')
    expect(pair.textContent).toContain('close within 0.5 percent')
    expect(pair.textContent).toContain('yfinance')
    expect(pair.textContent).toContain('2026-11-18T08:00:00Z')
    expect([...pair.querySelectorAll('[data-company]')].map((e) => e.getAttribute('data-company'))).toEqual(['Apple', 'Microsoft', 'Zeta'])
    const heads = ['键', '本站值', '对方值', '差异', '类型', '提示']
    expect([...pair.querySelectorAll('[data-company="Apple"] th')].map((c) => c.textContent)).toEqual(heads)
    expect(pair.querySelectorAll('[data-company] tbody tr')).toHaveLength(4)
  })

  it('renders skipped items with their reason', async () => {
    const h = await render(crossCheck)
    await open(h, 'bars')
    const pair = h.querySelector('[data-cross-check="R6"]') as HTMLElement
    expect(pair.textContent).toContain('skipped 1')
    const zeta = pair.querySelector('[data-company="Zeta"]') as HTMLElement
    expect(zeta.textContent).toContain('skipped')
    expect(zeta.textContent).toContain('来源限流，未比较')
  })

  it('renders no column text when cross_check is absent', async () => {
    const h = await render(crossCheck)
    expect(cellOf(h, 'fx').textContent).toBe('')
    await open(h, 'fx')
    expect(h.querySelector('[data-cross-check]')).toBeNull()
  })

  it('renders the reason line of a failed run', async () => {
    const h = await render(failedRun)
    expect(cellOf(h, 'bars').textContent).toContain('unknown')
    await open(h, 'bars')
    const pair = h.querySelector('[data-cross-check="R6"]') as HTMLElement
    expect(pair.querySelector('[data-reason]')?.textContent).toBe('来源限流，今日未完成核对')
    expect(pair.querySelector('table')).toBeNull()
  })
})
