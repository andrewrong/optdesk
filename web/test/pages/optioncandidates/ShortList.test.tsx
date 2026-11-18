import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { ShortList } from '@/pages/candidates/ShortList'
import { shortlist } from './fixtures/shortlist'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
async function show(list: Parameters<typeof ShortList>[0]['list']) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<ShortList list={list} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

describe('ShortList', () => {
  it('renders groups in payload order', async () => {
    const page = await show(shortlist)
    const heads = [...page.querySelectorAll('[data-group]')].map((e) => e.getAttribute('data-group'))
    expect(heads).toEqual(['ZZZ', 'AAA'])
    const keys = [...page.querySelectorAll('[data-short-row]')].map((e) => e.getAttribute('data-short-row'))
    expect(keys).toEqual(['645', '640', '85'])
    expect(page.querySelector('[data-group="ZZZ"]')!.textContent).toContain('spot $686.97')
    expect(page.querySelectorAll('thead th').length).toBe(8)
    expect(page.textContent).toContain(shortlist.counts)
    expect(page.querySelector('[data-short-caveat]')!.textContent).toBe(shortlist.caveat)
    expect(page.querySelectorAll('[data-filter]').length).toBe(2)
    expect(page.querySelector('[data-dropped]')!.textContent).toContain('No bid')
  })

  it('renders the gap text for an empty delta', async () => {
    const page = await show(shortlist)
    const cell = page.querySelector('[data-short-row="640"] [data-cell="delta"]')!
    expect(cell.textContent).toBe('no delta')
  })

  it('renders the buy link only when buy_link is set', async () => {
    const page = await show(shortlist)
    const link = page.querySelector('[data-short-row="640"] a')!
    expect(link.getAttribute('href')).toBe('/plan/price-lines')
    expect(page.querySelector('[data-short-row="645"] a')).toBeNull()
  })

  it('renders the buy source tag', async () => {
    const page = await show(shortlist)
    const tag = page.querySelector('[data-short-row="645"] [data-buy-source="hand"]')!
    expect(tag.textContent).toBe('shortlist hand label')
    expect(page.querySelector('[data-short-row="640"] [data-buy-source]')).toBeNull()
    expect(page.querySelector('[data-short-row="645"] [data-badge="wide_spread"]')).not.toBeNull()
  })

  it('renders nothing for a null short_put_list', async () => {
    expect((await show(null)).innerHTML).toBe('')
    await act(async () => root?.unmount())
    host.remove()
    expect((await show(undefined)).innerHTML).toBe('')
  })
})
