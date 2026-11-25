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
  ['市场', [['宏观', '/macro'], ['市场状态', '/market-state']]],
  ['持仓', [['家庭账户', '/household'], ['持仓明细', '/positions'], ['持有公司', '/held-family'], ['风险', '/risk']]],
  [
    '期权',
    [
      ['候选', '/options/candidates'],
      ['组合结构', '/options/structures'],
      ['价差腿', '/options/spread-legs'],
      ['结果', '/options/result'],
      ['通知', '/options/notices'],
      ['跟踪', '/options/tracking'],
      ['盈亏拆分', '/options/pnl'],
      ['策略回放', '/strategy-replay'],
    ],
  ],
  ['公司', [['自选清单', '/underlyings'], ['价位线', '/plan/price-lines'], ['估值', '/valuation'], ['导入估值', '/valuation-import'], ['数据', '/company-data']]],
  ['系统', [['设置', '/settings'], ['任务台', '/job-console'], ['基础数据', '/base-data'], ['数据健康', '/data-health'], ['期权数据', '/options/data']]],
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
    expect(children.map((e) => e.textContent)).toEqual(['首页', '市场', '持仓', '期权', '公司', '系统'])
    expect(bar?.className).not.toContain('flex-wrap')
    expect(bar?.className).not.toContain('gap-y-1')
    expect(bar?.className).toContain('hidden md:flex gap-1')
  })
})

describe('TestTopNavLabelsChinese', () => {
  it('the bar item texts are 首页 and the five group texts in order', async () => {
    await showAt('/home')
    const bar = host.querySelector('nav > div')
    expect([...(bar?.children ?? [])].map((e) => e.textContent)).toEqual(['首页', '市场', '持仓', '期权', '公司', '系统'])
  })
})

describe('TestTopNavGroupMenus', () => {
  it.each(GROUPS)('the %s menu lists its links in order', async (name, expected) => {
    await showAt('/home')
    const menu = await openMenu(name)
    expect(pairs(menu.querySelectorAll('a'))).toEqual(expected.map(([l, h]) => [l, h]))
    if (name === '期权') {
      const texts = [...menu.querySelectorAll('a, [data-slot="dropdown-menu-label"]')].map((e) => e.textContent)
      expect(texts).toEqual([
        '候选', '持有期权', '组合结构', '价差腿', '结果', '通知', '跟踪', '盈亏拆分', '策略回放',
      ])
    }
  })

  it('marks Option data in the System menu at options/data/9001', async () => {
    await showAt('/options/data/9001')
    const menu = await openMenu('系统')
    const marked = [...menu.querySelectorAll('a[aria-current="page"]')]
    expect(marked.map((a) => a.textContent)).toEqual(['期权数据'])
  })
})

describe('TestTopNavMarksGroup', () => {
  it.each([
    ['/home', '首页', 'page'],
    ['/macro', '市场', 'true'],
    ['/risk/', '持仓', 'true'],
    ['/options/data/9001', '系统', 'true'],
    ['/options/structures', '期权', 'true'],
    ['/plan/price-lines', '公司', 'true'],
    ['/valuation', '公司', 'true'],
    ['/job-console', '系统', 'true'],
    ['/data-health', '系统', 'true'],
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
    expect(headings).toEqual(['市场', '持仓', '期权', '持有期权', '公司', '系统'])
    expect(rows[0]?.textContent).toBe('首页')
    expect(rows[0]?.querySelector('a')?.getAttribute('href')).toBe('/home')
    const flat = rows.map((li) => li.querySelector('a')?.textContent ?? li.textContent)
    expect(flat).toEqual([
      '首页',
      '市场', '宏观', '市场状态',
      '持仓', '家庭账户', '持仓明细', '持有公司', '风险',
      '期权', '候选', '持有期权', '组合结构', '价差腿', '结果', '通知', '跟踪', '盈亏拆分', '策略回放',
      '公司', '自选清单', '价位线', '估值', '导入估值', '数据',
      '系统', '设置', '任务台', '基础数据', '数据健康', '期权数据',
    ])
    const expected = [['首页', '/home'], ...GROUPS.flatMap(([, l]) => l)].map(([l, h]) => [l, h])
    expect(pairs(sheet?.querySelectorAll('a') ?? [])).toEqual(expected)
    const structures = [...(sheet?.querySelectorAll('a') ?? [])].find((a) => a.textContent === '组合结构')
    expect(structures?.className).toContain('pl-6')
  })

  it('the phone sheet title reads 菜单', async () => {
    await showAt('/home')
    const sheet = await openSheet()
    expect(sheet?.querySelector('[data-slot="sheet-title"]')?.textContent).toBe('菜单')
  })

  it('TestTopNavSheetGroups: marks the link Option data at options/data/9001', async () => {
    await showAt('/options/data/9001')
    const sheet = await openSheet()
    const marked = [...(sheet?.querySelectorAll('[aria-current]') ?? [])]
    expect(marked.map((a) => [a.textContent, a.getAttribute('aria-current')])).toEqual([['期权数据', 'page']])
  })
})

describe('TestTopNavHasCompanyData', () => {
  it('the Companies menu ends with Import valuations and Data to /company-data and the bar keeps six items', async () => {
    await showAt('/home')
    const bar = host.querySelector('nav > div')
    expect([...(bar?.children ?? [])]).toHaveLength(6)
    const menu = await openMenu('公司')
    expect(pairs(menu.querySelectorAll('a')).slice(-2)).toEqual([
      ['导入估值', '/valuation-import'],
      ['数据', '/company-data'],
    ])
  })
})
