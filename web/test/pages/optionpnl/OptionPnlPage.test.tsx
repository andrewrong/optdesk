// The expected texts are read from the synthetic fixtures; the page must show each as sent.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { OptionPnlView, registration } from '@/pages/optionpnl/OptionPnlPage'
import { discoverCards } from '@/shell/pages'
import { validate } from '@/types/option_pnl_page'
import { gap } from './fixtures/gap'
import { lotGap } from './fixtures/lot-gap'
import { noQuotes } from './fixtures/no-quotes'
import { ok } from './fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion: unknown = JSON.parse(
  readFileSync(path.join(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: Parameters<typeof OptionPnlView>[0]['payload']): Promise<string> {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionPnlView payload={payload} />)
  })
  return host.textContent ?? ''
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('TestPnlRendersBookAndLots', async () => {
  const text = await show(ok)
  const book = host.querySelector('[data-book]')!
  expect(book.textContent).toContain('Synthetic counts text')
  expect(book.querySelector('[data-left-out]')?.textContent).toBe(ok.accounts[0].books[0]!.left_out)
  const ids = [...book.querySelectorAll('tbody tr')].map((tr) => tr.getAttribute('data-line'))
  expect(ids).toEqual(['rent', 'time', 'move'])
  const lot = host.querySelector('[data-lot]')!
  const lotIds = [...lot.querySelectorAll(':scope > div tbody tr')].map((tr) => tr.getAttribute('data-line'))
  expect(lotIds.slice(0, 3)).toEqual(['lot-time', 'lot-ledger', 'iv'])
  expect(text).toContain('Synthetic AAA summary')
  expect([...lot.querySelectorAll('[data-slot="badge"]')].map((b) => b.textContent)).toEqual(['2-day step', 'wide quote'])
  expect(host.querySelector('[data-line="lot-ledger"] [data-tone="muted"], [data-line="lot-ledger"] td[data-tone="muted"]')?.className).toContain('text-muted-foreground')
  const bad = host.querySelector('[data-line="move"] td[data-tone="bad"]')!
  expect(bad.className).toContain('text-down')
  const none = host.querySelector('[data-line="time"] td[data-tone="none"]')!
  expect(none.className).not.toMatch(/text-(success|down|muted-foreground)/)
  expect(text).toContain('Synthetic pnl note one')
})

it('TestPnlNoQuotesShowsLedgerOnly', async () => {
  const text = await show(noQuotes)
  const lot = host.querySelector('[data-lot]')!
  expect(lot.textContent).toContain('$12.00')
  expect(lot.textContent).not.toMatch(/(^|[^\d.$])0\.00/)
  expect(text).toContain(noQuotes.accounts[0].lots[0]!.days_line.label)
  expect(lot.querySelectorAll('[data-line="iv"], [data-line="assignment"]').length).toBe(0)
})

it('TestPnlLotGapShowsReason', async () => {
  await show(lotGap)
  const lot = host.querySelector('[data-lot]')!
  expect(lot.querySelector('[data-slot="alert"]')?.textContent).toBe('Synthetic lot gap reason')
  expect(lot.querySelector('table')).toBeNull()
  expect(lot.textContent).toContain('Synthetic EEE summary')
})

it('TestPnlGapAlertShown', async () => {
  const text = await show(gap)
  expect(text).toContain('Synthetic page gap text')
  expect(text).toContain('Synthetic account gap cause')
  expect(host.querySelector('[data-stale="true"]')?.textContent).toBe('Synthetic stale label text')
  expect(host.querySelector('table')).toBeNull()
})

it('TestPnlBadVersionIsRefused', () => {
  expect(validate(badVersion).ok).toBe(false)
  expect(validate(ok).ok).toBe(true)
  const found = discoverCards(import.meta.glob('@/pages/optionpnl/*.tsx', { eager: true }))
  expect(Object.keys(found)).toEqual(['option_pnl_page'])
  expect(registration.schema).toBe('option_pnl_page')
})

it('TestPnlHintBesideLabel', async () => {
  await show(ok)
  const row = host.querySelector('[data-line="time"]')!
  const cells = [...row.children].map((td) => td.textContent)
  expect(cells).toEqual(['Time (theta)', '$412.00', 'Synthetic time hint'])
  const bare = host.querySelector('[data-line="move"]')!
  expect(bare.lastElementChild?.textContent).toBe('')
})

it('TestPnlShowsTextUnchanged', async () => {
  await show(ok)
  const cell = host.querySelector('[data-line="rent"]')!.children[1]!
  expect(cell.textContent).toBe('−$1,234.50')
})

it('TestPnlLeftOutUnderHeadline', async () => {
  await show(ok)
  const book = host.querySelector('[data-book]')!
  const headline = book.querySelector('[data-line="rent"]')!
  const leftOut = book.querySelector('[data-left-out]')!
  expect(headline.compareDocumentPosition(leftOut) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

it('shows the cash due block under the books', async () => {
  await show(ok)
  const section = host.querySelector('[data-account]')!
  const block = section.querySelector('[data-cash-due]')!
  expect(block.textContent).toContain('Synthetic cash due text')
  const book = section.querySelector('[data-book]')!
  expect(book.compareDocumentPosition(block) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  const lot = section.querySelector('[data-lot]')!
  expect(block.compareDocumentPosition(lot) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})
