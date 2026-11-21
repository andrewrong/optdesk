// The implied-minus-realised volatility block: a shadcn table of the producer's figure and day texts, and its
// Chinese gloss in a tooltip. Every text is the payload's; nothing is parsed or computed.
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import type { VolGap as Block } from '@/generated/market_state_page.v2'
import { HintMark } from './HintMark'

export function VolGap({ block }: { block: Block }) {
  return (
    <section aria-label={block.title} data-vol-gap>
      <h2 className="mb-2 text-lg font-medium">
        {block.title}
        <HintMark hint={block.hint} className="ml-2" />
      </h2>
      <Table>
        <TableBody>
          {block.rows.map((row) => (
            <TableRow key={row.id} data-vol-gap-row={row.id}>
              <TableCell className="font-medium">{row.label}</TableCell>
              <TableCell>{row.figure.text}</TableCell>
              <TableCell className="text-sm text-muted-foreground">{row.day_text}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  )
}
