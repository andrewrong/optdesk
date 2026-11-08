// The core page shows the texts of the payload and the dots at the producer's positions. Expected values are read
// from the fixtures, not written here. The registration finds `core_page` once.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CorePage, registration } from '@/pages/core/CorePage'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host = document.createElement('div')
async function mount(payload: Parameters<typeof CorePage>[0]['payload']) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<CorePage payload={payload} />)
  })
}
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(ok.generated_at))
})
afterEach(async () => {
  await act(async () => root?.unmount())
  host.remove()
  vi.useRealTimers()
})

async function openAll() {
  for (const trigger of host.querySelectorAll<HTMLElement>('button[aria-expanded]')) {
    await act(async () => trigger.click())
  }
}

describe('CorePage', () => {
  it('registers core_page', () => {
    expect(registration.schema).toBe('core_page')
  })

  it('refuses a payload of schema_version 2', () => {
    const bad = JSON.parse(
      readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
    )
    expect(registration.page.validate(bad).ok).toBe(false)
    expect(registration.page.validate(ok).ok).toBe(true)
  })

  it('shows the sentence, notes, jp_note, legend and snapshot_text', async () => {
    await mount(ok)
    const text = host.textContent ?? ''
    for (const s of [ok.sentence, ok.basis_note, ok.jp_note, ok.snapshot_text, ...ok.legend]) {
      expect(s.length).toBeGreaterThan(0)
      expect(text).toContain(s)
    }
  })

  it('shows the returns of each company in its fold, with the dots at marker_percent', async () => {
    await mount(ok)
    await openAll()
    for (const row of ok.rows) {
      if (!Array.isArray(row.returns)) continue
      const item = host.querySelector(`[data-row="${row.symbol}"]`)!
      const returns = item.querySelectorAll('[data-return]')
      expect(returns.length).toBe(row.returns.length)
      row.returns.forEach((point, i) => {
        expect(item.textContent).toContain(point.text)
        const dot = returns[i]!.querySelector<HTMLElement>('[data-dot]')!
        expect(dot.style.left).toBe(`${point.marker_percent}%`)
        expect(returns[i]!.querySelector<HTMLElement>('[data-treasury]')!.style.left).toBe(
          `${ok.scale.treasury.marker_percent}%`,
        )
      })
    }
  })

  it('puts the treasury line on the scale', async () => {
    await mount(ok)
    const mark = host.querySelector<HTMLElement>('[data-scale] [data-treasury]')!
    expect(mark.style.left).toBe(`${ok.scale.treasury.marker_percent}%`)
    expect(host.textContent).toContain(ok.scale.treasury.text)
  })

  it('shows the flag and the cyclical mark', async () => {
    await mount(ok)
    expect(host.textContent).toContain(ok.rows[0]!.flag)
    expect(host.querySelector('[data-row="BBB"]')!.textContent).toContain('周期')
    expect(host.querySelector('[data-row="AAA"]')!.textContent).not.toContain('周期')
  })

  it('shows the gap text of a row whose returns are a Gap, and no dot', async () => {
    await mount(ok)
    await openAll()
    const row = ok.rows[2]!
    const el = host.querySelector(`[data-row="${row.symbol}"]`)!
    expect(el.textContent).toContain((row.returns as { text: string }).text)
    expect(el.querySelector('[data-dot]')).toBeNull()
  })

  it('shows the gaps of the page', async () => {
    await mount(gap)
    expect(host.textContent).toContain(gap.gaps[0]!.text)
  })

  it('shows the stale label', async () => {
    await mount(stale)
    expect(host.textContent).toContain(stale.stale.text)
  })
})
