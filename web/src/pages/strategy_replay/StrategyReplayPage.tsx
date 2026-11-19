// The strategy replay page (docs/design/strategy-replay-page.md#4): a native form posted to post_path, the notices, and
// the report under it as the server sent it. Every label, hint and figure is a payload field; nothing is computed here.
import { TriangleAlert } from 'lucide-react'
import type { Field } from '@/generated/strategy_replay_page.v1'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/strategy_replay_page'
import { ReplayReport } from './ReplayReport'

const SELECT_CLASS =
  'h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50'

function FieldRow({ field }: { field: Field }) {
  const id = `replay-${field.name}`
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{field.label}</Label>
      {field.kind === 'choice' ? (
        <select id={id} name={field.name} defaultValue={field.value} className={SELECT_CLASS}>
          {field.options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      ) : (
        <Input id={id} name={field.name} defaultValue={field.value} />
      )}
      <p className="text-xs text-muted-foreground">{field.hint}</p>
    </div>
  )
}

export function StrategyReplayPage({ payload }: { payload: Payload }) {
  const { form } = payload
  return (
    <div className="min-w-0 space-y-6" data-page="strategy-replay">
      <header>
        <h1 className="text-2xl font-semibold">{payload.title}</h1>
        {form ? <p className="mt-2 text-sm text-muted-foreground">{payload.intro}</p> : null}
      </header>
      {payload.notices.map((n) => (
        <Alert key={n.key} variant={n.key === 'form_off' ? 'default' : 'destructive'} data-notice={n.key}>
          <TriangleAlert />
          <AlertDescription>{n.text}</AlertDescription>
        </Alert>
      ))}
      {form ? (
        <Card>
          <CardContent className="pt-4">
            <form method="post" action={payload.post_path} className="space-y-4" data-form="replay">
              {form.fields.map((f) => (
                <FieldRow key={f.name} field={f} />
              ))}
              <div className="flex items-center gap-3">
                <Button type="submit">{form.submit_label}</Button>
                <span className="text-xs text-muted-foreground">{payload.restart_hint}</span>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : null}
      {form && payload.report ? <ReplayReport report={payload.report} /> : null}
    </div>
  )
}

export const registration = {
  schema: 'strategy_replay_page',
  page: page<Payload>(validate, StrategyReplayPage),
}
