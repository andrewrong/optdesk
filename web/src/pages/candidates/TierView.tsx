// The short-put tier view (Decisions 10 and 13 of docs/design/candidates-tiers.md): buy hint, the common marks,
// the legend, the company cards, the card note, the tiers in payload order, then the idle line. It renders payload texts and computes nothing.
import { Info } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { TooltipProvider } from '@/components/ui/tooltip'
import { CompanyStrip } from '@/pages/candidates/CompanyStrip'
import { TierLegend } from '@/pages/candidates/TierLegend'
import { CompanyBlock } from '@/pages/candidates/CompanyBlock'
import { TierHead, TierSection } from '@/pages/candidates/TierRows'
import type { TierView as TierViewPayload } from '@/generated/option_candidates_page.v1'

export function TierView({ tiers }: { tiers: TierViewPayload }) {
  const by = tiers.by_company
  return (
    <TooltipProvider>
      <section className="min-w-0 space-y-3" data-tier-view>
        {typeof tiers.buy_hint === 'string' ? (
          <Alert data-buy-hint>
            <Info aria-hidden />
            <AlertDescription>
              <a href="/underlyings" className="underline underline-offset-2">
                {tiers.buy_hint}
              </a>
            </AlertDescription>
          </Alert>
        ) : null}
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
        <TierLegend items={by ? by.legend : tiers.legend} />
        {tiers.heads ? (
          <div className="flex flex-wrap items-baseline gap-2" data-heads-companies>
            <h2 className="text-xl font-semibold">{tiers.heads.companies}</h2>
            <span className="text-xs text-muted-foreground" data-heads-note>
              {by ? by.heading_note : tiers.heads.companies_note}
            </span>
          </div>
        ) : null}
        {by ? <TierHead columns={by.columns} /> : null}
        {by
          ? by.blocks.map((block) => <CompanyBlock key={block.company} block={block} tiers={tiers} />)
          : tiers.companies.map((company) => <CompanyStrip key={company.key} company={company} labels={tiers.strip_labels} />)}
        {typeof tiers.card_note === 'string' ? (
          <p className="text-sm text-muted-foreground" data-card-note>
            {tiers.card_note}
          </p>
        ) : null}
        {by || !tiers.heads ? null : <h2 className="text-xl font-semibold" data-heads-tiers>{tiers.heads.tiers}</h2>}
        {by ? null : tiers.tiers.map((tier) => <TierSection key={tier.id} tier={tier} columns={tiers.columns} />)}
        {typeof tiers.idle_note === 'string' ? (
          <p className="text-sm text-muted-foreground" data-idle-note>
            {tiers.idle_note}
          </p>
        ) : null}
      </section>
    </TooltipProvider>
  )
}
