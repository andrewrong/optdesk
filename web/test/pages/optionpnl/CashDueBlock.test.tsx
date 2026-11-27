import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import type { Line } from '@/generated/option_pnl_page.v1'
import { CashDueBlock } from '@/pages/optionpnl/CashDueBlock'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
async function show(lines?: Line[] | null): Promise<void> {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<CashDueBlock lines={lines} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

const usd: Line = { id: 'cash-due-USD', label: 'Label A', text: '$38,000.00 · 2', hint: 'Hint A', tone: 'none' }
const jpy: Line = { id: 'cash-due-JPY', label: 'Label B', text: '¥5,000,000', hint: null, tone: 'muted' }

it('shows label text and hint as given', async () => {
  await show([usd])
  const row = host.querySelector('[data-line="cash-due-USD"]')!
  expect(row.children[0]!.textContent).toBe('Label A')
  expect(row.children[1]!.textContent).toBe('$38,000.00 · 2')
  expect(row.children[2]!.textContent).toBe('Hint A')
})

it('renders nothing without cash_due', async () => {
  await show(undefined)
  expect(host.innerHTML).toBe('')
  await act(async () => root?.unmount())
  host.remove()
  await show([])
  expect(host.innerHTML).toBe('')
})

it('one row per currency', async () => {
  await show([usd, jpy])
  expect([...host.querySelectorAll('tr')].map((tr) => tr.getAttribute('data-line'))).toEqual([
    'cash-due-USD',
    'cash-due-JPY',
  ])
})
