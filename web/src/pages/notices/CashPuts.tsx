// The short puts behind one cash day: one list row per put, all text as sent. No amount is shown per put.
import { Fragment } from 'react'
import { TableCell, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/option_notices_page'

type Cash = Payload['accounts'][number]['cash_by_date'][number]

export const CASH_COLUMNS = 3

// A table row spanning all columns that lists the puts of `row`; nothing when there are none.
export function CashPuts({ puts }: { puts: Cash['puts'] }) {
  if (!puts || puts.length === 0) return null
  return (
    <TableRow data-cash-puts>
      <TableCell colSpan={CASH_COLUMNS}>
        <ul className="space-y-1 text-sm text-muted-foreground">
          {puts.map((put, index) => (
            <Fragment key={`${index}-${put.symbol}-${put.expiry}-${put.strike_text}-${put.right}`}>
              <li data-cash-put className="flex flex-wrap gap-x-3">
                <span>{put.symbol}</span>
                <span>{put.expiry}</span>
                <span>{put.strike_text}</span>
                <span>{put.right}</span>
                <span>{put.quantity_text}</span>
              </li>
            </Fragment>
          ))}
        </ul>
      </TableCell>
    </TableRow>
  )
}
