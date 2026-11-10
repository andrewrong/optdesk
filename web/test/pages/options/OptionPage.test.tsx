// Issue #1204: the option page from invented fixtures. Expected strings are typed from board 5 (期权页, ab5).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { OptionPage, validateOptionPage } from '@/pages/options/OptionPage'
import { pages } from '@/shell/pages'

// The jest-dom matcher of the same name is not a dependency of web/: an element is visible when it is in the
// document and neither it nor an ancestor is hidden by `hidden`, display none or visibility hidden.
declare module 'vitest' {
  interface Assertion {
    toBeVisible(): void
  }
}
expect.extend({
  toBeVisible(el: Element) {
    let pass = el.isConnected
    for (let n: Element | null = el; n && pass; n = n.parentElement) {
      const style = getComputedStyle(n)
      if (n.hasAttribute('hidden') || style.display === 'none' || style.visibility === 'hidden') {
        pass = false
      }
    }
    return { pass, message: () => `expected the element ${pass ? 'not ' : ''}to be visible` }
  },
})

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../fixtures/option-page', name), 'utf8'),
  )
let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionPage payload={payload as never} />)
  })
}
const click = async (el: Element | null) => {
  await act(async () => {
    el!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
}
// Radix Tabs switch on mouse down, not click.
const pick = async (el: Element | null) => {
  await act(async () => {
    el!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
  })
}
const firstColumn = (table: Element) =>
  [...table.querySelectorAll('tbody tr')].map((r) => r.querySelector('td')!.textContent)

beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = null
  host?.remove()
})

describe('OptionPage', () => {
  it('is registered for the schema option_page and the fixtures validate', () => {
    expect(Object.keys(pages)).toContain('option_page')
    for (const name of ['ok.json', 'stale.json', 'gap.json']) {
      expect(validateOptionPage(fixture(name)).ok).toBe(true)
    }
    for (const name of ['bad-version']) {
      expect(validateOptionPage(fixture(`${name}.json`)).ok).toBe(false)
    }
  })

  it('shows one card per payload card and one row per payload row, with the payload strings', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const us = payload.markets[0]
    expect(host.querySelector('[data-headline]')!.textContent).toBe('4 个值得看，2 个勉强。')
    const cards = [...host.querySelectorAll('[data-market=US] [data-opportunity]')]
    expect(cards).toHaveLength(us.cards.length)
    expect(cards[0]!.textContent).toContain('值得看')
    expect(cards[0]!.textContent).toContain(us.cards[0].annualised_text)
    expect(cards[0]!.textContent).toContain(us.cards[0].contract_line)
    expect(cards[0]!.textContent).toContain(us.cards[0].size_line)
    const table = host.querySelector('[data-market=US] [data-table=candidates]')!
    expect(table.querySelectorAll('tbody tr')).toHaveLength(us.candidates.rows.length)
    expect(firstColumn(table)[0]).toBe('AAA')
    expect(host.querySelector('svg')).not.toBeNull() // Lucide icons
  })

  it('reorders the rows by TanStack sorting when a column header is clicked', async () => {
    await show(fixture('ok.json'))
    const table = host.querySelector('[data-market=US] [data-table=candidates]')!
    const header = [...table.querySelectorAll('th button')].find((b) => b.textContent === '标的')!
    await click(header)
    expect(firstColumn(table)).toEqual(['AAA', 'BBB', 'CCC', 'DDD', 'EEE', 'FFF', 'TGT'])
    await click(header)
    expect(firstColumn(table)).toEqual(['TGT', 'FFF', 'EEE', 'DDD', 'CCC', 'BBB', 'AAA'])
  })

  it('lists the eleven candidate columns and sorts the rows by the Δ sort value', async () => {
    // A stored-scan table as the builder writes it (issue #1758); the deltas are invented.
    const labels = [
      '标的',
      '判定',
      '年化',
      '合约',
      '行权价 %',
      'Δ',
      'Bid',
      'Ask',
      'IV',
      '到期',
      'E',
    ]
    const ids = [
      'symbol',
      'verdict',
      'annualised',
      'contract',
      'strike_pct',
      'delta',
      'bid',
      'ask',
      'iv',
      'dte',
      'earnings',
    ]
    const text = (t: string, value: number | string) => ({ kind: 'text', text: t, value })
    const gap = { kind: 'gap', reason: 'no_data', text: '缺' }
    const row = (symbol: string, delta: string, value: number) => ({
      id: symbol,
      cells: ids.map((column) => ({
        column,
        cell:
          column === 'symbol'
            ? text(symbol, symbol)
            : column === 'delta'
              ? text(delta, value)
              : column === 'verdict'
                ? text('值得看', 0)
                : column === 'dte'
                  ? text('30 天', 30)
                  : gap,
      })),
    })
    const payload = fixture('ok.json')
    payload.markets[0].candidates = {
      columns: ids.map((id, i) => ({ id, label: labels[i], align: 'right' })),
      rows: [row('AAA', '−0.16', -16), row('BBB', '−0.30', -30), row('CCC', '−0.05', -5)],
    }
    await show(payload)
    const table = host.querySelector('[data-market=US] [data-table=candidates]')!
    expect([...table.querySelectorAll('th')].map((h) => h.textContent)).toEqual(labels)
    const header = [...table.querySelectorAll('th button')].find((b) => b.textContent === 'Δ')!
    await click(header)
    expect(firstColumn(table)).toEqual(['BBB', 'AAA', 'CCC']) // −30, −16, −5 ascending
  })

  it('shows TGT as 跳过 with gap cells, and no zero, for Bid/Ask, spread, volume and open interest', async () => {
    await show(fixture('ok.json'))
    const row = [
      ...host.querySelectorAll('[data-market=US] [data-table=candidates] tbody tr'),
    ].find((r) => r.querySelector('td')!.textContent === 'TGT')!
    const cells = [...row.querySelectorAll('td')].map((c) => c.textContent)
    expect(cells[1]).toBe('跳过')
    expect(cells.filter((c) => c === '—缺')).toHaveLength(5) // annualised, Bid/Ask, spread %, volume, open interest
  })

  it('marks the chosen contract in the chain, which is shown without a click', async () => {
    await show(fixture('ok.json'))
    const chain = host.querySelector('[data-table=chain]')!
    expect(chain.textContent).toContain('已选')
    expect(chain.querySelectorAll('tbody tr')).toHaveLength(4)
    expect(host.querySelector('[data-chain] [aria-expanded]')).toBeNull()
  })

  it('links 今日机会 to the 全部候选 section with the text and target of the payload', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const pane = payload.markets[0]
    const section = host.querySelector(`[data-market=US] section:has(> div > h2)`)!
    const link = section.querySelector('a')!
    expect(link.textContent).toBe(pane.all_link.text)
    expect(link.getAttribute('href')).toBe(`#${pane.all_link.target}`)
    expect(host.querySelector(`[data-market=US] section#${pane.all_link.target}`)).not.toBeNull()
  })

  it('shows the gap sentence of gap.json once, and the stale label of stale.json', async () => {
    await show(fixture('gap.json'))
    const gaps = host.querySelector('[data-gaps]')!
    expect(gaps.textContent!.split('AAA 11/28 行权价 $46 缺 Mid')).toHaveLength(2)
    await act(async () => root?.unmount())
    host.remove()
    await show(fixture('stale.json'))
    expect(host.textContent).toContain('数据已过期：数据时间 2026-10-28 09:28（吉隆坡）')
  })

  it('shows the JP tab with JPY text and its ≈USD text from the payload', async () => {
    await show(fixture('ok.json'))
    const jp = [...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === '日股')!
    await pick(jp)
    const card = host.querySelector('[data-market=JP] [data-opportunity]')!
    expect(card.textContent).toContain('¥240,000（≈$1,600）')
  })

  it('shows the premium line of the payload card as given', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const card = host.querySelector('[data-market=US] [data-opportunity] [data-premium-line]')!
    expect(card.textContent).toBe(payload.markets[0].cards[0].premium_line)
    expect(card.textContent).toContain('到期在价内概率')
  })

  it('has a tooltip with the payload hint on the IVP header', async () => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    )
    const payload = fixture('ok.json')
    await show(payload)
    const hint = payload.markets[0].candidates.columns.find((c: { id: string }) => c.id === 'ivp').hint
    expect(hint).toBeTruthy()
    const table = host.querySelector('[data-market=US] [data-table=candidates]')!
    const button = [...table.querySelectorAll('th button')].find(
      (b) => b.textContent === 'IVP',
    ) as HTMLElement
    await act(async () => button.focus())
    expect(document.body.textContent).toContain(hint)
  })

  it('prints the contract line and the IV line of every card as written', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const check = (pane: { cards: { id: string; contract_line: string; iv_line: string }[] }) => {
      for (const card of pane.cards) {
        const el = host.querySelector(`[data-opportunity=${card.id}]`)!
        expect(el.textContent).toContain(card.contract_line)
        expect(el.textContent).toContain(card.iv_line)
      }
    }
    check(payload.markets[0])
    const jp = [...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === '日股')!
    await pick(jp)
    check(payload.markets[1])
    const first = payload.markets[0].cards[0]
    expect(first.contract_line).toBe('sell put 62 · 11-13 周五 · 22 天 · Δ −0.15')
    expect(first.iv_line).toBe('IV 38% · HV30 31% · IVP 72')
  })

  it('draws the E tags and the rich-premium tag as written on their cards, all secondary Badges', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const cards = payload.markets[0].cards as { id: string; tags: string[] }[]
    for (const text of ['E 11-12 在期内', 'E 无', '权利金偏高']) {
      const card = cards.find((c) => c.tags.includes(text))!
      const tag = [...host.querySelectorAll(`[data-opportunity=${card.id}] [data-slot=badge]`)].find(
        (b) => b.textContent === text,
      )!
      expect(tag).toBeTruthy()
      expect(tag.className).toContain('bg-secondary')
      expect(tag.className).not.toContain('bg-destructive')
    }
  })

  it('shows the IVP 缺 line without a number', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const card = payload.markets[0].cards.find((c: { iv_line: string }) => c.iv_line.includes('IVP 缺'))
    const el = host.querySelector(`[data-opportunity=${card.id}]`)!
    expect(card.iv_line).toBe('IV 31% · HV30 29% · IVP 缺')
    expect(el.textContent).toContain('IVP 缺')
    expect(el.textContent).not.toMatch(/IVP\s*\d/)
  })

  it('has the twelve candidate columns of the board in order, and the link with the count', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const labels = ['标的', '判定', '年化', 'strike %', 'DTE', 'Δ', 'Bid/Ask', 'spread %', 'Vol', 'OI', 'IVP', 'E']
    for (const pane of payload.markets) {
      expect(pane.candidates.columns.map((c: { label: string }) => c.label)).toEqual(labels)
    }
    const table = host.querySelector('[data-market=US] [data-table=candidates]')!
    expect([...table.querySelectorAll('th')].map((h) => h.textContent)).toEqual(labels)
    expect(host.querySelector('[data-market=US] [data-all-link]')!.textContent).toBe('全部 6 个机会')
  })

  it('shows every chain row on first render with no click', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const rows = [...host.querySelectorAll('[data-market=US] [data-table=chain] tbody tr')]
    expect(rows).toHaveLength(payload.markets[0].chain.table.rows.length)
    for (const row of rows) {
      expect(row.isConnected).toBe(true)
      expect(row).toBeVisible()
    }
  })

  it('shows 没有链数据 for a pane whose chain is null', async () => {
    const payload = fixture('ok.json')
    payload.markets[0].chain = null
    await show(payload)
    const pane = host.querySelector('[data-market=US]')!
    expect(pane.textContent).toContain('没有链数据')
    expect(pane.querySelector('[data-table=chain]')).toBeNull()
  })

  it('shows the verdict of the payload, not one derived from the numbers', async () => {
    const payload = fixture('ok.json')
    const card = payload.markets[0].cards[0]
    card.verdict = { key: 'skip', label: '跳过', tone: 'muted' } // while the annualised figure is the best one
    await show(payload)
    const shown = host.querySelector(`[data-opportunity=${card.id}] [data-verdict]`)!
    expect(shown.textContent).toBe('跳过')
    expect(host.querySelector('[data-headline]')!.textContent).toBe(payload.markets[0].headline)
  })

  it.each([
    ['good', 'text-success'],
    ['warn', 'text-warn'],
    ['neutral', 'bg-secondary'],
    ['muted', 'text-foreground'],
  ])('draws a verdict of tone %s with the class %s', async (tone, expected) => {
    const payload = fixture('ok.json')
    const card = payload.markets[0].cards[0]
    card.verdict = { key: 'k', label: 'L', tone }
    await show(payload)
    const shown = host.querySelector(`[data-opportunity=${card.id}] [data-verdict]`)!
    expect(shown.className).toContain(expected)
  })

  it('renders section labels supplied by Python', async () => {
    // Source: issue #1204 PR contract: Python writes every label.
    const payload = fixture('ok.json')
    const strings = new Set<string>()

    function collect(value: unknown): void {
      if (typeof value === 'string') strings.add(value)
      else if (Array.isArray(value)) value.forEach(collect)
      else if (value !== null && typeof value === 'object') Object.values(value).forEach(collect)
    }

    collect(payload)
    await show(payload)

    const headings = [...host.querySelectorAll('section h2')]
    expect(headings).toHaveLength(2)
    for (const heading of headings) {
      expect(strings.has(heading.textContent!)).toBe(true)
    }
  })
})
