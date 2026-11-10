// The fixtures are written from the schema alone; each text asserted is read from a fixture field.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { OptionCandidatesPage, registration, validateOptionCandidatesPage } from '@/pages/candidates/OptionCandidatesPage'
import { pages } from '@/shell/pages'
import { blocked } from './fixtures/blocked'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: typeof ok | typeof blocked | typeof gap | typeof stale) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionCandidatesPage payload={payload as never} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
const firstColumn = (scope: Element) =>
  [...scope.querySelectorAll('tbody tr')].map((tr) => tr.querySelector('td')!.textContent!.trim())

describe('OptionCandidatesPage', () => {
  it('shows the main table in payload order with the header texts and one tab per group', async () => {
    const page = await show(ok)
    const main = page.querySelector('[data-group=short_put] [data-table=main]')!
    expect(firstColumn(main)).toEqual(ok.groups[0].main.rows.map((r) => r.cells.company.text))
    const text = page.textContent ?? ''
    expect(text).toContain(ok.title)
    expect(text).toContain(ok.chain_day)
    expect(text).toContain(ok.computed_at)
    expect(text).toContain(ok.rule_version)
    expect(text).toContain(ok.groups[0].title)
    expect(text).toContain(ok.groups[1].title)
    expect(page.querySelectorAll('[data-tab]')).toHaveLength(2)
    for (const note of ok.notes) expect(text).toContain(note)
  })

  it('shows each reason text of the blocked table and draws unknown differently from block', async () => {
    const page = await show(blocked)
    await act(async () => page.querySelector<HTMLElement>('[data-other] button')!.click())
    const text = page.textContent ?? ''
    expect(text).toContain(`未通过 / 未确认 (${blocked.groups[0].other.rows.length})`)
    for (const row of blocked.groups[0].other.rows) {
      for (const reason of row.reasons) expect(text).toContain(reason.text)
    }
    expect(page.querySelector('[data-group=short_put] [data-table=main]')).toBeNull()
    const block = page.querySelector('[data-reason=block] .lucide')!
    const unknown = page.querySelector('[data-reason=unknown] .lucide')!
    expect(block.getAttribute('class')).not.toBe(unknown.getAttribute('class'))
    const badge = (kind: string) => page.querySelector(`[data-reason=${kind}] [data-slot=badge]`)!.getAttribute('data-variant')
    expect(badge('block')).not.toBe(badge('unknown'))
  })

  it('shows the text of a group with a gap and no table for it', async () => {
    const page = await show(gap)
    const group = page.querySelector('[data-group=short_put]')!
    expect(group.textContent).toContain(gap.groups[0].gap.text)
    expect(group.querySelector('table')).toBeNull()
    expect(group.querySelectorAll('[data-group-gap]')).toHaveLength(1)
  })

  it('shows the stale label of the page', async () => {
    const page = await show(stale)
    expect(page.querySelector('[data-stale]')!.textContent).toContain(stale.stale.text)
    expect(ok.stale).toBeNull()
    await act(async () => root?.unmount())
    host.remove()
    expect((await show(ok)).querySelector('[data-stale]')).toBeNull()
  })

  it('shows the estimate note', async () => {
    const page = await show(ok)
    expect(page.querySelector('[data-estimate-note]')!.textContent).toBe(ok.estimate_note)
  })

  it('shows the 过期 badge on the rows of a stale fixture and the stale alert of the group', async () => {
    const page = await show(stale)
    const main = page.querySelector('[data-group=short_put] [data-table=main]')!
    const rows = [...main.querySelectorAll('tbody tr')]
    expect(rows).toHaveLength(stale.groups[0].main.rows.length)
    for (const tr of rows) expect(tr.querySelector('td')!.textContent).toContain('过期')
    expect(page.querySelector('[data-group-stale]')!.textContent).toContain(stale.stale.text)
  })

  it('fails validation on a bad version and passes the fixtures', () => {
    expect(validateOptionCandidatesPage(badVersion).ok).toBe(false)
    for (const payload of [ok, blocked, gap, stale]) expect(validateOptionCandidatesPage(payload).ok).toBe(true)
  })

  it('is registered once under the schema name', () => {
    expect(registration.schema).toBe('option_candidates_page')
    expect(Object.keys(pages).filter((name) => name === 'option_candidates_page')).toHaveLength(1)
  })

  it('has no filter control, only the sort buttons of the data-table', async () => {
    const page = await show(ok)
    expect(page.querySelector('input, select, textarea')).toBeNull()
    expect(page.querySelectorAll('[data-table=main] thead button').length).toBeGreaterThan(0)
  })
})
