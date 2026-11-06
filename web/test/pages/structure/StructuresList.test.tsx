// Issues #1770, #1826: the list shows the groups and the rows in the order and with the strings Python sent; the fixtures are the
// independent reference.
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { StructuresList, validateStructuresList } from '@/pages/structure/StructuresList'
import { pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const dir = path.resolve(import.meta.dirname, '../../../fixtures/structures-list')
const fixture = (name: string) => JSON.parse(readFileSync(path.join(dir, name), 'utf8'))
let root: Root | undefined
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<StructuresList payload={payload as never} />)
  })
}
beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('registers the page and validates the fixtures', () => {
  expect(pages.structures_list).toBeDefined()
  for (const name of readdirSync(dir).filter(
    (name) => name.endsWith('.json') && (!name.startsWith('bad-') || name === 'bad-version.json'),
  )) {
    expect(validateStructuresList(fixture(name)).ok, name).toBe(!name.startsWith('bad-'))
  }
})

type Cell = string | { text: string }
interface Row {
  symbol: string
  href: string
  kind_text: string
  expiry_text: string
  moneyness: Cell
  distance: Cell
  earned: Cell
  ex_dividend: Cell | null
  max_loss: { text: string }
  max_gain: { text: string }
  pnl: { text: string }
}
const shown = (cell: Cell) => (typeof cell === 'string' ? cell : cell.text)

it('shows the two groups in order, the rows in payload order, and each text as sent', async () => {
  const payload = fixture('ok.json')
  await show(payload)
  const cards = host.querySelectorAll('[data-structures-group]')
  expect([...cards].map((card) => card.getAttribute('data-structures-group'))).toEqual(
    payload.groups.map((group: { title: string }) => group.title),
  )
  expect(payload.groups.map((group: { title: string }) => group.title)).toEqual([
    '本周要决定',
    '之后到期',
  ])
  payload.groups.forEach((group: { rows: Row[] }, g: number) => {
    const trs = cards[g]!.querySelectorAll('tbody tr')
    expect(trs).toHaveLength(group.rows.length)
    group.rows.forEach((row, i) => {
      const tr = trs[i]!
      const link = tr.querySelector('td a')
      expect(link?.getAttribute('href')).toBe(row.href)
      expect(link?.textContent).toBe(row.symbol)
      const t = tr.textContent ?? ''
      for (const cell of [
        row.kind_text,
        row.expiry_text,
        shown(row.moneyness),
        shown(row.distance),
        shown(row.earned),
        row.max_loss.text,
        row.max_gain.text,
        row.pnl.text,
      ]) {
        expect(t).toContain(cell)
      }
    })
  })
})

it('shows 价内/价外 and 除息 as badges from the payload text', async () => {
  const payload = fixture('ok.json')
  payload.groups[1].rows[0].ex_dividend = '除息 2026-11-18'
  await show(payload)
  const first = host.querySelectorAll('[data-structures-group]')[0]!.querySelector('tbody tr')!
  const badges = [...first.querySelectorAll('[data-slot="badge"]')].map((b) => b.textContent)
  expect(badges).toContain(payload.groups[0].rows[0].moneyness)
  const second = host.querySelectorAll('[data-structures-group]')[1]!.querySelector('tbody tr')!
  const flag = [...second.querySelectorAll('[data-slot="badge"]')].map((b) => b.textContent)
  expect(flag).toContain('除息 2026-11-18')
})

it('shows a gap and the note once', async () => {
  const payload = fixture('gap.json')
  await show(payload)
  const all = host.textContent ?? ''
  expect(all.split(payload.note)).toHaveLength(2)
  const gaps = payload.groups
    .flatMap((group: { rows: Row[] }) => group.rows)
    .flatMap((row: Row) => [row.distance, row.earned])
    .filter((cell: Cell) => typeof cell !== 'string')
  expect(gaps.length).toBeGreaterThan(0)
  for (const gap of gaps) {
    expect(all).toContain(gap.text)
  }
})
