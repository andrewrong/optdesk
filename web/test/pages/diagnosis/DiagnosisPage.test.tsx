// The diagnosis page shows the texts of the payload as sent. Expected strings are read from the fixtures.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DiagRow } from '@/generated/diagnosis_page.v1'
import { DiagnosisPage, registration } from '@/pages/diagnosis/DiagnosisPage'
import { gap } from './fixtures/gap'
import { cyclical, financial, ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: typeof ok | typeof gap | typeof stale) {
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
    root.render(<DiagnosisPage payload={payload as never} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  vi.unstubAllGlobals()
})

const isRow = (row: object): row is DiagRow => 'symbol' in row

describe('DiagnosisPage', () => {
  it('is registered as diagnosis_page', () => {
    expect(registration.schema).toBe('diagnosis_page')
  })

  it('refuses a payload of schema_version 2', () => {
    const bad = JSON.parse(
      readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
    )
    expect(registration.page.validate(bad).ok).toBe(false)
    expect(registration.page.validate(ok).ok).toBe(true)
  })

  it('shows the two groups in the order cyclical, financial and the texts of every row', async () => {
    const el = await render(ok)
    const text = el.textContent ?? ''
    expect(text.indexOf(cyclical.title)).toBeGreaterThanOrEqual(0)
    expect(text.indexOf(cyclical.title)).toBeLessThan(text.indexOf(financial.title))
    expect(text).toContain(ok.sentence)
    expect(text).toContain(ok.basis_note)
    for (const group of ok.groups) {
      for (const row of group.rows.filter(isRow)) {
        for (const shown of [row.latest, row.rank_text, row.extra, row.usual]) {
          expect(text).toContain(shown)
        }
      }
    }
  })

  it('opens a fold to the year table with one row per year and the bar width of the producer', async () => {
    const el = await render(ok)
    expect(el.querySelectorAll('table')).toHaveLength(0)
    const first = cyclical.rows.find(isRow)
    if (!first) throw new Error('fixture has no company row')
    const trigger = el.querySelector('button[data-slot="accordion-trigger"]') as HTMLElement
    await act(async () => trigger.click())
    expect(el.querySelectorAll('tbody tr')).toHaveLength(first.years.length)
    const bars = [...el.querySelectorAll<HTMLElement>('[data-mini-bar]')]
    expect(bars.map((bar) => bar.style.width)).toEqual(first.years.map((y) => `${y.bar_percent}%`))
  })

  it('shows a Gap row with its cause and no year table', async () => {
    const el = await render(gap)
    const text = el.textContent ?? ''
    for (const group of gap.groups) {
      for (const row of group.rows) if (!isRow(row)) expect(text).toContain(row.text)
    }
    expect(el.querySelectorAll('[data-gap-row]')).toHaveLength(2)
    expect(el.querySelectorAll('table')).toHaveLength(0)
  })

  it('shows the stale label', async () => {
    const el = await render(stale)
    expect(el.textContent).toContain(stale.stale.text)
  })
})
