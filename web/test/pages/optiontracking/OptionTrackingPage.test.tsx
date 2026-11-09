// The expected texts are read from the synthetic fixtures; the page must show each as sent.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { OptionTrackingView, registration } from '@/pages/optiontracking/OptionTrackingPage'
import { discoverCards } from '@/shell/pages'
import { validate } from '@/types/option_tracking_page'
import { gap } from './fixtures/gap'
import { noMarks } from './fixtures/no-marks'
import { ok } from './fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion: unknown = JSON.parse(
  readFileSync(path.join(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: typeof ok | typeof noMarks | typeof gap): Promise<string> {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionTrackingView payload={payload} />)
  })
  return host.textContent ?? ''
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const badgeOf = (text: string): Element => {
  const found = [...host.querySelectorAll('[data-slot="badge"]')].find((el) => el.textContent === text)
  if (!found) throw new Error(`no badge ${text}`)
  return found
}

it('shows the header and the open table of ok in payload order', async () => {
  const text = await show(ok)
  expect(text).toContain(ok.as_of)
  expect(text).toContain(ok.mark_note)
  const account = ok.accounts[0]
  expect(text).toContain(account.title)
  expect(text).toContain(account.mark_day)
  expect(text).toContain(account.guard_day)
  const open = host.querySelector('[data-table="open"]')!
  const cells = [...open.querySelectorAll('tbody tr')].map((tr) => tr.firstElementChild?.textContent)
  expect(cells).toEqual(['Row two contract', 'Row one contract'])
  expect(host.querySelector('[data-stale]')).toBeNull()
})

it('shows the badges as given', async () => {
  const text = await show(ok)
  for (const badge of ['价内', '价外']) expect(badgeOf(badge)).toBeTruthy()
  expect(text).toContain('在存续期内')
})

it('puts the recent and check tables in closed accordion items', async () => {
  const text = await show(ok)
  const items = [...host.querySelectorAll('[data-slot="accordion-item"]')]
  expect(items.map((el) => el.getAttribute('data-state'))).toEqual(['closed', 'closed'])
  expect(text).toContain('最近 30 天了结')
  expect(text).toContain('Flex 与账户核对')
})

it('shows the early-assignment badge once the recent item is opened', async () => {
  await show(ok)
  const trigger = host.querySelector('[data-slot="accordion-trigger"]') as HTMLElement
  await act(async () => trigger.click())
  expect(badgeOf('提前')).toBeTruthy()
})

it('shows the gap text of a no-mark row and not a zero', async () => {
  const text = await show(noMarks)
  expect(text).toContain('Synthetic no-mark gap text')
  const row = host.querySelector('[data-table="open"] tbody tr')!
  expect([...row.children].map((td) => td.textContent)).toEqual([
    'Unmarked contract',
    'Synthetic no-mark gap text',
  ])
  expect(text).not.toContain('0')
})

it('shows the unconfirmed guard badge in a tone other than the blocking one', async () => {
  await show(ok)
  const unknown = badgeOf('未确认')
  const blocked = badgeOf('阻止')
  expect(unknown.className).not.toBe(blocked.className)
})

it('shows the cause of an account gap and no table', async () => {
  const text = await show(gap)
  expect(text).toContain('Synthetic account gap cause')
  expect(host.querySelector('table')).toBeNull()
  expect(host.querySelector('[data-slot="accordion-item"]')).toBeNull()
})

it('shows the stale label', async () => {
  const text = await show(noMarks)
  expect(text).toContain('Synthetic stale label text')
  expect(host.querySelector('[data-stale="true"]')).not.toBeNull()
})

it('shows the left-out count of long legs and the page notes', async () => {
  const text = await show(ok)
  expect(text).toContain('3')
  for (const note of ok.notes) expect(text).toContain(note)
})

it('has no input element', async () => {
  await show(ok)
  expect(host.querySelector('input')).toBeNull()
})

it('refuses a version it cannot read', () => {
  expect(validate(badVersion).ok).toBe(false)
  expect(validate(ok).ok).toBe(true)
})

it('registers the schema once', () => {
  const found = discoverCards(import.meta.glob('@/pages/optiontracking/*.tsx', { eager: true }))
  expect(Object.keys(found)).toEqual(['option_tracking_page'])
  expect(registration.schema).toBe('option_tracking_page')
})
