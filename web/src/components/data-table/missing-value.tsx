// A missing value in a table cell: a dash, with the reason Python sent one tap away. The reason is the sr-only
// text of the button, a tooltip on hover and a popover on tap. The view writes no reason of its own.
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

export function MissingValue({ text }: { text: string }) {
  return (
    <TooltipProvider>
      <Popover>
        <Tooltip>
          <TooltipTrigger asChild>
            <PopoverTrigger asChild>
              <button type="button" className="cursor-help text-muted-foreground" data-missing>
                <span aria-hidden>—</span>
                <span className="sr-only">{text}</span>
              </button>
            </PopoverTrigger>
          </TooltipTrigger>
          <TooltipContent>{text}</TooltipContent>
        </Tooltip>
        <PopoverContent className="w-auto max-w-64 text-sm">{text}</PopoverContent>
      </Popover>
    </TooltipProvider>
  )
}
