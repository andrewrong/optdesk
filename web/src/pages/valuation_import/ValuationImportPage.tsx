// The valuation import page (docs/design/valuation-import.md#6): pick the file, Preview, read the table, choose the key of
// an unmatched name, tick, Confirm, read the result. The file stays in memory only; every figure and text shown is a
// payload field and nothing is computed here.
import { useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/valuation_import_page'
import { PlanTable } from './PlanTable'

const GENERAL = 'V is per listed share, in the listing currency'

function Lines({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null
  return (
    <p className="text-sm">
      <span className="text-muted-foreground">{title}: </span>
      {items.join(', ')}
    </p>
  )
}

function Summary({ payload }: { payload: Payload }) {
  const { r, margin, counts } = payload
  return (
    <Card>
      <CardHeader>
        <CardTitle>{payload.file?.name} · {payload.file?.as_of}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <p className="text-sm">
          insert {counts.insert} · same {counts.same} · older {counts.older} · refused {counts.refused} · needs tick{' '}
          {counts.needs_tick}
        </p>
        {r ? (
          <p className="text-sm">
            R (R) <Badge variant="secondary">{r.action}</Badge> US {r.file_us} JP {r.file_jp} HKCN {r.file_hkcn}
            <span className="text-muted-foreground">
              {' '}
              stored US {r.stored_us} JP {r.stored_jp} HKCN {r.stored_hkcn}
              {r.latest_yield ? ` · DGS10 ${r.latest_yield}` : ''}
            </span>
          </p>
        ) : null}
        {r?.action === 'older' ? (
          <Alert>
            <AlertDescription>older than stored</AlertDescription>
          </Alert>
        ) : null}
        {r?.problem ? (
          <Alert variant="destructive">
            <AlertDescription>{r.problem}</AlertDescription>
          </Alert>
        ) : null}
        {margin ? (
          <p className="text-sm">
            Margin table <Badge variant="secondary">{margin.action}</Badge>
            {margin.action === 'older' ? <span className="text-muted-foreground"> older than stored</span> : null}
          </p>
        ) : null}
        <Lines title="Not in file" items={payload.not_in_file} />
        <Lines title="No valuation in the file" items={payload.unvalued} />
        <Lines title="Removed" items={payload.removed} />
        {payload.flagged_note ? <p className="text-sm text-muted-foreground">{payload.flagged_note}</p> : null}
      </CardContent>
    </Card>
  )
}

function Result({ payload }: { payload: Payload }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Result</CardTitle>
      </CardHeader>
      <CardContent className="space-y-1">
        {payload.result.map((x, i) => (
          <p key={`${i}-${x.key}`} className="text-sm" data-key={x.key} data-outcome={x.outcome}>
            <span className="font-medium">{x.key}</span> <Badge variant={x.outcome === 'failed' ? 'destructive' : 'secondary'}>{x.outcome}</Badge>{' '}
            <span className="text-muted-foreground">{x.text}</span>
          </p>
        ))}
      </CardContent>
    </Card>
  )
}

export function ValuationImportPage({ payload }: { payload: Payload }) {
  const [general, setGeneral] = useState(false)
  const [ticks, setTicks] = useState<Record<string, boolean>>({})
  const [keys, setKeys] = useState<Record<string, string>>({})
  const [markets, setMarkets] = useState<Record<string, string>>({})

  const canConfirm = general && payload.rows.some((r) => r.action === 'insert')

  return (
    <div className="min-w-0 space-y-6" data-page="valuation-import">
      <header>
        <h1 className="text-2xl font-semibold">Import valuations</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Pick the export file, read the preview, then pick it again to confirm
        </p>
      </header>
      {payload.error ? (
        <Alert variant="destructive">
          <TriangleAlert />
          <AlertDescription>{payload.error}</AlertDescription>
        </Alert>
      ) : null}
      {payload.gaps.map((g, i) => (
        <Alert key={i}>
          <TriangleAlert />
          <AlertDescription>{g.text}</AlertDescription>
        </Alert>
      ))}
      {payload.problems.map((p, i) => (
        <Alert key={i} variant="destructive">
          <AlertDescription>{p.text}</AlertDescription>
        </Alert>
      ))}
      {payload.view !== 'result' ? (
        <form
          method="post"
          action={payload.post_path}
          encType="multipart/form-data"
          className="flex items-end gap-3"
          data-form="preview"
        >
          <input type="hidden" name="action" value="preview" />
          <div className="space-y-1">
            <Label htmlFor="valuation-file">File</Label>
            <Input id="valuation-file" type="file" name="file" accept=".json,application/json" />
          </div>
          <Button type="submit">Preview</Button>
        </form>
      ) : null}
      {payload.view !== 'empty' ? <Summary payload={payload} /> : null}
      {payload.view === 'preview' ? (
        <Card>
          <CardContent className="pt-4">
            <form
              method="post"
              action={payload.post_path}
              encType="multipart/form-data"
              className="space-y-4"
              data-form="confirm"
            >
              <input type="hidden" name="action" value="confirm" />
              <input type="hidden" name="file_sha256" value={payload.file?.sha256 ?? ''} />
              <PlanTable
                rows={payload.rows}
                keys={keys}
                markets={markets}
                ticks={ticks}
                onKey={(t, k) => setKeys((s) => ({ ...s, [t]: k }))}
                onMarket={(t, m) => setMarkets((s) => ({ ...s, [t]: m }))}
                onTick={(t, on) => setTicks((s) => ({ ...s, [t]: on }))}
              />
              <div className="flex items-center gap-2">
                <Checkbox id="general-tick" aria-label="general" checked={general} onCheckedChange={(c) => setGeneral(c === true)} />
                <Label htmlFor="general-tick">{GENERAL}</Label>
              </div>
              <div className="space-y-1">
                <Label htmlFor="valuation-file-confirm">Same file again</Label>
                <Input id="valuation-file-confirm" type="file" name="file" accept=".json,application/json" />
              </div>
              <Button type="submit" disabled={!canConfirm}>
                Confirm
              </Button>
            </form>
          </CardContent>
        </Card>
      ) : null}
      {payload.view === 'result' ? <Result payload={payload} /> : null}
    </div>
  )
}

export const registration = {
  schema: 'valuation_import_page',
  page: page<Payload>(validate, ValuationImportPage),
}
