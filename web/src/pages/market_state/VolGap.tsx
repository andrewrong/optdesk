// The implied-minus-realised volatility block: a shadcn table of the producer's figure and day texts, and its
// Chinese gloss in a tooltip. Every text is the payload's; nothing is parsed or computed.
import { useState } from 'react'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import type { VolGap as Block } from '@/generated/market_state_page.v2'

function HintMark({ hint }: { hint: string }) {
  const [open, setOpen] = useState(false)
  return (
    <TooltipProvider>
      <Tooltip open={open} onOpenChange={setOpen}>
        <TooltipTrigger asChild>
          <span
            role="button"
            tabIndex={0}
            data-hint-mark
            className="ml-2 cursor-help text-muted-foreground"
            onClick={() => setOpen(!open)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                setOpen(!open)
              }
            }}
          >
            ⓘ
          </span>
        </TooltipTrigger>
        <TooltipContent>{hint}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function VolGap({ block }: { block: Block }) {
  return (
    <section aria-label={block.title} data-vol-gap>
      <h2 className="mb-2 text-lg font-medium">
        {block.title}
        <HintMark hint={block.hint} />
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
