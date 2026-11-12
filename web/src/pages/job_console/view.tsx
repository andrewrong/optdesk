// The job console page (docs/design/job-console.md Decisions 12 and 13): one form per runnable job, the reason for a
// listed-only job, and the newest requests with a withdraw button and one line per key. Plain HTML forms post to the
// route of the payload; the only script is the reload timer driven by `refresh_seconds`. Every visible value is a
// payload field; this file computes nothing.
import { useEffect } from 'react'
import type { Job, Key, Request } from '@/generated/job_console.v1'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/job_console'

type Form = NonNullable<Payload['form']>

function Token({ form }: { form: Form }) {
  return (
    <>
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
    </>
  )
}

function RunForm({ job, form }: { job: Job; form: Form }) {
  const today = job.kind === 'today_only'
  return (
    <form method="post" action={form.post_path} data-run-form={job.id} className="flex flex-wrap items-center gap-3">
      <input type="hidden" name="action" value="run" />
      <input type="hidden" name="job" value={job.id} />
      <Token form={form} />
      <Label className="gap-2">
        Day
        {today ? (
          <Input type="date" name="day" value={job.today} readOnly className="w-auto" />
        ) : (
          <Input type="date" name="day" defaultValue={job.today} className="w-auto" />
        )}
      </Label>
      {job.force ? (
        <Label className="gap-2">
          <Switch name="force" />
          Force
        </Label>
      ) : (
        <span data-force-why className="text-sm text-muted-foreground">
          {job.force_why}
        </span>
      )}
      <Button type="submit">Run</Button>
    </form>
  )
}

function JobRow({ job, form }: { job: Job; form: Form | null }) {
  return (
    <TableRow data-job={job.id}>
      <TableCell className="font-medium">{job.id}</TableCell>
      <TableCell>
        {job.kind === 'listed_only' ? (
          <div data-listed-only className="space-y-1 text-sm">
            <p>{job.why_text}</p>
            <p className="text-muted-foreground">{job.needs_text}</p>
          </div>
        ) : form ? (
          <RunForm job={job} form={form} />
        ) : null}
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">{job.note}</TableCell>
    </TableRow>
  )
}

function JobsTable({ jobs, form }: { jobs: Job[]; form: Form | null }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Job</TableHead>
          <TableHead>Run</TableHead>
          <TableHead>Note</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {jobs.map((j) => (
          <JobRow key={j.id} job={j} form={form} />
        ))}
      </TableBody>
    </Table>
  )
}

function KeysTable({ keys }: { keys: Key[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Dataset</TableHead>
          <TableHead>Subject</TableHead>
          <TableHead>Outcome</TableHead>
          <TableHead>Key</TableHead>
          <TableHead>Reason</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {keys.map((k, i) => (
          <TableRow key={`${i}-${k.dataset}-${k.subject}`} data-outcome={k.outcome}>
            <TableCell>{k.dataset}</TableCell>
            <TableCell>{k.subject}</TableCell>
            <TableCell>
              <Badge variant="outline">{k.outcome}</Badge> <span data-outcome-text>{k.outcome_text}</span>
            </TableCell>
            <TableCell data-present>{k.present}</TableCell>
            <TableCell data-reason>{k.reason}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function RequestItem({ r, form }: { r: Request; form: Form | null }) {
  return (
    <AccordionItem value={String(r.id)} data-request={r.id}>
      <AccordionTrigger>
        <span className="flex flex-wrap items-center gap-2">
          <span className="font-medium">#{r.id}</span>
          <span>{r.job}</span>
          <span className="text-muted-foreground">{r.day}</span>
          <Badge data-state-badge={r.state} variant={r.state === 'failed' || r.state === 'refused' ? 'destructive' : 'secondary'}>
            {r.state}
          </Badge>
        </span>
      </AccordionTrigger>
      <AccordionContent className="space-y-3">
        <p data-state-text className="text-sm">
          {r.state_text}
        </p>
        <p className="text-sm text-muted-foreground">{r.at_text}</p>
        {r.state === 'queued' && form ? (
          <form method="post" action={form.post_path} data-withdraw-form={r.id}>
            <input type="hidden" name="action" value="withdraw" />
            <input type="hidden" name="id" value={r.id} />
            <Token form={form} />
            <Button type="submit" variant="outline">
              Withdraw
            </Button>
          </form>
        ) : null}
        {r.keys.length > 0 ? <KeysTable keys={r.keys} /> : null}
      </AccordionContent>
    </AccordionItem>
  )
}

function Reload({ seconds }: { seconds: number }) {
  useEffect(() => {
    if (seconds === 0) return
    const t = setTimeout(() => location.reload(), seconds * 1000)
    return () => clearTimeout(t)
  }, [seconds])
  return null
}

export function JobConsoleView({ payload }: { payload: Payload }) {
  const { form } = payload
  return (
    <div className="space-y-4">
      <Reload seconds={payload.refresh_seconds} />
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">{payload.title}</h1>
          </CardTitle>
          <p data-sentence className="text-sm text-muted-foreground">
            {payload.sentence}
          </p>
        </CardHeader>
        <CardContent className="space-y-3">
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          <JobsTable jobs={payload.jobs} form={form} />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Requests</CardTitle>
        </CardHeader>
        <CardContent>
          <Accordion type="multiple" defaultValue={payload.requests.map((r) => String(r.id))}>
            {payload.requests.map((r) => (
              <RequestItem key={r.id} r={r} form={form} />
            ))}
          </Accordion>
        </CardContent>
      </Card>
    </div>
  )
}

export const registration = { schema: 'job_console', page: page<Payload>(validate, JobConsoleView) }
