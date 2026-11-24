// The valuation pages print the document as given. Every number is a made-up fixture number.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act, Children, type ReactElement, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { GLOSS } from '@/pages/valuation/CompanyView'
import * as module from '@/pages/valuation/ValuationPage'
import type { Payload } from '@/types/valuation_page'

// ComposedChart of recharts is replaced by a stub that records its children and renders nothing.
const chartChildren = vi.hoisted(() => [] as unknown[])
vi.mock('recharts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('recharts')>()),
  ComposedChart: ({ children }: { children: unknown }) => {
    chartChildren.push(children)
    return null
  },
}))

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.valuation_page.v1.${name}.json`),
      'utf8',
    ),
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
  vi.useRealTimers()
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = null
  host?.remove()
})

describe('registration', () => {
  it('TestRegistrationsExported: the module exports the schema of its document', () => {
    expect(module.registration.schema).toBe('valuation_page')
  })
})

describe('table view', () => {
  it('TestUnvaluedRowHasEntryLink: one row per company, an entry button on an unvalued row only', async () => {
    const el = await render(fixture('table'))
    const rows = el.querySelectorAll('tbody tr')
    expect(rows).toHaveLength(3)
    const entry = (key: string) => el.querySelector(`tr[data-company="${key}"] td:first-child a[href^="/valuation/form"]`)
    expect(entry('US:AAA')).toBeNull()
    expect(entry('HK:0001')?.getAttribute('href')).toBe('/valuation/form?key=HK%3A0001')
    expect(entry('US:CCC')?.textContent).toContain('Enter')
    expect(entry('US:CCC')?.querySelector('svg')).not.toBeNull()
    expect(el.textContent).toContain('not entered')
  })

  it('TestEnterButtonInNameCell: the Enter link sits in the name cell and there is no entry column', async () => {
    const el = await render(fixture('table'))
    const heads = [...el.querySelectorAll('thead th')]
    expect(heads).toHaveLength(16)
    expect(heads.map((h) => h.textContent)).not.toContain('')
    const cell = el.querySelector('tr[data-company="HK:0001"] td') as HTMLElement
    expect(cell.querySelector('a[href="/valuation/form?key=HK%3A0001"]')?.textContent).toContain('Enter')
  })

  it('TestFigureCellsNoWrap: figure cells stay on one line, word cells wrap', async () => {
    const el = await render(fixture('table'))
    const heads = [...el.querySelectorAll('thead th')].map((h) => h.textContent ?? '')
    const cells = [...el.querySelectorAll('tr[data-company="US:AAA"] td')]
    const at = (head: string) => cells[heads.findIndex((h) => h.startsWith(head))]
    expect(at('V')?.className).not.toContain('whitespace-normal')
    expect(at('Buy zone')?.className).not.toContain('whitespace-normal')
    expect(at('Status')?.className).toContain('whitespace-normal')
    expect(at('Zone')?.className).toContain('whitespace-normal')
  })

  it('TestGlossesPresent: the heads of the table carry a gloss', async () => {
    const el = await render(fixture('table'))
    const thead = el.querySelector('thead') as HTMLElement
    const labels = [...thead.querySelectorAll('[aria-label]')].map((e) => e.getAttribute('aria-label'))
    expect(labels).toContain('fair price, V x R')
    expect(labels).toContain('predictability, 1 to 5')
    expect(thead.textContent).not.toContain('C: fair price')
  })

  it('TestHeadsHaveNoGlossLine: no head prints a gloss line, each glossed head holds one Info trigger', async () => {
    const el = await render(fixture('table'))
    expect(el.querySelector('thead .max-w-40')).toBeNull()
    const heads = [...el.querySelectorAll('thead th')]
    const v = heads.find((h) => h.textContent === 'V')
    expect(v?.textContent).toBe('V')
    expect(v?.querySelectorAll('[aria-label]')).toHaveLength(1)
    expect(heads.find((h) => h.textContent === 'Name')?.querySelectorAll('[aria-label]')).toHaveLength(0)
    for (const term of ['V', 'P', 'Version date', 'R', 'C', 'Heavy', 'Trim', 'Clear', 'Core']) {
      expect(GLOSS[term]).toBeTruthy()
      const th = heads.find((h) => h.textContent === term)
      expect(th?.querySelectorAll('[aria-label]')).toHaveLength(1)
    }
    for (const h of heads) expect(h.querySelectorAll('[aria-label]').length).toBeLessThanOrEqual(1)
  })

  it('TestHandPriceColumn: the hand-typed buy price column sits right after the buy zone', async () => {
    const el = await render(fixture('table'))
    const heads = [...el.querySelectorAll('thead th')].map((h) => h.textContent)
    const at = heads.indexOf('Buy zone')
    expect(heads[at + 1]).toBe('Hand-typed buy price')
    const cells = [...el.querySelectorAll('tr[data-company="US:AAA"] td')].map((c) => c.textContent)
    expect(cells[at]).toBe('67 - 72')
    expect(cells[at + 1]).toBe('70')
    const gamma = [...el.querySelectorAll('tr[data-company="US:CCC"] td')].map((c) => c.textContent)
    expect(gamma[at + 1]).toBe('40')
  })
})

describe('company view', () => {
  it('TestEveryTrueFlagPrinted: trim and clear both true print both, and the zone', async () => {
    const payload = fixture('company')
    if (payload.lines?.flags) {
      payload.lines.flags.at_trim = true
      payload.lines.flags.at_clear = true
    }
    const el = await render(payload)
    const lines = el.querySelector('[data-lines]') as HTMLElement
    expect(lines.querySelector('[data-flag="at_trim"]')?.textContent).toBe('At trim line')
    expect(lines.querySelector('[data-flag="at_clear"]')?.textContent).toBe('At clear line')
    expect(lines.querySelector('[data-flag="in_fair"]')).not.toBeNull()
    expect(lines.querySelector('[data-flag="at_core"]')).toBeNull()
    expect(lines.querySelector('[data-zone="fair"]')).not.toBeNull()
  })

  it('TestProfileLabels: the profile line prints labels, not codes', async () => {
    const el = await render(fixture('company'))
    const line = el.querySelector('[data-profile]') as HTMLElement
    expect(line.textContent).not.toContain('in_pool')
    expect(line.textContent).not.toContain('R market ')
    expect(line.textContent).toBe('AAA, US dollar, R market: United States, pool: in pool, 2026-10-01')
    const payload = fixture('company')
    if (payload.company) payload.company.currency = 'XYZ'
    await act(async () => root?.unmount())
    host?.remove()
    const el2 = await render(payload)
    expect(el2.querySelector('[data-profile]')?.textContent).toContain('AAA, XYZ,')
  })

  it('TestProfileNoTrailingComma: an empty pool date leaves its part out', async () => {
    const payload = fixture('company')
    if (payload.company) payload.company.pool_date = ''
    const el = await render(payload)
    const text = el.querySelector('[data-profile]')?.textContent ?? ''
    expect(text).toBe('AAA, US dollar, R market: United States, pool: in pool')
    expect(text.endsWith(',')).toBe(false)
  })

  it('TestGlossesPresent: the lines view has the glosses of its terms', async () => {
    const el = await render(fixture('company'))
    const lines = el.querySelector('[data-lines]') as HTMLElement
    for (const g of [
      'C: fair price, V x R',
      'R: the multiple of the valuation that sets the fair price (C = V x R)',
      'Heavy: the heavy point, C x (1 - H)',
      'Trim: C x (1 + S)',
      'Clear: V x 1.2',
      'Core: V x 1.5',
      'M: safety margin',
      'W: fair half width',
      'H: heavy discount',
      'S: trim premium',
    ])
      expect(lines.textContent).toContain(g)
  })

  it('TestHKCNNoZoneText: an empty zone prints the zone note and the prices', async () => {
    const payload = fixture('hkcn')
    if (payload.lines) payload.lines.zone_note = 'no_zone_market'
    const el = await render(payload)
    const lines = el.querySelector('[data-lines]') as HTMLElement
    expect(lines.textContent).toContain('This market has no price source, so there is no zone.')
    expect(lines.querySelector('[data-zone]')).toBeNull()
    for (const price of ['56', '59 - 64', '72 - 88', '100', '120', '150']) expect(lines.textContent).toContain(price)
  })

  it('draws the marks of the band as given', async () => {
    const el = await render(fixture('company'))
    const band = el.querySelector('[data-chart="price-band"]') as HTMLElement
    expect(band.textContent).toContain('close 95')
    expect(band.textContent).toContain('core 150')
  })

  it('TestPriceBandMarksAreSeriesData: the legend lists every mark and the only data points are the marks', async () => {
    chartChildren.length = 0
    const payload = fixture('company')
    const el = await render(payload)
    const marks = payload.band!.marks
    const legend = [...el.querySelectorAll('[data-chart="price-band"] li')].map((li) => li.textContent)
    expect(legend).toEqual(marks.map((m) => `${m.name} ${m.value}`))
    const elements = chartChildren.flatMap((c) => Children.toArray(c as ReactNode)) as ReactElement<{ data?: unknown }>[]
    const series = elements.filter((e) => Array.isArray(e.props.data))
    expect(series).toHaveLength(1)
    expect(series[0]!.props.data).toEqual(marks.map((m) => ({ pos: Number(m.pos) })))
    expect(chartChildren.length).toBeGreaterThan(0)
  })

  it('TestPriceBandWithoutMarksIsMissing: a band with no marks draws no chart and says it is missing', async () => {
    chartChildren.length = 0
    const payload = fixture('company')
    payload.band!.marks = []
    const el = await render(payload)
    const band = el.querySelector('[data-chart="price-band"]') as HTMLElement
    expect(band.hasAttribute('data-missing')).toBe(true)
    expect(band.textContent).toContain('The price band is missing: the payload holds no marks for it.')
    expect(chartChildren).toHaveLength(0)
    expect(band.querySelector('li')).toBeNull()
  })

  it('TestHistoryColumnsAndExpand: a row prints its columns and expands to detail and report text', async () => {
    const payload = fixture('company')
    const el = await render(payload)
    const row = el.querySelector('tr[data-review="rev-0003"]') as HTMLElement
    const cells = [...row.querySelectorAll('td')].map((c) => c.textContent)
    expect(cells.slice(0, 9)).toEqual(['2026-11-08', '100', '130', '30', 'event', 'b', 'invented typo', 'void', 'V typed wrong'])
    const history = row.closest('table') as HTMLElement
    expect(history.textContent).not.toContain('invented reaction')
    await act(async () => {
      ;(row.querySelector('button') as HTMLButtonElement).click()
    })
    expect(history.textContent).toContain('invented reaction')
    expect(history.textContent).toContain('Invented report text.')
    expect(history.textContent).toContain('invented risk')
  })

  it('TestDetailTimesShownAsDateAndMinute: entered_at prints as date and minute, not the raw RFC 3339 string', async () => {
    const payload = fixture('company')
    const row0 = (payload.history as NonNullable<Payload['history']>)[0]!
    row0.detail.entered_at = '2026-11-19T06:15:16Z'
    const el = await render(payload)
    const row = el.querySelector(`tr[data-review="${row0.review_id}"]`) as HTMLElement
    await act(async () => {
      ;(row.querySelector('button') as HTMLButtonElement).click()
    })
    const text = (row.closest('table') as HTMLElement).textContent
    expect(text).toContain('2026-11-19 06:15')
    expect(text).not.toContain('T06:15:16Z')
  })
})

describe('form view', () => {
  it('TestFourFieldsRequired: the four fields come first, marked required, the optional groups are collapsed', async () => {
    const payload = fixture('form')
    const el = await render(payload)
    const form = el.querySelector('form') as HTMLFormElement
    expect(form.getAttribute('action')).toBe(payload.form?.post_path)
    const first = ['type', 'version_date', 'v', 'p']
    const visible = [...form.querySelectorAll('[name]')].filter(
      (f) => !(f as HTMLInputElement).type?.includes('hidden') && !f.closest('details'),
    )
    expect(visible.map((f) => f.getAttribute('name')).slice(0, 4)).toEqual(first)
    for (const name of first) expect((form.querySelector(`[name="${name}"]`) as HTMLInputElement).required).toBe(true)
    const groups = [...form.querySelectorAll('details')]
    expect(groups.length).toBeGreaterThanOrEqual(3)
    expect(groups.every((d) => !d.open)).toBe(true)
    expect(form.querySelector('details [name="reason_p"]')).not.toBeNull()
    expect(form.querySelector('details [name="report_text"]')).not.toBeNull()
    const buttons = [...form.querySelectorAll('button[type="submit"]')].map((b) => [b.textContent, b.getAttribute('value')])
    expect(buttons).toEqual([
      ['Calculate', 'preview'],
      ['Save draft', 'draft'],
      ['Make effective', 'effective'],
      ['Void', 'void'],
    ])
  })

  it('TestReviewTypeLabels: the type options print labels and submit the codes', async () => {
    const el = await render(fixture('form'))
    const options = [...el.querySelectorAll('select[name="type"] option')].filter((o) => (o as HTMLOptionElement).value !== '')
    expect(options.map((o) => [o.textContent, (o as HTMLOptionElement).value])).toEqual([
      ['First valuation', 'first'],
      ['After earnings', 'post_earnings'],
      ['Event', 'event'],
      ['Backfill', 'backfill'],
    ])
  })

  it('TestProblemsListed: each problem is listed with its field', async () => {
    const el = await render(fixture('form'))
    const item = el.querySelector('[data-problems] li[data-field="p"]')
    expect(item?.textContent).toBe('p: P must be between 1 and 5 in steps of 0.5')
  })

  it('TestPreviewPostsAction: Calculate posts the preview action to the form path, without the required check', async () => {
    const payload = fixture('form')
    const el = await render(payload)
    const form = el.querySelector('form') as HTMLFormElement
    const calc = [...form.querySelectorAll('button[type="submit"]')].find((b) => b.textContent === 'Calculate')
    expect(calc?.getAttribute('name')).toBe('action')
    expect(calc?.getAttribute('value')).toBe('preview')
    expect((calc as HTMLButtonElement).formNoValidate).toBe(true)
    expect(form.getAttribute('method')).toBe('post')
    expect(el.querySelector('[data-lines]')?.textContent).toContain('112.5')
  })
})

describe('TestNoLogicInWeb', () => {
  it('exports only the registration and the component of the page', () => {
    expect(Object.keys(module).sort()).toEqual(['ValuationPage', 'registration'])
  })
})
