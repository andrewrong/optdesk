// The expected texts are read from the synthetic fixtures; the page must show each as sent.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { OptionResultView, registration } from '@/pages/optionresult/view'
import { validate } from '@/types/option_result_page'
import { gap } from './fixtures/gap'
import { ok } from './fixtures/ok'
import { stale } from './fixtures/stale'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const badVersion: unknown = JSON.parse(
  readFileSync(path.join(import.meta.dirname, 'fixtures/bad-version.json'), 'utf8'),
)
let root: Root | undefined
let host: HTMLElement
async function show(payload: typeof ok | typeof gap | typeof stale): Promise<string> {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<OptionResultView payload={payload} />)
  })
  return host.textContent ?? ''
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

it('shows the title, facts, sections, reliability and notes of ok', async () => {
  const text = await show(ok)
  expect(text).toContain(ok.title)
  for (const fact of ok.facts) expect(text).toContain(fact.label + fact.value)
  for (const section of ok.sections) {
    expect(text).toContain(section.heading)
    for (const figure of section.figures) expect(text).toContain(figure.value)
    expect(text).toContain(section.months.points[0]!.text)
    expect(text).toContain(section.books_lines[1])
  }
  for (const block of ok.reliability) {
    expect(text).toContain(block.heading)
    for (const line of block.lines) expect(text).toContain(line)
  }
  for (const note of ok.notes) expect(text).toContain(note)
  expect(host.querySelector('[data-stale]')).toBeNull()
  expect(host.querySelector('[data-nothing]')).toBeNull()
})

it('shows the nothing line and the empty text of a gap', async () => {
  const text = await show(gap)
  expect(text).toContain(gap.nothing)
  expect(text).toContain(gap.sections[0]!.months.empty)
})

it('shows the stale label', async () => {
  const text = await show(stale)
  expect(text).toContain(stale.stale.text)
  expect(text).toContain(stale.title)
})

it('refuses a version it cannot read and registers the schema', () => {
  expect(validate(badVersion).ok).toBe(false)
  expect(validate(ok).ok).toBe(true)
  expect(registration.schema).toBe('option_result_page')
})
