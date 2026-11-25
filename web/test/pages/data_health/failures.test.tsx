// The repeated failures tables of the data health page are built from the contract fixture `ok`, as sent
// (docs/design/repeated-failures.md#5).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as pageFolder from '@/pages/data_health/DataHealthPage'
import type { Payload } from '@/types/data_health_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const ok = (): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.data_health_page.v1.ok.json'), 'utf8'),
  )

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
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
  vi.unstubAllGlobals()
})
const cells = (row: Element) => [...row.querySelectorAll('td')].map((c) => c.textContent)

describe('repeated failures tables', () => {
  it('shows the columns and one row per item of the fixture, with the job as sent', async () => {
    const el = await render(ok())
    const section = el.querySelector('[data-failures="account_statement"]')!
    expect([...section.querySelectorAll('th')].map((h) => h.textContent)).toEqual([
      '任务', '键', '类型', '天数', '运行次数', '首日', '末日', '最近分类',
    ])
    const rows = section.querySelectorAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(cells(rows[0]!).slice(0, 2)).toEqual(['statement_capture', 'acct-synthetic-1'])
    expect(cells(rows[0]!)[2]).toBe('failed')
    expect(cells(rows[0]!).slice(3, 7)).toEqual(['4', '6', '2026-11-05', '2026-11-17'])
    expect(cells(rows[0]!)[7]).toContain('retryable')
    expect(cells(rows[0]!)[7]).toContain('Invented explanation of retryable.')
  })

  it('labels an empty row differently from a failed row', async () => {
    const el = await render(ok())
    const rows = el.querySelectorAll('[data-failures="account_statement"] tbody tr')
    expect(rows[0]!.getAttribute('data-kind')).toBe('failed')
    expect(rows[1]!.getAttribute('data-kind')).toBe('empty')
    expect(cells(rows[1]!)[2]).toBe('empty')
  })

  it('prints failures_note once and the none text for a dataset with no row', async () => {
    const p = ok()
    p.datasets.find((d) => d.dataset_id === 'option_eod')!.repeated_failures = []
    const el = await render(p)
    const count = (sub: string) => (el.textContent ?? '').split(sub).length - 1
    expect(count('Invented note on the repeated failures table.')).toBe(1)
    expect(el.querySelector('[data-failures="option_eod"] table')).toBeNull()
    expect(el.querySelector('[data-failures="option_eod"]')?.textContent).toContain(
      'Invented text for a dataset with no repeated failure.',
    )
  })

  it('shows - for a null job and prints failures_hidden_text as it is', async () => {
    const p = ok()
    const d = p.datasets[0]!
    d.job_id = null
    d.failures_hidden_text = 'Invented: more keys are not shown.'
    const el = await render(p)
    const section = el.querySelector('[data-failures="account_statement"]')!
    expect(cells(section.querySelector('tbody tr')!)[0]).toBe('-')
    expect(section.textContent).toContain('Invented: more keys are not shown.')
  })
})
