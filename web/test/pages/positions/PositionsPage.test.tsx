// Issue #1205: the positions page from invented fixtures. Expected strings are typed from issue #1205, board 6.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PositionsPage, validatePositionsPage } from '@/pages/positions/PositionsPage'
import { pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/positions-page', name),
      'utf8',
    ),
  )
let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<PositionsPage payload={payload as never} />)
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

beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = null
  host?.remove()
})

describe('PositionsPage', () => {
  it('renders each view exactly as payload cells and keeps stocks before option legs', async () => {
    // Source: #1205 board 6 and CI contract: browser displays every supplied string unchanged.
    const payload = fixture('ok.json')
    const original = structuredClone(payload)
    await show(payload)
    expect([...host.querySelectorAll('[role=tab]')].map((t) => t.textContent)).toEqual([
      'Me',
      'Spouse',
      'Household',
    ])
    for (const view of payload.views) {
      await pick(
        [...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === view.label)!,
      )
      expect(host.querySelector('[data-sentence]')!.textContent).toBe(view.sentence)
      expect(
        [...host.querySelectorAll('[data-table]')].map((t) => t.getAttribute('data-table')),
      ).toEqual(['stocks', 'options', 'trades'])
      for (const name of ['stocks', 'options']) {
        const table = view[name]
        const rows = [...host.querySelectorAll(`[data-table=${name}] tbody tr`)]
        expect(rows.map((r) => [...r.querySelectorAll('td')].map((c) => c.textContent))).toEqual(
          table.rows.map((r: Record<string, { text: string }>) =>
            table.columns.map((c: { id: string }) => {
              const cell = r[c.id] as { text: string; approx_usd?: { text: string } }
              return cell.approx_usd ? `${cell.text} ${cell.approx_usd.text}` : cell.text
            }),
          ),
        )
      }
    }
    expect(payload).toEqual(original)
  })
  it('wraps the tab list with eight views and keeps stocks, options, trades stacked in each', async () => {
    // Issue #1438: one to eight account views; the payload of eight is built from the fixture.
    const payload = fixture('ok.json')
    const base = payload.views
    payload.views = Array.from({ length: 8 }, (_, i) => ({
      ...structuredClone(base[i % base.length]),
      key: `view-${i}`,
      label: `Account ${i}`,
    }))
    await show(payload)
    expect(host.querySelectorAll('[role=tab]')).toHaveLength(8)
    const cls = host.querySelector('[role=tablist]')!.className
    for (const c of ['group-data-horizontal/tabs:h-auto', 'w-full', 'flex-wrap'])
      expect(cls.split(/\s+/)).toContain(c)
    expect(cls.split(/\s+/)).not.toContain('group-data-horizontal/tabs:h-8')
    expect(cls.split(/\s+/)).not.toContain('w-fit')
    for (let i = 0; i < 8; i++) {
      await pick(host.querySelectorAll('[role=tab]')[i]!)
      expect(
        [...host.querySelectorAll('[data-table]')].map((t) => t.getAttribute('data-table')),
      ).toEqual(['stocks', 'options', ...(payload.views[i].trades ? ['trades'] : [])])
    }
  })

  it('registers the page and accepts good fixtures while rejecting every bad fixture', () => {
    expect(pages.positions_page).toBeDefined()
    for (const name of ['ok', 'gap', 'stale'])
      expect(validatePositionsPage(fixture(`${name}.json`)).ok).toBe(true)
    for (const name of ['version']) {
      expect(validatePositionsPage(fixture(`bad-${name}.json`)).ok).toBe(false)
    }
  })

  it('shows every gap once and the supplied notes and stale label', async () => {
    // Source: #1205 CI contract: gap sentences appear once; no replacement price.
    const payload = fixture('gap.json')
    await show(payload)
    for (const gap of payload.gaps) expect(host.textContent!.split(gap.text)).toHaveLength(2)
    expect(host.querySelector('[data-gaps]')).not.toBeNull()
    expect(host.textContent).toContain(payload.notes[0])
    expect(host.querySelector('[data-table=stocks] tbody tr')!.textContent).toContain('no quote †')
    await act(async () => root?.unmount())
    host.remove()
    const stale = fixture('stale.json')
    await show(stale)
    expect(host.querySelector('[data-stale]')!.textContent).toBe(stale.stale.text)
  })

  it('sorts both tables on numeric values, preserves rows, and displays payload totals and tones', async () => {
    // Source: #1205 CI contract. Deliberately mismatched text proves the browser does not calculate.
    const payload = fixture('ok.json')
    const household = payload.views[2]
    household.sentence = 'household summary text from the test'
    household.stocks.rows[0].quantity.text = 'Python quantity text'
    household.stocks.rows[0].day = {
      value: 120,
      text: 'Python P&L text',
      unit: 'usd_signed',
      tone: 'down',
    }
    const original = structuredClone(payload)
    await show(payload)
    await pick([...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === 'Household')!)
    expect(host.querySelector('[data-sentence]')!.textContent).toBe('household summary text from the test')
    expect(host.querySelector('[data-table=stocks] .text-down')!.textContent).toBe(
      'Python P&L text',
    )
    for (const [name, ascending] of [
      ['stocks', ['JPA', 'AAA', 'AMD']],
      ['options', ['XYZ 11-20 P100', 'BBB 12-18 C50']],
    ] as const) {
      const table = host.querySelector(`[data-table=${name}]`)!
      const header = [...table.querySelectorAll('th button')].find((b) => b.textContent === 'Quantity')!
      const codes = () =>
        [...table.querySelectorAll('tbody tr')].map((r) => r.querySelector('td')!.textContent)
      await click(header)
      expect(codes()).toEqual(ascending)
      expect(header.closest('th')!.getAttribute('aria-sort')).toBe('ascending')
      await click(header)
      expect(codes()).toEqual([...ascending].reverse())
      expect(header.closest('th')!.getAttribute('aria-sort')).toBe('descending')
    }
    expect(payload).toEqual(original)
  })

  it('shows the gap text and no row for an empty table', async () => {
    const payload = fixture('ok.json')
    payload.views[1].stocks.rows = []
    payload.views[1].stocks.gap = { reason: 'no_data', text: 'no stock positions' }
    await show(payload)
    await pick([...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === 'Spouse')!)
    const table = host.querySelector('[data-table=stocks]')!
    expect(table.querySelector('[data-table-gap]')!.textContent).toBe('no stock positions')
    expect(table.querySelectorAll('tbody tr')).toHaveLength(0)
  })

  it("shows today's fills under the options table as payload cells, or the gap text", async () => {
    // Issue #1445: the columns and cells are Python's; a view without fills shows its gap, and a payload
    // from before the table (no `trades`) validates and shows no trades card.
    const payload = fixture('ok.json')
    await show(payload)
    const me = payload.views[0].trades
    expect(
      [...host.querySelectorAll('[data-table]')].map((t) => t.getAttribute('data-table')),
    ).toEqual(['stocks', 'options', 'trades'])
    expect([...host.querySelectorAll('[data-table=trades] th')].map((h) => h.textContent)).toEqual([
      'Time',
      'Account',
      'Action',
      'Underlying · structure',
      'Quantity',
      'Price',
    ])
    const rows = [...host.querySelectorAll('[data-table=trades] tbody tr')]
    const cellsOf = rows.map((r) => [...r.querySelectorAll('td')].map((c) => c.textContent))
    me.rows.forEach((r: Record<string, { text: string; reason?: string }>, i: number) =>
      me.columns.forEach((c: { id: string }, j: number) => {
        const cell = r[c.id]!
        if (cell.reason) {
          expect(cellsOf[i]![j]).toContain('—')
          expect(cellsOf[i]![j]).toContain(cell.text)
        } else expect(cellsOf[i]![j]).toBe(cell.text)
      }),
    )
    await act(async () => root?.unmount())
    host.remove()
    const gap = fixture('gap.json')
    await show(gap)
    expect(host.querySelector('[data-table=trades] [data-table-gap]')!.textContent).toBe(
      gap.views[0].trades.gap.text,
    )
    expect(host.querySelectorAll('[data-table=trades] tbody tr')).toHaveLength(0)
    await act(async () => root?.unmount())
    host.remove()
    const old = fixture('ok.json')
    for (const view of old.views) delete view.trades
    expect(validatePositionsPage(old).ok).toBe(true)
    await show(old)
    expect(host.querySelector('[data-table=trades]')).toBeNull()
  })

  it('shows expiry and days left as payload cells, sorts days on value, and shows a gap badge without an expiry', async () => {
    // Issue #1448: the columns come from Python; values 23 and 51 are the example's days counted from
    // 2026-10-28. Text differs from value so a browser calculation would show.
    const payload = fixture('ok.json')
    payload.views[2].options.rows[0].days_left.text = 'Python days text'
    payload.views[2].options.rows[1].expiry = { reason: 'missing_input', text: 'no expiry †' }
    payload.views[2].options.rows[1].days_left = { reason: 'missing_input', text: 'no expiry †' }
    expect(validatePositionsPage(payload).ok).toBe(true)
    await show(payload)
    await pick([...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === 'Household')!)
    const table = host.querySelector('[data-table=options]')!
    expect([...table.querySelectorAll('th')].map((h) => h.textContent)).toEqual([
      'Symbol',
      'Name',
      'Expiry',
      'Days left',
      'Owner',
      'Quantity',
      'Cost (per share)',
      'Last price (per share)',
      'Day P&L',
      'Unrealized P&L',
      'Remaining premium',
    ])
    const rows = () => [...table.querySelectorAll('tbody tr')]
    expect(
      rows().map((r) => [...r.querySelectorAll('td')].slice(2, 4).map((c) => c.textContent)),
    ).toEqual([
      ['2026-11-20', 'Python days text'],
      ['—no expiry †', '—no expiry †'],
    ])
    expect(rows()[1]!.querySelectorAll('td')[3]!.querySelector('[data-missing]')).not.toBeNull()
    expect(host.querySelector('[data-table=stocks]')!.querySelectorAll('th')).toHaveLength(8)
  })

  it('shows remaining premium as the Python text of the first option row', async () => {
    // Issue #1573: the v3 fixture's first option row is the invented XYZ put with $258.00 left.
    await show(fixture('ok.json'))
    const table = host.querySelector('[data-table=options]')!
    expect([...table.querySelectorAll('th')].at(-1)!.textContent).toBe('Remaining premium')
    expect(
      [...table.querySelectorAll('tbody tr')[0]!.querySelectorAll('td')].at(-1)!.textContent,
    ).toBe('$258.00')
  })

  it('sorts the options table by days left on the number Python sent', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    await pick([...host.querySelectorAll('[role=tab]')].find((t) => t.textContent === 'Household')!)
    const table = host.querySelector('[data-table=options]')!
    const header = [...table.querySelectorAll('th button')].find(
      (b) => b.textContent === 'Days left',
    )!
    const days = () =>
      [...table.querySelectorAll('tbody tr')].map((r) => r.querySelectorAll('td')[3]!.textContent)
    await click(header)
    expect(days()).toEqual(['23', '51'])
    await click(header)
    expect(days()).toEqual(['51', '23'])
  })

  it('still reads a page published as version 1, without the expiry columns', async () => {
    // Issue #1448: v1.json stays checked in; a v1 options table has 8 columns and no expiry cells.
    const old = fixture('ok.json')
    old.schema_version = 1
    for (const view of old.views) {
      view.options.columns = view.options.columns.filter(
        (c: { id: string }) => c.id !== 'expiry' && c.id !== 'days_left' && c.id !== 'open_premium',
      )
      for (const row of view.options.rows) {
        delete row.expiry
        delete row.days_left
        delete row.open_premium
      }
    }
    expect(validatePositionsPage(old).ok).toBe(true)
    await show(old)
    expect(host.querySelector('[data-table=options]')!.querySelectorAll('th')).toHaveLength(8)
  })
})
