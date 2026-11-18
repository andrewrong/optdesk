// The short puts behind one cash day: one list row per put, all text as sent. No amount is shown per put.
import type { Payload } from '@/types/option_notices_page'

type Cash = Payload['accounts'][number]['cash_by_date'][number]

// Lists the puts of one cash row under its date row; nothing when `puts` is absent or empty.
export function CashPuts({ puts }: { puts: Cash['puts'] }) {
  if (!puts || puts.length === 0) return null
  return (
    <ul data-cash-puts className="space-y-1 px-2 pb-2 text-sm text-muted-foreground">
      {puts.map((put, index) => (
        <li
          key={`${index}-${put.symbol}-${put.expiry}-${put.strike_text}-${put.right}`}
          data-cash-put
          className="flex flex-wrap gap-x-3"
        >
          <span>{put.symbol}</span>
          <span>{put.expiry}</span>
          <span data-label>Strike</span>
          <span>{put.strike_text}</span>
          <span>{put.right}</span>
          <span data-label>Qty</span>
          <span>{put.quantity_text}</span>
        </li>
      ))}
    </ul>
  )
}
