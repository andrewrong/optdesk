import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { CompanyStrip } from '@/pages/candidates/CompanyStrip'
import {
  handSource,
  noProfileLine,
  noReviewLine,
  notJudgedSource,
  notUsedLine,
  stripLabels,
  valueLine,
  valuationSource,
  withLine,
  withNull,
  withoutKey,
  withReason,
} from './fixtures/valueline'
import type { Company } from '../../../src/generated/option_candidates_page.v1'

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

describe('CompanyStrip', () => {
  it('TestStripTwoLines', async () => {
    const c = withLine
    const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
    const [l1, l2] = [page.querySelector('[data-line="1"]')!, page.querySelector('[data-line="2"]')!]
    expect(l1.textContent).toBe(
      `${c.tier!.glyph} ${c.tier!.word}${c.ticker}${stripLabels.buy_price}${c.buy_price}${stripLabels.spot} ${c.spot_text} ${c.spot_note}${stripLabels.next_earnings} ${c.earnings_text}`,
    )
    const v = c.valuation!
    expect(l2.textContent).toBe(
      `${v.line_label}${v.line_text}${v.note}${stripLabels.contracts} 2 ${stripLabels.fits} · 1 ${stripLabels.check} · 1 ${stripLabels.grey} ${stripLabels.grey_hint}`,
    )
    expect(l1.querySelector('[data-tier]')!.getAttribute('data-tier')).toBe('fits')
    expect(page.querySelector('[data-grey-hint]')!.textContent).toBe(stripLabels.grey_hint)
  })

  it('TestStripNoValuationLine', async () => {
    const none = await show(<CompanyStrip company={withNull} labels={stripLabels} />)
    expect(none.querySelector('[data-valuation]')).toBeNull()
    expect(none.querySelector('[data-line="1"]')).not.toBeNull()
    expect(none.querySelector('[data-counts]')).not.toBeNull()
    await act(async () => root?.unmount())
    host.remove()
    const c: Company = { ...withLine, valuation: noProfileLine }
    const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
    const val = page.querySelector('[data-valuation]')!
    expect(val.textContent).toBe(`${noProfileLine.line_label}${noProfileLine.line_text}${noProfileLine.note}`)
  })

  it('TestStripWordsFromPayload', async () => {
    const labels = { ...stripLabels, spot: 'ZQ1', buy_price: 'ZQ2', next_earnings: 'ZQ3', contracts: 'ZQ4', fits: 'ZQ5', check: 'ZQ6', grey: 'ZQ7', grey_hint: 'ZQ8' }
    const c: Company = {
      ...withLine,
      tier: { id: 'check', word: 'ZW', glyph: 'ZG' },
      spot_note: 'ZN',
      earnings_text: 'ZE',
      valuation: { ...valueLine, line_label: 'ZL', line_text: 'ZT', note: 'ZV' },
    }
    const page = await show(<CompanyStrip company={c} labels={labels} />)
    expect(page.textContent).toBe('ZG ZW' + 'AAA' + 'ZQ2$90.00' + 'ZQ1 $96.50 ZN' + 'ZQ3 ZE' + 'ZLZTZV' + 'ZQ4 2 ZQ5 · 1 ZQ6 · 1 ZQ7 ZQ8')
  })

  it('TestStripRunStateNotes', async () => {
    const used = { ...withLine }
    const notUsed = { ...withLine, valuation: notUsedLine }
    const none = { ...withLine, valuation: noReviewLine }
    for (const [c, state] of [[used, 'used'], [notUsed, 'not_used']] as const) {
      const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
      const note = page.querySelector('[data-valuation-note]')!
      expect(note.getAttribute('data-run-state')).toBe(state)
      expect(note.textContent).toBe(c.valuation!.note)
      await act(async () => root?.unmount())
      host.remove()
    }
    const page = await show(<CompanyStrip company={{ ...none, valuation: { ...noProfileLine, run_state: 'none' } }} labels={stripLabels} />)
    expect(page.querySelector('[data-valuation-note]')!.getAttribute('data-run-state')).toBe('none')
  })

  it('TestPillIsPayloadWord', async () => {
    const c: Company = { ...withLine, count_parts: { fits: '5', check: '0', grey: '0' }, tier: { id: 'grey', word: 'GREYWORD', glyph: '–' } }
    const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
    const pill = page.querySelector('[data-pill]')!
    expect(pill.textContent).toBe('– GREYWORD')
    expect(pill.getAttribute('data-tier')).toBe('grey')
  })

  it('TestStripNoGateLine', async () => {
    for (const c of [withLine, withReason, withNull, withoutKey, notUsedCompany()]) {
      const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
      expect(page.querySelector('[data-gate]')).toBeNull()
      await act(async () => root?.unmount())
      host.remove()
    }
  })

  it('TestStripShowsReasonText', async () => {
    const page = await show(<CompanyStrip company={withReason} labels={stripLabels} />)
    const value = page.querySelector('[data-valuation]')!
    expect(value.textContent).toContain(withReason.valuation!.line_text)
    expect(value.textContent).not.toContain('$')
    expect(page.textContent).toContain(withReason.price_note!)
  })

  it('TestStripNoValuationUnchanged', async () => {
    for (const c of [withNull, withoutKey]) {
      const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
      expect(page.querySelector('[data-valuation]')).toBeNull()
      expect(page.querySelector('[data-gate]')).toBeNull()
      expect(page.textContent).toContain(c.buy_price!)
      await act(async () => root?.unmount())
      host.remove()
    }
  })

  it('TestCardLeftEdgeByTier', async () => {
    const want = { fits: 'border-l-success', check: 'border-l-warn', grey: 'border-l-muted-foreground' } as const
    for (const id of ['fits', 'check', 'grey'] as const) {
      const page = await show(<CompanyStrip company={{ ...withLine, tier: { id, word: 'W', glyph: 'G' } }} labels={stripLabels} />)
      const card = page.querySelector('[data-company]')!
      expect(card.className).toContain('border-l-4')
      expect(card.className).toContain(want[id])
      await act(async () => root?.unmount())
      host.remove()
    }
    const page = await show(<CompanyStrip company={{ ...withLine, tier: undefined }} labels={stripLabels} />)
    expect(page.querySelector('[data-company]')!.className).not.toMatch(/border-l-(success|warn|muted-foreground)/)
  })

  it('TestNumbersUseNumClass', async () => {
    const c = withLine
    const page = await show(<CompanyStrip company={c} labels={stripLabels} />)
    expect(page.querySelector('[data-buy] .num')!.textContent).toBe(c.buy_price)
    expect(page.querySelector('[data-spot] .num')!.textContent).toBe(c.spot_text)
    expect(page.querySelector('[data-earnings] .num')!.textContent).toBe(c.earnings_text)
    expect(page.querySelector('[data-valuation] .num')!.textContent).toBe(c.valuation!.line_text)
    expect([...page.querySelectorAll('[data-counts] .num')].map((e) => e.textContent)).toEqual(['2', '1', '1'])
  })

  it('TestStripShowsValuationSource', async () => {
    const page = await show(<CompanyStrip company={valuationSource} labels={stripLabels} />)
    expect(page.textContent).toContain('$91.20')
    const b = page.querySelector('[data-buy-source="valuation"]')!
    expect(b.textContent).toBe('Valuation buy zone')
    expect(page.querySelector(`[data-hint="${valuationSource.source_badge!.gloss}"]`)).not.toBeNull()
  })

  it('TestStripShowsHandSource', async () => {
    const page = await show(<CompanyStrip company={handSource} labels={stripLabels} />)
    expect(page.querySelector('[data-buy-source="hand"]')!.textContent).toBe('Hand price')
  })

  it('TestStripNotJudgedHasNoBadge', async () => {
    const page = await show(<CompanyStrip company={notJudgedSource} labels={stripLabels} />)
    expect(page.textContent).toContain(notJudgedSource.price_note!)
    expect(page.querySelector('[data-buy-source]')).toBeNull()
  })
})

function notUsedCompany(): Company {
  return { ...withLine, valuation: notUsedLine }
}
