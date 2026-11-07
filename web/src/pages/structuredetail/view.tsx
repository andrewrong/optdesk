// The structure detail page (page.structure_detail.v3): read-only; every string and figure is a field of the payload.
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/structure_detail'
import { GapLines, KeyLevels, StaleLabel, Tiles, Timeline } from './Blocks'
import { PayoffChart } from './Payoff'
import { FillsTable, GreeksTable, LegsTable } from './Tables'

export function StructureDetailView({ payload }: { payload: Payload }) {
  const { greeks, payoff } = payload
  return (
    <div className="min-w-0 space-y-6" data-page="structure-detail">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        <p className="mt-2">{payload.sentence}</p>
        <p className="text-muted-foreground">{payload.subline}</p>
      </header>
      <StaleLabel stale={payload.stale} />
      <GapLines gaps={payload.gaps} />
      <Tiles tiles={payload.tiles} />
      <Card>
        <CardHeader>
          <CardTitle>{payload.legs_title}</CardTitle>
        </CardHeader>
        <CardContent>
          <LegsTable legs={payload.legs} total={payload.legs_total} />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{payload.greeks_title}</CardTitle>
        </CardHeader>
        <CardContent>
          {'reason' in greeks ? (
            <p data-gap={greeks.reason}>{greeks.text}</p>
          ) : (
            <>
              <p className="mb-2 text-xs text-muted-foreground">{greeks.model_text}</p>
              <GreeksTable greeks={greeks} />
            </>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{payload.fills_title}</CardTitle>
        </CardHeader>
        <CardContent>
          <FillsTable fills={payload.fills} />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{payload.payoff_title}</CardTitle>
        </CardHeader>
        <CardContent>
          {'reason' in payoff ? (
            <p data-gap={payoff.reason}>{payoff.text}</p>
          ) : (
            <PayoffChart payoff={payoff} />
          )}
        </CardContent>
      </Card>
      <KeyLevels title={payload.levels_title} levels={payload.key_levels} />
      <Timeline title={payload.timeline_title} stages={payload.timeline} />
    </div>
  )
}

export const registration = {
  schema: 'structure_detail',
  page: page<Payload>(validate, StructureDetailView),
}
