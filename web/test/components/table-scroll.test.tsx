// Issue #2492: a table wider than its card shows an always-visible horizontal scrollbar with a border-coloured thumb.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DataTable } from '@/components/data-table/data-table'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import { registration } from '@/pages/job_console/view'
import type { Payload } from '@/types/job_console'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const SCROLLBAR = ['overflow-x-auto', '[&::-webkit-scrollbar]:h-2', '[&::-webkit-scrollbar-thumb]:rounded-full', '[&::-webkit-scrollbar-thumb]:bg-border']

let root: Root | null = null
let host: HTMLElement | null = null
async function show(node: React.ReactElement) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(node)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  host?.remove()
  host = null
  vi.unstubAllGlobals()
})

function expectScrollbar(el: Element | null) {
  expect(el).not.toBeNull()
  for (const c of SCROLLBAR) expect(el?.classList.contains(c)).toBe(true)
}

describe('table container scrollbar', () => {
  it('Table container scrolls horizontally with a thin border-coloured scrollbar', async () => {
    const h = await show(
      <Table>
        <TableBody>
          <TableRow>
            <TableCell>x</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    )
    const box = h.querySelector('[data-slot="table-container"]')
    expectScrollbar(box)
    expect(box?.querySelector('table')).not.toBeNull()
  })

  it('container sets a scrollbar style that Firefox honours (thin width, border-colour thumb)', async () => {
    const h = await show(
      <Table>
        <TableBody>
          <TableRow>
            <TableCell>x</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    )
    const box = h.querySelector('[data-slot="table-container"]')!
    const cls = [...box.classList]
    expect(cls.some((c) => c.includes('scrollbar-width') || c === 'scrollbar-thin')).toBe(true)
    expect(cls.some((c) => c.includes('scrollbar-color'))).toBe(true)
  })

  it('DataTable renders inside that container', async () => {
    const h = await show(
      <DataTable columns={[{ id: 'a', label: 'A' }]} rows={[{ a: { text: 'one' } }]} />,
    )
    const box = h.querySelector('[data-slot="table-container"]')
    expectScrollbar(box)
    expect(box?.querySelector('table')?.textContent).toContain('one')
  })

  it('the job console table renders inside that container', async () => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    )
    const payload = JSON.parse(
      readFileSync(path.resolve(import.meta.dirname, '../../../contracts/fixtures/page.job_console.v1.golden.json'), 'utf8'),
    ) as Payload
    const View = registration.page.component as unknown as (props: { payload: Payload }) => React.ReactElement
    const h = await show(<View payload={payload} />)
    const table = h.querySelector('table')
    expect(table).not.toBeNull()
    expectScrollbar(table?.closest('[data-slot="table-container"]') ?? null)
  })
})
