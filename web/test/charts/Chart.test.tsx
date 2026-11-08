// The one chart file: the legend names each series once (a series marked `legendHidden` shares its neighbour's
// entry) and a `dashed` line carries a stroke-dasharray.
import { act } from 'react'
import { cloneElement, type ReactElement } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

// jsdom measures every element as 0 × 0, so the responsive container gets a fixed size.
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>()
  return {
    ...actual,
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
    { name: '收盘', points: points([1, 2, 3]) },
    { name: '200日均线', points: points([1, 1.5, 2]) },
    { name: '区间', points: points([0, 1, 2]), dashed: true },
    { name: '区间', points: points([2, 3, 4]), dashed: true, legendHidden: true },
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
    expect(legend()).toEqual(['收盘', '200日均线', '区间'].sort())
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

  it('leaves a bar series out of the legend when it is legendHidden', async () => {
    await show({
      kind: 'bar',
      legend: true,
      categories: ['a', 'b'],
      series: [
        { name: '一', points: points([1, 2]) },
        { name: '二', points: points([2, 1]), legendHidden: true },
      ],
    })
    expect(legend()).toEqual(['一'])
  })
})
