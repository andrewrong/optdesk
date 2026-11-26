// The data health page (docs/design/data-health.md#7): the coverage grid, the legend of the glossary and the
// overwrite note. Every visible value is a payload field; this file computes nothing and holds no rule of repair.
import { Info } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/data_health_page'
import { DataCheckBlock } from './DataCheckBlock'
import { CoverageGrid } from './CoverageGrid'
import { FailuresTable } from './FailuresTable'
import { GatewayBlock } from './GatewayBlock'

export function DataHealthView({ payload }: { payload: Payload }) {
  return (
    <div className="space-y-4">
      {payload.gateways && payload.gateways.length > 0 ? <GatewayBlock gateways={payload.gateways} /> : null}
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">数据健康</h1>
          </CardTitle>
          <p className="text-sm text-muted-foreground">{payload.overwrite_note}</p>
        </CardHeader>
        <CardContent className="space-y-3">
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          {(payload.running ?? []).map((r, i) => (
            <p key={`${i}-${r.line}`} data-running className="text-sm">
              {r.line}
            </p>
          ))}
          <CoverageGrid datasets={payload.datasets} form={payload.form} />
        </CardContent>
      </Card>
      {payload.data_check ? <DataCheckBlock check={payload.data_check} /> : null}
      {payload.datasets.some((d) => d.repeated_failures !== undefined) ? (
        <Card>
          <CardHeader>
            <CardTitle>重复失败</CardTitle>
            {payload.failures_note ? <p className="text-sm text-muted-foreground">{payload.failures_note}</p> : null}
          </CardHeader>
          <CardContent className="space-y-4">
            {payload.datasets.map((d) => (
              <FailuresTable key={d.dataset_id} dataset={d} noneText={payload.failures_none_text} />
            ))}
          </CardContent>
        </Card>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>图例</CardTitle>
        </CardHeader>
        <CardContent>
          <dl data-legend className="space-y-2 text-sm">
            {payload.glossary.map((g) => (
              <div key={g.term} data-term={g.term} className="flex items-start gap-2">
                <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                <div>
                  <dt className="inline font-medium">{g.term}</dt> <dd className="inline">{g.text}</dd>
                </div>
              </div>
            ))}
          </dl>
        </CardContent>
      </Card>
    </div>
  )
}

export const registration = { schema: 'data_health_page', page: page<Payload>(validate, DataHealthView) }
