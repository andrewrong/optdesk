import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { OptionCandidatesPage } from '@/pages/candidates/OptionCandidatesPage'
import { TierView } from '@/pages/candidates/TierView'
import { tiers } from './fixtures/tiers'
import { bycompany } from './fixtures/bycompany'
import { jp } from './fixtures/jp'
import { tiersNoHeads, tiersNull, tiersTextsNull } from './fixtures/tiers-null'

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
    expect(page.querySelectorAll('[data-tier]:not([data-company] *)')).toHaveLength(3)
    for (const tier of tiers.tiers) expect(text).toContain(tier.title)
    expect(text).toContain(tiers.companies[0]!.count_parts!.fits)
    expect(text).toContain(tiers.kept!)
    for (const item of tiers.legend) expect(text).toContain(item.text)
    expect(page.querySelectorAll('section[data-tier=fits] [data-row]')).toHaveLength(2)
  })

  it('TestTierWordAndIconPerTier', async () => {
    const page = await show(<TierView tiers={tiers} />)
    await open(page.querySelector('[data-tier=grey_in_the_money] button')!)
    for (const tier of tiers.tiers) {
      const scope = page.querySelector(`[data-tier=${tier.id}]:not([data-company] *)`)!
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
    const first = page.querySelector('section[data-tier=fits] [data-row]')!
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
    const gapRow = page.querySelectorAll('section[data-tier=fits] [data-row]')[1]!
    expect(gapRow.querySelector('[data-cell=margin]')!.textContent).toBe('tiers gap margin text')
  })

  it('TestBadgeColumnWide', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const heads = page.querySelectorAll('section[data-tier=fits] thead th')
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
    const fits = page.querySelector('section[data-tier=fits]')!
    // Decision 13: at 640px and below a row wraps to two lines, so no fixed wide minimum forces sideways scrolling.
    expect(fits.querySelector('.min-w-\\[60rem\\]')).toBeNull()
    const row = fits.querySelector('[data-row] tr')!
    expect(row.className).toMatch(/max-sm:|sm:table-row|flex-wrap/)
  })

  it('TestTierColoursAndHeaderLabel', async () => {
    const page = await show(<TierView tiers={tiers} />)
    expect(page.querySelector('section[data-tier=fits] h3')!.className).toContain('text-success')
    expect(page.querySelector('section[data-tier=check] h3')!.className).toContain('text-warn')
    expect(page.querySelector('[data-tier=grey_in_the_money] button')!.className).toContain('text-muted-foreground')
    expect(page.querySelector('section[data-tier=fits] [data-row] [data-slot=badge]')!.className).toContain('text-success')
    expect(page.querySelector('section[data-tier=check] [data-row] [data-slot=badge]')!.className).toContain('text-warn')
    const label = page.querySelector('section[data-tier=fits] [data-col=arrow] .sr-only')!
    expect(label.textContent).toBe(tiers.columns[0]!.label)
  })

  it('TestLegendIsOneStrip', async () => {
    const page = await show(<TierView tiers={tiers} />)
    expect(page.querySelectorAll('[data-legend]')).toHaveLength(1)
    const items = page.querySelectorAll('[data-legend-item]')
    expect(items).toHaveLength(4)
    const pills = [...page.querySelectorAll('[data-legend] [data-slot=badge]')]
    expect(pills.map((p) => p.textContent)).toEqual(tiers.legend.slice(0, 3).map((i) => i.title))
    for (const item of items) expect(item.className).not.toMatch(/bg-success\/10|bg-warn\/10|bg-muted/)
  })

  it('TestTierHeadingNoDoubleWord', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const heads = [
      page.querySelector('section[data-tier=fits] h3')!,
      page.querySelector('section[data-tier=check] h3')!,
      page.querySelector('[data-tier=grey_in_the_money] button')!,
    ]
    heads.forEach((h, i) => {
      const tier = tiers.tiers[i]!
      expect(h.textContent).toContain(tier.title)
      const count = (h.textContent ?? '').split(tier.word).length - 1
      expect(count).toBe(i === 2 ? 1 : 0)
      if (i < 2) expect(h.querySelector('[data-icon]')!.getAttribute('aria-label')).toBe(tier.word)
    })
  })

  it('TestNumericColumnsRightAligned', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const fits = page.querySelector('section[data-tier=fits]')!
    for (const id of ['margin', 'below_spot', 'premium']) {
      expect(fits.querySelector(`th[data-col=${id}]`)!.className).toContain('text-right')
      const cell = fits.querySelector(`[data-row] [data-cell=${id}]`)!
      expect(cell.className).toContain('text-right')
    }
    expect(fits.querySelector('[data-row] [data-cell=contract]')!.className).not.toContain('text-right')
    expect(fits.querySelector('th[data-col=contract]')!.className).not.toContain('text-right')
  })

  it('TestGreyGroupRowsHaveColumns', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const group = page.querySelector('[data-tier=grey_in_the_money]')!
    await open(group.querySelector('button')!)
    const fitsCells = page.querySelector('section[data-tier=fits] [data-row] tbody tr')!.querySelectorAll('td').length
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
      page.querySelectorAll('section[data-tier=fits] [data-row]')[1]!.querySelector('[data-cell=margin]')!,
      page.querySelector('[data-tier=grey_in_the_money] [data-row] [data-cell=below_spot]')!,
    ]
    for (const cell of cells) {
      const gapText = cell.textContent!
      expect(gapText).toContain('gap')
      const span = [...cell.querySelectorAll('span')].find((e) => e.textContent === gapText)!
      expect(span.className).toContain('text-muted-foreground')
    }
    const normal = page.querySelector('section[data-tier=fits] [data-row] [data-cell=margin] div')!
    expect(normal.className).not.toContain('text-muted-foreground')
  })

  it('renders the buy hint as a link to the watch list', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const link = page.querySelector('[data-buy-hint] a')!
    expect(link.getAttribute('href')).toBe('/underlyings')
    expect(link.textContent).toBe(tiers.buy_hint)
    const legend = page.querySelector('[data-legend-item]')!
    expect(page.querySelector('[data-buy-hint]')!.compareDocumentPosition(legend) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await act(async () => root?.unmount())
    host.remove()
    const none = await show(<TierView tiers={tiersTextsNull} />)
    expect(none.querySelector('[data-buy-hint]')).toBeNull()
  })

  it('renders the idle line once', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const lines = page.querySelectorAll('[data-idle-note]')
    expect(lines).toHaveLength(1)
    expect(lines[0]!.textContent).toBe(tiers.idle_note)
    const lastTier = [...page.querySelectorAll('[data-tier]:not([data-company] *)')].at(-1)!
    expect(lastTier.compareDocumentPosition(lines[0]!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    const strip = page.querySelector('[data-company]')!
    expect(strip.textContent).toContain(tiers.companies[0]!.spot_note!)
    await act(async () => root?.unmount())
    host.remove()
    const none = await show(<TierView tiers={tiersTextsNull} />)
    expect(none.querySelector('[data-idle-note]')).toBeNull()
  })

  it('TestCardsBeforeTable', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const follows = (a: Element, b: Element) => !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)
    const legend = page.querySelector('[data-legend]')!
    const card = page.querySelector('[data-company]')!
    const note = page.querySelector('[data-card-note]')!
    const table = page.querySelector('table')!
    expect(note.textContent).toBe(tiers.card_note)
    expect(follows(legend, card)).toBe(true)
    expect(follows(card, note)).toBe(true)
    expect(follows(note, table)).toBe(true)
  })

  it('TestColumnSubtitlesVisible', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const heads = [...page.querySelectorAll('section[data-tier=fits] thead th')]
    const subtitles = heads.map((h) => h.querySelector('[data-col-subtitle]')?.textContent)
    expect(subtitles).toEqual(tiers.columns.map((c) => c.hint))
    expect(page.querySelector('section[data-tier=fits] thead [data-hint]')).toBeNull()
  })

  it('TestNoWidthCap', () => {
    const all = import.meta.glob('/src/pages/candidates/*.tsx', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    expect(Object.keys(all).length).toBeGreaterThan(3)
    for (const text of Object.values(all)) expect(text).not.toMatch(/\bmax-w-/)
  })

  it('renders no table header for an empty tier', async () => {
    const empty = { ...tiers, tiers: tiers.tiers.map((t) => (t.id === 'check' ? { ...t, rows: [], explain: 'tiers None: nothing to check' } : t)) }
    const page = await show(<TierView tiers={empty} />)
    const scope = page.querySelector('section[data-tier=check]')!
    expect(scope.querySelector('thead')).toBeNull()
    expect(scope.querySelector('table')).toBeNull()
    expect(scope.textContent).toContain(tiers.tiers[1]!.title)
    expect(scope.textContent).toContain('tiers None: nothing to check')
    expect(page.querySelector('section[data-tier=fits] thead')).not.toBeNull()
  })

  it('prints the companies heading with its note and the tiers heading from heads', async () => {
    const page = await show(<TierView tiers={tiers} />)
    const h2s = [...page.querySelectorAll('h2')].map((h) => h.textContent)
    expect(h2s).toContain(tiers.heads!.companies)
    expect(h2s).toContain(tiers.heads!.tiers)
    expect(page.querySelector('[data-heads-note]')!.textContent).toBe(tiers.heads!.companies_note)
    const first = page.querySelector('section[data-tier]')!
    const tiersHeading = [...page.querySelectorAll('h2')].find((h) => h.textContent === tiers.heads!.tiers)!
    expect(tiersHeading.compareDocumentPosition(first) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('prints the same two headings for the Japanese tier view from its own heads', async () => {
    const jpTiers = jp.short_put_tiers_jpy!
    const page = await show(<TierView tiers={jpTiers} />)
    const h2s = [...page.querySelectorAll('h2')].map((h) => h.textContent)
    expect(h2s).toContain(jpTiers.heads!.companies)
    expect(h2s).toContain(jpTiers.heads!.tiers)
    expect(page.querySelector('[data-heads-note]')!.textContent).toBe(jpTiers.heads!.companies_note)
  })

  it('draws no heading and no error when heads is absent', async () => {
    const page = await show(<TierView tiers={tiersNoHeads} />)
    expect(page.querySelector('h2')).toBeNull()
    expect(page.querySelector('[data-heads-note]')).toBeNull()
    expect(page.querySelector('[data-tier-view]')).not.toBeNull()
    const none = await show(<OptionCandidatesPage payload={tiersNull as never} />)
    expect(none.querySelector('[data-tier-view]')).toBeNull()
    expect(none.querySelector('[data-heads-note]')).toBeNull()
  })

  it('TestByCompanyReplacesBands', async () => {
    const page = await show(<TierView tiers={bycompany} />)
    expect(page.querySelector('[data-tier]')).toBeNull()
    expect(page.querySelectorAll('[data-company-block]')).toHaveLength(4)
    expect(page.querySelectorAll('[data-company]')).toHaveLength(4)
    const text = page.textContent ?? ''
    for (const item of bycompany.by_company!.legend) expect(text).toContain(item.text)
    expect(text).not.toContain(bycompany.legend[0]!.text)
    expect(text).toContain(bycompany.by_company!.heading_note)
    expect(text).not.toContain(bycompany.heads!.companies_note)
    expect(page.querySelectorAll('th[data-col]')).toHaveLength(bycompany.by_company!.columns.length)
    const order = [...page.querySelectorAll('[data-company-block]')].map((b) => b.getAttribute('data-company-block'))
    expect(order).toEqual(['AAA', 'BBB', 'CCC', 'DDD'])
  })

  it('TestNoTierTablesHeading', async () => {
    const withBy = await show(<TierView tiers={bycompany} />)
    expect(withBy.textContent).not.toContain('合约分档')
    expect(withBy.querySelector('[data-heads-tiers]')).toBeNull()
    const without = await show(<TierView tiers={tiers} />)
    expect(without.textContent).toContain('合约分档')
  })

  it('TestNoByCompanyRendersAsToday', async () => {
    const page = await show(<TierView tiers={tiers} />)
    expect(page.querySelectorAll('section[data-tier]')).toHaveLength(2)
    expect(page.querySelector('[data-tier=grey_in_the_money]')).not.toBeNull()
    expect(page.querySelectorAll('[data-company]')).toHaveLength(tiers.companies.length)
    expect(page.querySelector('[data-company-block]')).toBeNull()
  })
})
