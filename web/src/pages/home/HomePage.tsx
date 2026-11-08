// The home page (home_page v2): a header with the gap line, five summary cards in one column each under its header link, and the report list. Every visible
// string is a field of the payload; the view computes nothing and holds no route table (links go to their `href`).
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
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

type Link = { href: string; text: string }

// The header row of a card: block link `i` of the payload, or nothing when the payload has fewer links.
function BlockRow({ link }: { link: Link | undefined }) {
  if (!link) return null
  return (
    <div data-block-row>
      <Button asChild variant="outline">
        <a href={link.href}>{link.text}</a>
      </Button>
    </div>
  )
}

// The first report is visible; the rest sit in the fold "全部报告". One report has no fold.
function Reports({ links }: { links: readonly Link[] }) {
  const [first, ...rest] = links
  if (!first) return null
  return (
    <Card data-links="report_links">
      <CardHeader>
        <CardTitle>报告</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Button asChild variant="outline">
          <a href={first.href}>{first.text}</a>
        </Button>
        {rest.length > 0 && (
          <Accordion type="single" collapsible>
            <AccordionItem value="all-reports">
              <AccordionTrigger>全部报告</AccordionTrigger>
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
  const rows = payload.block_links
  return (
    <div className="space-y-4" data-page="home-page">
      <Card>
        <CardHeader>
          <CardTitle>{payload.title}</CardTitle>
          <CardDescription>{payload.say}</CardDescription>
        </CardHeader>
        <CardContent>
          <Alert data-gap-line>
            <AlertDescription>{payload.gap_line}</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
      <div className="flex flex-col gap-4">
        <BlockRow link={rows[0]} />
        <MacroCard payload={payload.macro} />
        <BlockRow link={rows[1]} />
        <MarketSummaryCard payload={payload.market} />
        <BlockRow link={rows[2]} />
        <HouseholdCard payload={payload.household} />
        <BlockRow link={rows[3]} />
        <HeldCompaniesCard payload={payload.companies} />
        <BlockRow link={rows[4]} />
        <OptionsSummaryCard payload={payload.options} />
      </div>
      <Reports links={payload.report_links} />
    </div>
  )
}

export const registration = {
  schema: 'home_page',
  page: page<Payload>(validate, HomePage),
}
