// The page shows the texts of the payload as written; the fixtures are written from the schema alone.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { registration } from '@/pages/structuredetail/view'
import { StructureDetailView } from '@/pages/structuredetail/view'
import type { Payload } from '@/types/structure_detail'
import { validate } from '@/types/structure_detail'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion: unknown = JSON.parse(
  readFileSync(path.join(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)

let root: Root | undefined
let host: HTMLElement
async function show(payload: Payload): Promise<string> {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<StructureDetailView payload={payload} />)
  })
  return host.textContent ?? ''
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
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

describe('structure detail page', () => {
  it('shows the sentence, subline and tiles of ok', async () => {
    const text = await show(ok)
    expect(text).toContain(ok.sentence)
    expect(text).toContain(ok.subline)
    expect(text).toContain(ok.tiles.pnl.label)
    expect(text).toContain(ok.tiles.pnl.caption)
    expect(text).toContain(ok.tiles.max_gain.figure.text)
    expect(text).toContain(ok.tiles.max_loss.figure.text)
    expect(text).toContain(ok.tiles.break_even.figure.text)
  })

  it('shows the legs, legs_total, greeks and fills tables with their titles', async () => {
    const text = await show(ok)
    expect(text).toContain(ok.legs_title)
    expect(text).toContain(ok.legs[0].label)
    expect(text).toContain(ok.legs[0].strike.text)
    expect(text).toContain(ok.legs[1]?.pnl.text ?? '')
    expect(text).toContain(ok.legs_total.label)
    expect(text).toContain(ok.legs_total.pnl.text)
    expect(text).toContain(ok.greeks_title)
    expect(text).toContain(ok.greeks.model_text)
    expect(text).toContain(ok.greeks.rows[0].label)
    expect(text).toContain(ok.greeks.rows[1]?.cells[3].text ?? '')
    expect(text).toContain(ok.fills_title)
    expect(text).toContain(ok.fills[0]?.time_text ?? '')
    expect(text).toContain(ok.fills[0]?.fee.text ?? '')
  })

  it('draws the payoff as one chart with its texts', async () => {
    const text = await show(ok)
    expect(text).toContain(ok.payoff_title)
    expect(text).toContain(ok.payoff.band.text)
    expect(text).toContain(ok.payoff.max_gain.text)
    expect(text).toContain(ok.payoff.spot.text)
    expect(host.querySelectorAll('[data-chart="payoff"]').length).toBe(1)
    expect(host.querySelector('[data-chart="payoff"]')?.getAttribute('aria-label')).toBe(
      ok.payoff.aria_text,
    )
  })

  it('shows the key_levels and the timeline', async () => {
    const text = await show(ok)
    expect(text).toContain(ok.levels_title)
    expect(text).toContain(ok.key_levels[0].name)
    expect(text).toContain(ok.key_levels[0].note)
    expect(text).toContain(ok.key_levels[0].price.text)
    expect(text).toContain(ok.key_levels[0].distance)
    expect(text).toContain(ok.timeline_title)
    expect(text).toContain(ok.timeline[0].name)
    expect(text).toContain(ok.timeline[0].date_text)
    expect(text).toContain(ok.timeline[0].elapsed.text)
  })

  it('shows the stale label of stale and of ok', async () => {
    const text = await show(stale)
    expect(text).toContain(stale.stale.text)
    expect(text).toContain(stale.sentence)
    expect(text).not.toContain(ok.stale.text)
  })

  it('shows the gaps of gap in place of the figures', async () => {
    const text = await show(gap)
    expect(text).toContain(gap.gaps[0].text)
    expect(text).toContain(gap.gaps[1].text)
    expect(text).toContain(gap.legs[0].text)
    expect(text).toContain(gap.greeks.text)
    expect(text).toContain(gap.payoff.text)
    expect(text).toContain(gap.tiles.pnl.figure.text)
    expect(text).toContain(gap.legs_total.pnl.text)
    expect(host.querySelector('[data-chart="payoff"]')).toBeNull()
  })

  it('refuses a schema_version it does not read and registers under its schema', () => {
    const result = validate(badVersion)
    expect(result.ok).toBe(false)
    expect(validate(ok).ok).toBe(true)
    expect(registration.schema).toBe('structure_detail')
  })
})
