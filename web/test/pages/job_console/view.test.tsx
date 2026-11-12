// The job console view shows the strings of the payload as sent. Payloads are the contract golden with one change each.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { registration } from '@/pages/job_console/view'
import type { Payload } from '@/types/job_console'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const golden = (): Payload =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../../contracts/fixtures/page.job_console.v1.golden.json'), 'utf8'),
  )
type Req = Payload['requests'][number]
const request = (over: Partial<Req>): Req => ({ ...golden().requests[0], ...over }) as Req
const withRequests = (...rs: Req[]): Payload => ({ ...golden(), requests: rs as Payload['requests'] })

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
  const View = registration.page.component as unknown as (props: { payload: Payload }) => React.ReactElement
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
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('job console view', () => {
  it('is registered as job_console', () => {
    expect(registration.schema).toBe('job_console')
  })

  it('renders the seven state badges', async () => {
    const states = ['queued', 'running', 'done', 'refused', 'failed', 'interrupted', 'withdrawn'] as const
    const el = await render(withRequests(...states.map((s, i) => request({ id: i + 1, state: s, state_text: `text of ${s}`, keys: [] }))))
    const badges = [...el.querySelectorAll('[data-state-badge]')].map((b) => [b.getAttribute('data-state-badge'), b.textContent])
    expect(badges).toEqual(states.map((s) => [s, s]))
  })

  it('shows produced, nothing found, unconfirmed, partial and not run lines with their reasons', async () => {
    const el = await render(golden())
    const row = (o: string) => [...el.querySelectorAll(`tr[data-outcome="${o}"]`)]
    const expected: [string, string][] = [
      ['produced', ''],
      ['nothing_found', 'no_value'],
      ['unconfirmed', 'postcondition'],
      ['partial', 'cut'],
      ['not_run', 'slot_near'],
    ]
    for (const [outcome, reason] of expected) {
      const rows = row(outcome)
      expect(rows.length, outcome).toBeGreaterThan(0)
      expect(rows.some((r) => r.querySelector('[data-reason]')?.textContent === reason)).toBe(true)
    }
    expect(el.textContent).toContain('cut after 3 of 7 rows; counts shown')
  })

  it('shows the mark text beside the outcome', async () => {
    const el = await render(golden())
    const marks = [...el.querySelectorAll('[data-present]')].map((c) => c.textContent)
    expect(marks).toContain('the key is present now')
    expect(marks).toContain('the key is missing')
  })

  it('shows the re-run text for a re-run line', async () => {
    const el = await render(golden())
    expect(el.textContent).toContain(
      're-run returned 12 rows; the key was already present and the page cannot tell whether anything new was stored',
    )
  })

  it('shows runs at HH:MM UTC for a not due line', async () => {
    const p = withRequests(
      request({
        state: 'refused',
        reason: 'not_due',
        keys: [
          { dataset: 'quotes', subject: 'US:1530', outcome: 'waiting', outcome_text: 'runs at 15:30 UTC', present: 'the key is missing', reason: 'not_due' },
        ],
      }),
    )
    const el = await render(p)
    expect(el.querySelector('tr[data-outcome="waiting"]')?.textContent).toContain('runs at 15:30 UTC')
  })

  it('shows the held and withdrawal pending texts', async () => {
    const held = 'starts after 15:30 UTC (the slot of quote_snapshot)'
    const pending = "withdrawal pending: it takes effect when the scheduler's worker is next free"
    const el = await render(
      withRequests(request({ id: 2, state: 'queued', state_text: held, keys: [] }), request({ id: 1, state: 'queued', state_text: pending, keys: [] })),
    )
    const texts = [...el.querySelectorAll('[data-state-text]')].map((t) => t.textContent)
    expect(texts).toEqual([held, pending])
    const withdraw = [...el.querySelectorAll('form[data-withdraw-form]')]
    expect(withdraw).toHaveLength(2)
    const form = withdraw[0] as HTMLElement
    expect(form.getAttribute('method')).toBe('post')
    expect(form.querySelector<HTMLInputElement>('input[name="action"]')?.value).toBe('withdraw')
    expect(form.querySelector<HTMLInputElement>('input[name="id"]')?.value).toBe('2')
  })

  it('shows the nothing was produced line', async () => {
    const p = { ...withRequests(request({ keys: [] })), sentence: 'nothing was produced' }
    const el = await render(p)
    expect(el.querySelector('[data-sentence]')?.textContent).toBe('nothing was produced')
  })

  it('shows no form and the reason for a listed-only job', async () => {
    const p = golden()
    const job = p.jobs.find((j) => j.kind === 'listed_only')!
    const el = await render(p)
    const row = el.querySelector(`tr[data-job="${job.id}"]`)!
    expect(row.querySelector('form')).toBeNull()
    expect(row.textContent).toContain(job.why_text)
    expect(row.textContent).toContain(job.needs_text)
  })

  it('shows no force switch and the reason for a force-no job', async () => {
    const p = golden()
    const el = await render(p)
    const no = el.querySelector('tr[data-job="compute_figures"]')!
    expect(no.querySelector('[name="force"]')).toBeNull()
    expect(no.querySelector('[data-force-why]')?.textContent).toBe(p.jobs[0]?.force_why)
    const yes = el.querySelector('tr[data-job="calendar_refresh"]')!
    expect(yes.querySelector('[name="force"]')).not.toBeNull()
  })

  it('fixes the day of a today-only job to today', async () => {
    const el = await render(golden())
    const day = (id: string) => el.querySelector<HTMLInputElement>(`tr[data-job="${id}"] input[name="day"]`)!
    expect(day('option_eod_jp').value).toBe('2026-11-13')
    expect(day('option_eod_jp').readOnly).toBe(true)
    expect(day('compute_figures').value).toBe('2026-11-12')
    expect(day('compute_figures').readOnly).toBe(false)
    const form = el.querySelector('form[data-run-form="compute_figures"]')!
    for (const n of ['action', 'job', 'expires', 'mac']) expect(form.querySelector(`input[type="hidden"][name="${n}"]`), n).not.toBeNull()
    expect(form.querySelector('button')?.textContent).toBe('Run')
  })

  it('shows the note beside the form of a job', async () => {
    const el = await render(golden())
    expect(el.querySelector('tr[data-job="quote_snapshot"]')?.textContent).toContain('share one scheduled instant')
  })

  it('reloads after refresh_seconds', async () => {
    vi.useFakeTimers()
    const reload = vi.fn()
    vi.stubGlobal('location', { reload })
    await render({ ...golden(), refresh_seconds: 5 })
    vi.advanceTimersByTime(4999)
    expect(reload).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('sets no timer when refresh_seconds is 0', async () => {
    vi.useFakeTimers()
    const reload = vi.fn()
    vi.stubGlobal('location', { reload })
    await render({ ...golden(), refresh_seconds: 0 })
    vi.advanceTimersByTime(60000)
    expect(reload).not.toHaveBeenCalled()
  })
})
