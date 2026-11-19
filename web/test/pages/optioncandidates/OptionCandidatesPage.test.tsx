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
import { jp } from './fixtures/jp'
import { bycompany } from './fixtures/bycompany'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'
import { shortlist } from './fixtures/shortlist'
import { tiers } from './fixtures/tiers'

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
    const h = ok.head_labels
    expect(page.querySelector('[data-chain-day]')!.textContent).toBe(`${h.data_day} ${ok.chain_day}`)
    expect(page.querySelector('[data-run-status]')!.textContent).toBe(`${h.run_status} ${h.run_status_text}`)
    expect(page.querySelector('[data-rule-version]')!.textContent).toBe(`${h.rule_version} ${ok.rule_version}`)
    expect(text).toContain(ok.groups[0].title)
    expect(text).toContain(ok.groups[1].title)
    expect(page.querySelectorAll('[data-tab]')).toHaveLength(2)
    for (const note of ok.notes) expect(text).toContain(note)
  })

  it('shows each reason text of the blocked table and draws unknown differently from block', async () => {
    const page = await show(blocked)
    await act(async () => page.querySelector<HTMLElement>('[data-other] button')!.click())
    const text = page.textContent ?? ''
    expect(text).toContain(`Not passed / unconfirmed (${blocked.groups[0].other.rows.length})`)
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

  it('shows the Stale badge on the rows of a stale fixture and the stale alert of the group', async () => {
    const page = await show(stale)
    const main = page.querySelector('[data-group=short_put] [data-table=main]')!
    const rows = [...main.querySelectorAll('tbody tr')]
    expect(rows).toHaveLength(stale.groups[0].main.rows.length)
    for (const tr of rows) expect(tr.querySelector('td')!.textContent).toContain('Stale')
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

const open = (el: Element) => act(async () => (el as HTMLElement).click())

describe('OptionCandidatesPage tiers', () => {
  it('TestStoredTablesBelowTiers', async () => {
    const page = await show({ ...ok, short_put_tiers: tiers } as never)
    await open(page.querySelector('[data-stored-tables] button')!)
    const view = page.querySelector('[data-tier-view]')!
    const heading = page.querySelector('[data-stored-tables] button')!
    const tabs = page.querySelector('[data-tab]')!
    expect(view.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(heading.compareDocumentPosition(tabs) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('TestStoredTablesClosedByDefault', async () => {
    const page = await show({ ...ok, short_put_tiers: tiers } as never)
    const trigger = page.querySelector('[data-stored-tables] button')!
    expect(trigger.textContent).toBe(tiers.stored_title)
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(page.querySelector('[data-tab]')).toBeNull()
    await open(trigger)
    expect(page.querySelectorAll('[data-tab]').length).toBe(ok.groups.length)
  })

  it('folds the tier view under the closed accordion', async () => {
    const page = await show({ ...ok, short_put_list: shortlist, short_put_tiers: tiers } as never)
    expect(page.querySelector('[data-short-list]')).not.toBeNull()
    const trigger = page.querySelector('[data-folded] button')!
    expect(trigger.textContent).toBe(shortlist.folded_title)
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(page.querySelector('[data-tier-view]')).toBeNull()
    await open(trigger)
    expect(page.querySelector('[data-tier-view]')).not.toBeNull()
    expect(page.querySelector('[data-stored-tables]')).not.toBeNull()
  })

  it('hides ShortList for a tier view with by_company and shows it without', async () => {
    const withBy = await show({ ...ok, short_put_list: shortlist, short_put_tiers: bycompany } as never)
    expect(withBy.querySelector('[data-short-list]')).toBeNull()
    const without = await show({ ...ok, short_put_list: shortlist, short_put_tiers: tiers } as never)
    expect(without.querySelector('[data-short-list]')).not.toBeNull()
  })

  it('TestByCompanyNotFolded', async () => {
    const page = await show({ ...ok, short_put_list: shortlist, short_put_tiers: bycompany } as never)
    expect(page.querySelector('[data-folded]')).toBeNull()
    const blocks = page.querySelectorAll('[data-company-block]')
    expect(blocks).toHaveLength(4)
    for (const b of blocks) expect(b.closest('[data-folded]')).toBeNull()
    expect(page.querySelector('[data-stored-tables]')).not.toBeNull()
  })

  it('renders the JP group after the US short-put group with its currency, note, limit counts and yen lists', async () => {
    const page = await show(jp as never)
    const tabs = [...page.querySelectorAll('[data-tab]')]
    expect(tabs.map((t) => t.getAttribute('data-currency'))).toEqual([null, 'JPY', null])
    await act(async () => {
      tabs[1]!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
    })
    const jpGroup = page.querySelector('[role=tabpanel][data-state=active]')!
    expect(jpGroup.querySelector('[data-group-currency]')!.textContent).toBe('JPY')
    expect(jpGroup.querySelector('[data-group-note]')!.textContent).toBe(jp.groups[1]!.note)
    const counts = [...jpGroup.querySelectorAll('[data-limit-count]')]
    expect(counts.map((c) => c.textContent)).toEqual(
      jp.groups[1]!.limit_counts!.map((l) => `${l.text} ${l.count}`),
    )
    expect(jpGroup.querySelector('[data-short-counts]')!.textContent).toContain('jp yen list counts')
    expect(jpGroup.querySelector('[data-tier-view]')).not.toBeNull()
    expect(jpGroup.textContent).toContain('jp yen tiers kept')
  })

  it('renders a document without a JPY group with no currency label, note, limit counts or yen list', async () => {
    const page = await show(ok)
    expect(page.querySelector('[data-group-currency]')).toBeNull()
    expect(page.querySelector('[data-group-note]')).toBeNull()
    expect(page.querySelector('[data-limit-counts]')).toBeNull()
    expect(page.querySelectorAll('[data-tab]')).toHaveLength(2)
    expect(page.querySelector('[data-short-list]')).toBeNull()
  })

  it('prints the subtitle in a plain paragraph right after the title', async () => {
    const page = await show(ok)
    const h1 = page.querySelector('h1')!
    expect(h1.textContent).toBe(ok.title)
    const sub = page.querySelector('[data-subtitle]')!
    expect(sub.tagName).toBe('P')
    expect(sub.textContent).toBe(ok.subtitle)
    expect(sub.hasAttribute('title')).toBe(false)
    expect(h1.compareDocumentPosition(sub) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(sub.compareDocumentPosition(page.querySelector('[data-estimate-note]')!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('prints no subtitle paragraph and no error when the payload has none', async () => {
    const page = await show({ ...ok, subtitle: undefined } as never)
    expect(page.querySelector('[data-subtitle]')).toBeNull()
    expect(page.querySelector('h1')!.textContent).toBe(ok.title)
  })
})
