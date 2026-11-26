// The data check block of the data health page prints the payload as given (docs/design/data-check.md#10-data-check-web-block).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as pageFolder from '@/pages/data_health/DataHealthPage'
import type { Payload } from '@/types/data_health_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.data_health_page.v1.${name}.json`), 'utf8'),
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

describe('data check block', () => {
  it('TestRendersFindingsAsGiven', async () => {
    const p = fixture('datacheck')
    const dc = p.data_check!
    const el = await render(p)
    const block = el.querySelector('[data-data-check]')
    expect(block).not.toBeNull()
    const text = block!.textContent ?? ''
    expect(text).toContain(dc.title)
    expect(text).toContain(dc.note)
    expect(text).toContain(dc.check_day)
    expect(text).toContain(dc.finished_at)
    expect(text).toContain(String(dc.rules_total))
    expect(text).toContain(String(dc.rules_unknown))
    for (const sev of ['error', 'warn', 'unknown'] as const) {
      expect(block!.querySelector(`[data-count="${sev}"]`)?.textContent).toContain(String(dc.counts[sev]))
    }
    const rows = block!.querySelectorAll('[data-finding]')
    expect(rows).toHaveLength(dc.findings.length)
    dc.findings.forEach((f, i) => {
      const row = rows[i]!.textContent ?? ''
      for (const v of [f.label, f.dataset, f.day, f.severity_text, f.evidence_text, f.why_text]) expect(row).toContain(v)
      expect(rows[i]!.querySelector('[data-new]') !== null).toBe(f.is_new)
    })
  })

  it('TestCountsCarryNoWebWords', async () => {
    const p = fixture('datacheck')
    const el = await render(p)
    const counts = el.querySelector('[data-counts]')!.textContent ?? ''
    expect(counts).not.toMatch(/error|warn|unknown/)
  })

  it('TestNoRunShowsNoneText', async () => {
    const p = fixture('datacheck')
    p.data_check = { ...p.data_check!, none_text: 'Invented none text', findings: [] }
    const el = await render(p)
    const block = el.querySelector('[data-data-check]')
    expect(block?.textContent).toContain('Invented none text')
    expect(block!.querySelector('table')).toBeNull()
  })

  it('TestNullRendersNothing', async () => {
    const p = fixture('datacheck')
    p.data_check = null
    const el = await render(p)
    expect(el.querySelector('[data-data-check]')).toBeNull()
  })

  it('TestAbsentRendersNothing', async () => {
    const p = fixture('datacheck')
    delete p.data_check
    const el = await render(p)
    expect(el.querySelector('[data-data-check]')).toBeNull()
  })
})
