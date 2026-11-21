// The held family page prints the producer's text: hint under each label, rows in payload order, gap reasons, a minus kept.
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { HeldFamilyPage } from '@/pages/heldfamily/HeldFamilyPage'
import { held } from './fixtures/held'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

async function show() {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const root = createRoot(host)
  await act(async () => {
    root.render(<HeldFamilyPage payload={held} />)
  })
  return {
    host,
    done: async () => {
      await act(async () => root.unmount())
      host.remove()
    },
  }
}

describe('HeldFamilyPage', () => {
  it('shows the hint under each column label', async () => {
    const { host, done } = await show()
    for (const c of held.columns) {
      const hint = host.querySelector(`[data-hint="${c.id}"]`)
      expect(hint?.textContent).toBe(c.hint)
      expect(hint?.previousElementSibling?.textContent).toBe(c.label)
    }
    await done()
  })

  it('shows the rows in payload order, the sum and the lead note', async () => {
    const { host, done } = await show()
    const order = [...host.querySelectorAll('tbody tr')].map((r) => r.getAttribute('data-company'))
    expect(order).toEqual(held.rows.map((r) => r.company))
    expect(host.querySelector('[data-rows-sum]')?.textContent).toBe(held.rows_sum.text)
    expect(host.querySelector('[data-lead-notes]')?.textContent).toContain(held.lead_notes[0])
    await done()
  })

  it('shows a gap row with its reason in words', async () => {
    const { host, done } = await show()
    const row = host.querySelector('tr[data-company="DDD"]')
    expect(row?.textContent).toContain('gap: no mark')
    expect(row?.querySelectorAll('[data-gap="missing_quote"]').length).toBe(2)
    expect(host.querySelector('tr[data-company="EEE"]')?.textContent).toContain(
      'no date: the earnings job has no row for this company',
    )
    await done()
  })

  it('keeps the minus of a negative value', async () => {
    const { host, done } = await show()
    const row = host.querySelector('tr[data-company="EEE"]')
    expect(row?.textContent).toContain('-2,000')
    expect(row?.textContent).toContain('-0.6 %')
    await done()
  })
})
