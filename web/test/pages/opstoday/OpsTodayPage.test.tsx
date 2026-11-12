// The ops today page shows the producer's rows as sent. Expected strings are read from the fixtures.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { OpsTodayPage, registration } from '@/pages/ops_today/OpsTodayPage'
import { ok, stale } from './fixtures/ok'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: typeof ok | typeof stale) {
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
    root = createRoot(host as HTMLElement)
    root.render(<OpsTodayPage payload={payload as never} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  vi.unstubAllGlobals()
})

describe('OpsTodayPage', () => {
  it('is registered as ops_today_page and accepts a payload of the schema', () => {
    expect(registration.schema).toBe('ops_today_page')
    expect(registration.page.validate(ok).ok).toBe(true)
    expect(registration.page.validate({ schema_version: 99 }).ok).toBe(false)
  })

  it('shows one table row per producer row, in the producer order, with its texts', async () => {
    const page = await render(ok)
    expect(page.textContent).toContain(ok.title)
    const rows = [...page.querySelectorAll('tbody tr')]
    expect(rows).toHaveLength(ok.today.rows.length)
    ok.today.rows.forEach((row, i) => {
      const text = rows[i]!.textContent ?? ''
      for (const value of [row.dataset, row.key, row.market, row.status, row.detail ?? '']) {
        expect(text).toContain(value)
      }
      expect(text).toContain(String(row.returned_n))
    })
  })

  it('marks status and reason code with badges that carry the producer text', async () => {
    const page = await render(ok)
    const badges = [...page.querySelectorAll('tbody tr')[1]!.querySelectorAll('[data-slot=badge]')].map(
      (b) => b.textContent,
    )
    expect(badges).toContain(ok.today.rows[1]!.status)
    expect(badges).toContain(ok.today.rows[1]!.reason_code)
  })

  it('shows the stale label of the producer and nothing when it is null', async () => {
    const fresh = await render(ok)
    expect(fresh.querySelector('[data-stale]')).toBeNull()
    await act(async () => root?.unmount())
    fresh.remove()
    const page = await render(stale)
    expect(page.textContent).toContain(stale.stale.text)
  })
})
