// The entry form of a review (or of a profile): a plain HTML form posting to `form.post_path`. "Calculate" posts
// `preview`, whose answer page carries `lines`, printed as given; nothing is computed here. The debounced preview
// by script is not built: the lint rule [network] of eslint.config.js forbids `fetch` (see the pull request).
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Payload } from '@/types/valuation_page'
import { LinesView } from './CompanyView'

type Form = NonNullable<Payload['form']>

const TYPES = ['first', 'post_earnings', 'event', 'backfill'] as const
const TRIGGERS = ['a', 'b', 'c', 'd'] as const
const SCORES = ['moat', 'understand', 'management', 'financial'] as const
const REASONS = ['p', ...SCORES] as const
const ORIGIN = [
  ['fiscal_period_end', 'Fiscal period end'],
  ['report_date', 'Report date'],
  ['earnings_base', 'Earnings base'],
  ['base_basis', 'Base basis'],
  ['forward_factor', 'Forward factor'],
  ['locked_multiple', 'Locked multiple'],
  ['method_1', 'Method result 1'],
  ['method_2', 'Method result 2'],
  ['method_3', 'Method result 3'],
] as const

function Field({ name, label, required, children }: { name: string; label: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={`f-${name}`}>
        {label}
        {required ? <span aria-hidden> *</span> : null}
      </Label>
      {children}
    </div>
  )
}

function Text({ name, label, required }: { name: string; label: string; required?: boolean }) {
  return (
    <Field name={name} label={label} required={required}>
      <Input id={`f-${name}`} name={name} required={required} aria-required={required} />
    </Field>
  )
}

function Problems({ problems }: { problems: Payload['problems'] }) {
  if (problems.length === 0) return null
  return (
    <Alert variant="destructive" data-problems>
      <AlertDescription>
        <ul>
          {problems.map((p, at) => (
            <li key={at} data-field={p.field}>
              {p.field !== '' ? <strong>{p.field}: </strong> : null}
              {p.text}
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  )
}

export function ReviewForm({ payload, form }: { payload: Payload; form: Form }) {
  const { company } = payload
  const { lines } = payload
  return (
    <Card data-view="form">
      <CardHeader>
        <CardTitle>{company ? `Valuation of ${company.name}` : 'Valuation of a new name'}</CardTitle>
      </CardHeader>
      <CardContent>
        <form method="post" action={form.post_path} data-form="valuation-form" className="space-y-4">
          <input type="hidden" name="expires" value={form.expires} />
          <input type="hidden" name="mac" value={form.mac} />
          <input type="hidden" name="review_id" value={payload.current?.review_id ?? ''} />
          {company ? (
            <input type="hidden" name="company" value={company.company_key} />
          ) : (
            <Text name="key" label="Key" required />
          )}
          <Problems problems={payload.problems} />
          <div className="grid gap-3 sm:grid-cols-2">
            <Field name="type" label="Review type" required>
              <Select name="type" required defaultValue="first">
                <SelectTrigger id="f-type" aria-required>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TYPES.map((t) => (
                    <SelectItem key={t} value={t}>
                      {t}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Text name="version_date" label="Version date" required />
            <Text name="v" label="V" required />
            <Text name="p" label="P" required />
          </div>
          {lines ? <LinesView lines={lines} /> : null}

          <details>
            <summary className="cursor-pointer text-sm font-medium">Scores with reasons</summary>
            <div className="mt-2 grid gap-3 sm:grid-cols-2">
              {SCORES.map((s) => (
                <Text key={s} name={s} label={`Score ${s}`} />
              ))}
              {REASONS.map((s) => (
                <Text key={s} name={`reason_${s}`} label={`Reason ${s}`} />
              ))}
            </div>
          </details>
          <details>
            <summary className="cursor-pointer text-sm font-medium">Origin of V</summary>
            <div className="mt-2 grid gap-3 sm:grid-cols-2">
              {ORIGIN.map(([name, label]) => (
                <Text key={name} name={name} label={label} />
              ))}
              <Field name="trigger" label="Trigger">
                <Select name="trigger">
                  <SelectTrigger id="f-trigger">
                    <SelectValue placeholder="none" />
                  </SelectTrigger>
                  <SelectContent>
                    {TRIGGERS.map((t) => (
                      <SelectItem key={t} value={t}>
                        {t}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Text name="change_memo" label="Change memo" />
              <Text name="reaction" label="Reaction" />
            </div>
          </details>
          <details>
            <summary className="cursor-pointer text-sm font-medium">Report text</summary>
            <textarea
              name="report_text"
              rows={6}
              aria-label="Report text"
              className="mt-2 w-full rounded-lg border border-input bg-transparent p-2 text-sm"
            />
          </details>

          <div className="flex flex-wrap gap-2">
            <Button type="submit" name="action" value="preview" variant="secondary" formNoValidate>
              Calculate
            </Button>
            <Button type="submit" name="action" value="draft">
              Save draft
            </Button>
            <Button type="submit" name="action" value="effective">
              Make effective
            </Button>
          </div>
          <details>
            <summary className="cursor-pointer text-sm font-medium">Void a review</summary>
            <div className="mt-2 flex flex-wrap items-end gap-2">
              <Text name="void_reason" label="Void reason" />
              <Button type="submit" name="action" value="void" variant="destructive">
                Void
              </Button>
            </div>
          </details>
          {company === null ? (
            <details>
              <summary className="cursor-pointer text-sm font-medium">Profile of a new name</summary>
              <div className="mt-2 grid gap-3 sm:grid-cols-2">
                <Text name="name" label="Name" />
                <Text name="symbol" label="Symbol" />
                <Text name="currency" label="Currency" />
                <Field name="r_market" label="R market">
                  <Select name="r_market">
                    <SelectTrigger id="f-r_market">
                      <SelectValue placeholder="choose" />
                    </SelectTrigger>
                    <SelectContent>
                      {['US', 'JP', 'HKCN'].map((m) => (
                        <SelectItem key={m} value={m}>
                          {m}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
                <Text name="note" label="Note" />
                <div className="flex items-end">
                  <Button type="submit" name="action" value="profile" variant="outline">
                    Save profile
                  </Button>
                </div>
              </div>
            </details>
          ) : null}
        </form>
      </CardContent>
    </Card>
  )
}
