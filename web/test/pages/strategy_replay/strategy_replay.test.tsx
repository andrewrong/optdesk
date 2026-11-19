// The strategy replay page prints the document as given: the form fields by name, the report as the server sent it.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as module from '@/pages/strategy_replay/StrategyReplayPage'
import type { Payload } from '@/types/strategy_replay_page'
import { empty } from './fixtures/empty'
import { form } from './fixtures/form'
import { off } from './fixtures/off'
import { report } from './fixtures/report'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
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
})

const text = (el: Element | undefined) => el?.textContent ?? ''

describe('registration', () => {
  it('exports the schema of its document', () => {
    expect(module.registration.schema).toBe('strategy_replay_page')
  })
})

describe('strategy replay page, form', () => {
  it('posts a native form to post_path with one control per field name', async () => {
    const p = form()
    const el = await render(p)
    const f = el.querySelector('form') as HTMLFormElement
    expect(f.getAttribute('method')).toBe('post')
    expect(f.getAttribute('action')).toBe(p.post_path)
    const names = [...f.querySelectorAll('input[name], select[name]')].map((e) => e.getAttribute('name'))
    expect(names).toEqual(p.form?.fields.map((x) => x.name))
  })

  it('shows labels, hints, values, options, the submit label and the restart hint of the document', async () => {
    const p = form()
    const el = await render(p)
    for (const fld of p.form?.fields ?? []) {
      const c = el.querySelector(`[name="${fld.name}"]`) as HTMLInputElement | HTMLSelectElement
      expect(c.value).toBe(fld.value)
      expect(text(el)).toContain(fld.label)
      expect(text(el)).toContain(fld.hint)
      if (fld.kind === 'choice') {
        expect(c.tagName).toBe('SELECT')
        expect([...(c as HTMLSelectElement).options].map((o) => [o.value, o.textContent])).toEqual(
          fld.options.map((o) => [o.value, o.label]),
        )
      } else {
        expect(c.tagName).toBe('INPUT')
      }
    }
    expect(el.querySelector('button[type="submit"]')?.textContent).toBe(p.form?.submit_label)
    expect(text(el)).toContain(p.restart_hint)
    expect(text(el)).toContain(p.title)
    expect(text(el)).toContain(p.intro)
    expect(el.querySelector('table')).toBeNull()
  })
})

describe('strategy replay page, report', () => {
  it('shows counts, excluded rows, one table row per line with its lot count, and the notes', async () => {
    const p = report()
    const r = p.report!
    const el = await render(p)
    const t = text(el)
    for (const v of Object.values(r.counts)) expect(t).toContain(String(v))
    for (const x of r.excluded) {
      expect(t).toContain(x.text)
      expect(t).toContain(String(x.lots))
    }
    for (const n of r.notes) expect(t).toContain(n)
    const rows = [...el.querySelectorAll('tbody tr')]
    expect(rows).toHaveLength(r.lines.length)
    r.lines.forEach((l, i) => {
      const cells = [...(rows[i]?.querySelectorAll('td') ?? [])].map(text)
      expect(cells).toContain(String(l.lots))
      expect(cells).toContain(l.kept_share ?? 'n/a')
      expect(cells).toContain(l.mean_gross ?? 'n/a')
      expect(cells).toContain(l.worst_gross ?? 'n/a')
      expect(cells).toContain(l.actual_kept_share ?? 'n/a')
      expect(text(rows[i])).toContain(l.minimum_text ?? '')
    })
    expect(el.querySelector('form')).not.toBeNull()
  })

  it('shows n/a for a null figure and the minimum text only on a thin line', async () => {
    const p = report()
    const el = await render(p)
    const rows = [...el.querySelectorAll('tbody tr')]
    expect(text(rows[2])).toContain('n/a')
    expect(text(rows[2])).toContain('不足 20 笔 [T]：无结论')
    expect(text(rows[0])).not.toContain('n/a')
    expect(text(rows[0])).not.toContain('不足')
  })
})

describe('strategy replay page, report hints', () => {
  it('strategy replay page, report hints', async () => {
    const p = report()
    const r = p.report!
    const el = await render(p)
    const t = text(el)
    for (const v of Object.values(r.hints!.counts)) expect(t).toContain(v)
    for (const v of Object.values(r.hints!.columns)) expect(t).toContain(v)
    for (const v of r.variants) expect(t).toContain(v.hint as string)
    expect(el.querySelectorAll('thead th span.text-muted-foreground').length).toBe(15)
  })

  it('shows the table rows and counts when the payload has no hints', async () => {
    const p = report()
    const r = p.report!
    delete r.hints
    for (const v of r.variants) delete v.hint
    const el = await render(p)
    expect(el.querySelectorAll('tbody tr')).toHaveLength(r.lines.length)
    expect(text(el)).toContain(`lots ${r.counts.lots}`)
    expect(el.querySelectorAll('thead th span.text-muted-foreground')).toHaveLength(0)
  })
})

describe('strategy replay page, zero lots', () => {
  it('shows empty_text and no table', async () => {
    const p = empty()
    const el = await render(p)
    expect(text(el)).toContain(p.report?.empty_text as string)
    expect(el.querySelector('tbody tr')).toBeNull()
  })
})

describe('strategy replay page, form off', () => {
  it('shows only the notices: no form, no report', async () => {
    const p = off()
    const el = await render(p)
    expect(el.querySelector('form')).toBeNull()
    expect(el.querySelector('table')).toBeNull()
    for (const n of p.notices) expect(text(el)).toContain(n.text)
  })

  it('shows a post notice above the form', async () => {
    const p = form()
    p.notices = [{ key: 'timed_out', text: '回放超时' }]
    const el = await render(p)
    expect(text(el)).toContain('回放超时')
    expect(el.querySelector('form')).not.toBeNull()
  })
})
