// Issue #305: the macro_rates view shows the text of page.macro_rates.v3 and computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { chartSpecOf, MacroRatesView, registration } from '@/pages/macro_rates/view'
import { validate, type Payload } from '@/types/macro_rates'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): unknown =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../../contracts/fixtures', `page.macro_rates.v3.${name}.json`),
      'utf8',
    ),
  )

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)

let root: Root | null = null
let host: HTMLElement
async function show(payload: Payload) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MacroRatesView payload={payload} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  root = null
})

describe('macro_rates view', () => {
  for (const name of ['golden', 'gap', 'stale']) {
    it(`shows every string of ${name} as the fixture has it`, async () => {
      const payload = fixture(name) as Payload
      await show(payload)
      const text = host.textContent ?? ''
      expect(text).toContain(payload.title)
      expect(text).toContain(payload.sentence)
      for (const series of payload.series) {
        expect(text).toContain(series.title)
        expect(text).toContain(series.latest.text)
      }
      for (const gap of payload.gaps) {
        expect(text).toContain(gap.text)
      }
      const gapRows = [...host.querySelectorAll('[data-gap]')]
      expect(gapRows).toHaveLength(payload.gaps.length)
      payload.gaps.forEach((gap, i) => {
        const title = payload.series.find((s) => s.id === gap.series_id)?.title ?? gap.series_id
        expect(gapRows[i]!.textContent).not.toContain(gap.reason)
        expect(gapRows[i]!.querySelector('[data-slot=badge]')!.textContent).toBe(title)
      })
    })
  }

  it('shows no table or column header inside a series card', async () => {
    const payload = fixture('golden') as Payload
    await show(payload)
    for (const series of payload.series) {
      const card = host.querySelector(`[data-series="${series.id}"]`)!
      expect(card.textContent).toContain(series.title)
      expect(card.textContent).toContain(series.latest.text)
      expect(card.querySelector('table, [role="table"], [role="grid"], th, [role="columnheader"]')).toBeNull()
    }
  })

  it('shows the gap latest of gap and stale in an alert', async () => {
    const payload = fixture('stale') as Payload
    await show(payload)
    const alerts = [...host.querySelectorAll('[role="alert"]')].map((a) => a.textContent)
    expect(alerts.some((t) => t?.includes('latest point 2026-10-28 is old'))).toBe(true)
  })

  it('builds a chart spec of plain data with a null point kept as null', () => {
    const payload = fixture('golden') as Payload
    const spec = chartSpecOf(payload.series[0]!)
    expect(spec.kind).toBe('line')
    expect(spec.categories).toEqual(['2026-11-02', '2026-11-03', '2026-11-04'])
    expect(spec.series[0]!.points.map((p) => p.value)).toEqual([4.1, null, 4.2])
    expect(spec.series[0]!.points.map((p) => p.text)).toEqual(['4.10%', 'no value', '4.20%'])
  })

  it('passes the unit of a percent series to the chart spec', () => {
    const payload = fixture('golden') as Payload
    const series = { ...payload.series[0]!, unit: 'percent' as const }
    expect(chartSpecOf(series).unit).toBe('percent')
  })

  it('refuses bad-version with reason unsupported_version', () => {
    const checked = validate(fixture('bad-version'))
    expect(checked.ok).toBe(false)
    if (!checked.ok) expect(checked.gap.reason).toBe('unsupported_version')
  })

  it('registers itself as macro_rates', () => {
    expect(registration.schema).toBe('macro_rates')
    expect(registration.page.validate(fixture('golden')).ok).toBe(true)
  })
})
