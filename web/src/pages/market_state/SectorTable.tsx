// The sector table: the rows in payload order, every figure and gap as its text, the rank as `at of of`.
import type { Payload } from '@/types/market_state_page'

type Sectors = Payload['sectors']
type Row = Sectors['rows'][number]
type Cell = { text: string }

const rank = (r: Row['rank_21']) => ('kind' in r ? r.text : `${r.at} of ${r.of}`)
const day = (d: Sectors['end_day']) => (typeof d === 'string' ? d : d.text)

export function SectorTable({ block }: { block: Sectors }) {
  const figure = (c: Cell) => c.text
  return (
    <section aria-label="Sectors" data-sector-table>
      <p className="text-sm text-muted-foreground" data-sector-caption>
        {block.caption}
      </p>
      <p className="text-sm text-muted-foreground" data-sector-end-day>
        {day(block.end_day)}
      </p>
      <div className="overflow-x-auto">
        <table className="mt-2 w-full text-sm">
          <thead>
            <tr className="text-left">
              <th>Symbol</th>
              <th>Name</th>
              <th>Group</th>
              <th>21</th>
              <th>63</th>
              <th>Rank 21</th>
              <th>Rank 63</th>
            </tr>
          </thead>
          <tbody>
            {block.rows.map((row) => (
              <tr key={row.symbol} data-sector-row={row.symbol}>
                <td>{row.symbol}</td>
                <td>{row.label}</td>
                <td>{row.group}</td>
                <td>{figure(row.gap_21)}</td>
                <td>{figure(row.gap_63)}</td>
                <td>{rank(row.rank_21)}</td>
                <td>{rank(row.rank_63)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
