// Issue #1209: the weather card shows the text fields of the market_weather payload and nothing else. The fixtures
// are those of fixtures/market-weather/; expected strings are read from them, not from the component.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { registration, WeatherCard } from '@/pages/home/WeatherCard'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(
      path.resolve(import.meta.dirname, '../../../fixtures/market-weather', `${name}.json`),
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
    root.render(<WeatherCard payload={payload as never} />)
  })
}
const voteRows = () => [...host.querySelectorAll('[data-vote]')] as HTMLElement[]
const tableRows = () =>
  [...host.querySelectorAll('[data-mismatches] tbody tr')].map((r) =>
    [...r.querySelectorAll('td')].map((c) => c.textContent),
  )
// The badge words of the three verdicts (board v2.2: Good/Bad/Unknown).
const WORD = { ok: 'Good', bad: 'Bad', unknown: 'Unknown' } as const
// jsdom has no ResizeObserver, which the Radix tooltip popper needs once it opens.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})

describe('WeatherCard', () => {
  it('is registered for the schema market_weather', () => {
    expect(registration.schema).toBe('market_weather')
  })

  it('shows three vote rows with verdict badge, source and observed date from the ok fixture', async () => {
    const ok = fixture('ok')
    await show(ok)
    const rows = voteRows()
    expect(rows).toHaveLength(3)
    ok.rows.forEach(
      (
        row: { verdict: keyof typeof WORD; source: string; observed: string; figure_text: string },
        i: number,
      ) => {
        expect(rows[i]!.querySelector('[data-slot=badge]')?.textContent).toBe(WORD[row.verdict])
        expect(rows[i]!.querySelector('[data-source]')?.textContent).toBe(row.source)
        expect(rows[i]!.querySelector('[data-observed]')?.textContent).toBe(row.observed)
        expect(rows[i]!.textContent).toContain(row.figure_text)
      },
    )
    expect(host.textContent).toContain(ok.headline)
    expect(host.textContent).toContain(ok.footnote)
  })

  it('renders no table when there is no mismatch', async () => {
    await show(fixture('ok'))
    expect(host.querySelector('table')).toBeNull()
  })

  it('renders one table row per mismatch line with both values and the diff', async () => {
    const mismatch = fixture('mismatch')
    await show(mismatch)
    const rows = tableRows()
    expect(rows).toHaveLength(mismatch.mismatches.length)
    mismatch.mismatches.forEach(
      (
        line: {
          day: string
          ibkr: { text: string }
          fred: { text: string }
          diff: { text: string }
        },
        i: number,
      ) => {
        expect(rows[i]).toEqual([line.day, line.ibkr.text, line.fred.text, line.diff.text])
      },
    )
  })

  it('heads the difference column Diff, not with the verdict word Bad', async () => {
    await show(fixture('mismatch'))
    const heads = [...host.querySelectorAll('[data-mismatches] thead th')].map((h) => h.textContent)
    expect(heads).toContain('Diff')
    expect(heads).not.toContain('Bad')
  })

  it('shows the stale label of the stale fixture and none for ok', async () => {
    const stale = fixture('stale')
    await show(stale)
    expect(host.querySelector('[data-stale]')?.textContent).toBe(stale.stale.text)
    await act(async () => root?.unmount())
    host.remove()
    await show(fixture('ok'))
    expect(host.querySelector('[data-stale]')).toBeNull()
  })

  it('shows the gap sentence once and no vote rows for the gap fixture', async () => {
    const gap = fixture('gap')
    await show(gap)
    expect(voteRows()).toHaveLength(0)
    expect(host.textContent?.split(gap.headline).length).toBe(2)
  })

  it('passes figures through unchanged, without formatting or comparing', async () => {
    const mismatch = fixture('mismatch')
    mismatch.mismatches[0].ibkr.text = '1e2'
    mismatch.mismatches[0].diff.text = '25.00'
    mismatch.rows[0].figure_text = '1e2'
    await show(mismatch)
    expect(tableRows()[0]).toEqual([
      mismatch.mismatches[0].day,
      '1e2',
      mismatch.mismatches[0].fred.text,
      '25.00',
    ])
    expect(voteRows()[0]!.textContent).toContain('1e2')
  })

  it('lets keyboard users focus the source and read its first IBKR date', async () => {
    const ok = fixture('ok')
    await show(ok)
    const source = host.querySelector<HTMLElement>('[data-source]')!

    await act(async () => {
      source.focus()
    })

    expect(document.activeElement).toBe(source)
    // Expected date comes from the Python-generated fixture.
    expect(document.body.textContent).toContain(ok.rows[0].first_ibkr_date)
  })
})
