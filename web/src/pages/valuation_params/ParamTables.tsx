// The read-only tables of the valuation params page (R table, margin table, R history) and the owner forms of the
// two tables and of the R version. Every figure is a decimal text of the payload, printed as given.
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { Payload } from '@/types/valuation_params_page'

type Form = NonNullable<Payload['form']>

// One line of static English text beside a term; none is a payload text and none computes.
// The source of each gloss is on its line (docs/design/valuation-structure.md).
export const HKCN_GLOSS = 'HKCN: Hong Kong and China names' // line 24
export const AI_GLOSS = 'AI switch: stored with the R version, changes no price yet' // line 32
const YIELD_GLOSS = 'Yield used: the 10-year yield the R was proposed from' // line 106
const MARGIN_GLOSS = 'P from: lowest P of the row; M: safety margin; W: fair half width; H: heavy discount; S: trim premium' // line 61
export function Gloss({ text }: { text: string }) {
  return <p className="text-xs font-normal whitespace-normal text-muted-foreground">{text}</p>
}

export function OwnerForm({
  form,
  action,
  children,
}: {
  form: Form
  action: string
  children: React.ReactNode
}) {
  return (
    <form method="post" action={form.post_path} data-form={action} className="space-y-3">
      <input type="hidden" name="action" value={action} />
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
      {children}
    </form>
  )
}

export function Field({ id, label, children }: { id: string; label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  )
}

export function RTableView({ table }: { table: NonNullable<Payload['r_table']> }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>R table, effective {table.effective_date}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <Gloss text={HKCN_GLOSS} />
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{'收益率上限'}</TableHead>
              <TableHead>US</TableHead>
              <TableHead>JP</TableHead>
              <TableHead>HKCN</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {table.rows.map((row, at) => (
              <TableRow key={at}>
                <TableCell>{row.yield_up_to}</TableCell>
                <TableCell>{row.us}</TableCell>
                <TableCell>{row.jp}</TableCell>
                <TableCell>{row.hkcn}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

export function MarginTableView({ table }: { table: NonNullable<Payload['margin_table']> }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Margin table, effective {table.effective_date}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <Gloss text={MARGIN_GLOSS} />
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{'P 起点'}</TableHead>
              <TableHead>M</TableHead>
              <TableHead>W</TableHead>
              <TableHead>H</TableHead>
              <TableHead>S</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {table.rows.map((row, at) => (
              <TableRow key={at}>
                <TableCell>{row.p_from}</TableCell>
                <TableCell>{row.m}</TableCell>
                <TableCell>{row.w}</TableCell>
                <TableCell>{row.h}</TableCell>
                <TableCell>{row.s}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

export function RHistoryView({ history }: { history: Payload['r_history'] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{'R 历史'}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <Gloss text={YIELD_GLOSS} />
        <Gloss text={AI_GLOSS} />
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{'生效日'}</TableHead>
              <TableHead>{'使用的收益率'}</TableHead>
              <TableHead>US</TableHead>
              <TableHead>JP</TableHead>
              <TableHead>HKCN</TableHead>
              <TableHead>{'AI 开关'}</TableHead>
              <TableHead>{'AI 到期日'}</TableHead>
              <TableHead>{'备注'}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {history.map((v, at) => (
              <TableRow key={at}>
                <TableCell>{v.effective_date}</TableCell>
                <TableCell>{v.yield_used}</TableCell>
                <TableCell>{v.r.us}</TableCell>
                <TableCell>{v.r.jp}</TableCell>
                <TableCell>{v.r.hkcn}</TableCell>
                <TableCell>{v.ai_switch ? 'on' : 'off'}</TableCell>
                <TableCell>{v.ai_expiry}</TableCell>
                <TableCell>{v.note}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function TableVersionForm({ form, action, title, hint }: { form: Form; action: string; title: string; hint: string }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <OwnerForm form={form} action={action}>
          <Field id={`${action}-date`} label="生效日">
            <Input id={`${action}-date`} name="effective_date" placeholder="YYYY-MM-DD" required />
          </Field>
          <Field id={`${action}-table`} label={hint}>
            <textarea
              id={`${action}-table`}
              name="table"
              rows={6}
              required
              className="w-full rounded-lg border border-input bg-transparent p-2 font-mono text-sm"
            />
          </Field>
          <Button type="submit">{'保存表'}</Button>
        </OwnerForm>
      </CardContent>
    </Card>
  )
}

export function OwnerForms({ form, latest }: { form: Form; latest: string }) {
  return (
    <>
      <TableVersionForm
        form={form}
        action="r_table"
        title="新增 R 表版本"
        hint="每行一档：收益率上限、US、JP、HKCN"
      />
      <TableVersionForm
        form={form}
        action="margin_table"
        title="新增保证金表版本"
        hint="每行一档：P 起点、M、W、H、S"
      />
      <Card>
        <CardHeader>
          <CardTitle>{'新增 R 版本'}</CardTitle>
        </CardHeader>
        <CardContent>
          <OwnerForm form={form} action="r_version">
            <div className="grid gap-3 sm:grid-cols-3">
              <Field id="rv-date" label="生效日">
                <Input id="rv-date" name="effective_date" placeholder="YYYY-MM-DD" required />
              </Field>
              <Field id="rv-us" label="R US">
                <Input id="rv-us" name="r_us" required />
              </Field>
              <Field id="rv-jp" label="R JP">
                <Input id="rv-jp" name="r_jp" required />
              </Field>
              <Field id="rv-hkcn" label="R HKCN">
                <Input id="rv-hkcn" name="r_hkcn" required />
              </Field>
              <Field id="rv-yield" label="使用的收益率">
                <Input id="rv-yield" name="yield_used" defaultValue={latest} />
              </Field>
              <Field id="rv-ai" label="AI 开关">
                <Select name="ai_switch" defaultValue="off">
                  <SelectTrigger id="rv-ai">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="off">off</SelectItem>
                    <SelectItem value="on">on</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field id="rv-expiry" label="AI 到期日（开启时必填）">
                <Input id="rv-expiry" name="ai_expiry" />
              </Field>
              <Field id="rv-note" label="备注">
                <Input id="rv-note" name="note" />
              </Field>
            </div>
            <Button type="submit">{'保存 R 版本'}</Button>
          </OwnerForm>
        </CardContent>
      </Card>
    </>
  )
}
