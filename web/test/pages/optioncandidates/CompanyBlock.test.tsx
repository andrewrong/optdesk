import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { CompanyBlock } from '@/pages/candidates/CompanyBlock'
import { bycompany } from './fixtures/bycompany'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
async function show(node: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(node)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const blocks = bycompany.by_company!.blocks
const block = (key: string) => blocks.find((b) => b.company === key)!
const draw = (key: string) => show(<CompanyBlock block={block(key)} tiers={bycompany} />)

describe('CompanyBlock', () => {
  it('TestBlockRowsInPayloadOrder', async () => {
    const page = await draw('AAA')
    const ids = [...page.querySelectorAll('[data-company-block] > [data-row]')].map((r) => r.getAttribute('data-row'))
    expect(ids).toEqual(['a1', 'a2', 'a3'])
  })

  it('TestVerdictPillAndText', async () => {
    const page = await draw('AAA')
    const b = block('AAA')
    expect(page.querySelector('[data-pill]')!.textContent).toBe(`${b.verdict.glyph} ${b.verdict.word}`)
    expect(page.querySelector('[data-pill]')!.getAttribute('data-verdict')).toBe('sellable')
    expect(page.querySelector('[data-line="3"]')!.textContent).toBe(b.verdict_text)
    expect(page.querySelector('[data-best-yield]')!.textContent).toBe(b.best_yield)
    const none = await draw('BBB')
    expect(none.querySelector('[data-best-yield]')).toBeNull()
  })

  it('TestFoldedClosedWithSummary', async () => {
    const page = await draw('AAA')
    const f = block('AAA').folded!
    const trigger = page.querySelector('[data-block-folded] button')!
    expect(trigger.textContent).toContain(f.title)
    expect(trigger.textContent).toContain(f.summary)
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(page.textContent).not.toContain('bc 折叠行 a4')
    await act(async () => (trigger as HTMLElement).click())
    expect(page.textContent).toContain('bc 折叠行 a4')
    expect((await draw('DDD')).querySelector('[data-block-folded]')).toBeNull()
  })

  it('TestNotJudgedWordOnlyForNoBuyPrice', async () => {
    for (const b of blocks) {
      const page = await draw(b.company)
      expect((page.textContent ?? '').includes('未判定')).toBe(b.verdict.id === 'no_buy_price')
    }
  })

  it('TestRowCarriesOwnTierBadge', async () => {
    const page = await draw('AAA')
    const fits = page.querySelector('[data-row=a1]')!
    const check = page.querySelector('[data-row=a3]')!
    expect(fits.textContent).toContain('bc 适合')
    expect(fits.querySelector('[data-icon]')!.getAttribute('data-icon')).toBe('check')
    expect(fits.querySelector('[data-fit-row]')!.className).toContain('border-l-success')
    expect(check.textContent).toContain('bc 待查')
    expect(check.querySelector('[data-icon]')!.getAttribute('data-icon')).toBe('alert')
    expect(check.querySelector('[data-fit-row]')!.className).toContain('border-l-warn')
  })

  it('TestBlockRowsFitWidth', async () => {
    const page = await draw('AAA')
    for (const el of page.querySelectorAll('[data-company-block], [data-company-block] *')) {
      const cls = el.getAttribute('class') ?? ''
      expect(cls).not.toContain('overflow-x-auto')
      expect(cls).not.toContain('min-w-[66rem]')
    }
    const badges = page.querySelectorAll('[data-cell="badges"]')
    expect(badges.length).toBeGreaterThan(0)
    for (const cell of badges) expect(cell.className).not.toContain('min-w-64')
  })
})
