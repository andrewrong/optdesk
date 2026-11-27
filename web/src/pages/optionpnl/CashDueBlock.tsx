// The cash due block of an account: one line per currency, as the payload gives it. Every text is a field of
// the payload; nothing is compared, summed, sorted or formatted here.
import type { Line } from '@/generated/option_pnl_page.v1'
import { LinesTable } from './BookCard'

export function CashDueBlock({ lines }: { lines?: Line[] | null }) {
  if (!lines || lines.length === 0) return null
  return (
    <section className="min-w-0" data-cash-due>
      <LinesTable lines={lines} />
    </section>
  )
}
