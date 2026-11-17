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

  it('prints a row per dataset with its cells, the state and the counts as given', async () => {
    const el = await render(withCell({ state: 'missing', keys: 3, ok: 1, missing: 2, missing_keys: ['AAA', 'BBB'], repair: 'none' }))
    expect(el.querySelectorAll('[data-dataset]')).toHaveLength(3)
    const cell = el.querySelector('[data-dataset="account_statement"] [data-cell="2026-11-10"]')
    expect(cell?.getAttribute('data-state')).toBe('missing')
    expect(cell?.textContent).toContain('1 of 3')
    expect(cell?.textContent).toContain('AAA')
    expect(cell?.textContent).toContain('BBB')
    expect(el.textContent).toContain('DGS10')
    expect(el.textContent).toContain('3')
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
    expect(button?.textContent).toBe('repair')
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
    expect(forms[0]?.querySelector('button')?.textContent).toBe('repair 2 days')
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

  it('TestNoLogicInWeb: the page folder exports only components and the registration', () => {
    const names = [pageFolder, grid, repair].flatMap((m) => Object.keys(m)).sort()
    expect(names).toEqual(['CellRepair', 'CoverageGrid', 'DataHealthView', 'RepairAllForm', 'registration'].sort())
  })
})
