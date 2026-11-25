import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { ScrollHint } from '@/components/ScrollHint'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('ScrollHint', () => {
  it('renders the hint with sm:hidden', async () => {
    const host = document.createElement('div')
    const root = createRoot(host)
    await act(async () => root.render(<ScrollHint />))
    const p = host.querySelector('p[data-scroll-hint]')
    expect(p?.textContent).toBe('左右滑动可看全部列')
    expect(p?.className).toBe('text-xs text-muted-foreground sm:hidden')
    await act(async () => root.unmount())
  })
})
