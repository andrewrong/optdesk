// The home page shows the text fields of the payload and its five members as sent. Expected strings are read from
// the fixture, not written here.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { HomePage, registration } from '@/pages/home/HomePage'
import { ok } from './home-v2/fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const bad = (name: string) =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, 'home-v2/fixtures', name), 'utf8'))

let root: Root | null = null
let host: HTMLElement
beforeEach(async () => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(ok.generated_at))
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<HomePage payload={ok} />)
  })
})
afterEach(async () => {
  await act(async () => root?.unmount())
  host.remove()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('HomePage', () => {
  it('shows the title of each of the five members', () => {
    const text = host.textContent ?? ''
    for (const title of [
      ok.household.card.title,
      ok.companies.card.title,
      ok.macro.card.title,
      ok.market.card.title,
      ok.options.card.title,
    ]) {
      expect(title.length).toBeGreaterThan(0)
      expect(text).toContain(title)
    }
  })

  it('shows title, say and gap_line', () => {
    const text = host.textContent ?? ''
    expect(text).toContain(ok.title)
    expect(text).toContain(ok.say)
    expect(host.querySelector('[data-gap-line]')?.textContent).toBe(ok.gap_line)
  })

  it('renders block_links and report_links as links to their href', () => {
    const anchors = [...host.querySelectorAll('a')]
    for (const link of [...ok.block_links, ...ok.report_links]) {
      const found = anchors.find((a) => a.getAttribute('href') === link.href && a.textContent === link.text)
      expect(found, link.text).toBeDefined()
    }
  })
})

describe('registration', () => {
  it('is the home_page schema', () => {
    expect(registration.schema).toBe('home_page')
  })
  it('accepts ok', () => {
    expect(registration.page.validate(ok).ok).toBe(true)
  })
  it('refuses bad-version.json', () => {
    const result = registration.page.validate(bad('bad-version.json'))
    expect(result.ok).toBe(false)
  })
  it('refuses bad-member.json with a gap', () => {
    const result = registration.page.validate(bad('bad-member.json'))
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.gap.text.length).toBeGreaterThan(0)
  })
})
