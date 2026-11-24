// The sector table: the rows in payload order, every figure and gap as its text, the rank as `at of of`.
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/market_state_page'

type Sectors = Payload['sectors']
type Row = Sectors['rows'][number]

const rank = (r: Row['rank_21']) => ('kind' in r ? r.text : `${r.at} of ${r.of}`)
const day = (d: Sectors['end_day']) => (typeof d === 'string' ? d : d.text)

export function SectorTable({ block }: { block: Sectors }) {
  return (
    <section aria-label="Sectors" data-sector-table>
      <p className="text-sm text-muted-foreground" data-sector-caption>
        {block.caption}
      </p>
      <p className="text-sm text-muted-foreground" data-sector-end-day>
        {day(block.end_day)}
      </p>
      <Table className="mt-2">
        <TableHeader>
          <TableRow>
            <TableHead>Symbol</TableHead>
            <TableHead>Name</TableHead>
            <TableHead>Group</TableHead>
            <TableHead>21</TableHead>
            <TableHead>63</TableHead>
            <TableHead>Rank 21</TableHead>
            <TableHead>Rank 63</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {block.rows.map((row) => (
            <TableRow key={row.symbol} data-sector-row={row.symbol}>
              <TableCell>{row.symbol}</TableCell>
              <TableCell className="whitespace-normal break-words">{row.label}</TableCell>
              <TableCell>{row.group}</TableCell>
              <TableCell>{row.gap_21.text}</TableCell>
              <TableCell>{row.gap_63.text}</TableCell>
              <TableCell>{rank(row.rank_21)}</TableCell>
              <TableCell>{rank(row.rank_63)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  )
}
