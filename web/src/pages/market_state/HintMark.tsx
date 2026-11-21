// The ⓘ mark of a market state text: its hint opens in a tooltip on click or Enter/Space. The hint is the payload's.
import { useState } from 'react'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

export function HintMark({ hint, className }: { hint: string; className: string }) {
  const [open, setOpen] = useState(false)
  return (
    <TooltipProvider>
      <Tooltip open={open} onOpenChange={setOpen}>
        <TooltipTrigger asChild>
          <span
            role="button"
            tabIndex={0}
            data-hint-mark
            className={`${className} cursor-help text-muted-foreground`}
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
