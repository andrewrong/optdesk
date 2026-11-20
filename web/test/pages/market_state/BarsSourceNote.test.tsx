import { readFileSync } from 'node:fs'
import path from 'node:path'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { BarsSourceNote } from '@/components/BarsSourceNote'
import { CompanyDataPage } from '@/pages/company_data/CompanyDataPage'
import { MarketStatePage } from '@/pages/market_state/MarketStatePage'
import { PositionsPage } from '@/pages/positions/PositionsPage'
import { validate as validateCompany } from '@/types/company_data_page'
import { validate as validateMarket } from '@/types/market_state_page'
import { validate as validatePositions } from '@/types/positions_page'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

beforeEach(() => {
  vi.stubGlobal('matchMedia', () => ({ addEventListener: () => {}, removeEventListener: () => {} }))
})

// The entry 1 fixtures carry no note; a mixed note, as shared.BarSourceOf words it, is added to them here.
const mixed = '日线含 Yahoo 数据，当日指标可能基于 Yahoo 价格；IB 历史数据恢复后自动改回 IB 数据'
const fixture = (name: string): Record<string, unknown> =>
  JSON.parse(readFileSync(path.resolve(import.meta.dirname, `../../../../contracts/fixtures/${name}`), 'utf8'))

async function html(node: React.ReactNode): Promise<HTMLElement> {
  const host = document.createElement('div')
  const root = createRoot(host)
  await act(async () => {
    root.render(node)
  })
  return host
}

describe('BarsSourceNote', () => {
  it('renders a mixed note as text', async () => {
    const host = await html(<BarsSourceNote payload={{ bars_source: 'mixed', bars_source_note: mixed }} />)
    expect(host.textContent).toBe(mixed)
  })
  it('renders nothing for an empty or absent note', async () => {
    expect((await html(<BarsSourceNote payload={{ bars_source_note: '' }} />)).innerHTML).toBe('')
    expect((await html(<BarsSourceNote payload={{}} />)).innerHTML).toBe('')
  })
})

const pages = [
  { name: 'market state', file: 'page.market_state_page.v2.ok.json', validate: validateMarket, Page: MarketStatePage },
  { name: 'company data', file: 'page.company_data_page.v2.ok.json', validate: validateCompany, Page: CompanyDataPage },
  { name: 'positions', file: 'page.positions_page.v4.ok.json', validate: validatePositions, Page: PositionsPage },
] as const

describe.each(pages)('$name page', ({ file, validate, Page }) => {
  it('accepts the fixture and shows the note of a payload that carries one', async () => {
    const ok = fixture(file)
    expect(validate(ok).ok).toBe(true)
    const withNote = { ...ok, bars_source: 'mixed', bars_source_note: mixed }
    expect(validate(withNote).ok).toBe(true)
    const P = Page as (props: { payload: never }) => React.ReactNode
    const host = await html(<P payload={withNote as never} />)
    expect(host.querySelector('[data-bars-source-note]')?.textContent).toBe(mixed)
  })
  it('renders no note for a payload without the fields', async () => {
    const P = Page as (props: { payload: never }) => React.ReactNode
    const host = await html(<P payload={fixture(file) as never} />)
    expect(host.querySelector('[data-bars-source-note]')).toBeNull()
  })
})
