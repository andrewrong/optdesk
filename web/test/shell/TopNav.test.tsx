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

describe('TopNav', () => {
  it('has fourteen items: the fixed links and the Option positions menu', async () => {
    await showAt('/home')
    expect(host.querySelector('nav')?.getAttribute('aria-label')).toBe('Main navigation')
    expect(links().map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Home', '/home'],
      ['Macro', '/macro'],
      ['Market state', '/market-state'],
      ['Household', '/household'],
      ['Positions', '/positions'],
      ['Risk', '/risk'],
      ['Options', '/options'],
      ['Option data', '/options/data'],
      ['Option candidates', '/options/candidates'],
      ['Advice', '/advice'],
      ['Settings', '/settings'],
      ['Price lines', '/plan/price-lines'],
      ['Job console', '/job-console'],
    ])
    const items = [...host.querySelectorAll('nav > div > a, nav > div > button')].map((e) => e.textContent)
    expect(items).toHaveLength(14)
    expect(items[13]).toBe('Option positions')
  })

  it('the Option positions menu holds the links Structures, Result and Notices', async () => {
    await showAt('/home')
    const trigger = [...host.querySelectorAll('button')].find((b) => b.textContent === 'Option positions')
    expect(trigger).toBeDefined()
    await act(async () => {
      trigger?.focus()
      trigger?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    })
    const menu = [...document.querySelectorAll('[role="menu"] a')]
    expect(menu.map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Structures', '/options/structures'],
      ['Result', '/options/result'],
      ['Notices', '/options/notices'],
      ['Tracking', '/options/tracking'],
    ])
  })

  it.each([
    ['/positions', 'Positions'],
    ['/risk/', 'Risk'],
    ['/', 'Home'],
    ['/options/data', 'Option data'],
    ['/options/data/9001', 'Option data'],
    ['/options', 'Options'],
    ['/macro/', 'Macro'],
    ['/settings', 'Settings'],
  ])('marks only the link of %s', async (pathname, label) => {
    await showAt(pathname)
    const marked = links().filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked.map((a) => a.textContent)).toEqual([label])
  })

  it('lists the job console link', async () => {
    await showAt('/job-console')
    const marked = links().filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked.map((a) => [a.textContent, a.getAttribute('href')])).toEqual([['Job console', '/job-console']])
  })

  it('the desktop bar wraps onto further rows and holds the Job console link', async () => {
    await showAt('/home')
    const bar = host.querySelector('nav > div')
    expect(bar?.className).toContain('flex-wrap')
    expect(bar?.className).toContain('gap-y-1')
    const job = links().find((a) => a.textContent === 'Job console')
    expect(job).toBeDefined()
    expect(bar?.contains(job ?? null)).toBe(true)
  })

  it('TopNav lists Price lines', async () => {
    await showAt('/plan/price-lines')
    const marked = links().filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked.map((a) => [a.textContent, a.getAttribute('href')])).toEqual([['Price lines', '/plan/price-lines']])
  })

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

  it('the sheet opens and lists the fourteen items, the position links under Option positions', async () => {
    await showAt('/home')
    const sheet = await openSheet()
    expect(sheet).not.toBeNull()
    expect(sheet?.getAttribute('data-side')).toBe('left')
    const hrefs = [...(sheet?.querySelectorAll('a') ?? [])].map((a) => [a.textContent, a.getAttribute('href')])
    expect(hrefs).toEqual([
      ['Home', '/home'],
      ['Macro', '/macro'],
      ['Market state', '/market-state'],
      ['Household', '/household'],
      ['Positions', '/positions'],
      ['Risk', '/risk'],
      ['Options', '/options'],
      ['Option data', '/options/data'],
      ['Option candidates', '/options/candidates'],
      ['Advice', '/advice'],
      ['Settings', '/settings'],
      ['Price lines', '/plan/price-lines'],
      ['Job console', '/job-console'],
      ['Structures', '/options/structures'],
      ['Result', '/options/result'],
      ['Notices', '/options/notices'],
      ['Tracking', '/options/tracking'],
    ])
    expect(sheet?.textContent).toContain('Option positions')
  })

  it('the sheet marks the current page with aria-current', async () => {
    await showAt('/options/data/9001')
    const sheet = await openSheet()
    const marked = [...(sheet?.querySelectorAll('a[aria-current="page"]') ?? [])]
    expect(marked.map((a) => a.textContent)).toEqual(['Option data'])
  })
})
