// The line chart the view hands to Chart: the points as one series and the reference line as a second one, all
// values and texts as sent. Chart is replaced by a probe that prints the spec, since recharts draws nothing in jsdom.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ChartSpec } from '@/charts/spec'
import { IndicatorDetailView } from '@/pages/indicator/view'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

vi.mock('@/charts/Chart', () => ({
  Chart: ({ spec }: { spec: ChartSpec }) => <pre data-spec>{JSON.stringify(spec)}</pre>,
}))

let root: Root | null = null
async function specOf(payload: Parameters<typeof IndicatorDetailView>[0]['payload']): Promise<ChartSpec> {
  const host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<IndicatorDetailView payload={payload} />)
  })
  return JSON.parse(host.querySelector('[data-spec]')?.textContent ?? 'null')
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})

describe('indicator chart spec', () => {
  it('draws ok.points as one line and ok.reference_line as a flat second line', async () => {
    const spec = await specOf(ok)
    expect(spec.kind).toBe('line')
    expect(spec.categories).toEqual(
      ok.points.map((point) => ('day' in point.released ? point.released.day : point.observed_on)),
    )
    expect(spec.series[0]!.points.map((point) => point.value)).toEqual(ok.points.map((point) => point.value))
    expect(spec.series[0]!.points.map((point) => point.text)).toEqual(ok.points.map((point) => point.text))
    expect(spec.series[1]!.name).toBe(ok.reference_line.label)
    expect(spec.series[1]!.points.every((point) => point.value === ok.reference_line.value.value)).toBe(true)
  })

  it('draws the points of gap without a series for a missing reference line', async () => {
    const spec = await specOf({ ...gap, reference_line: null })
    expect(spec.series).toHaveLength(1)
    expect(spec.series[0]!.points).toHaveLength(gap.points.length)
  })
})
