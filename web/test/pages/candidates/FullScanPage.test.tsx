// Issue #1801: the full-scan page from the generated fixtures; every string is the payload's.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { FullScanPage, validateFullScan } from '@/pages/candidates/FullScanPage'
import { pages } from '@/shell/pages'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const fixture = (name: string) =>
  JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../../fixtures/full-scan', name), 'utf8'),
  )
let root: Root | null = null
let host: HTMLElement
async function show(payload: unknown) {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<FullScanPage payload={payload as never} />)
  })
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})
afterEach(async () => {
  vi.unstubAllGlobals()
  await act(async () => root?.unmount())
  root = null
  host?.remove()
})

describe('FullScanPage', () => {
  it('is registered as full_scan and the fixtures validate', () => {
    expect(Object.keys(pages)).toContain('full_scan')
    for (const name of ['ok.json', 'stale.json', 'gap.json']) {
      expect(validateFullScan(fixture(name)).ok).toBe(true)
    }
    for (const name of ['bad-version']) {
      expect(validateFullScan(fixture(`${name}.json`)).ok).toBe(false)
    }
  })

  it('shows one table per section that has rows, with the payload rows and strings only', async () => {
    const payload = fixture('ok.json')
    await show(payload)
    const [put, call] = payload.sections
    const putTable = host.querySelector('[data-section=short_put] [data-table=short_put]')!
    expect(putTable.querySelectorAll('tbody tr')).toHaveLength(put.table.rows.length)
    expect(putTable.querySelectorAll('thead th')).toHaveLength(put.table.columns.length)
    const callTable = host.querySelector('[data-section=covered_call] [data-table=covered_call]')!
    expect(callTable.querySelectorAll('tbody tr')).toHaveLength(call.table.rows.length)
    expect(host.querySelectorAll('[data-table]')).toHaveLength(3)
    expect(host.querySelector('[data-section=short_put] [data-liquidity]')!.textContent).toBe(
      'SYN2 缺报价，年化、Bid、Ask 无法给出',
    )
    expect(host.querySelector('[data-back]')!.getAttribute('href')).toBe('/report/us-open')
    expect(host.textContent).toContain('成交量、未平仓量和 IV 分位没有存入扫描')
  })

  it('gives each strategy section the twelve columns of board 17, the missing row and the E text', async () => {
    await show(fixture('ok.json'))
    const twelve = ['标的', '判定', '年化', 'strike %', 'DTE', 'Δ', 'Bid/Ask', 'spread %', 'Vol', 'OI', 'IVP', 'E']
    for (const name of ['short_put', 'covered_call', 'put_credit_spread']) {
      const table = host.querySelector(`[data-section=${name}] [data-table=${name}]`)!
      const headers = [...table.querySelectorAll('thead th')].map((th) => th.textContent!.trim())
      expect(headers).toEqual(twelve)
      expect(headers.some((h) => /collar|roll|领口|展期/i.test(h))).toBe(false)
    }
    const put = host.querySelector('[data-section=short_put]')!
    const rows = [...put.querySelectorAll('tbody tr')].map((tr) =>
      [...tr.querySelectorAll('td')].map((td) => td.textContent!.trim()),
    )
    expect(rows.map((r) => r[11])).toEqual(['E 10-28', '—'])
    const missing = rows.find((r) => r[0] === 'SYN2')!
    expect(missing[2]).toContain('—')
    expect(missing[2]).toContain('缺')
    expect(missing[6]).toContain('—')
    expect(missing[6]).toContain('缺')
    expect(put.textContent!.split('缺报价')).toHaveLength(2) // the cause, once
    expect(rows.find((r) => r[0] === 'SYN1')![6]).toBe('1.20 / 1.30')
  })

  it('shows a section Gap once and no table for it', async () => {
    await show(fixture('gap.json'))
    expect(host.querySelectorAll('[data-table]')).toHaveLength(0)
    const gaps = [...host.querySelectorAll('[data-section-gap]')]
    expect(gaps.map((g) => g.textContent)).toEqual([
      '今天没有候选扫描',
      '今天没有候选扫描',
      '今天没有候选扫描',
    ])
    for (const section of host.querySelectorAll('[data-section]')) {
      expect(section.textContent!.split('今天没有候选扫描')).toHaveLength(2) // once per section
    }
  })
})
