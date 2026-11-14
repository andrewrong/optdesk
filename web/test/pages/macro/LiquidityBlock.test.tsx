// The four sentences, taps and notes are the strings of the contract fixtures; the block adds none.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { LiquidityBlock } from '@/pages/macro/LiquidityBlock'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const v2 = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.macro_page.v2.${name}.json`),
      'utf8',
    ),
  )
let root: Root | undefined
let host: HTMLElement
async function show(title: string, liquidity: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<LiquidityBlock title={title} liquidity={liquidity as never} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('shows the title, the background and the four sentences in order with label and source', async () => {
  const p = v2('golden')
  await show(p.liquidity_title, p.liquidity)
  expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(p.liquidity_title)
  expect(host.textContent).toContain(p.liquidity.background_text)
  const lines = [...host.querySelectorAll('[data-liquidity-line]')]
  expect(lines).toHaveLength(4)
  p.liquidity.lines.forEach(
    (l: { label: string; sentence: string; source_text: string }, i: number) => {
      expect(lines[i]!.textContent).toContain(l.label)
      expect(lines[i]!.textContent).toContain(l.sentence)
      expect(lines[i]!.textContent).toContain(l.source_text)
    },
  )
})

it('opens a tap on click and shows its rows', async () => {
  const p = v2('golden')
  await show(p.liquidity_title, p.liquidity)
  const tap = p.liquidity.taps[0]
  const item = host.querySelector('[data-liquidity-tap=stlfsi]')!
  expect(item.querySelector('[data-slot=accordion-trigger]')?.textContent).toBe(tap.label)
  expect(item.textContent).not.toContain(tap.rows[0].label)
  await act(async () => {
    item.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
  })
  expect(item.textContent).toContain(tap.rows[0].label)
  expect(item.textContent).toContain(tap.rows[0].text)
  expect(item.textContent).toContain(tap.rows[0].data_day)
})

it('shows a Gap row by its text', async () => {
  const p = v2('golden')
  const gapText = 'no data for this row'
  const taps = [
    { ...p.liquidity.taps[0], rows: [{ label: 'STLFSI4', text: { reason: 'no_data', text: gapText }, data_day: null }] },
  ]
  await show(p.liquidity_title, { ...p.liquidity, taps })
  await act(async () => {
    host.querySelector<HTMLElement>('[data-slot=accordion-trigger]')!.click()
  })
  expect(host.querySelector('[data-liquidity-gap]')?.textContent).toBe(gapText)
})

it('shows a Gap liquidity by its text', async () => {
  const gap = { reason: 'no_data', text: 'Liquidity: no data yet.' }
  await show('Funding', gap)
  expect(host.textContent).toContain(gap.text)
  expect(host.querySelector('[data-liquidity-line]')).toBeNull()
})

it('shows the net_liquidity note inside its fold only', async () => {
  const p = v2('golden')
  const net = p.liquidity.taps.find((t: { key: string }) => t.key === 'net_liquidity')
  expect(net.note).not.toBeNull()
  await show(p.liquidity_title, p.liquidity)
  expect(host.textContent).not.toContain(net.note)
  await act(async () => {
    host
      .querySelector<HTMLElement>('[data-liquidity-tap=net_liquidity] [data-slot=accordion-trigger]')!
      .click()
  })
  expect(host.querySelector('[data-liquidity-tap=net_liquidity]')?.textContent).toContain(net.note)
  expect(host.querySelectorAll('[data-liquidity-note]')).toHaveLength(1)
})
