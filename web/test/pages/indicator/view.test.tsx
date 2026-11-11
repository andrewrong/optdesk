// The indicator detail view shows the text fields of the payload and nothing else. Expected strings are read from
// the synthetic fixtures of this directory, not written here.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { IndicatorDetailView, registration } from '@/pages/indicator/view'
import { validate } from '@/types/indicator_detail'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)

let root: Root | null = null
let host: HTMLElement
async function show(node: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(node)
  })
}
beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

const text = () => host.textContent ?? ''

describe('IndicatorDetailView', () => {
  it('shows title, sentence, stale label, latest, revision, explanation and the chart of ok', async () => {
    await show(<IndicatorDetailView payload={ok} />)
    expect(text()).toContain(ok.title)
    expect(text()).toContain(ok.sentence)
    expect(text()).toContain(ok.stale.text)
    expect(text()).toContain(ok.latest.value.text)
    expect(text()).toContain(ok.latest.released.text)
    expect(text()).toContain(ok.revision.text)
    expect(text()).toContain(ok.explanation_title)
    for (const line of ok.explanation) expect(text()).toContain(line)
    expect(host.querySelector('[data-chart=indicator-detail]')?.getAttribute('aria-label')).toBe(ok.series_title)
  })

  it('shows the stale label and the gaps of stale', async () => {
    await show(<IndicatorDetailView payload={stale} />)
    expect(text()).toContain(stale.stale.text)
    for (const entry of stale.gaps) expect(text()).toContain(entry.text)
  })

  it('shows the gap text in place of the latest figure of gap', async () => {
    await show(<IndicatorDetailView payload={gap} />)
    expect(text()).toContain(gap.latest.text)
    for (const entry of gap.gaps) expect(text()).toContain(entry.text)
    expect(host.querySelector('[data-latest]')).toBeNull()
  })
})

describe('IndicatorDetailView recent table', () => {
  it('shows the four headers, every recent row, and meta_line of ok', async () => {
    await show(<IndicatorDetailView payload={ok} />)
    const heads = [...host.querySelectorAll('th')].map((th) => th.textContent)
    expect(heads).toEqual(['Period', 'Released', 'Value', 'Change'])
    expect(text()).toContain(ok.recent.title)
    expect(ok.recent.rows.length).toBeGreaterThan(0)
    const body = [...host.querySelectorAll('tbody tr')].map((tr) =>
      [...tr.querySelectorAll('td')].map((td) => td.textContent),
    )
    expect(body).toEqual(ok.recent.rows.map((r) => [r.period, r.released, r.value, r.change]))
    expect(text()).toContain(ok.meta_line)
  })

  it('shows the gap text and no table when recent is a gap', async () => {
    await show(<IndicatorDetailView payload={gap} />)
    expect(text()).toContain(gap.recent.text)
    expect(host.querySelector('table')).toBeNull()
    expect(text()).toContain(gap.meta_line)
  })
})

describe('validate', () => {
  it('refuses the bad-version fixture and accepts ok', () => {
    expect(validate(badVersion).ok).toBe(false)
    expect(validate(ok).ok).toBe(true)
    expect(registration.schema).toBe('indicator_detail')
  })
})
