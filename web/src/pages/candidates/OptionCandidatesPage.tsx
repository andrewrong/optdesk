// The option candidates page (Decisions 4 to 11 of docs/design/option-candidates-page.md): a header card, one tab per
// group present, the notes at the foot. Every visible string is a payload field; this file counts, sorts, compares
// and words nothing. The table sort buttons are those of the shared data-table; there is no filter control.
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CandidateGroup } from '@/pages/candidates/CandidateGroup'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/option_candidates_page'

export { validate as validateOptionCandidatesPage } from '@/types/option_candidates_page'
export type { Payload as OptionCandidatesPayload } from '@/types/option_candidates_page'

export function OptionCandidatesPage({ payload }: { payload: Payload }) {
  const staleText = payload.stale ? payload.stale.text : null
  return (
    <div className="min-w-0 space-y-4" data-page="option-candidates">
      <Card data-header>
        <CardHeader>
          <CardTitle>
            <h1 className="text-2xl font-semibold">{payload.title}</h1>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex flex-wrap gap-2">
            {payload.chain_day ? <Badge data-chain-day>{payload.chain_day}</Badge> : null}
            {payload.computed_at ? <Badge variant="secondary" data-computed-at>{payload.computed_at}</Badge> : null}
            {payload.rule_version ? <Badge variant="outline" data-rule-version>Rule {payload.rule_version}</Badge> : null}
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
      <Tabs defaultValue={payload.groups[0].kind}>
        <TabsList>
          {payload.groups.map((group) => (
            <TabsTrigger key={group.kind} value={group.kind} data-tab={group.kind}>
              {group.title}
              <Badge variant="secondary">{group.counts.ok}</Badge>
              <Badge variant="outline">{group.counts.blocked}</Badge>
              <Badge variant="outline">{group.counts.unknown}</Badge>
            </TabsTrigger>
          ))}
        </TabsList>
        {payload.groups.map((group) => (
          <TabsContent key={group.kind} value={group.kind}>
            <CandidateGroup group={group} staleText={staleText} />
          </TabsContent>
        ))}
      </Tabs>
      {payload.notes.map((note, index) => (
        <p key={`${index}-${note}`} className="text-xs text-muted-foreground" data-note>
          {note}
        </p>
      ))}
    </div>
  )
}

export const registration = { schema: 'option_candidates_page', page: page<Payload>(validate, OptionCandidatesPage) }
