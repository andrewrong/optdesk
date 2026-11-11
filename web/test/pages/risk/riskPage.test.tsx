// The household_risk page shows the text fields of the payload and nothing else. Expected strings are read from the
// fixtures, not written here; the clock and the network are not read.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { HouseholdRiskView, registration } from '@/pages/risk/view'
import { validate } from '@/types/household_risk'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
async function show(payload: typeof ok | typeof gap | typeof stale) {
  const host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<HouseholdRiskView payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})

function expectBody(text: string, strings: string[]) {
  for (const s of strings) expect(text).toContain(s)
}

describe('HouseholdRiskView', () => {
  it('shows the sentence, stale label and every block of ok as sent', async () => {
    const host = await show(ok)
    const text = host.textContent ?? ''
    expectBody(text, [ok.title, ok.sentence, ok.stale.text])
    expectBody(text, [ok.delta.title, ok.delta.meaning, ok.delta.total.text, ...ok.delta.heads])
    for (const row of ok.delta.rows) expectBody(text, [row.name, row.delta.text])
    expectBody(text, [ok.beta_delta.title, ok.beta_delta.benchmark_text, ok.beta_delta.total.text])
    for (const row of ok.beta_delta.rows) {
      expectBody(text, [row.name, row.beta.text, row.weighted_delta.text])
    }
    expectBody(text, [ok.sigma.title, ok.sigma.meaning, ...ok.sigma.columns, ok.sigma.scale_low.text])
    for (const row of ok.sigma.rows) {
      expectBody(text, [row.name, ...row.cells.map((cell) => cell.text)])
    }
    expect(host.querySelector('[role=group]')?.getAttribute('aria-label')).toBe(ok.sigma.aria_text)
    expectBody(text, [ok.vega_title, ok.vega.meaning, ok.vega.total.text])
    for (const row of ok.vega.rows) expectBody(text, [row.expiry_text, row.vega.text])
    expectBody(text, [ok.standard_metrics.title])
    expect(ok.standard_metrics.metrics.length).toBeGreaterThanOrEqual(4)
    for (const metric of ok.standard_metrics.metrics) {
      expectBody(text, [metric.label, metric.value_text, metric.explanation])
    }
    const table = ok.standard_metrics.metrics[0].table
    expectBody(text, [...table.columns, ...table.rows[0].cells])
    expectBody(text, ok.notes)
    expect(host.querySelector('[data-gaps]')).toBeNull()
  })

  it('lays the blocks out in board 25 order, with only the standard metrics cell full width from lg', async () => {
    const host = await show(ok)
    const cells = [...host.querySelectorAll('[data-cell]')]
    expect(cells.map((cell) => cell.getAttribute('data-cell'))).toEqual([
      'delta',
      'vega',
      'beta',
      'sigma',
      'standard-metrics',
    ])
    const spanning = cells.filter((cell) => cell.classList.contains('lg:col-span-2'))
    expect(spanning.map((cell) => cell.getAttribute('data-cell'))).toEqual(['standard-metrics'])
    const grid = cells[0]?.parentElement
    expect(grid?.classList.contains('grid-cols-1')).toBe(true)
    expect(grid?.classList.contains('lg:grid-cols-2')).toBe(true)
    expect(host.querySelector('[data-page=household-risk]')?.classList.contains('min-w-0')).toBe(true)
  })

  it('shows the beta delta meaning', async () => {
    const host = await show(ok)
    expect(host.textContent ?? '').toContain(ok.beta_delta.meaning)
  })

  it('shows the stale label of stale as sent', async () => {
    const host = await show(stale)
    expect(host.querySelector('[data-stale]')?.textContent).toBe(stale.stale.text)
    expect(host.textContent).toContain(stale.sentence)
    expectBody(host.textContent ?? '', stale.notes)
  })

  it('shows the gap text of a gap block and of the page, with no figure invented', async () => {
    const host = await show(gap)
    const text = host.textContent ?? ''
    expect(host.querySelector('[data-block=delta]')?.textContent).toContain(gap.delta.gap.text)
    expect(host.querySelector('[data-block=sigma]')?.textContent).toContain(gap.sigma.gap.text)
    expect(host.querySelector('[data-block=sigma] table')).toBeNull()
    expect(host.querySelector('[data-block=delta] table')).toBeNull()
    for (const item of gap.gaps) expect(host.querySelector('[data-gaps]')?.textContent).toContain(item.text)
    expectBody(text, [gap.beta_delta.title, gap.vega.meaning])
  })

  it('is registered under its schema and validate refuses a version it does not read', () => {
    expect(registration.schema).toBe('household_risk')
    expect(validate(ok).ok).toBe(true)
    const bad = JSON.parse(
      readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
    )
    const result = validate(bad)
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.gap.reason).toBe('unsupported_version')
  })
})
