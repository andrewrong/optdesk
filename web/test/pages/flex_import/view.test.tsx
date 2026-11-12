// The flex_import view shows the strings of the payload as sent. Expected strings are read from the contract fixtures.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { registration } from '@/pages/flex_import/view'
import { validate, type Payload } from '@/types/flex_import'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.flex_import.v1.${name}.json`), 'utf8'),
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

describe('flex_import view', () => {
  it('is registered as flex_import', () => {
    expect(registration.schema).toBe('flex_import')
  })

  it('shows every string of the result payload as sent', async () => {
    const p = fixture('result')
    const el = await render(p)
    const t = el.textContent ?? ''
    for (const s of [p.title, p.sentence, p.scope_note]) expect(t).toContain(s)
    for (const f of p.files) {
      expect(t).toContain(f.name)
      expect(t).toContain(f.status_text)
      if (f.reason) expect(t).toContain(f.reason)
      for (const a of f.aliases) expect(t).toContain(a.alias)
    }
    for (const r of p.recent) {
      expect(t).toContain(r.at_text)
      expect(t).toContain(r.status)
    }
    const unmapped = p.files.find((f) => f.unmapped_statements > 0)
    expect(unmapped).toBeDefined()
    expect(el.querySelector('[data-unmapped]')?.textContent).toBe(String(unmapped?.unmapped_statements))
  })

  it('shows the upload form with its token, multiple files and the limit text', async () => {
    const p = fixture('result')
    const el = await render(p)
    const form = el.querySelector('form') as HTMLFormElement
    expect(form.method).toBe('post')
    expect(form.enctype).toBe('multipart/form-data')
    expect(form.getAttribute('action')).toBe(p.form?.post_path)
    expect((form.querySelector('input[name=expires]') as HTMLInputElement).value).toBe(p.form?.expires)
    expect((form.querySelector('input[name=mac]') as HTMLInputElement).value).toBe(p.form?.mac)
    const file = form.querySelector('input[type=file]') as HTMLInputElement
    expect(file.multiple).toBe(true)
    expect(file.accept).toBe('.xml,.gz')
    expect(el.textContent).toContain(p.form?.max_file_text)
  })

  it('shows a failed file and a nothing_found file with their own status text and reason', async () => {
    const p = fixture('result')
    const el = await render(p)
    for (const status of ['failed', 'nothing_found'] as const) {
      const f = p.files.find((x) => x.status === status)
      expect(f).toBeDefined()
      const block = el.querySelector(`[data-file="${f?.name}"]`)
      expect(block?.textContent).toContain(f?.status_text)
      if (f?.reason) expect(block?.textContent).toContain(f.reason)
    }
  })

  it('renders no form for the disabled payload', async () => {
    const p = fixture('disabled')
    const el = await render(p)
    expect(el.querySelector('form')).toBeNull()
    expect(el.textContent).toContain(p.title)
    for (const g of p.gaps) expect(el.textContent).toContain(g.text)
  })

  it('shows the golden payload gaps and recent table', async () => {
    const p = fixture('golden')
    const el = await render(p)
    for (const g of p.gaps) expect(el.textContent).toContain(g.text)
    for (const r of p.recent) expect(el.textContent).toContain(r.at_text)
  })

  it('refuses bad-version with unsupported_version', () => {
    const r = validate(fixture('bad-version'))
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.gap.reason).toBe('unsupported_version')
  })
})
