// Issue #2492: on each of the five pages a table sits inside the container that always shows a scrollbar.
// The shell dispatches the payload to the page as in the browser; the payloads are the repository's recorded or
// synthetic fixtures. jsdom has no layout, so the check is that every table has the scroll container as its parent.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from '@/shell/App'
import optionPage from '../../fixtures/option-page/ok.json'
import positions from '../../fixtures/positions-page/ok.json'
import { ok as candidates } from '../pages/optioncandidates/fixtures/ok'
import { ok as tracking } from '../pages/optiontracking/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const jobConsole: unknown = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, '../../../contracts/fixtures/page.job_console.v1.golden.json'), 'utf8'),
)

const PAGES: [string, unknown][] = [
  ['positions', positions],
  ['options', optionPage],
  ['options/candidates', candidates],
  ['job-console', jobConsole],
  ['options/tracking', tracking],
]
const STYLED = ['overflow-x-auto', '[scrollbar-width:thin]', '[&::-webkit-scrollbar-thumb]:bg-border']

let root: Root | null = null
let host: HTMLElement | null = null
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  host?.remove()
  host = null
  vi.unstubAllGlobals()
})

describe('tables of the five pages', () => {
  it.each(PAGES)('%s: every table has the always-visible scrollbar container as its parent', async (_name, payload) => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    )
    host = document.createElement('div')
    document.body.append(host)
    await act(async () => {
      root = createRoot(host as HTMLElement)
      root.render(<App payload={payload} />)
    })
    const tables = [...host.querySelectorAll('table')]
    expect(tables.length).toBeGreaterThan(0)
    for (const t of tables) {
      const box = t.closest('[data-slot="table-container"]')
      expect(box).not.toBeNull()
      for (const c of STYLED) expect(box?.classList.contains(c)).toBe(true)
    }
  })
})
