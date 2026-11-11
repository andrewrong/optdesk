// The home page (home_page v3): the overview card, five summary cards in a responsive grid, each title a link, and the report list. Every visible
// string is a field of the payload; the view computes nothing and holds no route table (links go to their `href`).
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate as validateHeld } from '@/types/held_companies_summary'
import { validate as validateTop, type Gap, type Payload } from '@/types/home_page'
import { validate as validateHousehold } from '@/types/household_summary'
import { validate as validateMacro } from '@/types/macro_summary'
import { validate as validateMarket } from '@/types/market_summary'
import { validate as validateOptions } from '@/types/options_summary'
import { HeldCompaniesCard } from './HeldCompaniesCard'
import { HouseholdCard } from './HouseholdCard'
import { MacroCard } from './MacroCard'
import { MarketSummaryCard } from './MarketSummaryCard'
import { OptionsSummaryCard } from './OptionsSummaryCard'
import { OverviewCard } from './OverviewCard'

type Validation = { ok: true; payload: Payload } | { ok: false; gap: Gap; errors: never[] }

// The top-level check, then the five members in order; the first gap is the answer.
export function validate(input: unknown): Validation {
  const top = validateTop(input)
  if (!top.ok) return top
  const p = top.payload
  const members: { ok: boolean; gap?: Gap }[] = [
    validateHousehold(p.household),
    validateHeld(p.companies),
    validateMacro(p.macro),
    validateMarket(p.market),
    validateOptions(p.options),
  ]
  for (const m of members) {
    if (!m.ok && m.gap) return { ok: false, gap: m.gap, errors: [] }
  }
  return top
}

type Link = { href: string; text: string }

// The first report is visible; the rest sit in the fold "All reports". One report has no fold.
function Reports({ links }: { links: readonly Link[] }) {
  const [first, ...rest] = links
  if (!first) return null
  return (
    <Card data-links="report_links">
      <CardHeader>
        <CardTitle>Reports</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Button asChild variant="outline">
          <a href={first.href}>{first.text}</a>
        </Button>
        {rest.length > 0 && (
          <Accordion type="single" collapsible>
            <AccordionItem value="all-reports">
              <AccordionTrigger>All reports</AccordionTrigger>
              <AccordionContent className="flex flex-wrap gap-2">
                {rest.map((link, index) => (
                  <Button key={`${index}-${link.href}`} asChild variant="outline">
                    <a href={link.href}>{link.text}</a>
                  </Button>
                ))}
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        )}
      </CardContent>
    </Card>
  )
}

export function HomePage({ payload }: { payload: Payload }) {
  // Card i of the page takes the href of overview line i for its title (macro, market, household, companies, options).
  const [macro, market, household, companies, options] = payload.overview.items.map((item) => item.href)
  return (
    <div className="space-y-4" data-page="home-page">
      <OverviewCard payload={payload} />
      <div className="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-4">
        <div className="min-w-0">
          <MacroCard payload={payload.macro} titleHref={macro} />
        </div>
        <div className="min-w-0">
          <MarketSummaryCard payload={payload.market} titleHref={market} />
        </div>
        <div className="min-w-0 lg:col-span-2 2xl:col-span-1">
          <HouseholdCard payload={payload.household} titleHref={household} />
        </div>
        <div className="min-w-0 col-span-full">
          <HeldCompaniesCard payload={payload.companies} titleHref={companies} />
        </div>
        <div className="min-w-0 col-span-full">
          <OptionsSummaryCard payload={payload.options} titleHref={options} />
        </div>
      </div>
      <Reports links={payload.report_links} />
    </div>
  )
}

export const registration = {
  schema: 'home_page',
  page: page<Payload>(validate, HomePage),
}
