// The valuation params page (docs/design/valuation-structure.md, entry 8): the newest R, the proposal with its
// "Update R" form, the two tables, the R history and the owner forms. Every figure is a payload text; nothing is
// computed here.
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/valuation_params_page'
import { MarginTableView, OwnerForm, OwnerForms, RHistoryView, RTableView } from './ParamTables'

function Warnings({ lines }: { lines: string[] }) {
  return lines.length === 0 ? null : (
    <Alert>
      <TriangleAlert />
      <AlertDescription>
        <ul>
          {lines.map((line, at) => (
            <li key={at}>{line}</li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  )
}

function ProposalCard({ payload }: { payload: Payload }) {
  const { proposal, current_r: current, latest_yield: latest, form } = payload
  if (proposal === null) return null
  return (
    <Card>
      <CardHeader>
        <CardTitle>Proposal of R</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p>
          {latest ? `For the 10-year yield ${latest.value} of ${latest.day}: ` : ''}
          proposed R is US {proposal.r.us}, JP {proposal.r.jp}, HKCN {proposal.r.hkcn}
          {current ? ` (now US ${current.r.us}, JP ${current.r.jp}, HKCN ${current.r.hkcn})` : ''}.
        </p>
        <Warnings lines={proposal.step_warnings} />
        {form ? (
          <OwnerForm form={form} action="accept_r">
            <div className="flex flex-wrap items-end gap-3">
              <div className="space-y-1">
                <Label htmlFor="ar-yield">Yield</Label>
                <Input id="ar-yield" name="yield" defaultValue={latest?.value ?? ''} required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="ar-date">Effective date</Label>
                <Input id="ar-date" name="effective_date" defaultValue={latest?.day ?? ''} required />
              </div>
              <Button type="submit">Update R</Button>
            </div>
          </OwnerForm>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function ValuationParamsPage({ payload }: { payload: Payload }) {
  const current = payload.current_r
  return (
    <div className="min-w-0 space-y-6" data-page="valuation_params">
      <header>
        <h1 className="text-2xl font-semibold">Valuation parameters</h1>
        <p className="mt-2 text-sm">
          <a href="/valuation" className="underline">
            Valuation table
          </a>
        </p>
      </header>
      {payload.error !== '' ? (
        <Alert variant="destructive">
          <TriangleAlert />
          <AlertDescription>{payload.error}</AlertDescription>
        </Alert>
      ) : null}
      {payload.gaps.map((gap, at) => (
        <Alert key={at}>
          <TriangleAlert />
          <AlertDescription>{gap.text}</AlertDescription>
        </Alert>
      ))}
      {current ? (
        <Card>
          <CardHeader>
            <CardTitle>Current R, effective {current.effective_date}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <p>
              US {current.r.us}, JP {current.r.jp}, HKCN {current.r.hkcn}; AI switch {current.ai_switch ? 'on' : 'off'}
              {current.ai_expiry ? `, ${current.ai_expiry}` : ''}
            </p>
            <Warnings lines={current.step_warnings} />
          </CardContent>
        </Card>
      ) : null}
      <ProposalCard payload={payload} />
      {payload.r_table ? <RTableView table={payload.r_table} /> : null}
      {payload.margin_table ? <MarginTableView table={payload.margin_table} /> : null}
      <RHistoryView history={payload.r_history} />
      {payload.form ? <OwnerForms form={payload.form} latest={payload.latest_yield?.value ?? ''} /> : null}
    </div>
  )
}

export const registration = {
  schema: 'valuation_params_page',
  page: page<Payload>(validate, ValuationParamsPage),
}
