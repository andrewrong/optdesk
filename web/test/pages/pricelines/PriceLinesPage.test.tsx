// The price_lines_page prints the texts of the payload and posts plain forms. Expected strings are read from the fixtures.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { PriceLinesPage, registration } from '@/pages/pricelines/PriceLinesPage'
import { pages } from '@/shell/pages'
import type { Payload } from '@/types/price_lines_page'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)

let root: Root | null = null
async function show(payload: Payload) {
  const host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<PriceLinesPage payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})

function count(haystack: string, needle: string) {
  return haystack.split(needle).length - 1
}

describe('PriceLinesPage', () => {
  it('posts every form plainly with the hidden expires and mac of the payload', async () => {
    const host = await show(ok)
    const forms = [...host.querySelectorAll('form')]
    expect(forms.length).toBe(ok.rows.length + 1 + ok.unset.length)
    for (const form of forms) {
      expect(form.getAttribute('method')).toBe('post')
      expect(form.getAttribute('action')).toBe(ok.form.post_path)
      const expires = form.querySelector<HTMLInputElement>('input[type=hidden][name=expires]')
      const mac = form.querySelector<HTMLInputElement>('input[type=hidden][name=mac]')
      expect(expires?.value).toBe(ok.form.expires)
      expect(mac?.value).toBe(ok.form.mac)
    }
  })

  it('shows the intro, saved message, rows and footnote as sent', async () => {
    const host = await show(ok)
    const text = host.textContent ?? ''
    for (const s of [ok.title, ok.intro, ok.saved, ok.add.hint, ok.footnote]) expect(text).toContain(s)
    for (const row of ok.rows) {
      expect(text).toContain(row.symbol)
      expect(text).toContain(row.price)
      if (row.note) expect(text).toContain(row.note)
    }
  })

  it('gives each unset company its own prefilled add form', async () => {
    const host = await show(ok)
    for (const row of ok.unset) {
      const form = [...host.querySelectorAll('form')].find(
        (f) => f.querySelector<HTMLInputElement>('input[name=symbol]')?.value === row.symbol,
      )
      expect(form).toBeTruthy()
      expect(form?.querySelector<HTMLSelectElement>('select[name=market]')?.value).toBe(row.market)
    }
  })

  it('offers the markets of add in the add form', async () => {
    const host = await show(ok)
    const select = host.querySelector('form[data-add] select[name=market]')
    expect([...(select?.querySelectorAll('option') ?? [])].map((o) => o.value)).toEqual(ok.add.markets)
  })

  it('shows error in an alert and no saved message when saved is null', async () => {
    const host = await show({ ...ok, saved: null, error: 'Error text ok: price is not a number' })
    const alerts = [...host.querySelectorAll('[role=alert]')].map((a) => a.textContent ?? '')
    expect(alerts.some((a) => a.includes('Error text ok: price is not a number'))).toBe(true)
    expect(host.textContent).not.toContain(ok.saved)
    expect(host.querySelector('[data-saved]')).toBeNull()
  })

  it('shows saved in an alert', async () => {
    const host = await show(ok)
    expect(host.querySelector('[data-saved]')?.textContent).toContain(ok.saved)
  })

  it('names each envelope gap once and shows no price for a null price', async () => {
    const host = await show(gap)
    const text = host.textContent ?? ''
    for (const g of gap.gaps) expect(count(text, g.text)).toBe(1)
    const row = [...host.querySelectorAll('[data-row]')].find((r) => r.textContent?.includes('BETA'))
    expect(row?.querySelector('[data-price]')).toBeNull()
    expect(row?.querySelector<HTMLInputElement>('input[name=price]')?.value).toBe('')
  })

  it('shows the stale label with the producer text', async () => {
    const host = await show(stale)
    expect(host.querySelector('[data-stale]')?.textContent).toContain(stale.stale.text)
  })

  it('refuses bad-version and accepts ok', () => {
    expect(registration.page.validate(badVersion).ok).toBe(false)
    expect(registration.page.validate(ok).ok).toBe(true)
  })

  it('registers itself under price_lines_page', () => {
    expect(registration.schema).toBe('price_lines_page')
    expect(pages.price_lines_page).toBe(registration.page)
  })
})
