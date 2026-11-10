// StaleAlert compares the device clock with `stale_after` and flags a clock far behind `generated_at`.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { StaleAlert } from '@/pages/home/StaleAlert'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const STALE = { text: '数据截至 10-23 收盘', stale_after: '2026-10-24T00:00:00Z' }
let root: Root | null = null

async function show(now: string, generatedAt: string): Promise<HTMLElement> {
  vi.setSystemTime(new Date(now))
  const host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<StaleAlert stale={STALE} generatedAt={generatedAt} />)
  })
  return host
}

beforeEach(() => {
  vi.useFakeTimers()
})
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.replaceChildren()
  vi.useRealTimers()
})

describe('StaleAlert', () => {
  it('is grey text before stale_after', async () => {
    const host = await show('2026-10-23T12:00:00Z', '2026-10-23T11:59:00Z')
    const notice = host.querySelector('[data-stale]') as HTMLElement
    expect(notice.getAttribute('data-stale')).toBe('false')
    expect(notice.className).toContain('text-muted-foreground')
    expect(notice.textContent).toBe(STALE.text)
    expect(host.querySelector('[data-clock-skew]')).toBeNull()
  })

  it('is the destructive alert after stale_after', async () => {
    const host = await show('2026-10-24T00:00:01Z', '2026-10-23T11:59:00Z')
    const notice = host.querySelector('[data-stale]') as HTMLElement
    expect(notice.getAttribute('data-stale')).toBe('true')
    expect(notice.getAttribute('role')).toBe('alert')
    expect(host.querySelector('[data-clock-skew]')).toBeNull()
  })

  it('flags a clock more than 5 minutes behind generated_at', async () => {
    const host = await show('2026-10-23T12:00:00Z', '2026-10-23T12:05:01Z')
    expect(host.querySelector('[data-clock-skew]')).not.toBeNull()
  })

  it('does not flag a clock 5 minutes behind or ahead', async () => {
    const host = await show('2026-10-23T12:00:00Z', '2026-10-23T12:05:00Z')
    expect(host.querySelector('[data-clock-skew]')).toBeNull()
  })
})
