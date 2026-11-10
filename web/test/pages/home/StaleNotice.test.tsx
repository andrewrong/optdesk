// The stale notice shows the text it is given and carries the flag of the producer in data-stale.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { StaleNotice } from '@/pages/home/StaleNotice'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
async function show(text: string, isStale: boolean): Promise<HTMLElement> {
  const host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<StaleNotice text={text} isStale={isStale} />)
  })
  return host.querySelector('[data-stale]') as HTMLElement
}

afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  document.body.replaceChildren()
})

describe('StaleNotice', () => {
  it('flags a stale label and shows its text', async () => {
    const alert = await show('数据已过期，截至 10-23 收盘', true)
    expect(alert.getAttribute('data-stale')).toBe('true')
    expect(alert.getAttribute('role')).toBe('alert')
    expect(alert.querySelector('svg')).not.toBeNull()
    expect(alert.textContent).toBe('数据已过期，截至 10-23 收盘')
  })

  it('does not flag a fresh label and shows its text', async () => {
    const alert = await show('数据截至 10-23 收盘', false)
    expect(alert.getAttribute('data-stale')).toBe('false')
    expect(alert.tagName).toBe('P')
    expect(alert.className).toContain('text-muted-foreground')
    expect(alert.getAttribute('role')).toBeNull()
    expect(alert.querySelector('svg')).toBeNull()
    expect(alert.textContent).toBe('数据截至 10-23 收盘')
  })
})
