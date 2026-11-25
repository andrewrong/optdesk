// The repair forms of the data health page (docs/design/data-health.md Decisions 4 and 8). Plain HTML forms post to
// `form.post_path`; the server recomputes everything but the job, the day, the dataset and the token. No `force` field.
import { Button } from '@/components/ui/button'
import type { Payload } from '@/types/data_health_page'

type Form = NonNullable<Payload['form']>

function Token({ form }: { form: Form }) {
  return (
    <>
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
    </>
  )
}

// One cell's button: the existing `action=run` of the job console for one job and one day.
export function CellRepair({ form, job, day }: { form: Form; job: string; day: string }) {
  return (
    <form method="post" action={form.post_path} data-repair-cell={day}>
      <input type="hidden" name="action" value="run" />
      <input type="hidden" name="job" value={job} />
      <input type="hidden" name="day" value={day} />
      <Token form={form} />
      <Button type="submit" size="sm" variant="outline">
        修复
      </Button>
    </form>
  )
}

// The button of a dataset: its label carries `gap_days` as the builder sent it.
export function RepairAllForm({ form, dataset, gapDays }: { form: Form; dataset: string; gapDays: number }) {
  return (
    <form method="post" action={form.post_path} data-repair-all={dataset}>
      <input type="hidden" name="action" value="repair" />
      <input type="hidden" name="dataset" value={dataset} />
      <Token form={form} />
      <Button type="submit" size="sm">
        修复 {gapDays} 天
      </Button>
    </form>
  )
}
