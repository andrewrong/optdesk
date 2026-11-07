// The choices fixture of contracts/fixtures is the reference: label and href are shown as written, in order.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { UnderlyingLinks } from '@/pages/optiondata/UnderlyingLinks'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const dir = path.resolve(import.meta.dirname, '../../../../contracts/fixtures')
const choices: { id: number; label: string; href: string; current: boolean }[] = JSON.parse(
  readFileSync(path.join(dir, 'page.option_data_page.v1.choices.json'), 'utf8'),
).underlyings

let root: Root | undefined
let host: HTMLElement
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})
async function show(items: typeof choices) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<UnderlyingLinks underlyings={items} />)
  })
}
const anchors = () => [...host.querySelectorAll('a')]

describe('UnderlyingLinks', () => {
  it('shows one anchor per choice with the fixture hrefs, in order', async () => {
    await show(choices)
    expect(anchors()).toHaveLength(3)
    expect(anchors().map((a) => a.getAttribute('href'))).toEqual(choices.map((c) => c.href))
    expect(anchors().map((a) => a.textContent)).toEqual(
      choices.map((c) => `${c.label}${c.id}`),
    )
  })

  it('marks aria-current on the current choice only', async () => {
    await show(choices)
    const marked = anchors().filter((a) => a.getAttribute('aria-current') === 'page')
    expect(marked.map((a) => a.getAttribute('href'))).toEqual(
      choices.filter((c) => c.current).map((c) => c.href),
    )
  })

  it('keeps two choices with one label apart by the id', async () => {
    await show([
      { id: 1, label: 'Same', href: '/a', current: false },
      { id: 2, label: 'Same', href: '/b', current: false },
    ])
    expect(anchors().map((a) => a.textContent)).toEqual(['Same1', 'Same2'])
  })
})
