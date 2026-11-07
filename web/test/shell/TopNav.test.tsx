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
  it('has seven links with the fixed labels and paths', async () => {
    await showAt('/home')
    expect(host.querySelector('nav')?.getAttribute('aria-label')).toBe('主导航')
    expect(links().map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['首页', '/home'],
      ['家庭资产', '/household'],
      ['持仓', '/positions'],
      ['风险', '/risk'],
      ['期权', '/options'],
      ['期权数据', '/options/data'],
      ['建议', '/advice'],
    ])
  })

  it.each([
    ['/positions', '持仓'],
    ['/risk/', '风险'],
    ['/', '首页'],
    ['/options/data', '期权数据'],
    ['/options/data/9001', '期权数据'],
    ['/options', '期权'],
  ])('marks only the link of %s', async (pathname, label) => {
    await showAt(pathname)
    const marked = links().filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked.map((a) => a.textContent)).toEqual([label])
  })

  it('marks none on a page that is not in the bar', async () => {
    await showAt('/structure')
    expect(links().some((a) => a.hasAttribute('aria-current'))).toBe(false)
  })
})
