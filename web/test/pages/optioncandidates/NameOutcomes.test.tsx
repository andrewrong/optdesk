import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { CandidateGroup } from '@/pages/candidates/CandidateGroup'
import { jp } from './fixtures/jp'
import { nameOutcomes } from './fixtures/nameoutcomes'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | undefined
let host: HTMLElement
async function show(node: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(node)
  })
  return host
}
afterEach(async () => {
  await act(async () => root?.unmount())
  root = undefined
  host?.remove()
})

const jpGroup = jp.groups[1] as never as Parameters<typeof CandidateGroup>[0]['group']

describe('NameOutcomes', () => {
  it('TestNameOutcomesErrorClass', async () => {
    const page = await show(<CandidateGroup group={{ ...jpGroup, name_outcomes: nameOutcomes }} staleText={null} />)
    const items = [...page.querySelectorAll('[data-name-outcome]')]
    expect(items.map((i) => i.textContent)).toEqual(nameOutcomes.map((o) => `${o.name} ${o.text} ${o.counts}`))
    const noQuote = page.querySelector('[data-name-outcome="no_quote"]')!
    const failed = page.querySelector('[data-name-outcome="failed"]')!
    expect(noQuote.className).not.toContain('destructive')
    expect(failed.className).toContain('destructive')
  })

  it('TestNameOutcomesAbsentDrawsAsBefore', async () => {
    const page = await show(<CandidateGroup group={jpGroup} staleText={null} />)
    expect(page.querySelector('[data-name-outcomes]')).toBeNull()
    expect(page.querySelector('[data-group-currency]')).not.toBeNull()
  })
})
