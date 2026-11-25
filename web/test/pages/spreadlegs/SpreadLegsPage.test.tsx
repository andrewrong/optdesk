// The spread legs page prints the producer's text: legs per card, the break-evens in order, gaps as sent.
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import type { SpreadLegsPage as Payload } from '@/generated/spread_legs_page.v1'
import { SpreadLegsPage, registration, validateSpreadLegsPage } from '@/pages/spreadlegs/SpreadLegsPage'
import { spreads } from './fixtures/spreads'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

async function show(payload: Payload = spreads) {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const root = createRoot(host)
  await act(async () => {
    root.render(<SpreadLegsPage payload={payload} />)
  })
  return {
    host,
    done: async () => {
      await act(async () => root.unmount())
      host.remove()
    },
  }
}

const card = (host: HTMLElement, symbol: string) =>
  [...host.querySelectorAll('section[data-structure]')].find((c) => c.querySelector('h3')?.textContent === symbol)!

describe('SpreadLegsPage', () => {
  it('shows the put spread with 2 legs and one break-even, the symbol linked', async () => {
    const { host, done } = await show()
    const c = card(host, 'AAA')
    expect(c.querySelectorAll('tbody tr').length).toBe(2)
    expect(c.querySelector('[data-figure="break_even"] dd')?.textContent).toBe('93.70')
    expect(c.querySelector('h3 a')?.getAttribute('href')).toBe('/options/structures/put_spread-aaa-2026-12-18')
    expect(c.querySelector('[data-kind-gloss]')?.textContent).toBe('卖出价格较高的认沽、买入价格较低的认沽')
    expect(c.querySelector('[data-figure="net_credit"]')?.textContent).toContain('+260.00 美元')
    expect(c.querySelector('tbody tr')?.textContent).toContain('卖出认沽 Short put')
    await done()
  })

  it('TestSpreadLegsFixedLabelsChinese', async () => {
    const { host, done } = await show()
    const c = card(host, 'AAA')
    expect([...c.querySelectorAll('thead th')].map((h) => h.textContent)).toEqual(['方向', '数量', '行权价', '开仓价'])
    const names = (f: string) => c.querySelector(`[data-figure="${f}"] dt`)?.textContent
    expect(names('net_credit')).toBe('净权利金收入')
    expect(names('max_loss')).toBe('最大亏损')
    expect(names('max_gain')).toBe('最大盈利')
    expect(names('break_even')).toBe('盈亏平衡点')
    await done()
  })

  it('shows the iron condor with two break-evens in the order sent', async () => {
    const { host, done } = await show()
    expect(card(host, 'BBB').querySelector('[data-figure="break_even"] dd')?.textContent).toBe('86.90 和 113.10')
    expect(card(host, 'BBB').querySelectorAll('tbody tr').length).toBe(4)
    await done()
  })

  it('shows the gap texts of the covered call as sent and no number for them', async () => {
    const { host, done } = await show()
    const c = card(host, 'CCC')
    expect(c.querySelector('[data-figure="max_loss"]')?.textContent).toContain('无上限规则（含股票，此页不算）')
    expect(c.querySelector('[data-figure="break_even"]')?.textContent).toContain('此页不算（含股票）')
    expect(c.querySelectorAll('[data-break-even-value]').length).toBe(0)
    expect(c.querySelectorAll('[data-gap="no_rule"]').length).toBe(2)
    await done()
  })

  it('shows no card for an empty group', async () => {
    const empty: Payload = { ...spreads, groups: [spreads.groups[0], { ...spreads.groups[1], structures: [] }] }
    const { host, done } = await show(empty)
    expect(host.querySelectorAll('section[data-structure]').length).toBe(3)
    expect(host.querySelector('[data-group="1"] section')).toBeNull()
    await done()
  })

  it('returns the gap for a bad version and registers as spread_legs_page', () => {
    const bad = validateSpreadLegsPage({ ...spreads, schema_version: 2 })
    expect(bad.ok).toBe(false)
    expect(validateSpreadLegsPage(spreads).ok).toBe(true)
    expect(registration.schema).toBe('spread_legs_page')
  })
})
