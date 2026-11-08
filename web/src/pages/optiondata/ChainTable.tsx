// The chain of an option_data_page: one column per ChainRow cell, the text as sent, sorting on demand only.
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import type { Payload } from '@/types/option_data_page'

type ChainRow = Payload['chain']['rows'][number]
type PayloadCell = ChainRow['bid']

const COLUMNS: Column[] = [
  { id: 'expiry', label: '到期' },
  { id: 'right', label: '类型' },
  { id: 'strike', label: '行权价', align: 'right' },
  { id: 'status', label: '状态' },
  { id: 'bid', label: '买价', align: 'right' },
  { id: 'ask', label: '卖价', align: 'right' },
  { id: 'last', label: '成交价', align: 'right' },
  { id: 'volume', label: '成交量', align: 'right' },
  { id: 'open_interest', label: 'OI', align: 'right' },
  { id: 'iv', label: 'IV', align: 'right' },
  { id: 'delta', label: 'Δ', align: 'right' },
  { id: 'gamma', label: 'Γ', align: 'right' },
  { id: 'theta', label: 'Θ', align: 'right' },
  { id: 'vega', label: 'Vega', align: 'right' },
]

// A figure keeps its number for sorting; the ≈USD text sits beside it as sent; a gap keeps its reason and text and has no value.
const cellOf = (cell: PayloadCell): Cell =>
  cell.kind === 'value'
    ? {
        text: cell.text,
        value: cell.value,
        approx_usd: cell.approx_usd_text ? { text: cell.approx_usd_text } : null,
      }
    : { reason: cell.reason, text: cell.text }

const plain = (text: string | null): Cell => ({ text: text ?? '-', value: text })

export function ChainTable({ rows }: { rows: ChainRow[] }) {
  const data = rows.map((row) => ({
    expiry: plain(row.expiry),
    right: plain(row.right),
    strike: cellOf(row.strike),
    status: { text: row.status.text, value: row.status.code, badge: true },
    bid: cellOf(row.bid),
    ask: cellOf(row.ask),
    last: cellOf(row.last),
    volume: cellOf(row.volume),
    open_interest: cellOf(row.open_interest),
    iv: cellOf(row.iv),
    delta: cellOf(row.delta),
    gamma: cellOf(row.gamma),
    theta: cellOf(row.theta),
    vega: cellOf(row.vega),
  }))
  return <DataTable columns={COLUMNS} rows={data} />
}
