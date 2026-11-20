// The lead block of the macro page (docs/design/macro-page-v2.md Decision 2): every string, day and flag is
// sent by Python; this file formats, parses and compares nothing and invents no text.
import type { ReactNode } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import type { MacroPageV2 } from '@/types/macro_page'

type Lead = NonNullable<MacroPageV2['lead']>

// A term with Python's hint: a hover tooltip and the same text as aria-label. No hint, no tooltip.
export function Hinted({ hint, children }: { hint?: string | null; children: ReactNode }) {
  if (!hint) return <>{children}</>
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-help underline decoration-dotted" aria-label={hint} data-hint>
            {children}
          </span>
        </TooltipTrigger>
        <TooltipContent>{hint}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function LeadBlock({ lead }: { lead: Lead }) {
  return (
    <Card data-lead>
      <CardHeader>
        <CardTitle>{lead.title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <ul className="space-y-2">
          {lead.rows.map((row) => (
            <li key={row.key} data-lead-row={row.key}>
              <div className="text-xs text-muted-foreground">
                <Hinted hint={row.hint}>{row.label}</Hinted>
              </div>
              {row.state === 'gap' ? (
                <div className="text-muted-foreground" data-lead-gap>
                  {row.text}
                </div>
              ) : (
                <div>{row.text}</div>
              )}
              {row.change_text !== null && (
                <div className="text-xs text-muted-foreground" data-lead-change>
                  {row.change_text}
                </div>
              )}
              {row.data_day !== null && (
                <div className="text-xs text-muted-foreground" data-lead-day>
                  {row.data_day}
                </div>
              )}
            </li>
          ))}
        </ul>
        <CardDescription data-lead-note>{lead.note}</CardDescription>
      </CardContent>
    </Card>
  )
}
