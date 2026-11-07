// The fixtures are written from the schema alone; each text asserted is read from a fixture field.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { registration, View } from '@/pages/report/view'
import { validate, type Payload } from '@/types/report_page'
import { gap } from './fixtures/gap'
import { okClose } from './fixtures/ok-close'
import { okOpen } from './fixtures/ok-open'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: Payload) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<View payload={payload} />)
  })
  return host
}
const text = (el: Element) => el.textContent ?? ''
const frame = (key: string) => host.querySelector(`[data-block-key="${key}"]`)
const keysInOrder = () => [...host.querySelectorAll('[data-block-key]')].map((el) => el.getAttribute('data-block-key'))

beforeAll(() => {
  // jsdom has no ResizeObserver, which the recharts container needs.
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

describe('report page', () => {
  it('registers the schema report_page', () => {
    expect(registration.schema).toBe('report_page')
  })

  it('draws the header of okClose', async () => {
    await show(okClose)
    const page = text(host)
    for (const part of [
      okClose.title,
      okClose.say,
      okClose.banner.left,
      okClose.banner.right,
      okClose.design_note,
      ...okClose.chips.map((chip) => chip.text),
      ...okClose.status_lines.flatMap((line) => [line.label, line.text]),
    ]) {
      expect(page).toContain(part)
    }
  })

  it('draws the five close blocks of okClose in order, each with its title', async () => {
    await show(okClose)
    expect(keysInOrder()).toEqual(['actions', 'account_moves', 'risks', 'market_watch', 'global_assets'])
    expect(okClose.slot).toBe('close')
    for (const block of Object.values(okClose.blocks)) {
      expect(getAllByText(block.title).length).toBeGreaterThan(0)
      expect(within(block.key)).toContain(block.title)
    }
    expect(text(host)).toContain(okClose.blocks.risks.items[2]?.title)
    expect(host.querySelector('[data-level="d"]')).not.toBeNull()
    expect(text(host)).toContain(okClose.blocks.actions.trades.footnote)
    expect(text(host)).toContain(okClose.blocks.global_assets.rotation.detail)
  })

  it('draws the eleven blocks of okOpen, the open ones first', async () => {
    await show(okOpen)
    expect(Object.keys(okOpen.blocks).sort()).toEqual(
      [
        'account_moves', 'accounts', 'actions', 'candidates', 'global_assets', 'market_line',
        'market_watch', 'reminders', 'risks', 'system_status', 'tiles',
      ],
    )
    for (const value of Object.values(okOpen.blocks)) expect(value).not.toBeNull()
    expect(keysInOrder()).toEqual([
      'market_line', 'tiles', 'accounts', 'candidates', 'reminders', 'system_status',
      'actions', 'account_moves', 'risks', 'market_watch', 'global_assets',
    ])
    const titles = [
      okOpen.blocks.market_line.title,
      okOpen.blocks.tiles.title,
      okOpen.blocks.accounts.title,
      okOpen.blocks.candidates.title,
      okOpen.blocks.reminders.title,
      okOpen.blocks.system_status.title,
      okOpen.blocks.actions.title,
      okOpen.blocks.account_moves.title,
      okOpen.blocks.risks.title,
      okOpen.blocks.market_watch.title,
      okOpen.blocks.global_assets.title,
    ]
    for (const title of titles) {
      expect(getAllByText(title).length).toBeGreaterThan(0)
    }
    expect(text(host)).toContain(okOpen.blocks.market_line.sentence)
    expect(text(host)).toContain(okOpen.blocks.candidates.covered_call.text)
    expect(text(host)).toContain(okOpen.blocks.candidates.put_spread.text)
    expect(host.querySelector('[data-block-key="candidates"] a')?.getAttribute('href')).toBe(
      okOpen.blocks.candidates.full_page.href,
    )
    expect(host.querySelectorAll('[role="progressbar"]').length).toBe(okOpen.blocks.accounts.meters.length)
  })

  it('collapses system_status when collapsed is true', async () => {
    await show(okOpen)
    expect(okOpen.blocks.system_status.collapsed).toBe(true)
    expect(text(frame('system_status')!)).not.toContain(okOpen.blocks.system_status.lines[0].text)
  })

  it('draws a block with its gap_text and its Gap texts when its state is not ok', async () => {
    await show(gap)
    expect(text(frame('account_moves')!)).toContain(gap.blocks.account_moves.gap_text)
    expect(text(frame('account_moves')!)).toContain(gap.blocks.account_moves.gaps[0].text)
    expect(text(frame('risks')!)).toContain(gap.blocks.risks.gap_text)
    expect(text(frame('global_assets')!)).toContain(gap.blocks.global_assets.gap_text)
    expect(text(host)).toContain(gap.blocks.actions.trades.text)
    expect(text(host)).toContain(gap.blocks.account_moves.accounts.rows[0].net.text)
    expect(text(host)).toContain(gap.blocks.account_moves.movers.text)
    expect(text(host)).toContain(gap.blocks.risks.items.text)
  })

  it('draws no chart and no chart_aria name when proxies is a whole Gap, and shows its text', async () => {
    await show(gap)
    const assets = frame('global_assets')!
    expect(assets.querySelector('[data-chart]')).toBeNull()
    expect(assets.querySelector('[role="img"]')).toBeNull()
    expect(assets.querySelector(`[aria-label="${gap.blocks.global_assets.chart_aria}"]`)).toBeNull()
    expect(text(assets)).toContain(gap.blocks.global_assets.proxies.text)
  })

  it('names the proxies chart of okClose with chart_aria', async () => {
    await show(okClose)
    const chart = frame('global_assets')!.querySelector('[data-chart]')
    expect(chart?.getAttribute('role')).toBe('img')
    expect(chart?.getAttribute('aria-label')).toBe(okClose.blocks.global_assets.chart_aria)
  })

  it('draws the stale label', async () => {
    await show(stale)
    expect(text(host)).toContain(stale.stale.text)
  })

  it('draws no frame for an optional block set to null', async () => {
    await show({ ...okOpen, blocks: { ...okOpen.blocks, tiles: null, system_status: null } })
    expect(frame('tiles')).toBeNull()
    expect(frame('system_status')).toBeNull()
    expect(frame('market_line')).not.toBeNull()
  })

  it('refuses bad-version', () => {
    expect(validate(badVersion).ok).toBe(false)
    expect(validate(okClose).ok).toBe(true)
  })
})

function within(key: string): string {
  const el = frame(key)
  if (!el) throw new Error(`no frame ${key}`)
  return text(el)
}
// Elements whose own text is exactly `title` (a title can repeat in sub or say).
function getAllByText(title: string): Element[] {
  return [...host.querySelectorAll('*')].filter(
    (el) =>
      [...el.childNodes]
        .filter((node) => node.nodeType === Node.TEXT_NODE)
        .map((node) => node.textContent)
        .join('') === title,
  )
}
