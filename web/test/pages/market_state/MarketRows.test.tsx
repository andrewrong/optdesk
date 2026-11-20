// A value figure prints its payload text alone: the text already carries its unit, so nothing follows it.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'
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
  it('prints the figure text alone for a percent unit, with no suffix or id after it', async () => {
    const { text, span } = await figureOf('percent')
    expect(span).toBeNull()
    expect(text).toContain('7.2')
    expect(text).not.toContain('7.2%')
    expect(text).not.toContain('percent')
  })

  it('prints nothing after the figure of a unit with no suffix', async () => {
    const { text, span } = await figureOf('index_points')
    expect(span).toBeNull()
    expect(text).not.toContain('index_points')
  })
})

type TestRow = Payload['market']['rows'][number]

async function render(patch: Partial<TestRow>): Promise<{ host: HTMLElement; done: () => Promise<void> }> {
  const base = ok.market.rows[0]!
  const block = { ...ok.market, rows: [{ ...base, ...patch }] } as unknown as Payload['market']
  const host = document.createElement('div')
  document.body.appendChild(host)
  const root = createRoot(host)
  await act(async () => {
    root.render(<MarketRows block={block} />)
  })
  return {
    host,
    done: async () => {
      await act(async () => root.unmount())
      host.remove()
    },
  }
}

describe('MarketRows range and hint', () => {
  // jsdom has no ResizeObserver, which the Radix tooltip measures with.
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  it('range badge shows state, cuts and notice', async () => {
    const { host, done } = await render({
      range: {
        state: 'caution',
        state_text: 'Caution (trial)',
        cuts: [
          { level: 'caution', text: 'Caution from 20', basis: 'published', source: 'A' },
          { level: 'alarm', text: 'Alarm from 30', basis: 'published', source: 'B' },
        ],
        notice: 'A notice only; nothing is blocked.',
      },
    } as Partial<TestRow>)
    const badge = host.querySelector('[data-range-state]')!
    expect(badge.textContent).toBe('Caution (trial)')
    expect(badge.getAttribute('data-range-state')).toBe('caution')
    const text = host.textContent ?? ''
    expect(text).toContain('Caution from 20')
    expect(text).toContain('Alarm from 30')
    expect(text).toContain('A notice only; nothing is blocked.')
    await done()
  })

  it('range gap prints its text', async () => {
    const { host, done } = await render({
      range: { kind: 'gap', reason: 'no_data', text: 'Range not available' },
    } as unknown as Partial<TestRow>)
    expect(host.querySelector('[data-range-state]')).toBeNull()
    expect(host.querySelector('[data-range-gap]')?.textContent).toBe('Range not available')
    await done()
  })

  it('hint opens as tooltip on the label', async () => {
    const { host, done } = await render({ hint: '这是提示文本' })
    expect(document.body.textContent).not.toContain('这是提示文本')
    const mark = host.querySelector('[data-hint-mark]') as HTMLElement
    expect(mark).not.toBeNull()
    await act(async () => {
      mark.click()
    })
    expect(document.body.textContent).toContain('这是提示文本')
    await done()
  })

  it('row without range is unchanged', async () => {
    const { host, done } = await render({ range: undefined, hint: undefined })
    expect(host.querySelector('[data-range-state]')).toBeNull()
    expect(host.querySelector('[data-range-gap]')).toBeNull()
    expect(host.querySelector('[data-hint-mark]')).toBeNull()
    expect(host.querySelector('[data-range-notice]')).toBeNull()
    await done()
  })
})
