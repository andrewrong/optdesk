// The one chart file: the legend names each series once (a series marked `legendHidden` shares its neighbour's
// entry) and a `dashed` line carries a stroke-dasharray.
import { act } from 'react'
import { cloneElement, createElement, type ReactElement } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

// jsdom measures every element as 0 × 0, so the responsive container gets a fixed size.
const yAxes: Record<string, unknown>[] = []
const xAxes: Record<string, unknown>[] = []
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>()
  return {
    ...actual,
    XAxis: (props: Record<string, unknown>) => {
      xAxes.push(props)
      return createElement(actual.XAxis, props)
    },
    YAxis: (props: Record<string, unknown>) => {
      yAxes.push(props)
      return createElement(actual.YAxis, props)
    },
    ResponsiveContainer: ({ children }: { children: ReactElement<{ width: number; height: number }> }) => (
      <div style={{ width: 800, height: 400 }}>{cloneElement(children, { width: 800, height: 400 })}</div>
    ),
  }
})

const points = (values: number[]) => values.map((value) => ({ value }))
const SPEC: ChartSpec = {
  kind: 'line',
  legend: true,
  categories: ['a', 'b', 'c'],
  series: [
    { name: 'Close', points: points([1, 2, 3]) },
    { name: '200-day average', points: points([1, 1.5, 2]) },
    { name: 'dashed', points: points([0, 1, 2]), dashed: true },
    { name: 'dashed', points: points([2, 3, 4]), dashed: true, legendHidden: true },
  ],
}

let root: Root | null = null
let host: HTMLElement
async function show(spec: ChartSpec) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<Chart spec={spec} ariaText="aria" name="t" />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})
// The entries of the legend, sorted: recharts decides their order.
const legend = () =>
  [...host.querySelectorAll('.recharts-legend-wrapper > div > div')].map((e) => e.textContent).sort()

describe('Chart', () => {
  it('shows each series name once in the legend, the hidden series sharing its neighbour', async () => {
    await show(SPEC)
    expect(legend()).toEqual(['Close', '200-day average', 'dashed'].sort())
  })

  it('shows no legend unless the spec turns it on', async () => {
    await show({ ...SPEC, legend: false })
    expect(legend()).toEqual([])
  })

  it('draws a dashed line with stroke-dasharray and a plain line without', async () => {
    await show(SPEC)
    const dashed = [...host.querySelectorAll('path.recharts-line-curve')].map((p) =>
      p.getAttribute('stroke-dasharray'),
    )
    expect(dashed).toEqual([null, null, '4 4', '4 4'])
  })

  it('draws straight lines, never a smoothed curve', async () => {
    await show(SPEC)
    const d = [...host.querySelectorAll('path.recharts-line-curve')].map((p) => p.getAttribute('d') ?? '')
    expect(d).toHaveLength(4)
    for (const path of d) expect(path).not.toContain('C')
  })

  it('draws the band as one shaded area with one legend entry and no line', async () => {
    await show({ ...SPEC, band: { name: 'range', low: points([0, 1, 2]), high: points([2, 3, 4]) } })
    expect(host.querySelectorAll('.recharts-area-area')).toHaveLength(1)
    expect(host.querySelector('.recharts-area-curve')).toBeNull()
    expect(legend()).toContain('range')
  })

  it('shows five y axis ticks, without axis or tick line, with the suffix of the unit appended', async () => {
    yAxes.length = 0
    await show({ ...SPEC, unit: 'percent' })
    const axis = yAxes[yAxes.length - 1]!
    expect(axis.tickCount).toBe(5)
    expect(axis.axisLine).toBe(false)
    expect(axis.tickLine).toBe(false)
    expect(axis.hide).toBeUndefined()
    expect((axis.tickFormatter as (tick: number) => string)(5400)).toBe('5400%')
    expect(host.querySelectorAll('.recharts-yAxis .recharts-cartesian-axis-tick')).toHaveLength(5)
  })

  it('keeps the first and last points inside the plot with a padding on the x axis', async () => {
    xAxes.length = 0
    await show(SPEC)
    const padding = xAxes[xAxes.length - 1]!.padding as { left: number; right: number }
    expect(padding.left).toBeGreaterThanOrEqual(40)
    expect(padding.right).toBeGreaterThan(0)
  })

  it('sits the x axis labels below the lowest y tick with a tick margin', async () => {
    xAxes.length = 0
    await show(SPEC)
    expect(xAxes[xAxes.length - 1]!.tickMargin as number).toBeGreaterThanOrEqual(8)
  })

  it('labels the last point of each line with its text', async () => {
    await show({
      ...SPEC,
      series: [{ name: 'Close', points: [{ value: 1 }, { value: 2 }, { value: 3, text: 'three' }] }],
    })
    const labels = [...host.querySelectorAll('.recharts-label-list text')].map((e) => e.textContent)
    expect(labels).toEqual(['three'])
  })

  it('breaks a hole series at a missing point and bridges the others', async () => {
    const gap = [{ value: 1 }, { value: null }, { value: 3 }]
    await show({
      ...SPEC,
      series: [
        { name: 'hole', points: gap, hole: true },
        { name: 'bridge', points: gap },
      ],
    })
    const d = [...host.querySelectorAll('path.recharts-line-curve')].map((p) => p.getAttribute('d') ?? '')
    expect(d[0]!.match(/M/g)).toHaveLength(2)
    expect(d[1]!.match(/M/g)).toHaveLength(1)
  })

  it('leaves a bar series out of the legend when it is legendHidden', async () => {
    await show({
      kind: 'bar',
      legend: true,
      categories: ['a', 'b'],
      series: [
        { name: 'one', points: points([1, 2]) },
        { name: 'two', points: points([2, 1]), legendHidden: true },
      ],
    })
    expect(legend()).toEqual(['one'])
  })
})
