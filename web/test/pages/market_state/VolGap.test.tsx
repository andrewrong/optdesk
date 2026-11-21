// The volatility block prints the producer's figure text, day text and hint; nothing is parsed or computed.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
import { MarketStatePage } from '@/pages/market_state/MarketStatePage'
import type { MarketStatePagePayload } from '@/pages/market_state/MarketStatePage'
import { VolGap } from '@/pages/market_state/VolGap'
import type { VolGap as Block } from '@/generated/market_state_page.v2'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
// jsdom has no ResizeObserver, which the Radix tooltip measures with.
vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })

const block: Block = {
  title: 'Implied minus realised volatility',
  hint: '隐含波动率减去 20 日实现波动率，单位为波动率点。',
  rows: [
    { id: 'spx', label: 'SPX', figure: { kind: 'value', value: 2.4, text: '+2.4', unit: 'vol_points' }, day_text: '2026-11-13' },
    { id: 'ndx', label: 'NDX', figure: { kind: 'value', value: -1.1, text: '-1.1', unit: 'vol_points' }, day_text: '2026-11-12' },
    { id: 'rut', label: 'RUT', figure: { kind: 'gap', reason: 'no_data', text: 'RUT has no reading' }, day_text: 'no day' },
  ],
}

async function show() {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const root = createRoot(host)
  await act(async () => {
    root.render(<VolGap block={block} />)
  })
  return {
    host,
    done: async () => {
      await act(async () => root.unmount())
      host.remove()
    },
  }
}

describe('VolGap', () => {
  it('shows the figure, the day and the hint', async () => {
    const { host, done } = await show()
    const spx = host.querySelector('[data-vol-gap-row="spx"]')!
    expect(spx.textContent).toContain('SPX')
    expect(spx.textContent).toContain('+2.4')
    expect(spx.textContent).toContain('2026-11-13')
    expect(host.textContent).toContain('Implied minus realised volatility')
    expect(document.body.textContent).not.toContain(block.hint)
    const mark = host.querySelector('[data-hint-mark]') as HTMLElement
    await act(async () => {
      mark.click()
    })
    expect(document.body.textContent).toContain(block.hint)
    await done()
  })

  it('shows a gap text', async () => {
    const { host, done } = await show()
    const rut = host.querySelector('[data-vol-gap-row="rut"]')!
    expect(rut.textContent).toContain('RUT has no reading')
    expect(rut.textContent).toContain('no day')
    await done()
  })
})

describe('MarketStatePage vol_gap', () => {
  const v2 = JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.market_state_page.v2.ok.json'),
      'utf8',
    ),
  ) as MarketStatePagePayload & { vol_gap?: Block }

  async function page(payload: MarketStatePagePayload) {
    const host = document.createElement('div')
    document.body.appendChild(host)
    const root = createRoot(host)
    await act(async () => {
      root.render(<MarketStatePage payload={payload} />)
    })
    return {
      host,
      done: async () => {
        await act(async () => root.unmount())
        host.remove()
      },
    }
  }

  it('mounts the volatility block when the payload has vol_gap', async () => {
    const { host, done } = await page(v2)
    expect(host.querySelectorAll('[data-vol-gap]')).toHaveLength(1)
    expect(host.querySelector('[data-vol-gap]')?.textContent).toContain(v2.vol_gap!.title)
    await done()
  })

  it('mounts no volatility block when the payload has no vol_gap', async () => {
    const without = { ...v2 }
    delete without.vol_gap
    const { host, done } = await page(without)
    expect(host.querySelector('[data-vol-gap]')).toBeNull()
    expect(host.querySelector('[data-market-rows]')).not.toBeNull()
    await done()
  })
})
