// The gateway block of the data health page prints each gateway as sent (docs/design/gateway-health.md#8-gateway-health-web).
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as pageFolder from '@/pages/data_health/DataHealthPage'
import type { Payload } from '@/types/data_health_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/page.data_health_page.v1.${name}.json`), 'utf8'),
  )

let root: Root | null = null
let host: HTMLElement | null = null
async function render(payload: Payload) {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  const View = pageFolder.registration.page.component as unknown as (props: { payload: Payload }) => React.ReactElement
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host as HTMLElement)
    root.render(<View payload={payload} />)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  vi.unstubAllGlobals()
})

describe('gateway block', () => {
  it('prints both gateways with their verdict text and the down farm', async () => {
    const el = await render(fixture('gateways'))
    const block = el.querySelector('[data-gateways]')
    expect(block).not.toBeNull()
    const main = block!.querySelector('[data-gateway="main"]')
    const wife = block!.querySelector('[data-gateway="wife"]')
    expect(main).not.toBeNull()
    expect(wife).not.toBeNull()
    expect(main!.textContent).toContain('历史行情服务 ushmds 中断：请重新登录这个网关；重连没有用。')
    expect(wife!.textContent).toContain('这个网关没有回答。')
    expect(main!.querySelector('[data-farm="usfarm"]')?.textContent).toContain('ok')
    const down = main!.querySelector('[data-farm="ushmds"]')
    expect(down?.getAttribute('data-farm-state')).toBe('down')
    expect(down?.textContent).toContain('down')
    expect(wife!.querySelectorAll('[data-farm]')).toHaveLength(0)
  })

  it('shows no gateway block for the ok fixture or an empty list', async () => {
    const p = fixture('ok')
    expect((await render(p)).querySelector('[data-gateways]')).toBeNull()
    await act(async () => root?.unmount())
    host?.remove()
    p.gateways = []
    expect((await render(p)).querySelector('[data-gateways]')).toBeNull()
  })
})
