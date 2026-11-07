// The home page (home_page v2): five summary cards under a header, the gap line and two link lists. Every visible
// string is a field of the payload; the view computes nothing and holds no route table (links go to their `href`).
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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

function Links({ label, links }: { label: string; links: readonly { href: string; text: string }[] }) {
  if (links.length === 0) return null
  return (
    <nav aria-label={label} className="flex flex-wrap gap-2" data-links={label}>
      {links.map((link, index) => (
        <Button key={`${index}-${link.href}`} asChild variant="outline">
          <a href={link.href}>{link.text}</a>
        </Button>
      ))}
    </nav>
  )
}

export function HomePage({ payload }: { payload: Payload }) {
  return (
    <div className="space-y-4" data-page="home-page">
      <Card>
        <CardHeader>
          <CardTitle>{payload.title}</CardTitle>
          <CardDescription>{payload.say}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <Alert data-gap-line>
            <AlertDescription>{payload.gap_line}</AlertDescription>
          </Alert>
          <Links label="block_links" links={payload.block_links} />
          <Links label="report_links" links={payload.report_links} />
        </CardContent>
      </Card>
      <div className="grid gap-4 lg:grid-cols-2">
        <HouseholdCard payload={payload.household} />
        <HeldCompaniesCard payload={payload.companies} />
        <MacroCard payload={payload.macro} />
        <MarketSummaryCard payload={payload.market} />
        <OptionsSummaryCard payload={payload.options} />
      </div>
    </div>
  )
}

export const registration = {
  schema: 'home_page',
  page: page<Payload>(validate, HomePage),
}
