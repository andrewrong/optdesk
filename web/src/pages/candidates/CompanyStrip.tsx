// The company strip: one card per company with the stored spot, buy price, next earnings and counts, as given.
// The valuation block adds two rows (label, text, note); every text is a payload field, nothing is computed here.
import { Info } from 'lucide-react'
import { Alert } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import type { Company } from '@/generated/option_candidates_page.v1'

function Label({ text, hint }: { text: string; hint: string | null }) {
  if (hint === null) return <span className="text-muted-foreground">{text}</span>
  return (
    <TooltipProvider>
    <Tooltip>
      <TooltipTrigger asChild>
        <span data-hint={hint} tabIndex={0} className="cursor-help text-muted-foreground underline decoration-dotted underline-offset-2">
          {text}
        </span>
      </TooltipTrigger>
      <TooltipContent>{hint}</TooltipContent>
    </Tooltip>
    </TooltipProvider>
  )
}

export function CompanyStrip({ company, spotHint = null }: { company: Company; spotHint?: string | null }) {
  const v = company.valuation
  return (
    <Card data-company={company.key}>
      <CardContent className="flex flex-col gap-1 text-sm">
        <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <span className="font-semibold">{company.ticker}</span>
          <span>
            <Label text="Spot" hint={spotHint} /> {company.spot} (rows of {company.spot_day})
          </span>
          <span>
            Buy price {company.buy_price ?? company.buy_price_note}
          </span>
          <span>Next earnings {company.next_earnings}</span>
          <span className="text-muted-foreground">{company.counts}</span>
        </div>
        {v ? (
          <>
            <div data-valuation className="grid grid-cols-[auto_1fr] items-baseline gap-x-4">
              <Label text={v.label} hint={v.hint} />
              {v.reason ? (
                <Alert className="flex items-center gap-1 border-0 bg-transparent p-0 text-muted-foreground">
                  <Info className="size-4" aria-hidden />
                  <span>{v.text}</span>
                </Alert>
              ) : (
                <span>{v.text}</span>
              )}
            </div>
            <div data-gate className="grid grid-cols-[auto_1fr] items-baseline gap-x-4">
              <Label text={v.gate_label} hint={v.gate_hint} />
              <span>{v.gate_text}</span>
            </div>
          </>
        ) : null}
      </CardContent>
    </Card>
  )
}
