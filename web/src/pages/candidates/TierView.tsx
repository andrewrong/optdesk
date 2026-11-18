// The short-put tier view (Decisions 10 and 13 of docs/design/candidates-tiers.md): company strips, the common marks,
// the legend, then the tiers in payload order. It renders payload texts and computes nothing.
import { Info } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { TooltipProvider } from '@/components/ui/tooltip'
import { CompanyStrip } from '@/pages/candidates/CompanyStrip'
import { TierLegend } from '@/pages/candidates/TierLegend'
import { TierSection } from '@/pages/candidates/TierRows'
import type { TierView as TierViewPayload } from '@/generated/option_candidates_page.v1'

export function TierView({ tiers }: { tiers: TierViewPayload }) {
  return (
    <TooltipProvider>
      <section className="min-w-0 space-y-3" data-tier-view>
        {tiers.companies.map((company) => (
          <CompanyStrip key={company.key} company={company} />
        ))}
        {tiers.common.map((mark) => (
          <Alert key={mark.code} data-common-mark={mark.code}>
            <Info aria-hidden />
            <AlertTitle>{mark.label}</AlertTitle>
            <AlertDescription>{mark.gloss}</AlertDescription>
          </Alert>
        ))}
        {tiers.kept !== null ? (
          <p className="text-sm" data-kept>
            {tiers.kept}
          </p>
        ) : null}
        {tiers.kept !== null && tiers.kept_note ? (
          <p className="text-xs text-muted-foreground" data-kept-note>
            {tiers.kept_note}
          </p>
        ) : null}
        <TierLegend items={tiers.legend} />
        {tiers.tiers.map((tier) => (
          <TierSection key={tier.id} tier={tier} columns={tiers.columns} />
        ))}
      </section>
    </TooltipProvider>
  )
}
