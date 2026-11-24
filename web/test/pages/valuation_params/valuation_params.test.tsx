// The valuation params page prints the document as given, with the owner forms only when `form` is set.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as module from '@/pages/valuation_params/ValuationParamsPage'
import type { Payload } from '@/types/valuation_params_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const base = (): Payload =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.valuation_params_page.v1.ok.json'),
      'utf8',
    ),
  )

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

describe('registration', () => {
  it('TestRegistrationsExported: the module exports the schema of its document', () => {
    expect(module.registration.schema).toBe('valuation_params_page')
  })
})

describe('header and glosses', () => {
  it('TestParamsHeaderLinks: both links, Parameters marked current', async () => {
    const el = await render(base())
    const links = [...el.querySelectorAll('header a')]
    expect(links.map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Table', '/valuation'],
      ['Parameters', '/valuation-params'],
    ])
    expect(links[0]?.getAttribute('aria-current')).toBeNull()
    expect(links[1]?.getAttribute('aria-current')).toBe('page')
    expect(links[1]?.className).not.toContain('underline')
  })

  it('TestParamsGlosses: the terms have a gloss', async () => {
    const el = await render(base())
    for (const g of [
      'HKCN: Hong Kong and China names',
      'AI switch: stored with the R version, changes no price yet',
      'Yield used: the 10-year yield the R was proposed from',
      'P from: lowest P of the row',
      'M: safety margin',
      'W: fair half width',
      'H: heavy discount',
      'S: trim premium',
    ])
      expect(el.textContent).toContain(g)
  })
})

describe('proposal and warnings', () => {
  it('TestProposalAndWarnings: the three R values and the warning sit next to the Update R form', async () => {
    const payload = base()
    const el = await render(payload)
    const form = el.querySelector('form[data-form="accept_r"]') as HTMLFormElement
    expect(form).not.toBeNull()
    const card = form.parentElement as HTMLElement
    expect(card.textContent).toContain('US 0.95, JP 1.10, HKCN 0.80')
    expect(card.textContent).toContain(payload.proposal?.step_warnings[0])
    expect(form.getAttribute('action')).toBe(payload.form?.post_path)
    expect(form.getAttribute('method')).toBe('post')
    const submit = form.querySelector('button[type="submit"]')
    expect(submit?.textContent).toBe('Update R')
    expect((form.querySelector('input[name="action"]') as HTMLInputElement).value).toBe('accept_r')
    expect((form.querySelector('input[name="yield"]') as HTMLInputElement).value).toBe('4.20')
    expect((form.querySelector('input[name="mac"]') as HTMLInputElement).value).toBe(payload.form?.mac)
  })

  it('prints the tables, the history and the gaps and the error as given', async () => {
    const payload = base()
    payload.error = 'line 2: not ascending'
    payload.gaps = [{ kind: 'gap', reason: 'no_r_table', text: 'No R table yet.' }]
    const el = await render(payload)
    expect(el.textContent).toContain('line 2: not ascending')
    expect(el.textContent).toContain('No R table yet.')
    expect(el.querySelectorAll('table').length).toBeGreaterThanOrEqual(3)
    expect(el.textContent).toContain('until the invented chip cycle turns')
  })
})

describe('forms', () => {
  it('TestFormsHiddenWithoutOwnerEdit: no form without `form`, none for the proposal without `proposal`', async () => {
    const noForm = base()
    noForm.form = null
    const el = await render(noForm)
    expect(el.querySelector('form')).toBeNull()
    expect(el.textContent).toContain('0.95')
    await act(async () => root?.unmount())
    el.remove()
    const noProposal = base()
    noProposal.proposal = null
    const el2 = await render(noProposal)
    expect(el2.querySelector('form[data-form="accept_r"]')).toBeNull()
    expect(el2.querySelector('form[data-form="r_table"]')).not.toBeNull()
  })

  it('has the add-a-version forms of the two tables with one text area each and the R version form', async () => {
    const el = await render(base())
    for (const action of ['r_table', 'margin_table']) {
      const form = el.querySelector(`form[data-form="${action}"]`) as HTMLFormElement
      expect(form.querySelectorAll('textarea')).toHaveLength(1)
      expect(form.querySelector('textarea')?.getAttribute('name')).toBe('table')
      expect(form.querySelector('input[name="effective_date"]')).not.toBeNull()
    }
    const rv = el.querySelector('form[data-form="r_version"]') as HTMLFormElement
    for (const name of ['r_us', 'r_jp', 'r_hkcn', 'yield_used', 'ai_expiry', 'note', 'effective_date'])
      expect(rv.querySelector(`[name="${name}"]`)).not.toBeNull()
    expect(rv.querySelector('select[name="ai_switch"], input[name="ai_switch"]')).not.toBeNull()
  })

  it('the two table forms carry the Chinese hints', async () => {
    const el = await render(base())
    expect(el.querySelector('form[data-form="r_table"]')?.textContent).toContain(
      '\u6bcf\u884c\u4e00\u6863\uff1a\u6536\u76ca\u7387\u4e0a\u9650\u3001US\u3001JP\u3001HKCN',
    )
    expect(el.querySelector('form[data-form="margin_table"]')?.textContent).toContain(
      '\u6bcf\u884c\u4e00\u6863\uff1aP \u8d77\u70b9\u3001M\u3001W\u3001H\u3001S',
    )
  })
})

describe('TestNoLogicInWeb', () => {
  it('exports only the registration and the component of the page, no function on a document value', () => {
    expect(Object.keys(module).sort()).toEqual(['ValuationParamsPage', 'registration'])
  })
})
