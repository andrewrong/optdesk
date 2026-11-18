import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { OptionCandidatesPage } from '@/pages/candidates/OptionCandidatesPage'
import { TierView } from '@/pages/candidates/TierView'
import { tiers } from './fixtures/tiers'
import { tiersNull } from './fixtures/tiers-null'

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
const open = (el: Element) => act(async () => (el as HTMLElement).click())

describe('TierView', () => {
  it('TestTierViewRendersTiers', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const text = page.textContent ?? ''
    expect(page.querySelectorAll('[data-tier]')).toHaveLength(3)
    for (const tier of tiers.tiers) expect(text).toContain(tier.title)
    expect(text).toContain(tiers.companies[0]!.counts)
    expect(text).toContain(tiers.kept!)
    for (const item of tiers.legend) expect(text).toContain(item.text)
    expect(page.querySelectorAll('[data-tier=fits] [data-row]')).toHaveLength(2)
  })

  it('TestTierWordAndIconPerTier', async () => {
    const page = await show(<TierView tiers={tiers} />)
    await open(page.querySelector('[data-tier=grey_in_the_money] button')!)
    for (const tier of tiers.tiers) {
      const scope = page.querySelector(`[data-tier=${tier.id}]`)!
      expect(scope.textContent).toContain(tier.word)
      const icon = scope.querySelector(`[data-icon=${tier.icon}]`)!
      expect(icon.getAttribute('aria-label')).toBe(tier.word)
    }
  })

  it('TestGreyGroupsCollapsedWithCount', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const group = page.querySelector('[data-tier=grey_in_the_money]')!
    expect(group.querySelector('button')!.textContent).toContain(tiers.tiers[2]!.title)
    expect(group.querySelector('button')!.getAttribute('aria-expanded')).toBe('false')
    expect(group.textContent).not.toContain(tiers.tiers[2]!.rows[0]!.line!)
    await open(group.querySelector('button')!)
    expect(group.querySelectorAll('[data-row]')).toHaveLength(tiers.tiers[2]!.rows.length)
    await open(group.querySelector('[data-arrow]')!)
    expect(group.textContent).toContain(tiers.tiers[2]!.rows[0]!.line!)
  })

  it('TestRowExpansionFields', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const first = page.querySelector('[data-tier=fits] [data-row]')!
    await open(first.querySelector('[data-arrow]')!)
    const labels = [...first.querySelectorAll('[data-field]')].map((f) => f.querySelector('[data-field-label]')!.textContent)
    expect(labels).toEqual(tiers.tiers[0]!.rows[0]!.fields.map((f) => f.label))
    const hints = [...first.querySelectorAll('[data-field-label] [data-hint]')].map((h) => h.getAttribute('data-hint'))
    expect(hints).toEqual(tiers.tiers[0]!.rows[0]!.fields.map((f) => f.hint))
    const text = first.textContent ?? ''
    expect(text).toContain('tiers gap no delta')
    expect(text).toContain('tiers as_of label')
    expect(text).toContain('tiers value iv')
  })

  it('TestCommonMarkAlertOnce', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const alerts = page.querySelectorAll('[data-common-mark]')
    expect(alerts).toHaveLength(1)
    expect(alerts[0]!.getAttribute('role')).toBe('alert')
    expect(alerts[0]!.textContent).toContain(tiers.common[0]!.label)
    expect(alerts[0]!.textContent).toContain(tiers.common[0]!.gloss)
  })

  it('TestTierViewShowsTextUnchanged', async () => {
    const page = await show(<TierView tiers={tiers} />)
    expect(page.textContent).toContain('−$1,234.50')
    const gapRow = page.querySelectorAll('[data-tier=fits] [data-row]')[1]!
    expect(gapRow.querySelector('[data-cell=margin]')!.textContent).toBe('tiers gap margin text')
  })

  it('TestBadgeColumnWide', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const heads = page.querySelectorAll('[data-tier=fits] thead th')
    expect(heads).toHaveLength(7)
    expect(heads[6]!.textContent).toContain('tiers col badges')
    expect(heads[6]!.className).toContain('min-w-64')
  })

  it('TestTierViewNullIsAbsent', async () => {
    const page = await show(<OptionCandidatesPage payload={tiersNull as never} />)
    expect(page.querySelector('[data-tier-view]')).toBeNull()
    expect(page.textContent).not.toContain('Stored tables')
  })

  it('TestRowWrapsOnPhone', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const fits = page.querySelector('[data-tier=fits]')!
    // Decision 13: at 640px and below a row wraps to two lines, so no fixed wide minimum forces sideways scrolling.
    expect(fits.querySelector('.min-w-\\[60rem\\]')).toBeNull()
    const row = fits.querySelector('[data-row] tr')!
    expect(row.className).toMatch(/max-sm:|sm:table-row|flex-wrap/)
  })

  it('TestTierColoursAndHeaderLabel', async () => {
    const page = await show(<TierView tiers={tiers} />)
    expect(page.querySelector('[data-tier=fits] h3')!.className).toContain('text-success')
    expect(page.querySelector('[data-tier=check] h3')!.className).toContain('text-warn')
    expect(page.querySelector('[data-tier=grey_in_the_money] button')!.className).toContain('text-muted-foreground')
    expect(page.querySelector('[data-tier=fits] [data-row] [data-slot=badge]')!.className).toContain('text-success')
    expect(page.querySelector('[data-tier=check] [data-row] [data-slot=badge]')!.className).toContain('text-warn')
    expect(page.querySelector('[data-legend-item=fits]')!.className).toContain('bg-success/10')
    expect(page.querySelector('[data-legend-item=check]')!.className).toContain('bg-warn/10')
    expect(page.querySelector('[data-legend-item=grey]')!.className).toContain('bg-muted')
    const label = page.querySelector('[data-tier=fits] [data-col=arrow] .sr-only')!
    expect(label.textContent).toBe(tiers.columns[0]!.label)
  })

  it('TestGreyGroupRowsHaveColumns', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const group = page.querySelector('[data-tier=grey_in_the_money]')!
    await open(group.querySelector('button')!)
    const fitsCells = page.querySelector('[data-tier=fits] [data-row] tbody tr')!.querySelectorAll('td').length
    const rows = group.querySelectorAll('[data-row] tbody tr')
    expect(rows).toHaveLength(tiers.tiers[2]!.rows.length)
    for (const tr of rows) expect(tr.querySelectorAll('td')).toHaveLength(fitsCells)
    const line = tiers.tiers[2]!.rows[0]!.line!
    expect(group.textContent).not.toContain(line)
    await open(group.querySelector('[data-arrow]')!)
    expect(group.querySelector('[data-expand]')!.textContent).toContain(line)
  })

  it('TestNoExplainUnderHeader', async () => {
    const page = await show(<TierView tiers={tiers} />)
    for (const id of ['fits', 'check']) {
      const scope = page.querySelector(`[data-tier=${id}]`)!
      const tier = tiers.tiers.find((t) => t.id === id)!
      expect(scope.textContent).not.toContain(tier.explain)
    }
  })

  it('TestGreyGlossShownWhenOpened', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const group = page.querySelector('[data-tier=grey_in_the_money]')!
    const gloss = tiers.tiers[2]!.explain
    expect(group.textContent).not.toContain(gloss)
    await open(group.querySelector('button')!)
    expect(group.textContent).toContain(gloss)
  })

  it('TestKeptNoteAfterKept', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const kept = page.querySelector('[data-kept]')!
    const note = page.querySelector('[data-kept-note]')!
    expect(note.textContent).toBe(tiers.kept_note)
    expect(kept.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await act(async () => root?.unmount())
    host.remove()
    const none = await show(<TierView tiers={{ ...tiers, kept_note: null }} />)
    expect(none.querySelector('[data-kept-note]')).toBeNull()
  })

  it('TestGapCellsMuted', async () => {
    const page = await show(<TierView tiers={tiers} />)
    await open(page.querySelector('[data-tier=grey_in_the_money] button')!)
    const cells = [
      page.querySelectorAll('[data-tier=fits] [data-row]')[1]!.querySelector('[data-cell=margin]')!,
      page.querySelector('[data-tier=grey_in_the_money] [data-row] [data-cell=below_spot]')!,
    ]
    for (const cell of cells) {
      const gapText = cell.textContent!
      expect(gapText).toContain('gap')
      const span = [...cell.querySelectorAll('span')].find((e) => e.textContent === gapText)!
      expect(span.className).toContain('text-muted-foreground')
    }
    const normal = page.querySelector('[data-tier=fits] [data-row] [data-cell=margin] div')!
    expect(normal.className).not.toContain('text-muted-foreground')
  })
})
