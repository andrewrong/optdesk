// The option page, board 5 of design v2.2 (ADR 0034, issue #1204): per market tab, today's opportunity cards, the
// table of all candidates and one chain, always shown. Every visible string is a `text` or `label` of the payload; this file
// compares, sums and formats nothing. `value` is used only by the Data Table's user-started sort.
import { ChevronRight } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import type { Payload } from '@/types/option_page'
import { CardNotes } from '@/pages/home/CardNotes'
import { StaleAlert } from '@/pages/home/StaleAlert'

export { validate as validateOptionPage } from '@/types/option_page'
export type { Payload as OptionPagePayload } from '@/types/option_page'

type Pane = Payload['markets'][number]
type Opportunity = Pane['cards'][number]
type PayloadTable = Pane['candidates']

// The tone is written by Python; this only maps it to a Badge variant.
const VERDICT_VARIANT = {
  good: 'success',
  warn: 'warn',
  neutral: 'secondary',
  muted: 'outline',
} as const

function OpportunityCard({ card }: { card: Opportunity }) {
  return (
    <Card size="sm" data-opportunity={card.id} className="min-w-0">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {card.symbol}
          <Badge variant={VERDICT_VARIANT[card.verdict.tone]} data-verdict={card.verdict.key}>
            {card.verdict.label}
          </Badge>
        </CardTitle>
        <CardDescription>{card.reason}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-1 text-sm">
        <div className="text-lg font-semibold">{card.annualised_text}</div>
        <div className="flex items-center gap-1">
          <ChevronRight className="size-4 shrink-0" aria-hidden />
          <span>{card.contract_line}</span>
        </div>
        <div className="text-muted-foreground">{card.liquidity_line}</div>
        <div>{card.size_line}</div>
        <div data-premium-line>{card.premium_line}</div>
        <div className="text-muted-foreground">{card.iv_line}</div>
        {card.tags.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {card.tags.map((tag) => (
              <Badge key={tag} variant="secondary">
                {tag}
              </Badge>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

function PayloadTableView({ table, name }: { table: PayloadTable; name: string }) {
  const columns: Column[] = table.columns.map(({ id, label, align, hint }) => ({
    id,
    label,
    align,
    hint,
  }))
  const rows = table.rows.map((row) =>
    Object.fromEntries(row.cells.map(({ column, cell }) => [column, cell as Cell])),
  )
  return (
    <div className="min-w-0" data-table={name}>
      <DataTable columns={columns} rows={rows} />
    </div>
  )
}

function MarketTab({ pane }: { pane: Pane }) {
  return (
    <div className="min-w-0 space-y-4" data-market={pane.market}>
      <p className="text-[15px]" data-headline>
        {pane.headline}
      </p>
      <section className="space-y-2">
        <div className="flex items-baseline justify-between gap-2">
          <h2 className="text-base font-semibold">{pane.cards_title}</h2>
          <Button variant="link" size="sm" asChild>
            <a href={`#${pane.all_link.target}`} data-all-link>
              {pane.all_link.text}
            </a>
          </Button>
        </div>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {pane.cards.map((card) => (
            <OpportunityCard key={card.id} card={card} />
          ))}
        </div>
      </section>
      <section id={pane.all_link.target} className="space-y-2">
        <h2 className="text-base font-semibold">{pane.candidates_title}</h2>
        <PayloadTableView table={pane.candidates} name="candidates" />
      </section>
      {pane.chain ? (
        <Card data-chain>
          <CardHeader>
            <CardTitle>{pane.chain.title}</CardTitle>
            <CardDescription>{pane.chain.summary}</CardDescription>
          </CardHeader>
          <CardContent>
            <PayloadTableView table={pane.chain.table} name="chain" />
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

export function OptionPage({ payload }: { payload: Payload }) {
  const first = payload.markets[0]
  return (
    <div className="min-w-0 space-y-4" data-page="option-page">
      <h1 className="text-2xl font-semibold">{payload.title}</h1>
      <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} />
      <Tabs defaultValue={first?.market} className="min-w-0">
        <TabsList>
          {payload.markets.map((pane) => (
            <TabsTrigger key={pane.market} value={pane.market}>
              {pane.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {payload.markets.map((pane) => (
          <TabsContent key={pane.market} value={pane.market} className="min-w-0">
            <MarketTab pane={pane} />
          </TabsContent>
        ))}
      </Tabs>
      <CardNotes notes={payload.notes} gaps={payload.gaps} />
    </div>
  )
}
