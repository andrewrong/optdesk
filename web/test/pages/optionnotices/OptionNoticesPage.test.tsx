// The fixtures are written from the schema alone; each text asserted is read from a fixture field.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { OptionNoticesPage, registration, validateOptionNoticesPage } from '@/pages/notices/OptionNoticesPage'
import { pages } from '@/shell/pages'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { secondMissing } from './fixtures/second-missing'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: typeof ok | typeof secondMissing | typeof gap) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionNoticesPage payload={payload as never} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

describe('OptionNoticesPage', () => {
  it('shows the notices of each alias in payload order with a severity badge, the main alias first', async () => {
    const page = await show(ok)
    const blocks = [...page.querySelectorAll('[data-alias]')]
    expect(blocks.map((b) => b.getAttribute('data-alias'))).toEqual(ok.accounts.map((a) => a.alias))
    const rows = [...blocks[0]!.querySelectorAll('[data-notice]')]
    expect(rows.map((r) => r.querySelector('[data-notice-title]')!.textContent)).toEqual(
      ok.accounts[0]!.notices.map((n) => n.title),
    )
    rows.forEach((row, i) => {
      const badge = row.querySelector('[data-slot=badge][data-severity]')!
      expect(badge.textContent).toBe(
        { info: 'Info', warn: 'Warning', act: 'Action needed' }[ok.accounts[0]!.notices[i]!.severity],
      )
      expect(badge.getAttribute('data-severity')).toBe(ok.accounts[0]!.notices[i]!.severity)
    })
    expect(page.textContent).toContain(ok.accounts[0]!.notices[0]!.facts[0]!.text)
    expect(page.textContent).toContain(ok.accounts[0]!.notices[0]!.reasons[0]!)
    expect(page.textContent).toContain(ok.title)
    expect(page.querySelector('[data-stale]')!.textContent).toContain(ok.stale.text)
  })

  it('shows the status badge as a word and keeps the code in data-status', async () => {
    const okPage = await show(ok)
    const okBadge = okPage.querySelector('[data-alias=ok-main] [data-status]')!
    expect(okBadge.textContent).toBe('Current')
    expect(okBadge.getAttribute('data-status')).toBe('ok')
    await act(async () => root?.unmount())
    host.remove()
    const missing = await show(secondMissing)
    const badge = missing.querySelector('[data-status=no_capture]')!
    expect(badge.textContent).toBe('No capture')
  })

  it('shows the cash by date table in payload order and the cushion text', async () => {
    const page = await show(ok)
    const block = page.querySelector('[data-alias=ok-main]')!
    const days = [...block.querySelectorAll('[data-cash] tbody tr')].map((tr) => tr.querySelector('td')!.textContent)
    expect(days).toEqual(ok.accounts[0]!.cash_by_date.map((r) => r.day))
    const amounts = [...block.querySelectorAll('[data-cash] tbody tr')].map((tr) => tr.lastElementChild!.textContent)
    expect(amounts).toEqual(ok.accounts[0]!.cash_by_date.map((r) => r.amount_text))
    expect(block.querySelector('[data-cushion]')!.textContent).toBe(ok.accounts[0]!.cushion_text)
    expect(page.querySelector('[data-alias=ok-second] table')).toBeNull()
  })

  it('shows a no_capture block with its note and no table', async () => {
    const page = await show(secondMissing)
    const block = page.querySelector('[data-alias=miss-second]')!
    expect(block.querySelector('[data-note]')!.textContent).toContain(secondMissing.accounts[1]!.note)
    expect(block.querySelector('table')).toBeNull()
    expect(page.querySelector('[data-stale]')).toBeNull()
  })

  it('shows the page gap cause once', async () => {
    const page = await show(gap)
    expect(page.querySelectorAll('[data-page-gap]')).toHaveLength(1)
    expect((page.textContent ?? '').split(gap.gaps[0]!.text)).toHaveLength(2)
    expect(page.querySelector('[data-alias]')).toBeNull()
  })

  it('fails validation on a bad version and passes the fixtures', () => {
    expect(validateOptionNoticesPage(badVersion).ok).toBe(false)
    for (const payload of [ok, secondMissing, gap]) expect(validateOptionNoticesPage(payload).ok).toBe(true)
  })

  it('is registered once under the schema name', () => {
    expect(registration.schema).toBe('option_notices_page')
    expect(Object.keys(pages).filter((name) => name === 'option_notices_page')).toHaveLength(1)
  })
})
