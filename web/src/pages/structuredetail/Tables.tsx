// The legs, greeks and fills tables of the structure detail page: every cell is a text of the payload.
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import type { Payload } from '@/types/structure_detail'

type Row = Record<string, Cell>
type Figure = { text: string; value: number } | { reason: string; text: string }

const figure = (f: Figure): Cell =>
  'reason' in f ? { reason: f.reason, text: f.text } : { text: f.text, value: f.value }
const label = (text: string): Cell => ({ text })

const LEG_COLUMNS: Column[] = [
  { id: 'label', label: '', wrap: true },
  { id: 'side', label: '' },
  { id: 'right', label: '' },
  { id: 'expiry', label: '' },
  { id: 'quantity', label: '', align: 'right' },
  { id: 'strike', label: '', align: 'right' },
  { id: 'price', label: '', align: 'right' },
  { id: 'mark', label: '', align: 'right' },
  { id: 'pnl', label: '', align: 'right' },
]

export function LegsTable({ legs, total }: { legs: Payload['legs']; total: Payload['legs_total'] }) {
  const rows: Row[] = legs.map((leg): Row =>
    'reason' in leg
      ? { label: figure(leg) }
      : {
          label: label(leg.label),
          side: label(leg.side_text),
          right: label(leg.right_text),
          expiry: label(leg.expiry_text),
          quantity: { text: `${leg.quantity}`, value: leg.quantity },
          strike: figure(leg.strike),
          price: figure(leg.price),
          mark: figure(leg.mark),
          pnl: figure(leg.pnl),
        },
  )
  rows.push({ label: label(total.label), pnl: figure(total.pnl) })
  return <DataTable columns={LEG_COLUMNS} rows={rows} />
}

const GREEK_COLUMNS: Column[] = [
  { id: 'label', label: '' },
  { id: 'delta', label: 'Delta', align: 'right' },
  { id: 'gamma', label: 'Gamma', align: 'right' },
  { id: 'theta', label: 'Theta', align: 'right' },
  { id: 'vega', label: 'Vega', align: 'right' },
]
const GREEK_IDS = ['delta', 'gamma', 'theta', 'vega'] as const

export function GreeksTable({ greeks }: { greeks: Exclude<Payload['greeks'], { reason: string }> }) {
  const rows: Row[] = greeks.rows.map((row) => {
    const out: Row = { label: label(row.label) }
    row.cells.forEach((cell, at) => {
      const id = GREEK_IDS[at]
      if (id) out[id] = figure(cell)
    })
    return out
  })
  return <DataTable columns={GREEK_COLUMNS} rows={rows} />
}

const FILL_COLUMNS: Column[] = [
  { id: 'time', label: '' },
  { id: 'action', label: '' },
  { id: 'leg', label: '', wrap: true },
  { id: 'quantity', label: '', align: 'right' },
  { id: 'price', label: '', align: 'right' },
  { id: 'fee', label: '', align: 'right' },
]

export function FillsTable({ fills }: { fills: Payload['fills'] }) {
  const rows: Row[] = fills.map((fill) => ({
    time: label(fill.time_text),
    action: label(fill.action_text),
    leg: label(fill.leg_text),
    quantity: { text: `${fill.quantity}`, value: fill.quantity },
    price: figure(fill.price),
    fee: figure(fill.fee),
  }))
  return <DataTable columns={FILL_COLUMNS} rows={rows} />
}
