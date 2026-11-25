// The data health page prints the payload as sent: the grid, the repair forms, the legend. Payloads are the contract
// fixture `ok` with one change each (docs/design/data-health.md#Decisions 3, 8, 12).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as pageFolder from '@/pages/data_health/DataHealthPage'
import * as grid from '@/pages/data_health/CoverageGrid'
import * as repair from '@/pages/data_health/RepairButton'
import * as failures from '@/pages/data_health/FailuresTable'
import type { Payload } from '@/types/data_health_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const ok = (): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.data_health_page.v1.ok.json'), 'utf8'),
  )
type Cell = Payload['datasets'][number]['cells'][number]
// The ok payload with the first cell of the first dataset changed.
const withCell = (over: Partial<Cell>, form: Payload['form'] | undefined = undefined): Payload => {
  const p = ok()
  const first = p.datasets[0]!
  first.cells[0] = { ...first.cells[0]!, ...over }
  if (form !== undefined) p.form = form
  return p
}

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
const fields = (form: Element) =>
  Object.fromEntries([...form.querySelectorAll('input')].map((i) => [i.getAttribute('name'), i.getAttribute('value')]))

describe('data health page', () => {
  it('is registered as data_health_page', () => {
    expect(pageFolder.registration.schema).toBe('data_health_page')
  })

  it('prints the page title, card titles and the dataset head in Chinese', async () => {
    const p = ok()
    p.gateways = JSON.parse(readFileSync(path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.data_health_page.v1.gateways.json'), 'utf8')).gateways
    const el = await render(p)
    expect(el.querySelector('h1')?.textContent).toBe('数据健康')
    const titles = [...el.querySelectorAll('[data-slot="card-title"]')].map((t) => t.textContent)
    expect(titles).toEqual(['网关', '数据健康', '重复失败', '图例'])
    expect(el.querySelector('thead th')?.textContent).toBe('数据集')
  })

  it('prints a row per dataset with its cells, the state and the counts as given', async () => {
    const el = await render(withCell({ state: 'missing', keys: 3, ok: 1, missing: 2, missing_keys: ['AAA', 'BBB'], repair: 'none' }))
    expect(el.querySelectorAll('[data-dataset]')).toHaveLength(3)
    const cell = el.querySelector('[data-dataset="account_statement"] [data-cell="2026-11-10"]')
    expect(cell?.getAttribute('data-state')).toBe('missing')
    expect(cell?.textContent).toContain('1 ok, 2 missing')
    expect(cell?.textContent).toContain('AAA')
    expect(cell?.textContent).toContain('BBB')
    expect(el.textContent).toContain('DGS10')
    expect(el.textContent).toContain('3')
  })

  it('TestCellPrintsEveryNonZeroCount', async () => {
    const mixed = await render(withCell({ state: 'expired', keys: 46, ok: 44, expired: 2, no_value: 0, missing: 0, missing_keys: [], repair: 'none' }))
    const a = mixed.querySelector('[data-dataset="account_statement"] [data-cell="2026-11-10"]')?.textContent ?? ''
    expect(a).toContain('44 ok, 2 expired')
    expect(a).not.toContain('of')
    await act(async () => root?.unmount())
    host?.remove()
    const none = await render(withCell({ state: 'no_value', keys: 35, ok: 0, expired: 0, no_value: 35, missing: 0, missing_keys: [], repair: 'none' }))
    const b = none.querySelector('[data-dataset="account_statement"] [data-cell="2026-11-10"]')?.textContent ?? ''
    expect(b).toContain('35 no_value')
    expect(b).not.toContain('0 of')
  })

  it('a button cell posts action=run with job, day and the token and no force', async () => {
    const el = await render(withCell({ state: 'missing', repair: 'button' }))
    const form = el.querySelector('[data-cell="2026-11-10"] form') as HTMLFormElement
    expect(form.getAttribute('action')).toBe('/owner-job-console')
    expect(form.getAttribute('method')).toBe('post')
    expect(fields(form)).toEqual({
      action: 'run',
      job: 'statement_capture',
      day: '2026-11-10',
      expires: '2026-11-17T10:00:00Z',
      mac: 'invented-mac-0001',
    })
    const button = form.querySelector('button')
    expect(button?.getAttribute('type')).toBe('submit')
    expect(button?.textContent).toBe('修复')
  })

  it('not_refetchable prints why_text and no form', async () => {
    const el = await render(withCell({ state: 'missing', repair: 'not_refetchable', why_text: 'lost: the source gives only the present' }))
    const cell = el.querySelector('[data-cell="2026-11-10"]')
    expect(cell?.textContent).toContain('lost: the source gives only the present')
    expect(cell?.querySelector('form')).toBeNull()
  })

  it('form null renders no form anywhere', async () => {
    const p = withCell({ state: 'missing', repair: 'button' }, null)
    const el = await render(p)
    expect(el.querySelector('form')).toBeNull()
  })

  it('repair_all button renders one form with action=repair, the dataset and the token, labelled with gap_days', async () => {
    const el = await render(ok())
    const forms = el.querySelectorAll('[data-dataset="account_statement"] form[data-repair-all]')
    expect(forms).toHaveLength(1)
    expect(fields(forms[0]!)).toEqual({
      action: 'repair',
      dataset: 'account_statement',
      expires: '2026-11-17T10:00:00Z',
      mac: 'invented-mac-0001',
    })
    expect(forms[0]?.querySelector('button')?.textContent).toBe('修复 2 天')
    expect(el.querySelector('[data-dataset="option_eod"] form[data-repair-all]')).toBeNull()
  })

  it('TestLegendShowsGlossary: every glossary term is printed with its text and an Info icon, and the overwrite note', async () => {
    const p = ok()
    const el = await render(p)
    const legend = el.querySelector('[data-legend]')
    expect(legend).not.toBeNull()
    const items = [...(legend?.querySelectorAll('[data-term]') ?? [])]
    expect(items.map((i) => i.getAttribute('data-term'))).toEqual(p.glossary.map((g) => g.term))
    for (const [i, g] of p.glossary.entries()) {
      expect(items[i]?.textContent).toContain(g.term)
      expect(items[i]?.textContent).toContain(g.text)
      expect(items[i]?.querySelector('svg.lucide-info')).not.toBeNull()
    }
    expect(el.textContent).toContain(p.overwrite_note)
  })

  it('TestDueTextPrinted: a cell with due_text prints it and renders no form', async () => {
    const el = await render(withCell({ state: 'missing', repair: 'none', due_text: 'due at 11:00 EST' }))
    const cell = el.querySelector('[data-cell="2026-11-10"]')
    expect(cell?.textContent).toContain('due at 11:00 EST')
    expect(cell?.querySelector('form')).toBeNull()
  })

  it('prints each gap in an Alert', async () => {
    const p = ok()
    p.gaps = [{ kind: 'gap', reason: 'coverage_unavailable', text: 'The ledger could not be read.' }]
    const el = await render(p)
    expect(el.querySelector('[role="alert"][data-gap="coverage_unavailable"]')?.textContent).toBe('The ledger could not be read.')
  })

  it('TestDatasetIdPrintedOnceWhenEqualToLabel', async () => {
    const p = ok()
    const first = p.datasets[0]!
    first.label = first.dataset_id
    const second = p.datasets[1]!
    const h = await render(p)
    const count = (text: string, sub: string) => text.split(sub).length - 1
    const firstCell = h.querySelector(`[data-dataset="${first.dataset_id}"] td`)!.textContent ?? ''
    expect(count(firstCell, first.dataset_id)).toBe(1)
    const secondCell = h.querySelector(`[data-dataset="${second.dataset_id}"] td`)!.textContent ?? ''
    expect(secondCell).toContain(second.label)
    expect(secondCell).toContain(second.dataset_id)
  })

  it('a payload without the failures fields shows no failures table and no error', async () => {
    const p = ok()
    delete p.failures_note
    delete p.failures_none_text
    for (const d of p.datasets) {
      delete d.failures_total
      delete d.failures_hidden
      delete d.failures_hidden_text
      delete d.repeated_failures
    }
    const el = await render(p)
    expect(el.querySelector('[data-failures]')).toBeNull()
    expect(el.querySelector('[data-gap]')).toBeNull()
    expect(el.querySelectorAll('[data-dataset]')).toHaveLength(3)
  })

  it('the failures_unavailable gap prints its text and the coverage grid is still drawn', async () => {
    const p = ok()
    p.gaps = [{ kind: 'gap', reason: 'failures_unavailable', text: 'Invented: the failure read is unavailable.' }]
    for (const d of p.datasets) d.repeated_failures = []
    const el = await render(p)
    expect(el.querySelector('[data-gap="failures_unavailable"]')?.textContent).toBe('Invented: the failure read is unavailable.')
    expect(el.querySelectorAll('[data-dataset]')).toHaveLength(3)
  })

  it('renders the line of each running row above the grid', async () => {
    const p = ok()
    p.running = [{ line: 'Invented line one' }, { line: 'Invented line two', slow: true }]
    const el = await render(p)
    const rows = [...el.querySelectorAll('[data-running]')]
    expect(rows.map((r) => r.textContent)).toEqual(['Invented line one', 'Invented line two'])
    const gridEl = el.querySelector('[data-dataset]') as Element
    for (const r of rows) expect(r.compareDocumentPosition(gridEl) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('renders no running line when the payload has no running rows', async () => {
    const p = ok()
    delete p.running
    const el = await render(p)
    expect(el.querySelectorAll('[data-running]')).toHaveLength(0)
    expect(el.querySelectorAll('[data-dataset]').length).toBeGreaterThan(0)
  })

  it('TestNoLogicInWeb:the page folder exports only components and the registration', () => {
    const names = [pageFolder, grid, repair, failures].flatMap((m) => Object.keys(m)).sort()
    expect(names).toEqual(['CellRepair', 'CoverageGrid', 'DataHealthView', 'FailuresTable', 'RepairAllForm', 'registration'].sort())
  })
})
