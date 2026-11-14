// A value figure is followed by the display suffix of its unit, never by the unit id; an empty suffix prints nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { MarketRows } from '@/pages/market_state/MarketRows'
import type { Payload } from '@/types/market_state_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const ok = JSON.parse(
  readFileSync(
    path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.market_state_page.v1.ok.json'),
    'utf8',
  ),
) as Payload

async function figureOf(unit: string): Promise<{ text: string; span: Element | null }> {
  const base = ok.market.rows[0]!
  const block = {
    ...ok.market,
    rows: [{ ...base, figure: { kind: 'value', text: '7.2', unit } }],
  } as unknown as Payload['market']
  const host = document.createElement('div')
  const root = createRoot(host)
  await act(async () => {
    root.render(<MarketRows block={block} />)
  })
  const row = host.querySelector('[data-market-row]')!
  const span = row.querySelector('span.ml-1')
  const text = row.textContent ?? ''
  await act(async () => root.unmount())
  return { text, span }
}

describe('MarketRows figure unit', () => {
  it('prints % after the figure of a percent unit, never the id', async () => {
    const { text, span } = await figureOf('percent')
    expect(span?.textContent).toBe('%')
    expect(text).toContain('7.2%')
    expect(text).not.toContain('percent')
  })

  it('prints nothing after the figure of a unit with no suffix', async () => {
    const { text, span } = await figureOf('index_points')
    expect(span).toBeNull()
    expect(text).not.toContain('index_points')
  })
})
