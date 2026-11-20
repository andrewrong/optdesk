// Decision 2 and 8 of docs/design/macro-page-v2.md: the lead block prints what Python sent, with its hints.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { LeadBlock } from '@/pages/macro/LeadBlock'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

// The payload is built from the golden file: its title and gap text, plus a lead of four rows.
const golden = JSON.parse(
  readFileSync(
    path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.macro_page.v2.golden.json'),
    'utf8',
  ),
)
function makeLead() {
  const base = { data_day: null, change_text: null, state: 'ok' }
  return {
    title: 'What a put seller reads first',
    note: 'display only; no rule reads this',
    rows: [
      { ...base, key: 'tbill', label: '3-month bill', text: '4.31%', change_text: '-0.05 pp over five sessions', data_day: '2026-11-18', hint: '三个月美国国债收益率' },
      { ...base, key: 'hy_spread', label: 'High-yield spread', text: '3.10%', hint: '高收益债信用利差' },
      {
        ...base,
        key: 'next_event',
        label: 'Next big event',
        state: 'gap',
        text: golden.events[0].previous.text,
        hint: '期权窗口内的重大事件',
      },
      { ...base, key: 'index_iv', label: 'Index IV', text: 'VIX 17.2 · VXN 21.0 · RVX 20.1', hint: '指数隐含波动率' },
    ],
  }
}
async function show(lead: ReturnType<typeof makeLead>) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<LeadBlock lead={lead as never} />)
  })
}

it('TestLeadBlockRows: title, four rows in order, change and day when present, note', async () => {
  const lead = makeLead()
  await show(lead)
  expect(host.textContent).toContain(lead.title)
  const rows = [...host.querySelectorAll('[data-lead-row]')]
  expect(rows.map((r) => r.getAttribute('data-lead-row'))).toEqual(['tbill', 'hy_spread', 'next_event', 'index_iv'])
  expect(rows[0]!.textContent).toContain('4.31%')
  expect(rows[0]!.querySelector('[data-lead-change]')?.textContent).toBe('-0.05 pp over five sessions')
  expect(rows[0]!.querySelector('[data-lead-day]')?.textContent).toBe('2026-11-18')
  expect(rows[1]!.querySelector('[data-lead-change]')).toBeNull()
  expect(rows[1]!.querySelector('[data-lead-day]')).toBeNull()
  expect(rows[3]!.textContent).toContain('VIX 17.2 · VXN 21.0 · RVX 20.1')
  expect(host.querySelector('[data-lead-note]')?.textContent).toBe(lead.note)
})

it('TestLeadBlockGapRow: a gap row prints its text in the gap style', async () => {
  const lead = makeLead()
  await show(lead)
  const rows = host.querySelectorAll('[data-lead-row]')
  const gap = rows[2]!.querySelector('[data-lead-gap]')!
  expect(gap.textContent).toBe(lead.rows[2]!.text)
  expect(gap.classList).toContain('text-muted-foreground')
  expect(rows[0]!.querySelector('[data-lead-gap]')).toBeNull()
})

it('TestLeadBlockHint: each row has its hint as aria-label', async () => {
  const lead = makeLead()
  await show(lead)
  lead.rows.forEach((row, i) => {
    const node = host.querySelectorAll('[data-lead-row]')[i]!.querySelector('[data-hint]')!
    expect(node.getAttribute('aria-label')).toBe(row.hint)
    expect(node.textContent).toBe(row.label)
  })
})
