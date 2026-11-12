// The volatility view of the stock option page (page.stock_option_page.v3 `vol_view`): every label, text, data
// day, gap code, the method and the coaching sentence are fields of the payload. This file computes nothing.
import type { VolView } from '@/generated/stock_option_page.v3'

export function VolBlock({ vol }: { vol: VolView }) {
  return (
    <section className="min-w-0 space-y-2" data-vol-view>
      <h2 className="text-lg font-semibold">{vol.name}</h2>
      <table className="w-full text-sm">
        <tbody>
          {vol.rows.map((row) => (
            <tr key={row.label} data-vol-row data-gap={row.gap ? row.gap : undefined}>
              <th scope="row" className="py-1 pr-4 text-left font-normal text-muted-foreground">
                {row.label}
              </th>
              <td className="py-1 pr-4">{row.gap ? row.gap : row.text}</td>
              <td className="py-1 text-xs text-muted-foreground">{row.data_day}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-xs text-muted-foreground" data-vol-method>
        {vol.method}
      </p>
      <p className="text-sm" data-vol-coaching>
        {vol.coaching}
      </p>
    </section>
  )
}
