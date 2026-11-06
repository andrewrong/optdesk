// Issue #1603: the /advice page shows the text fields of `advice_page` and nothing else. The fixtures of
// fixtures/advice-page/ are the independent reference; the browser computes nothing.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { AdvicePage, validateAdvicePage } from '@/pages/advice/AdvicePage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const fixture = (name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../fixtures/advice-page', name), 'utf8'),
  )
let root: Root | undefined
let host: HTMLElement
async function show(element: React.ReactElement) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(element)
  })
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('shows the delta and the acted word of the ok fixture', async () => {
  const payload = fixture('ok.json')
  expect(validateAdvicePage(payload).ok).toBe(true)
  expect(validateAdvicePage(fixture('bad-version.json')).ok).toBe(false)
  await show(<AdvicePage payload={payload} />)
  const first = host.querySelector('tbody tr')!.textContent!
  expect(first).toContain('-0.40')
  expect(first).toContain('已执行')
  expect(host.querySelectorAll('tbody tr')).toHaveLength(payload.rows.length)
})

it('shows the gap of the gap fixture once and no table row', async () => {
  const payload = fixture('gap.json')
  await show(<AdvicePage payload={payload} />)
  const text = host.textContent!
  expect(text.split(payload.gap.text)).toHaveLength(2)
  expect(host.querySelectorAll('tbody tr')).toHaveLength(0)
})
