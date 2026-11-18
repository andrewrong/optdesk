import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { CompanyStrip } from '@/pages/candidates/CompanyStrip'
import { withLine, withNull, withoutKey, withReason } from './fixtures/valueline'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
async function show(node: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(node)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

describe('CompanyStrip', () => {
  it('TestStripShowsValueLine', async () => {
    const page = await show(<CompanyStrip company={withLine} />)
    const value = page.querySelector('[data-valuation]')!
    expect(value.textContent).toContain('Valuation buy line B_hi')
    expect(value.textContent).toContain('$91.20 (review 2026-11-10)')
    const gate = page.querySelector('[data-gate]')!
    expect(gate.textContent).toContain('Buy price used by the gate (hand-typed)')
    expect(gate.textContent).toContain('$90.00')
  })

  it('TestStripShowsReasonText', async () => {
    const page = await show(<CompanyStrip company={withReason} />)
    const value = page.querySelector('[data-valuation]')!
    expect(value.textContent).toContain(withReason.valuation!.text)
    expect(value.textContent).not.toContain('$')
  })

  it('TestStripNoValuationUnchanged', async () => {
    const expected = (c: typeof withNull) =>
      `${c.ticker}Spot ${c.spot} (rows of ${c.spot_day})Buy price ${c.buy_price}Next earnings ${c.next_earnings}${c.counts}`
    for (const c of [withNull, withoutKey]) {
      const page = await show(<CompanyStrip company={c} />)
      expect(page.textContent).toBe(expected(c))
      expect(page.querySelector('[data-valuation]')).toBeNull()
      expect(page.querySelector('[data-gate]')).toBeNull()
      await act(async () => root?.unmount())
      host.remove()
    }
  })

  it('TestStripHintBesideLabel', async () => {
    const page = await show(<CompanyStrip company={withLine} />)
    const hint = page.querySelector('[data-valuation] [data-hint]')!
    expect(hint.getAttribute('data-hint')).toBe(withLine.valuation!.hint)
    expect(hint.textContent).toBe('Valuation buy line B_hi')
    const gateHint = page.querySelector('[data-gate] [data-hint]')!
    expect(gateHint.getAttribute('data-hint')).toBe(withLine.valuation!.gate_hint)
    expect(gateHint.textContent).toBe('Buy price used by the gate (hand-typed)')
  })

  it('shows the spot hint on the word Spot', async () => {
    const page = await show(<CompanyStrip company={withNull} spotHint="hint 中文" />)
    expect(page.querySelector('[data-hint="hint 中文"]')!.textContent).toBe('Spot')
    expect(page.textContent).toContain(`rows of ${withNull.spot_day}`)
  })
})
