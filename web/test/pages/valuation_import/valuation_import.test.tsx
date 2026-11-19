// The valuation import page prints the document as given and posts the file twice: preview, then confirm.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as module from '@/pages/valuation_import/ValuationImportPage'
import type { Payload } from '@/types/valuation_import_page'
import { empty, preview } from './fixtures/preview'
import { result } from './fixtures/result'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

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
  const View = module.registration.page.component as unknown as (p: { payload: Payload }) => React.ReactElement
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
  root = null
  host?.remove()
  vi.unstubAllGlobals()
})

const button = (el: HTMLElement, name: string) =>
  [...el.querySelectorAll('button')].find((b) => b.textContent?.startsWith(`${name} (`) || b.textContent === name) as HTMLButtonElement
const box = (el: HTMLElement, label: string) => el.querySelector(`[role="checkbox"][aria-label="${label}"]`) as HTMLElement
const click = (n: Element) => act(async () => n.dispatchEvent(new MouseEvent('click', { bubbles: true })))

async function type(input: HTMLInputElement, value: string) {
  const set = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
  await act(async () => {
    set?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

describe('registration', () => {
  it('exports the schema of its document', () => {
    expect(module.registration.schema).toBe('valuation_import_page')
  })
})

describe('valuation import page', () => {
  it('labels the R line with the English name of R', async () => {
    const el = await render(preview())
    expect(el.textContent).toContain('R (discount multiple)')
    expect(el.textContent).not.toContain('R (R)')
  })

  it('renders the empty page with a file field', async () => {
    const el = await render(empty())
    expect(el.querySelector('input[type="file"]')).not.toBeNull()
    expect(button(el, 'Preview')).toBeDefined()
    expect(el.querySelector('table')).toBeNull()
  })

  it('renders the empty state texts from the labels', async () => {
    const p = empty()
    const el = await render(p)
    expect(el.querySelector('h1')?.textContent).toBe(p.labels?.title)
    expect(el.querySelector('header p')?.textContent).toBe(p.labels?.intro)
    expect(el.querySelector('label[for="valuation-file"]')?.textContent).toBe(p.labels?.file)
    expect(el.querySelector('form[data-form="preview"] button')?.textContent).toBe(p.labels?.preview)
  })

  it('renders the count words, same file again and confirm from the labels', async () => {
    const p = preview()
    const el = await render(p)
    const l = p.labels
    expect(el.textContent).toContain(`${l?.count_insert} ${p.counts.insert}`)
    expect(el.textContent).toContain(`${l?.count_needs_tick} ${p.counts.needs_tick}`)
    expect(el.querySelector('label[for="valuation-file-confirm"]')?.textContent).toBe(l?.same_file_again)
    expect(el.querySelector('form[data-form="confirm"] button[type="submit"]')?.textContent).toBe(l?.confirm)
  })

  it('renders the remaining labels', async () => {
    const p = preview()
    p.labels = { ...p.labels, removed: 'LBL-removed', margin_table: 'LBL-margin', stored: 'LBL-stored', count_refused: 'LBL-refused' } as Payload['labels']
    p.removed = ['XYZ']
    const el = await render(p)
    for (const t of ['LBL-removed', 'LBL-margin', 'LBL-stored', 'LBL-refused']) expect(el.textContent).toContain(t)
  })

  it('prints the count, older, not-in-file, unvalued and result labels from the payload', async () => {
    const p = preview()
    p.labels = {
      ...p.labels,
      count_same: 'LBL-same',
      count_older: 'LBL-older',
      older_than_stored: 'LBL-older-than-stored',
      not_in_file: 'LBL-notinfile',
      unvalued: 'LBL-unvalued',
      result: 'LBL-result',
    } as Payload['labels']
    p.not_in_file = ['AAA']
    p.unvalued = ['BBB']
    if (p.r) p.r.action = 'older'
    if (p.margin) p.margin.action = 'older'
    const el = await render(p)
    for (const t of ['LBL-same', 'LBL-older', 'LBL-notinfile', 'LBL-unvalued']) expect(el.textContent).toContain(t)
    expect(el.textContent?.split('LBL-older-than-stored').length).toBe(3)
    const r = result()
    r.labels = { ...r.labels, result: 'LBL-result' } as Payload['labels']
    const el2 = await render(r)
    expect(el2.querySelector('[data-outcome]')?.closest('.space-y-1')?.parentElement?.textContent).toContain('LBL-result')
  })

  it('prints the label texts of the payload', async () => {
    const p = preview()
    p.labels = {
      r: 'invented r label',
      buy_zone_top_site: 'invented site head',
      buy_zone_top_file: 'invented file head',
      general_tick: 'invented general tick',
    }
    const el = await render(p)
    for (const t of ['invented r label', 'invented site head', 'invented file head', 'invented general tick']) {
      expect(el.textContent).toContain(t)
    }
  })

  it('renders one row per company with both buy-zone tops', async () => {
    const p = preview()
    const el = await render(p)
    const rows = [...el.querySelectorAll('tbody tr[data-ticker]')]
    expect(rows.map((r) => r.getAttribute('data-ticker'))).toEqual(p.rows.map((r) => r.ticker))
    const first = rows[0]?.textContent ?? ''
    expect(first).toContain('96.00')
    expect(first).toContain('96.10')
    expect(first).toContain('insert')
    expect(el.textContent).toContain('older')
  })

  it('renders an unmatched row with an editable key', async () => {
    const el = await render(preview())
    const key = el.querySelector('input[name="key.QQB"]') as HTMLInputElement
    expect(key.value).toBe('HK:0999')
    await type(key, 'HK:1234')
    expect((el.querySelector('input[name="key.QQB"]') as HTMLInputElement).value).toBe('HK:1234')
    expect(el.querySelector('input[name="key.ZZA"]')).toBeNull()
    expect(box(el, 'tick QQB')).not.toBeNull()
    expect(box(el, 'tick ZZA')).toBeNull()
  })

  it('disables confirm until the general tick is set', async () => {
    const el = await render(preview())
    expect(button(el, 'Confirm').disabled).toBe(true)
    await click(box(el, 'general'))
    expect(button(el, 'Confirm').disabled).toBe(false)
  })

  it('disables confirm when no row has the action insert', async () => {
    const p = preview()
    p.rows = p.rows.filter((r) => r.action !== 'insert')
    const el = await render(p)
    await click(box(el, 'general'))
    expect(button(el, 'Confirm').disabled).toBe(true)
  })

  it('the confirm form carries the sha256, the ticked tickers and the picked keys of the preview payload as named fields and posts multipart to post_path', async () => {
    const p = preview()
    const el = await render(p)
    const pre = el.querySelector('form[data-form="preview"]') as HTMLFormElement
    expect(pre.getAttribute('method')).toBe('post')
    expect(pre.getAttribute('action')).toBe(p.post_path)
    expect(pre.enctype).toBe('multipart/form-data')
    expect(new FormData(pre).get('action')).toBe('preview')

    await type(el.querySelector('input[name="key.QQB"]') as HTMLInputElement, 'HK:1234')
    await click(box(el, 'tick QQB'))
    await click(box(el, 'general'))
    const form = el.querySelector('form[data-form="confirm"]') as HTMLFormElement
    expect(form.getAttribute('method')).toBe('post')
    expect(form.getAttribute('action')).toBe(p.post_path)
    expect(form.enctype).toBe('multipart/form-data')
    const data = new FormData(form)
    expect(data.get('action')).toBe('confirm')
    expect(data.get('file_sha256')).toBe(p.file?.sha256)
    expect(data.getAll('tick')).toEqual(['QQB'])
    expect(data.get('key.QQB')).toBe('HK:1234')
    expect(data.get('r_market.QQB')).toBe('HKCN')
    expect(data.get('key.ZZA')).toBeNull()
    const names = [...form.querySelectorAll('input[name]')].map((i) => i.getAttribute('name'))
    expect(names[names.length - 1]).toBe('file')
    expect(form.querySelector('input[type="file"]')).not.toBeNull()
  })

  it('lets the owner change the R market of an unmatched row', async () => {
    const el = await render(preview())
    const rm = el.querySelector('[name="r_market.QQB"]') as HTMLInputElement
    expect(rm.type).not.toBe('hidden')
    await type(rm, 'US')
    const form = el.querySelector('form[data-form="confirm"]') as HTMLFormElement
    expect(new FormData(form).get('r_market.QQB')).toBe('US')
  })

  it('renders the result outcomes', async () => {
    const r = result()
    const el = await render(r)
    const items = [...el.querySelectorAll('[data-outcome]')]
    expect(items.map((i) => [i.getAttribute('data-key'), i.getAttribute('data-outcome')])).toEqual(
      r.result.map((x) => [x.key, x.outcome]),
    )
    expect(el.textContent).toContain('not confirmed')
    expect(button(el, 'Confirm')).toBeUndefined()
    expect(el.querySelector('form')).toBeNull()
  })
})
