// Issue #1341: a missing table cell shows a dash; the reason Python sent stays as screen-reader text of the
// same button and opens in a popover on tap.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { MissingValue } from '@/components/data-table/missing-value'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null
let host: HTMLElement
async function show(text: string) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<MissingValue text={text} />)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host.remove()
})

describe('MissingValue', () => {
  it('shows a dash on one button and keeps the reason as sr-only text', async () => {
    await show('数据源未接入')
    const button = host.querySelector('button')!
    expect(button.textContent).toContain('—')
    const hidden = button.querySelector('.sr-only')!
    expect(hidden.textContent).toBe('数据源未接入')
    expect(host.querySelectorAll('button')).toHaveLength(1)
  })

  it('opens the reason in a popover on tap', async () => {
    await show('数据源未接入')
    await act(async () => host.querySelector('button')!.click())
    const popover = document.body.querySelector('[data-slot="popover-content"]')
    expect(popover?.textContent).toBe('数据源未接入')
  })
})
