// The volatility block prints the producer's figure text, day text and hint; nothing is parsed or computed.
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
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
