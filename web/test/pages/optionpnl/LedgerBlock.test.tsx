import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { OptionPnlView } from '@/pages/optionpnl/OptionPnlPage'
import { ledger } from './fixtures/ledger'
import { ok } from './fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
type Payload = Parameters<typeof OptionPnlView>[0]['payload']
function withLedger(value: unknown): Payload {
  return { ...ok, accounts: [{ ...ok.accounts[0], ledger: value }] } as Payload
}
async function show(payload: Payload): Promise<string> {
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

it('TestLedgerTabsSwitchViews', async () => {
  await show(withLedger(ledger))
  const groups = () => [...host.querySelectorAll('[data-ledger-group]')].map((g) => g.getAttribute('data-ledger-group'))
  expect(groups()).toEqual(['grp-2026-11', 'grp-2026-10'])
  const tabs = [...host.querySelectorAll('[role="tab"]')]
  expect(tabs.map((t) => t.textContent)).toEqual(['By month', 'By company'])
  await act(async () => {
    tabs[1]!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0, ctrlKey: false }))
  })
  expect(groups()).toEqual(['grp-AAA'])
})

it('TestLedgerShowsTextUnchanged', async () => {
  const text = await show(withLedger(ledger))
  expect(text).toContain('−$186.00 · 5 lots')
  expect(host.querySelector('[data-ledger-left-out]')?.textContent).toBe(ledger.views[0]?.groups[0]?.books[0]?.left_out)
  expect(text).toContain('Synthetic no ended lots this month')
})

it('TestLedgerNoConclusionShown', async () => {
  await show(withLedger(ledger))
  expect(host.querySelector('[data-line="lgm0-0-sample"]')?.textContent).toContain('fewer than 20')
  expect(host.querySelector('[data-line="lgm0-0-sample"] [data-tone]')?.className).toContain('text-muted-foreground')
  const tab = host.querySelectorAll('[role="tab"]')[1]!
  await act(async () => {
    tab.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
  })
  expect(host.querySelector('[data-ledger-group="grp-AAA"]')?.textContent).toContain('20 lots')
  expect(host.querySelector('[data-line$="sample"]')).toBeNull()
})

it('TestLedgerAbsentRendersNothing', async () => {
  await show(ok as Payload)
  const titles = () => [...host.querySelectorAll('[data-slot="card-title"]')].map((t) => t.textContent)
  expect(titles()).not.toContain('Ledger')
  expect(host.querySelector('[data-ledger]')).toBeNull()
  await act(async () => root?.unmount())
  host.remove()
  await show(withLedger(null))
  expect(titles()).not.toContain('Ledger')
  expect(host.querySelector('[data-ledger]')).toBeNull()
})

it('TestLedgerHintBesideLabel', async () => {
  await show(withLedger(ledger))
  const row = host.querySelector('[data-line="lgm0-0-time"]')!
  const cells = [...row.querySelectorAll('td')].map((c) => c.textContent)
  expect(cells).toEqual(['Time (theta)', '$412.00 · 5 lots', 'Synthetic time hint'])
})
