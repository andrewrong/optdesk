// Issue #570: a `sample_page` payload shows the named page from carried sample data under one banner.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { samples } from '@/samples'
import { App } from '@/shell/App'
import { ok as homeTs } from '../pages/home/home-v2/fixtures/ok'
import { ok as riskTs } from '../pages/risk/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const BANNER = '示例数据，非实时数据 / Sample data, not live data'
const NAMES = [
  'home_page',
  'household_page',
  'positions_page',
  'household_risk',
  'option_page',
  'advice_page',
] as const
const BAR = ['/home', '/macro', '/household', '/positions', '/risk', '/options', '/options/data', '/advice']
const fixtureJson = (dir: string): unknown =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, '../../fixtures', dir, 'ok.json'), 'utf8'))
const marker = (page: string, version = 1) => ({ schema: 'sample_page', schema_version: version, page })
// A string of the payload that its page shows.
const titleOf = (payload: unknown) => (payload as { title: string }).title

let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<App payload={payload} />)
  })
}
const text = () => host.textContent ?? ''

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
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
  vi.restoreAllMocks()
  await act(async () => root?.unmount())
  root = null
  host?.remove()
})

function hrefs(value: unknown, out: string[] = []): string[] {
  if (Array.isArray(value)) value.forEach((v) => hrefs(v, out))
  else if (typeof value === 'object' && value !== null) {
    for (const [k, v] of Object.entries(value)) {
      if (k === 'href' && typeof v === 'string') out.push(v)
      else hrefs(v, out)
    }
  }
  return out
}

describe('sample pages', () => {
  it.each(NAMES)('%s shows the page and one banner after the nav', async (name) => {
    await show(marker(name))
    expect(text()).toContain(titleOf(samples[name]))
    const banners = [...host.querySelectorAll('[role="status"]')].filter((e) => e.textContent === BANNER)
    expect(banners).toHaveLength(1)
    const nav = host.querySelector('nav')!
    const main = host.querySelector('main')!
    expect(nav.compareDocumentPosition(banners[0]!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(banners[0]!.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it.each(NAMES)('%s shows no banner for its own payload', async (name) => {
    await show(samples[name])
    expect(text()).toContain(titleOf(samples[name]))
    expect(text()).not.toContain(BANNER)
  })

  it('throws for an unknown page', async () => {
    await expect(show(marker('nope'))).rejects.toThrow()
    await expect(show(marker('__proto__'))).rejects.toThrow()
  })

  it('throws for version 2', async () => {
    await expect(show(marker('home_page', 2))).rejects.toThrow()
  })

  it('maps the six names to the fixture files', () => {
    expect(Object.keys(samples).sort()).toEqual([...NAMES].sort())
    expect(samples['advice_page']).toEqual(fixtureJson('advice-page'))
    expect(samples['household_page']).toEqual(fixtureJson('household-page'))
    expect(samples['positions_page']).toEqual(fixtureJson('positions-page'))
    expect(samples['option_page']).toEqual(fixtureJson('option-page'))
  })

  it('home-page/ok.json is the TS ok with bar links only and no report links', () => {
    const json = fixtureJson('home-page') as typeof homeTs
    expect(json.block_links.map((l) => l.href)).toEqual(['/macro', '/household', '/options'])
    expect(json.report_links).toEqual([])
    const other = (o: object) =>
      Object.fromEntries(Object.entries(o).filter(([k]) => k !== 'block_links' && k !== 'report_links'))
    expect(other(json)).toEqual(JSON.parse(JSON.stringify(other(homeTs))))
  })

  it('household-risk/ok.json is the TS ok', () => {
    expect(fixtureJson('household-risk')).toEqual(JSON.parse(JSON.stringify(riskTs)))
  })

  it('no href of a sample leaves the eight bar routes', () => {
    for (const name of NAMES) for (const href of hrefs(samples[name])) expect(BAR).toContain(href)
  })
})
