import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { BarsSourceNote } from '@/components/BarsSourceNote'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const mixed = '日线含 Yahoo 数据，当日指标可能基于 Yahoo 价格；IB 历史数据恢复后自动改回 IB 数据'

async function html(note?: string): Promise<string> {
  const host = document.createElement('div')
  const root = createRoot(host)
  await act(async () => {
    root.render(<BarsSourceNote note={note} />)
  })
  return host.innerHTML
}

describe('BarsSourceNote', () => {
  it('renders a mixed note as text', async () => {
    const host = document.createElement('div')
    host.innerHTML = await html(mixed)
    expect(host.textContent).toBe(mixed)
  })
  it('renders nothing for an empty or absent note', async () => {
    expect(await html('')).toBe('')
    expect(await html()).toBe('')
  })
})
