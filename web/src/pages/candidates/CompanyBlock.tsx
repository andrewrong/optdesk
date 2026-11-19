// One company block of the by_company layout (docs/design/candidates-by-company.md#5): the strip with the verdict, the
// rows named by row_ids in payload order, then the folded rows under one closed accordion. A row is found by id in
// tiers[].rows (a join, the only data step); every text is a payload field.
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { CompanyStrip } from '@/pages/candidates/CompanyStrip'
import { DataRow } from '@/pages/candidates/TierRows'
import type { CompanyBlock as BlockT, TierRow, TierView } from '@/generated/option_candidates_page.v1'

function rowsOf(tiers: TierView, ids: string[]): TierRow[] {
  const all = tiers.tiers.flatMap((tier) => tier.rows)
  return ids.flatMap((id) => all.find((row) => row.id === id) ?? [])
}

export function CompanyBlock({ block, tiers }: { block: BlockT; tiers: TierView }) {
  const company = tiers.companies.find((c) => c.key === block.company)
  if (!company) return null
  const folded = block.folded
  return (
    <section data-company-block={block.company} className="min-w-0 space-y-1">
      <CompanyStrip
        company={company}
        labels={tiers.strip_labels}
        verdict={{ verdict: block.verdict, text: block.verdict_text, bestYield: block.best_yield }}
      />
      {rowsOf(tiers, block.row_ids).map((row) => (
        <DataRow key={row.id} tier={null} row={row} fit />
      ))}
      {folded ? (
        <Accordion type="single" collapsible data-block-folded>
          <AccordionItem value="folded">
            <AccordionTrigger className="text-muted-foreground">
              <span className="flex flex-wrap items-baseline gap-2">
                <span>{folded.title}</span>
                <span data-folded-summary className="text-xs font-normal">
                  {folded.summary}
                </span>
              </span>
            </AccordionTrigger>
            <AccordionContent>
              {rowsOf(tiers, folded.row_ids).map((row) => (
                <p key={row.id} data-row={row.id} data-line className="p-2 text-sm">
                  {row.line}
                </p>
              ))}
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : null}
    </section>
  )
}
