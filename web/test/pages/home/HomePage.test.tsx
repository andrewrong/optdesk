// The home page shows the text fields of the payload and its five members as sent. Expected strings are read from
// the fixture, not written here.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { HomePage, registration } from '@/pages/home/HomePage'
import { ok } from './home-v2/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const bad = (name: string) =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, 'home-v2/fixtures', name), 'utf8'))

let root: Root | null = null
let host: HTMLElement
beforeEach(async () => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(ok.generated_at))
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<HomePage payload={ok} />)
  })
})
afterEach(async () => {
  await act(async () => root?.unmount())
  host.remove()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('HomePage', () => {
  it('shows the title of each of the five members', () => {
    const text = host.textContent ?? ''
    for (const title of [
      ok.household.card.title,
      ok.companies.card.title,
      ok.macro.card.title,
      ok.market.card.title,
      ok.options.card.title,
    ]) {
      expect(title.length).toBeGreaterThan(0)
      expect(text).toContain(title)
    }
  })

  it('shows title, say and gap_line', () => {
    const text = host.textContent ?? ''
    expect(text).toContain(ok.title)
    expect(text).toContain(ok.say)
    expect(host.querySelector('[data-gap-line]')?.textContent).toBe(ok.gap_line)
  })

  it('renders block_links and report_links as links to their href', () => {
    const anchors = [...host.querySelectorAll('a')]
    for (const link of [...ok.block_links, ok.report_links[0]]) {
      const found = anchors.find((a) => a.getAttribute('href') === link.href && a.textContent === link.text)
      expect(found, link.text).toBeDefined()
    }
  })
})

const CARDS: string[] = ['macro-summary', 'market-summary', 'household-summary', 'held-companies-summary', 'options-summary']
const card = (name: string | undefined) => host.querySelector(`[data-card="${name}"]`) as Element
const anchorOf = (link: { href: string; text: string } | undefined) =>
  [...host.querySelectorAll('a')].find((a) => a.getAttribute('href') === link?.href && a.textContent === link?.text) as Element
const ABOVE = Node.DOCUMENT_POSITION_FOLLOWING
const FOLLOWS = Node.DOCUMENT_POSITION_FOLLOWING

async function renderWith(payload: typeof ok) {
  await act(async () => root?.unmount())
  await act(async () => {
    root = createRoot(host)
    root.render(<HomePage payload={payload} />)
  })
}

describe('HomePage layout', () => {
  it('shows the gap line once', () => {
    expect(host.querySelectorAll('[data-gap-line]')).toHaveLength(1)
    expect((host.textContent ?? '').split(ok.gap_line)).toHaveLength(2)
  })

  it('orders the cards macro, market, household, companies, options', () => {
    for (let i = 0; i < CARDS.length - 1; i++) {
      expect(card(CARDS[i]), CARDS[i]).not.toBeNull()
      expect(card(CARDS[i]).compareDocumentPosition(card(CARDS[i + 1])) & FOLLOWS).toBeTruthy()
    }
  })

  it('puts block link i above card i', () => {
    expect(ok.block_links).toHaveLength(5)
    ok.block_links.forEach((link, i) => {
      const a = anchorOf(link)
      expect(a, link.text).toBeDefined()
      expect(a.compareDocumentPosition(card(CARDS[i])) & ABOVE).toBeTruthy()
      if (i > 0) expect(card(CARDS[i - 1]).compareDocumentPosition(a) & ABOVE).toBeTruthy()
    })
  })

  it('renders one row per block link', async () => {
    await renderWith({ ...ok, block_links: ok.block_links.slice(0, 2) as unknown as typeof ok.block_links })
    expect(host.querySelectorAll('[data-block-row]')).toHaveLength(2)
  })

  it('shows the first report link outside the fold and the rest inside', async () => {
    const [first, ...rest] = ok.report_links
    const fold = host.querySelector('[data-links="report_links"] [data-slot="accordion"]') as Element
    expect(fold).not.toBeNull()
    expect(fold.contains(anchorOf(first))).toBe(false)
    await act(async () => {
      ;(fold.querySelector('[data-slot="accordion-trigger"]') as HTMLElement).click()
    })
    expect(fold.textContent).toContain('全部报告')
    for (const link of rest) expect(fold.contains(anchorOf(link)), link.text).toBe(true)
  })

  it('has no fold for a single report link', async () => {
    await renderWith({ ...ok, report_links: ok.report_links.slice(0, 1) as unknown as typeof ok.report_links })
    expect(host.querySelector('[data-links="report_links"] [data-slot="accordion"]')).toBeNull()
    expect(anchorOf(ok.report_links[0])).toBeDefined()
  })
})

describe('registration', () => {
  it('is the home_page schema', () => {
    expect(registration.schema).toBe('home_page')
  })
  it('accepts ok', () => {
    expect(registration.page.validate(ok).ok).toBe(true)
  })
  it('refuses bad-version.json', () => {
    const result = registration.page.validate(bad('bad-version.json'))
    expect(result.ok).toBe(false)
  })
  it('refuses bad-member.json with a gap', () => {
    const result = registration.page.validate(bad('bad-member.json'))
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.gap.text.length).toBeGreaterThan(0)
  })
})
