// The underlyings view shows the strings of the payload as sent. Expected strings are read from the contract fixtures.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { registration } from '@/pages/underlyings/view'
import { validate, type Payload } from '@/types/underlyings'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.underlyings.v1.${name}.json`), 'utf8'),
  )

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  const View = registration.page.component as unknown as (props: { payload: Payload }) => React.ReactElement
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(<View payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  vi.unstubAllGlobals()
})

describe('underlyings view', () => {
  it('is registered as underlyings', () => {
    expect(registration.schema).toBe('underlyings')
  })

  it('shows the title, sentence, rows and history of the golden payload', async () => {
    const p = fixture('golden')
    const el = await render(p)
    const text = el.textContent ?? ''
    for (const s of [p.title, p.sentence]) expect(text).toContain(s)
    for (const r of p.rows) {
      for (const s of [r.symbol, r.market, r.kind, r.name, r.status_text, r.followed_text, r.buy_price, r.sale_price, r.note])
        expect(text).toContain(s)
      for (const g of r.followed_by) expect(text).toContain(g)
    }
    for (const h of p.history) for (const s of [h.at, h.key, h.text]) expect(text).toContain(s)
    expect(el.querySelector('[data-confirm]')).toBeNull()
  })

  it('shows the status text of a pending row', async () => {
    const p = fixture('golden')
    const pending = p.rows.find((r) => r.status === 'pending')!
    expect((await render(p)).textContent).toContain(pending.status_text)
  })

  it('posts plain forms with the hidden token and an edit control only for editable fields', async () => {
    const p = fixture('golden')
    const el = await render(p)
    const forms = [...el.querySelectorAll('form')]
    expect(forms.length).toBe(p.rows.length + 1)
    for (const f of forms) {
      expect(f.getAttribute('method')).toBe('post')
      expect(f.getAttribute('action')).toBe('/owner-underlyings')
      expect((f.querySelector('input[name=expires]') as HTMLInputElement).value).toBe(p.form!.expires)
      expect((f.querySelector('input[name=mac]') as HTMLInputElement).value).toBe(p.form!.mac)
    }
    for (const r of p.rows) {
      const f = el.querySelector(`form[data-row="${r.key}"]`)!
      expect(f.querySelector('[name=issuer]') !== null).toBe(r.editable.includes('issuer'))
      expect(f.querySelector('[name=exchange]') !== null).toBe(r.editable.includes('exchange'))
      expect(f.querySelector('[name=options]') !== null).toBe(r.editable.includes('options'))
    }
  })

  it('renders no form for the read-only payload and shows its gaps', async () => {
    const p = fixture('readonly')
    const el = await render(p)
    expect(el.querySelector('form')).toBeNull()
    expect(el.querySelector('input,button[role=switch]:not([disabled])')).toBeNull()
    for (const g of p.gaps) expect(el.textContent).toContain(g.text)
  })

  it('shows the changes, the stops and the notice of the confirm payload', async () => {
    const p = fixture('confirm')
    const el = await render(p)
    const text = el.textContent ?? ''
    for (const c of p.confirm!.changes) for (const s of [c.label, c.old, c.new]) expect(text).toContain(s)
    for (const s of p.confirm!.stops) expect(text).toContain(s)
    expect(text).toContain(p.notice)
    expect(el.querySelectorAll('[data-stop]').length).toBe(p.confirm!.stops.length)
  })

  it('refuses the bad-version payload', () => {
    const r = validate(fixture('bad-version'))
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.gap.reason).toBe('unsupported_version')
  })
})
