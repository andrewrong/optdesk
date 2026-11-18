// The company strip: one card per company with the stored spot, buy price, next earnings and counts, as given.
import { Card, CardContent } from '@/components/ui/card'
import type { Company } from '@/generated/option_candidates_page.v1'

export function CompanyStrip({ company }: { company: Company }) {
  return (
    <Card data-company={company.key}>
      <CardContent className="flex flex-wrap items-baseline gap-x-4 gap-y-1 text-sm">
        <span className="font-semibold">{company.ticker}</span>
        <span>
          Spot {company.spot} ({company.spot_day})
        </span>
        <span>
          Buy price {company.buy_price ?? company.buy_price_note}
        </span>
        <span>Next earnings {company.next_earnings}</span>
        <span className="text-muted-foreground">{company.counts}</span>
      </CardContent>
    </Card>
  )
}
