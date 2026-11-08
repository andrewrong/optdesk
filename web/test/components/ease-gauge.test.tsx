// The ease gauge draws payload fields only: a track, a band from low to high and a dot at marker_percent.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { EaseGauge } from '@/components/ease-gauge/EaseGauge'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const ease = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, '../../fixtures/macro-summary/ok.json'), 'utf8'),
).card.ease

let root: Root | undefined
let host: HTMLElement
async function show() {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<EaseGauge ease={ease} />)
  })
}
// jsdom has no ResizeObserver, which the Radix slider measures its thumb with.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('draws a dot at marker_percent and no filled bar', async () => {
  await show()
  const dots = host.querySelectorAll('[role=slider]')
  expect(dots).toHaveLength(1)
  expect(dots[0]!.getAttribute('aria-valuenow')).toBe('56')
  expect(dots[0]!.getAttribute('aria-label')).toBe(ease.aria_text)
  expect(host.querySelector('[role=progressbar]')).toBeNull()
  const range = host.querySelector('[data-slot=slider-range]')
  expect(range === null || range.className.includes('hidden')).toBe(true)
  expect([...host.querySelectorAll('[data-ticks] span')].map((s) => s.textContent)).toEqual(
    ease.ticks,
  )
})

it('draws the neutral band from low to high', async () => {
  await show()
  const band = host.querySelector<HTMLElement>('[data-neutral-band-shade]')!
  expect(band.style.left).toBe('34%')
  // jsdom's CSSOM folds calc(100% - 66%) into calc(34%); a browser keeps the sent form. Both are the same length.
  expect(['calc(100% - 66%)', 'calc(34%)']).toContain(band.style.right)
})
