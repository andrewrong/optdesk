// Issue #1126: the promoted DataTable sorts on `value`, shows `text` as sent, shows a gap cell's text in a Badge
// and applies the tone class that Python wrote.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { DataTable, type Cell } from '@/components/data-table/data-table'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const COLUMNS = [
  { id: 'name', label: '名称' },
  { id: 'amount', label: '金额', align: 'right' as const },
]
// Invented rows. By text the amounts sort "10", "9", "—", by value 9 before 10 (the gap has none and goes last).
const ROWS: Record<string, Cell>[] = [
  { name: { text: 'ten' }, amount: { text: '10', value: 10, tone: 'up' } },
  { name: { text: 'nine' }, amount: { text: '9', value: 9, tone: 'down' } },
  { name: { text: 'none' }, amount: { reason: 'no_data', text: '无数据' } },
  { name: { text: 'five' }, amount: { text: '5', value: 5, tone: 'flat' } },
  // A JPY cell with its Python-made ≈USD text (issue #1362): shown beside the amount, not computed.
  {
    name: { text: 'yen' },
    amount: { text: '¥12,300', value: 12300, approx_usd: { text: '≈$82' } },
  },
]

let root: Root | null = null
let host: HTMLElement
async function show() {
  host = document.createElement('div')
  document.body.append(host)
  await act(async () => {
    root = createRoot(host)
    root.render(<DataTable columns={COLUMNS} rows={ROWS} />)
  })
}
const names = () =>
  [...host.querySelectorAll('[data-slot="table-body"] tr')].map(
    (r) => r.firstElementChild?.textContent,
  )
afterEach(async () => {
  await act(async () => root?.unmount())
  root = null
  host.remove()
})

describe('DataTable sharp mark', () => {
  it('shows a destructive Badge with the mark text beside the figure and none without a mark', async () => {
    host = document.createElement('div')
    document.body.append(host)
    const rows: Record<string, Cell>[] = [
      { name: { text: 'hot' }, amount: { text: '+4.0%', value: 4, sharp: { text: '大幅' } } },
      { name: { text: 'calm' }, amount: { text: '+1.0%', value: 1, sharp: null } },
      { name: { text: 'plain' }, amount: { text: '+0.5%', value: 0.5 } },
    ]
    await act(async () => {
      root = createRoot(host)
      root.render(<DataTable columns={COLUMNS} rows={rows} />)
    })
    const badges = [...host.querySelectorAll('[data-sharp]')]
    expect(badges.map((b) => b.textContent)).toEqual(['大幅'])
    expect(badges[0]?.closest('td')?.textContent).toBe('+4.0% 大幅')
    expect(badges[0]?.getAttribute('data-variant')).toBe('destructive')
    expect(badges[0]?.querySelector('svg')).not.toBeNull()
  })
})

describe('DataTable', () => {
  it('sorts a column by value and not by text', async () => {
    await show()
    expect(names()).toEqual(['ten', 'nine', 'none', 'five', 'yen']) // as sent
    const button = host.querySelectorAll('[data-slot="table-header"] button')[0] as HTMLElement
    await act(async () => button.click())
    expect(names()).toEqual(['five', 'nine', 'ten', 'yen', 'none']) // 5, 9, 10, 12300: text order would put "10" first
    await act(async () => button.click())
    expect(names().filter((n) => n !== 'none')).toEqual(['yen', 'ten', 'nine', 'five']) // descending
  })

  it('shows a gap cell as a dash with its text as sr-only and shows no zero for it', async () => {
    await show()
    const gap = host.querySelector('[data-missing]')
    expect(gap?.textContent).toContain('—')
    expect(gap?.querySelector('.sr-only')?.textContent).toBe('无数据')
    expect(gap?.closest('td')?.textContent).toContain('无数据')
    expect(gap?.closest('td')?.textContent).not.toContain('0')
  })

  it('gives a column with no value no sort button and a sortable one a faint two-way arrow', async () => {
    await show()
    const headers = [...host.querySelectorAll('[data-slot="table-head"]')]
    expect(headers[0]!.querySelector('button')).toBeNull()
    expect(headers[0]!.querySelector('svg')).toBeNull()
    expect(headers[1]!.querySelector('button svg')).not.toBeNull()
  })

  it('applies the tone class that names its token for up, down and flat, and none has no tag', async () => {
    await show()
    const cell = (text: string) =>
      [...host.querySelectorAll('[data-slot="table-cell"]')].find((c) => c.textContent === text)
    expect(cell('10')?.querySelector('span')?.className).toContain('text-success')
    expect(cell('9')?.querySelector('span')?.className).toContain('text-down')
    expect(cell('5')?.querySelector('span')?.className).toContain('text-muted-foreground')
  })

  it('shows approx_usd text beside a JPY amount and nothing for a USD cell', async () => {
    await show()
    const cells = [...host.querySelectorAll('[data-slot="table-cell"]')].map((c) => c.textContent)
    expect(cells).toContain('¥12,300 ≈$82')
    expect(cells.filter((c) => c?.includes('≈'))).toEqual(['¥12,300 ≈$82'])
  })

  it('aligns a right-aligned column', async () => {
    await show()
    const heads = host.querySelectorAll('th')
    expect(heads[0]?.className).not.toContain('text-right')
    expect(heads[1]?.className).toContain('text-right')
    expect(host.querySelector('td:nth-child(2)')?.className).toContain('text-right')
  })
})

describe('DataTable source badge status', () => {
  const sourceBadge = async (sourceStatus?: 'good' | 'late' | 'missing') => {
    host = document.createElement('div')
    document.body.append(host)
    await act(async () => {
      root = createRoot(host)
      root.render(
        <DataTable
          columns={[{ id: 'name', label: '名称' }]}
          rows={[{ name: { text: 'x', source: '标签', source_status: sourceStatus } }]}
        />,
      )
    })
    return host.querySelector('[data-slot=badge]')!
  }
  it.each([
    ['good', 'text-success'],
    ['late', 'text-warning'],
    ['missing', 'text-destructive'],
  ] as const)('a source status %s renders %s', async (status, cls) => {
    expect((await sourceBadge(status)).classList).toContain(cls)
  })
  it('a source without status stays the secondary Badge', async () => {
    const badge = await sourceBadge()
    expect(badge.classList).toContain('bg-secondary')
    expect(badge.className).not.toMatch(/text-(success|warning|destructive)/)
  })
})
