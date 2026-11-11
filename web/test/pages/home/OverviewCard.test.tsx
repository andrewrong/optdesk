// The overview card shows the payload's overview as sent: one line per card with its label as a link, the text as
// given, and the gaps or the gap line. Expected strings are read from the fixture.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { OverviewCard } from '@/pages/home/OverviewCard'
import { ok } from './home-v2/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement
async function show(payload: typeof ok) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OverviewCard payload={payload} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.innerHTML = ''
})
const lines = () => [...host.querySelectorAll('[data-overview-item]')]

describe('OverviewCard', () => {
  it('shows five lines in payload order, each label a link to its href and each text as sent', async () => {
    await show(ok)
    expect(lines()).toHaveLength(5)
    lines().forEach((line, i) => {
      const item = ok.overview.items[i]!
      const a = line.querySelector('a')
      expect(a?.getAttribute('href')).toBe(item.href)
      expect(a?.textContent).toBe(item.label)
      expect(line.querySelector('[data-overview-text]')?.textContent).toBe(item.text)
    })
  })

  it('shows the title and say of the page', async () => {
    await show(ok)
    expect(host.querySelector('[data-slot=card-title]')?.textContent).toBe(ok.title)
    expect(host.textContent).toContain(ok.say)
  })

  it('lists each gap text in the grey gap line when there are gaps', async () => {
    await show(ok)
    const alert = host.querySelector('[data-gap-line]')!
    expect(ok.overview.gaps.length).toBeGreaterThan(0)
    for (const gap of ok.overview.gaps) {
      expect(alert.textContent).toContain(gap.text)
      expect(alert.textContent).toContain(gap.card)
    }
    expect(alert.querySelectorAll('[data-overview-gaps] li')).toHaveLength(ok.overview.gaps.length)
  })

  it('shows gap_line and no list when there is no gap', async () => {
    await show({ ...ok, gap_line: 'No gaps today', overview: { ...ok.overview, gaps: [] as never } })
    const alert = host.querySelector('[data-gap-line]')!
    expect(alert.textContent).toBe('No gaps today')
    expect(alert.querySelector('[data-overview-gaps]')).toBeNull()
  })

  it('shows a changed text as it is', async () => {
    const changed = structuredClone(ok)
    changed.overview.items[0].text = 'Another sentence'
    await show(changed)
    expect(lines()[0]!.querySelector('[data-overview-text]')?.textContent).toBe('Another sentence')
  })
})
