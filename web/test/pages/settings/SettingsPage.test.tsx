// The fixtures are written from the schema alone; each text asserted is read from a fixture field.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SettingsPage, registration } from '@/pages/settings/SettingsPage'
import { pages } from '@/shell/pages'
import { confirmStep } from './fixtures/confirm'
import { editable } from './fixtures/editable'
import { readOnly } from './fixtures/read-only'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
async function show(payload: typeof editable) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<SettingsPage payload={payload as never} />)
  })
  return host
}
beforeEach(() => {
  // Radix Switch measures its hidden checkbox with a ResizeObserver, which jsdom lacks.
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const hidden = (scope: ParentNode, name: string) =>
  [...scope.querySelectorAll<HTMLInputElement>(`input[type=hidden][name="${name}"]`)].map((i) => i.value)

describe('SettingsPage', () => {
  it('is a post form to /owner-settings whose hidden token equals payload.form', async () => {
    const page = await show(editable)
    const form = page.querySelector<HTMLFormElement>('form[data-edit-form]')!
    expect(form.getAttribute('method')).toBe('post')
    expect(form.getAttribute('action')).toBe('/owner-settings')
    expect(hidden(form, 'expires')).toEqual([editable.form!.expires])
    expect(hidden(form, 'mac')).toEqual([editable.form!.mac])
    expect(page.querySelector('script')).toBeNull()
  })

  it('has one param.<key> input per params row with the default as placeholder', async () => {
    const page = await show(editable)
    const inputs = [...page.querySelectorAll<HTMLInputElement>('input:not([type=hidden])[name^="param."]')]
    expect(inputs.map((i) => i.name)).toEqual(editable.params.map((p) => `param.${p.key}`))
    inputs.forEach((input, i) => {
      expect(input.placeholder).toBe(editable.params[i]!.default)
      expect(input.value).toBe(editable.params[i]!.value)
    })
    expect(inputs.map((i) => i.inputMode)).toEqual(['numeric', 'decimal'])
    const current = [...page.querySelectorAll('[data-param] [data-current]')].map((c) => c.textContent)
    expect(current).toEqual(['25', '0.10'])
    expect(page.textContent).toContain(editable.params[0]!.label)
    expect(page.querySelector('[data-bounds]')!.textContent).toBe('7 – 120')
  })

  it('has one switch per rows entry, the ids as row inputs, and the flag as checked state', async () => {
    const page = await show(editable)
    const switches = [...page.querySelectorAll<HTMLElement>('[role=switch]')]
    expect(switches).toHaveLength(editable.rows.length)
    expect(switches.map((s) => s.getAttribute('aria-checked'))).toEqual(['true', 'false', 'false'])
    expect(hidden(page.querySelector('form[data-edit-form]')!, 'row')).toEqual(editable.rows.map((r) => String(r.instrument_id)))
    const boxes = [...page.querySelectorAll<HTMLInputElement>('input[type=checkbox][name=excluded]')]
    expect(boxes.map((b) => b.value)).toEqual(editable.rows.map((r) => String(r.instrument_id)))
  })

  it('shows the notice as an alert and the history with one revert form per row', async () => {
    const page = await show(editable)
    expect(page.querySelector('[data-notice=saved]')!.textContent).toContain(editable.notice!.text)
    const reverts = [...page.querySelectorAll<HTMLFormElement>('form[data-revert]')]
    expect(reverts).toHaveLength(editable.history.length)
    expect(hidden(reverts[0]!, 'param.min_dte')).toEqual(['30'])
    expect(hidden(reverts[0]!, 'mac')).toEqual([editable.form!.mac])
    expect(hidden(reverts[1]!, 'row')).toEqual(['9001'])
    expect(hidden(reverts[1]!, 'excluded')).toEqual(['9001'])
  })

  it('disables every control and shows the gap text when form is null', async () => {
    const page = await show(readOnly)
    const controls = [...page.querySelectorAll<HTMLElement>('input:not([type=hidden]), button, [role=switch]')]
    expect(controls.length).toBeGreaterThan(readOnly.params.length + readOnly.rows.length)
    controls.forEach((c) => expect(c.hasAttribute('disabled')).toBe(true))
    expect(hidden(page, 'expires')).toEqual([])
    expect(hidden(page, 'mac')).toEqual([])
    expect(page.querySelector('[data-gap]')!.textContent).toContain(readOnly.gap!.text)
    expect(page.querySelector('[data-notice]')).toBeNull()
  })

  it('shows the confirm table and a second form with confirm=1 and the token of confirm.form', async () => {
    const page = await show(confirmStep)
    const rows = [...page.querySelectorAll('[data-confirm-row]')].map((r) => [...r.children].map((c) => c.textContent))
    expect(rows).toEqual(confirmStep.confirm!.rows.map((r) => [r.label, r.old, r.new]))
    const form = page.querySelector<HTMLFormElement>('form[data-confirm-form]')!
    expect(form.getAttribute('method')).toBe('post')
    expect(hidden(form, 'confirm')).toEqual(['1'])
    expect(hidden(form, 'expires')).toEqual([confirmStep.confirm!.form.expires])
    expect(hidden(form, 'mac')).toEqual([confirmStep.confirm!.form.mac])
    expect(hidden(form, 'param.min_dte')).toEqual(['25'])
    expect(hidden(form, 'row')).toEqual(['9001', '9002', '9003'])
    expect(hidden(form, 'excluded')).toEqual(['9001'])
    expect(page.querySelector('[data-cancel]')!.getAttribute('href')).toBe('/settings')
  })

  it('is registered under settings_page and found by the shell', () => {
    expect(registration.schema).toBe('settings_page')
    expect(pages.settings_page).toBe(registration.page)
  })
})
