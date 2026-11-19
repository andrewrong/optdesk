// The company card: two lines per company. Line 1 is the verdict pill, the ticker, the buy price with its source,
// the spot and the next earnings; line 2 is the valuation line with its note and the three counts. Every word is a
// payload field (the labels come from strip_labels); nothing is derived or computed here.
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import type { Badge as BadgeT, Company, TierView } from '@/generated/option_candidates_page.v1'

export type StripLabels = NonNullable<TierView['strip_labels']>

const tones = { warn: 'warn', info: 'secondary', none: 'outline' } as const
const pillVariants = { fits: 'success', check: 'warn', grey: 'outline' } as const

function SourceBadge({ badge }: { badge: BadgeT }) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span data-hint={badge.gloss} tabIndex={0} className="cursor-help">
            <Badge variant={tones[badge.tone]} data-buy-source={badge.code}>
              {badge.label}
            </Badge>
          </span>
        </TooltipTrigger>
        <TooltipContent>{badge.gloss}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function Counts({ company, labels }: { company: Company; labels: StripLabels | undefined }) {
  const parts = company.count_parts
  if (!parts || !labels) return null
  return (
    <span data-counts className="text-muted-foreground">
      {labels.contracts} {parts.fits} {labels.fits} · {parts.check} {labels.check} · {parts.grey} {labels.grey}{' '}
      <span data-grey-hint>{labels.grey_hint}</span>
    </span>
  )
}

export function CompanyStrip({ company, labels }: { company: Company; labels?: StripLabels }) {
  const v = company.valuation
  const tier = company.tier
  return (
    <Card data-company={company.key}>
      <CardContent className="flex flex-col gap-1 text-sm">
        <div data-line="1" className="flex flex-wrap items-center gap-x-4 gap-y-1">
          {tier ? (
            <Badge variant={pillVariants[tier.id]} data-tier={tier.id} data-pill>
              {tier.glyph} {tier.word}
            </Badge>
          ) : null}
          <span className="font-semibold">{company.ticker}</span>
          <span data-buy className="flex items-center gap-2">
            {labels ? <span className="text-muted-foreground">{labels.buy_price}</span> : null}
            <span>{company.buy_price ?? company.price_note}</span>
            {company.source_badge ? <SourceBadge badge={company.source_badge} /> : null}
          </span>
          {company.spot_text ? (
            <span data-spot>
              {labels ? <span className="text-muted-foreground">{labels.spot} </span> : null}
              {company.spot_text}
              {company.spot_note ? <span className="text-muted-foreground"> {company.spot_note}</span> : null}
            </span>
          ) : null}
          <span data-earnings>
            {labels ? <span className="text-muted-foreground">{labels.next_earnings} </span> : null}
            {company.earnings_text}
          </span>
        </div>
        <div data-line="2" className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
          {v ? (
            <span data-valuation className="flex flex-wrap items-baseline gap-x-2">
              <span className="text-muted-foreground">{v.line_label}</span>
              <span>{v.line_text}</span>
              {v.note ? (
                <span data-run-state={v.run_state} data-valuation-note className="text-muted-foreground">
                  {v.note}
                </span>
              ) : null}
            </span>
          ) : null}
          <Counts company={company} labels={labels} />
        </div>
      </CardContent>
    </Card>
  )
}
