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
    readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.underlyings.v2.${name}.json`), 'utf8'),
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
  document.body.querySelectorAll('[role=dialog]').forEach((d) => d.remove())
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

  it('wraps the name cells and shows the scroll hint', async () => {
    const el = await render(fixture('golden'))
    const table = el.querySelector('table[data-names]')!
    expect(table.closest('[data-slot=table-container]')!.previousElementSibling!.hasAttribute('data-scroll-hint')).toBe(true)
    const cells = table.querySelectorAll('tbody tr')[0]!.children
    expect(cells[0]!.className).toContain('whitespace-normal break-words')
    expect(cells[0]!.className).toContain('sticky left-0 bg-background')
    expect(cells[2]!.className).toContain('break-words')
    expect(cells[6]!.className).not.toContain('break-words')
  })

  it('shows the flags of each row as On or Off text and its followed_by groups as badges', async () => {
    const p = fixture('golden')
    const el = await render(p)
    for (const r of p.rows) {
      const tr = el.querySelector(`tr[data-name="${r.key}"]`)!
      for (const f of ['options', 'fundamentals', 'held'] as const) {
        const col = ['options', 'fundamentals', 'held'].indexOf(f) + 3
        expect(tr.children[col]!.textContent).toBe(r[f] ? 'On' : 'Off')
      }
      const badges = [...tr.querySelectorAll('[data-slot=badge]')].map((b) => b.textContent)
      expect(badges).toEqual([r.status_text, ...r.followed_by])
    }
  })

  it('new: each row has an Edit link to the id of its edit form', async () => {
    const p = fixture('golden')
    const el = await render(p)
    expect(p.rows.some((r) => r.key.includes(':'))).toBe(true)
    for (const r of p.rows) {
      const id = `edit-${r.key.replace(/[^A-Za-z0-9_-]/g, '-')}`
      const a = el.querySelector(`tr[data-name="${r.key}"] a`)!
      expect(a.textContent).toBe('Edit')
      expect(a.getAttribute('href')).toBe(`#${id}`)
      const forms = el.querySelectorAll(`form[id="${id}"]`)
      expect(forms.length).toBe(1)
      expect(forms[0]!.getAttribute('tabindex')).toBe('-1')
      expect(forms[0]!.getAttribute('data-row')).toBe(r.key)
    }
  })

  it('new: each edit form is hidden unless targeted', async () => {
    const p = fixture('golden')
    const el = await render(p)
    const forms = [...el.querySelectorAll('form[data-row]')]
    expect(forms.length).toBe(p.rows.length)
    for (const f of forms) {
      expect(f.classList.contains('hidden')).toBe(true)
      expect(f.classList.contains('target:grid')).toBe(true)
    }
    const add = el.querySelector('form[data-add-form]')!
    expect(add.classList.contains('hidden')).toBe(false)
    expect(add.classList.contains('target:grid')).toBe(false)
  })

  it('new: edit labels carry a gloss and keep the posted names', async () => {
    const p = fixture('golden')
    const el = await render(p)
    const r = p.rows.find((x) => x.editable.includes('buy_price') && x.editable.includes('report_date_override'))!
    const f = el.querySelector(`form[data-row="${r.key}"]`)!
    const label = (name: string) => f.querySelector(`label[for="${r.key}-${name}"]`)!.textContent
    expect(label('buy_price')).toContain('buy_price (price you would buy at)')
    expect(label('report_date_override')).toContain('(your own report date, YYYY-MM-DD)')
    expect(f.querySelector('input[name="buy_price"]')).not.toBeNull()
    expect(f.querySelector('input[name="report_date_override"]')).not.toBeNull()
  })

  it('new: the table has no switch and no Edit link without a form', async () => {
    const golden = await render(fixture('golden'))
    expect(golden.querySelector('[data-names] button[role=switch]')).toBeNull()
    await act(async () => root?.unmount())
    golden.remove()
    const el = await render(fixture('readonly'))
    expect(el.querySelector('a[href^="#edit-"]')).toBeNull()
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
    await render(p)
    const text = document.body.textContent ?? ''
    for (const c of p.confirm!.changes) for (const s of [c.label, c.old, c.new]) expect(text).toContain(s)
    for (const s of p.confirm!.stops) expect(text).toContain(s)
    expect(text).toContain(p.notice)
    expect(document.body.querySelectorAll('[data-stop]').length).toBe(p.confirm!.stops.length)
  })

  it('new: the confirm form posts the fields of the payload, the token and confirm=1', async () => {
    const p = fixture('confirm')
    await render(p)
    const f = document.body.querySelector('form[data-confirm-form]')!
    expect(f.closest('[data-confirm]')).not.toBeNull()
    expect(f.getAttribute('method')).toBe('post')
    expect(f.getAttribute('action')).toBe(p.form!.post_path)
    const pairs = [...f.querySelectorAll<HTMLInputElement>('input[name]')].map((i) => [i.name, i.value])
    expect(pairs).toEqual([
      ['expires', p.form!.expires],
      ['mac', p.form!.mac],
      ...p.confirm!.fields.map((x) => [x.name, x.value]),
      ['confirm', '1'],
    ])
    const b = f.querySelector('button[type=submit]')!
    expect(b.textContent).toBe('Confirm and save')
  })

  it('new: the confirm form keeps an unchecked switch off', async () => {
    const p = fixture('confirm')
    p.confirm!.fields = p.confirm!.fields.filter((x) => x.name !== 'options') as NonNullable<Payload['confirm']>['fields']
    await render(p)
    const f = document.body.querySelector('form[data-confirm-form]')!
    expect(f.querySelector('input[name=options]')).toBeNull()
  })

  it('new: the confirm card has no form without the page form', async () => {
    const p = { ...fixture('confirm'), form: null }
    await render(p)
    expect(document.body.querySelector('[data-confirm]')).not.toBeNull()
    expect(document.body.querySelector('form[data-confirm-form]')).toBeNull()
  })

  it('TestUnderlyingsConfirmDialogFlow', async () => {
    const p = fixture('confirm')
    await render(p)
    const dlg = document.body.querySelector<HTMLElement>('[role=dialog]')!
    expect(dlg).not.toBeNull()
    expect(dlg.getAttribute('data-state')).toBe('open')
    expect(dlg.contains(document.activeElement)).toBe(true)
    const f = dlg.querySelector('form[data-confirm-form]')!
    expect(f).not.toBeNull()
    const pairs = [...f.querySelectorAll<HTMLInputElement>('input[name]')].map((i) => [i.name, i.value])
    expect(pairs).toEqual([
      ['expires', p.form!.expires],
      ['mac', p.form!.mac],
      ...p.confirm!.fields.map((x) => [x.name, x.value]),
      ['confirm', '1'],
    ])
    expect(f.getAttribute('action')).toBe(p.form!.post_path)
    expect(f.querySelector('button[type=submit]')!.textContent).toBe('Confirm and save')
  })

  it('TestUnderlyingsConfirmDialogCancel', async () => {
    const p = fixture('confirm')
    const fetchSpy = vi.fn()
    vi.stubGlobal('fetch', fetchSpy)
    const submitSpy = vi.spyOn(HTMLFormElement.prototype, 'requestSubmit')
    await render(p)
    const cancel = [...document.body.querySelectorAll<HTMLButtonElement>('[role=dialog] button')].find(
      (b) => b.textContent === 'Cancel',
    )!
    expect(cancel.type).toBe('button')
    await act(async () => cancel.click())
    expect(document.body.querySelector('[role=dialog]')).toBeNull()
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(submitSpy).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain(p.title)
    submitSpy.mockRestore()
  })

  it('TestUnderlyingsConfirmDialogAbsent', async () => {
    await render(fixture('golden'))
    expect(document.body.querySelector('[role=dialog]')).toBeNull()
  })

  it('refuses the bad-version payload', () => {
    const bad: Payload = JSON.parse(
      readFileSync(path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.underlyings.v2.bad-version.json'), 'utf8'),
    )
    const r = validate(bad)
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.gap.reason).toBe('unsupported_version')
  })
})
