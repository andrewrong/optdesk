// The refresh button and the quote cell draw payload fields only: the notice, the enabled flag and the quote words.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { OptionQuoteCell, OptionQuoteList } from '@/components/option-quote-cell'
import { OptionRefresh } from '@/components/option-refresh'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | undefined
let host: HTMLElement
async function show(node: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(node)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host.remove()
})

const refresh = (enabled: boolean) => ({
  enabled,
  fresh_count: 0,
  notice: 'market closed; stored quotes of 09:45',
  symbols: ['AAPL'] as ['AAPL'],
  total_count: 3,
})

it('disables the button when refresh.enabled is false and shows the notice', async () => {
  await show(<OptionRefresh refresh={refresh(false)} />)
  expect(host.querySelector<HTMLButtonElement>('[data-refresh-button]')?.disabled).toBe(true)
  expect(host.querySelector('[data-refresh-notice]')?.textContent).toContain('market closed')
})

it('posts the symbols of the payload to the refresh route and disables itself while the call runs', async () => {
  await show(<OptionRefresh refresh={refresh(true)} />)
  const form = host.querySelector<HTMLFormElement>('form')!
  expect(form.method).toBe('post')
  expect(form.getAttribute('action')).toBe('/options/refresh')
  expect([...form.querySelectorAll<HTMLInputElement>('input[name=symbols]')].map((i) => i.value)).toEqual(['AAPL'])
  const button = host.querySelector<HTMLButtonElement>('[data-refresh-button]')!
  expect(button.disabled).toBe(false)
  form.addEventListener('submit', (event) => event.preventDefault())
  await act(async () => button.click())
  expect(button.disabled).toBe(true)
})

it('renders nothing without a refresh object (a v1 payload)', async () => {
  await show(<OptionRefresh refresh={undefined} />)
  expect(host.querySelector('[data-refresh]')).toBeNull()
})

it('copies time, data type and source of the quote', async () => {
  await show(
    <OptionQuoteCell
      quote={{ as_of: '09:45:12', data_type: 'delayed', source: 'refresh', skew: false, outside_band: false, stale: false } as never}
    />,
  )
  expect(host.querySelector('[data-quote-time]')?.textContent).toBe('09:45:12')
  expect(host.querySelector('[data-quote-type]')?.textContent).toBe('delayed')
  expect(host.querySelector('[data-quote-source]')?.textContent).toBe('refresh')
})

it('prints skew and outside-band on the quote cell', async () => {
  await show(
    <OptionQuoteCell
      quote={{ as_of: '09:45:12', data_type: 'live', source: 'open_pull', skew: true, outside_band: true } as never}
    />,
  )
  expect(host.textContent).toContain('skew')
  expect(host.textContent).toContain('outside the planned band')
})

it('shows a data type other than live in red words and a live one plainly', async () => {
  const quote = (data_type: string) =>
    ({ as_of: '09:45:12', data_type, source: 'open_pull', skew: false, outside_band: false }) as never
  await show(
    <>
      <OptionQuoteCell quote={quote('frozen')} />
      <OptionQuoteCell quote={quote('live')} />
    </>,
  )
  const types = host.querySelectorAll('[data-quote-type]')
  expect(types[0]?.className).toContain('text-destructive')
  expect(types[1]?.className).not.toContain('text-destructive')
})

const quoteOf = (stale: boolean) =>
  ({ as_of: '09:45:12', data_type: 'live', source: 'open_pull', skew: false, outside_band: false, stale }) as never

it('greys a quote the page marks stale and leaves a fresh one plain', async () => {
  await show(
    <>
      <OptionQuoteCell quote={quoteOf(false)} />
      <OptionQuoteCell quote={quoteOf(true)} />
    </>,
  )
  const cells = [...host.querySelectorAll('[data-quote]')]
  expect(cells.map((cell) => cell.hasAttribute('data-quote-stale'))).toEqual([false, true])
  expect(cells[1]?.className).toContain('opacity-50')
  expect(cells[0]?.className).not.toContain('opacity-50')
})

it('greys a stale quote on the stock page list', async () => {
  await show(<OptionQuoteList items={[{ id: '0', quote: quoteOf(true) }]} />)
  expect(host.querySelector('[data-quote-stale]')).not.toBeNull()
})
