// The option candidates page (Decisions 4 to 11 of docs/design/option-candidates-page.md): a header card, one tab per
// group present, the notes at the foot. Every visible string is a payload field; this file counts, sorts, compares
// and words nothing. The table sort buttons are those of the shared data-table; there is no filter control.
import { TriangleAlert } from 'lucide-react'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CandidateGroup } from '@/pages/candidates/CandidateGroup'
import { ShortList } from '@/pages/candidates/ShortList'
import { TierView } from '@/pages/candidates/TierView'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/option_candidates_page'

export { validate as validateOptionCandidatesPage } from '@/types/option_candidates_page'
export type { Payload as OptionCandidatesPayload } from '@/types/option_candidates_page'

type Group = Payload['groups'][number]
const isJpy = (group: Group) => group.currency === 'JPY'
const groupKey = (group: Group) => (group.currency ? `${group.kind}:${group.currency}` : group.kind)

// The JP group is placed right after the US short-put group; the other groups keep the document order.
function placed(groups: readonly Group[]): Group[] {
  const jp = groups.filter(isJpy)
  const rest = groups.filter((group) => !isJpy(group))
  const at = rest.findIndex((group) => group.kind === 'short_put') + 1
  return [...rest.slice(0, at), ...jp, ...rest.slice(at)]
}

export function OptionCandidatesPage({ payload }: { payload: Payload }) {
  const staleText = payload.stale ? payload.stale.text : null
  const groups = placed(payload.groups)
  const labels = payload.head_labels
  const byCompany = payload.short_put_tiers?.by_company !== undefined
  const tabs = (
    <Tabs defaultValue={groupKey(groups[0]!)}>
      <TabsList>
        {groups.map((group) => (
          <TabsTrigger key={groupKey(group)} value={groupKey(group)} data-tab={group.kind} data-currency={group.currency ?? undefined}>
            {group.title}
            <Badge variant="secondary">{group.counts.ok}</Badge>
            <Badge variant="outline">{group.counts.blocked}</Badge>
            <Badge variant="outline">{group.counts.unknown}</Badge>
          </TabsTrigger>
        ))}
      </TabsList>
      {groups.map((group) => (
        <TabsContent key={groupKey(group)} value={groupKey(group)} className="space-y-4">
          {isJpy(group) && !payload.short_put_tiers_jpy?.by_company ? <ShortList list={payload.short_put_list_jpy} /> : null}
          {isJpy(group) && payload.short_put_tiers_jpy ? <TierView tiers={payload.short_put_tiers_jpy} /> : null}
          <CandidateGroup group={group} staleText={staleText} />
        </TabsContent>
      ))}
    </Tabs>
  )
  const lower = (
    <>
        {payload.short_put_tiers ? <TierView tiers={payload.short_put_tiers} /> : null}
        {payload.short_put_tiers ? (
          <Accordion type="single" collapsible data-stored-tables>
            <AccordionItem value="stored">
              <AccordionTrigger className="text-xl font-semibold">
                {payload.short_put_tiers.stored_title}
              </AccordionTrigger>
              <AccordionContent>
                {tabs}
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        ) : (
          tabs
        )}
    </>
  )
  return (
    <div className="min-w-0 space-y-4" data-page="option-candidates">
      <Card data-header>
        <CardHeader>
          <CardTitle>
            <h1 className="text-2xl font-semibold">{payload.title}</h1>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {payload.subtitle ? (
            <p data-subtitle className="text-muted-foreground">
              {payload.subtitle}
            </p>
          ) : null}
          {payload.source_counts ? (
            <p data-source-counts className="text-muted-foreground">
              Open chain: {payload.source_counts.open} names, last close: {payload.source_counts.eod_fallback} names
              {payload.source_counts.none === undefined ? null : `, none: ${payload.source_counts.none}`}
            </p>
          ) : null}
          <div className="flex flex-wrap gap-2">
            {payload.chain_day ? (
              <Badge data-chain-day>
                {labels?.data_day} {payload.chain_day}
              </Badge>
            ) : null}
            {labels ? (
              <Badge variant="secondary" data-run-status>
                {labels.run_status} {labels.run_status_text}
              </Badge>
            ) : null}
            {payload.computed_at ? <Badge variant="secondary" data-computed-at>{payload.computed_at}</Badge> : null}
            {payload.rule_version ? (
              <Badge variant="outline" data-rule-version>
                {labels?.rule_version} {payload.rule_version}
              </Badge>
            ) : null}
          </div>
          <p data-estimate-note className="text-muted-foreground">
            {payload.estimate_note}
          </p>
          {payload.stale ? (
            <Alert variant={payload.stale.is_stale ? 'destructive' : 'default'} data-stale>
              <TriangleAlert aria-hidden />
              <AlertDescription>{payload.stale.text}</AlertDescription>
            </Alert>
          ) : null}
          {payload.gaps.map((gap, index) => (
            <Alert key={`${index}-${gap.reason}`} data-page-gap={gap.reason}>
              <AlertDescription>{gap.text}</AlertDescription>
            </Alert>
          ))}
        </CardContent>
      </Card>
      {byCompany ? null : <ShortList list={payload.short_put_list} />}
      {payload.short_put_list && !byCompany ? (
        <Accordion type="single" collapsible data-folded>
          <AccordionItem value="folded">
            <AccordionTrigger className="text-xl font-semibold">{payload.short_put_list.folded_title}</AccordionTrigger>
            <AccordionContent className="space-y-4">
              {lower}
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : (
        lower
      )}
      {payload.notes.map((note, index) => (
        <p key={`${index}-${note}`} className="text-xs text-muted-foreground" data-note>
          {note}
        </p>
      ))}
    </div>
  )
}

export const registration = { schema: 'option_candidates_page', page: page<Payload>(validate, OptionCandidatesPage) }
