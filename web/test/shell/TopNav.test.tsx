// Issue #1507: one top bar of live pages; the page being viewed is marked.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { TopNav } from '@/shell/TopNav'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement
async function showAt(pathname: string) {
  window.history.pushState({}, '', pathname)
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<TopNav />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  host.remove()
})
const links = () => [...host.querySelectorAll('a')]

const GROUPS: readonly (readonly [string, readonly (readonly [string, string])[]])[] = [
  ['Market', [['Macro', '/macro'], ['Market state', '/market-state']]],
  ['Portfolio', [['Household', '/household'], ['Positions', '/positions'], ['Risk', '/risk']]],
  [
    'Options',
    [
      ['Candidates', '/options/candidates'],
      ['Structures', '/options/structures'],
      ['Result', '/options/result'],
      ['Notices', '/options/notices'],
      ['Tracking', '/options/tracking'],
      ['P&L split', '/options/pnl'],
    ],
  ],
  ['Companies', [['Watch list', '/underlyings'], ['Price lines', '/plan/price-lines'], ['Valuation', '/valuation'], ['Import valuations', '/valuation-import'], ['Data', '/company-data']]],
  ['System', [['Settings', '/settings'], ['Job console', '/job-console'], ['Base data', '/base-data'], ['Data health', '/data-health'], ['Option data', '/options/data']]],
]

const triggers = () => [...host.querySelectorAll('nav > div > button')] as HTMLElement[]
async function openMenu(name: string) {
  const trigger = triggers().find((b) => b.textContent === name)
  expect(trigger).toBeDefined()
  await act(async () => {
    trigger?.focus()
    trigger?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  })
  const menus = document.querySelectorAll('[role="menu"]')
  expect(menus).toHaveLength(1)
  return menus[0] as HTMLElement
}
const pairs = (els: Iterable<Element>) => [...els].map((a) => [a.textContent, a.getAttribute('href')])

describe('TestTopNavSixTopLevelItems', () => {
  it('the bar has six children and no flex-wrap', async () => {
    await showAt('/home')
    expect(host.querySelector('nav')?.getAttribute('aria-label')).toBe('Main navigation')
    const bar = host.querySelector('nav > div')
    const children = [...(bar?.children ?? [])]
    expect(children.length).toBeLessThanOrEqual(6)
    expect(children.map((e) => e.textContent)).toEqual(['Home', 'Market', 'Portfolio', 'Options', 'Companies', 'System'])
    expect(bar?.className).not.toContain('flex-wrap')
    expect(bar?.className).not.toContain('gap-y-1')
    expect(bar?.className).toContain('hidden md:flex gap-1')
  })
})

describe('TestTopNavGroupMenus', () => {
  it.each(GROUPS)('the %s menu lists its links in order', async (name, expected) => {
    await showAt('/home')
    const menu = await openMenu(name)
    expect(pairs(menu.querySelectorAll('a'))).toEqual(expected.map(([l, h]) => [l, h]))
    if (name === 'Options') {
      const texts = [...menu.querySelectorAll('a, [data-slot="dropdown-menu-label"]')].map((e) => e.textContent)
      expect(texts).toEqual([
        'Candidates', 'Held options', 'Structures', 'Result', 'Notices', 'Tracking', 'P&L split',
      ])
    }
  })

  it('marks Option data in the System menu at options/data/9001', async () => {
    await showAt('/options/data/9001')
    const menu = await openMenu('System')
    const marked = [...menu.querySelectorAll('a[aria-current="page"]')]
    expect(marked.map((a) => a.textContent)).toEqual(['Option data'])
  })
})

describe('TestTopNavMarksGroup', () => {
  it.each([
    ['/home', 'Home', 'page'],
    ['/macro', 'Market', 'true'],
    ['/risk/', 'Portfolio', 'true'],
    ['/options/data/9001', 'System', 'true'],
    ['/options/structures', 'Options', 'true'],
    ['/plan/price-lines', 'Companies', 'true'],
    ['/valuation', 'Companies', 'true'],
    ['/job-console', 'System', 'true'],
    ['/data-health', 'System', 'true'],
  ])('at %s the bar marks %s', async (pathname, label, value) => {
    await showAt(pathname)
    const marked = [...host.querySelectorAll('nav > div [aria-current]')].filter((e) => e.matches('button, a'))
    expect(marked.map((e) => [e.textContent, e.getAttribute('aria-current')])).toEqual([[label, value]])
  })

  it('at /structure the bar marks nothing among its buttons and links', async () => {
    await showAt('/structure')
    const marked = [...host.querySelectorAll('nav > div [aria-current]')].filter((e) => e.matches('button, a'))
    expect(marked).toHaveLength(0)
  })
})

describe('TopNav', () => {
  it('marks none on a page that is not in the bar', async () => {
    await showAt('/structure')
    expect(links().some((a) => a.hasAttribute('aria-current'))).toBe(false)
  })

  const openSheet = async () => {
    const button = host.querySelector('button[aria-label="Menu"]')
    expect(button).not.toBeNull()
    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    return document.querySelector('[role="dialog"]')
  }

  it('the sheet is closed at first and the phone button is 44 px, hidden from md', async () => {
    await showAt('/home')
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    const button = host.querySelector('button[aria-label="Menu"]')
    expect(button?.className).toContain('size-11')
    expect(button?.className).toContain('md:hidden')
    expect(host.querySelector('nav > div')?.className).toContain('hidden md:flex')
  })

  it('TestTopNavSheetGroups: Home first, five heading rows, Held options sub-heading', async () => {
    await showAt('/home')
    const sheet = await openSheet()
    expect(sheet?.getAttribute('data-side')).toBe('left')
    const rows = [...(sheet?.querySelectorAll('ul > li') ?? [])]
    const headings = rows.filter((li) => !li.querySelector('a')).map((li) => li.textContent)
    expect(headings).toEqual(['Market', 'Portfolio', 'Options', 'Held options', 'Companies', 'System'])
    expect(rows[0]?.textContent).toBe('Home')
    expect(rows[0]?.querySelector('a')?.getAttribute('href')).toBe('/home')
    const flat = rows.map((li) => li.querySelector('a')?.textContent ?? li.textContent)
    expect(flat).toEqual([
      'Home',
      'Market', 'Macro', 'Market state',
      'Portfolio', 'Household', 'Positions', 'Risk',
      'Options', 'Candidates', 'Held options', 'Structures', 'Result', 'Notices', 'Tracking', 'P&L split',
      'Companies', 'Watch list', 'Price lines', 'Valuation', 'Import valuations', 'Data',
      'System', 'Settings', 'Job console', 'Base data', 'Data health', 'Option data',
    ])
    const expected = [['Home', '/home'], ...GROUPS.flatMap(([, l]) => l)].map(([l, h]) => [l, h])
    expect(pairs(sheet?.querySelectorAll('a') ?? [])).toEqual(expected)
    const structures = [...(sheet?.querySelectorAll('a') ?? [])].find((a) => a.textContent === 'Structures')
    expect(structures?.className).toContain('pl-6')
  })

  it('TestTopNavSheetGroups: marks the link Option data at options/data/9001', async () => {
    await showAt('/options/data/9001')
    const sheet = await openSheet()
    const marked = [...(sheet?.querySelectorAll('[aria-current]') ?? [])]
    expect(marked.map((a) => [a.textContent, a.getAttribute('aria-current')])).toEqual([['Option data', 'page']])
  })
})

describe('TestTopNavHasCompanyData', () => {
  it('the Companies menu ends with Import valuations and Data to /company-data and the bar keeps six items', async () => {
    await showAt('/home')
    const bar = host.querySelector('nav > div')
    expect([...(bar?.children ?? [])]).toHaveLength(6)
    const menu = await openMenu('Companies')
    expect(pairs(menu.querySelectorAll('a')).slice(-2)).toEqual([
      ['Import valuations', '/valuation-import'],
      ['Data', '/company-data'],
    ])
  })
})
