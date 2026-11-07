// The chain of an option_data_page: one column per ChainRow cell, the text as sent, sorting on demand only.
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import type { Payload } from '@/types/option_data_page'

type ChainRow = Payload['chain']['rows'][number]
type PayloadCell = ChainRow['bid']

const COLUMNS: Column[] = [
  { id: 'expiry', label: 'Expiry' },
  { id: 'right', label: 'Right' },
  { id: 'strike', label: 'Strike', align: 'right' },
  { id: 'status', label: 'Status' },
  { id: 'bid', label: 'Bid', align: 'right' },
  { id: 'ask', label: 'Ask', align: 'right' },
  { id: 'last', label: 'Last', align: 'right' },
  { id: 'volume', label: 'Volume', align: 'right' },
  { id: 'open_interest', label: 'Open interest', align: 'right' },
  { id: 'iv', label: 'IV', align: 'right' },
  { id: 'delta', label: 'Delta', align: 'right' },
  { id: 'gamma', label: 'Gamma', align: 'right' },
  { id: 'theta', label: 'Theta', align: 'right' },
  { id: 'vega', label: 'Vega', align: 'right' },
]

// A figure keeps its number for sorting; a gap keeps its reason and text and has no value.
const cellOf = (cell: PayloadCell): Cell =>
  cell.kind === 'value'
    ? { text: cell.text, value: cell.value }
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
