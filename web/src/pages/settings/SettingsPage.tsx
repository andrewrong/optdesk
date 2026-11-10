// The owner settings page (Decision 11 of docs/design/settings-form.md): the rule parameter table, one switch per
// underlying, the confirm step and the history, as plain HTML forms that post to the route of the payload. Every visible
// value is a payload field; this file posts nothing by script, computes nothing and compares nothing.
import { CircleCheck, Info, TriangleAlert, type LucideIcon } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/settings_page'

export { validate as validateSettingsPage } from '@/types/settings_page'
export type { Payload as SettingsPayload } from '@/types/settings_page'

type Form = NonNullable<Payload['form']>
type HistoryRow = Payload['history'][number]

const NOTICE_ICON: Record<NonNullable<Payload['notice']>['kind'], LucideIcon> = {
  saved: CircleCheck,
  nothing_changed: Info,
  error: TriangleAlert,
}

// The token of a form, as the two hidden inputs the route verifies.
function Token({ form }: { form: Form }) {
  return (
    <>
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
    </>
  )
}

// What "use this value" posts for one history row: the row's own value as a first post.
function RevertFields({ row }: { row: HistoryRow }) {
  if (row.kind === 'param') return <input type="hidden" name={`param.${row.key ?? ''}`} value={row.value} />
  return (
    <>
      <input type="hidden" name="row" value={row.instrument_id ?? ''} />
      {row.excluded ? <input type="hidden" name="excluded" value={row.instrument_id ?? ''} /> : null}
    </>
  )
}

function ConfirmCard({ payload, confirm }: { payload: Payload; confirm: NonNullable<Payload['confirm']> }) {
  return (
    <Card data-confirm>
      <CardHeader>
        <CardTitle>
          <h2 className="text-lg font-semibold">确认修改</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>项目</TableHead>
              <TableHead>原值</TableHead>
              <TableHead>新值</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {confirm.rows.map((row, index) => (
              <TableRow key={`${index}-${row.label}`} data-confirm-row>
                <TableCell>{row.label}</TableCell>
                <TableCell>{row.old}</TableCell>
                <TableCell>{row.new}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <form method="post" action={confirm.form.post_path} data-confirm-form className="flex items-center gap-2">
          <Token form={confirm.form} />
          <input type="hidden" name="confirm" value="1" />
          {payload.params.map((param) => (
            <input key={param.key} type="hidden" name={`param.${param.key}`} value={param.value} />
          ))}
          {payload.rows.map((row) => (
            <input key={row.instrument_id} type="hidden" name="row" value={row.instrument_id} />
          ))}
          {payload.rows.map((row) =>
            row.excluded ? <input key={row.instrument_id} type="hidden" name="excluded" value={row.instrument_id} /> : null,
          )}
          <Button type="submit">确认保存</Button>
          <a href="/settings" className="text-sm underline" data-cancel>
            取消
          </a>
        </form>
      </CardContent>
    </Card>
  )
}

function EditCard({ payload }: { payload: Payload }) {
  const { form } = payload
  const locked = form === null
  return (
    <form method="post" action="/owner-settings" data-edit-form className="space-y-4">
      {form ? <Token form={form} /> : null}
      <Card>
        <CardHeader>
          <CardTitle>
            <h2 className="text-lg font-semibold">规则参数</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Table data-params>
            <TableHeader>
              <TableRow>
                <TableHead>参数</TableHead>
                <TableHead>单位</TableHead>
                <TableHead>当前值</TableHead>
                <TableHead>修改</TableHead>
                <TableHead>范围</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {payload.params.map((param) => (
                <TableRow key={param.key} data-param={param.key}>
                  <TableCell>
                    <Label htmlFor={`param-${param.key}`}>{param.label}</Label>
                  </TableCell>
                  <TableCell>{param.unit}</TableCell>
                  <TableCell data-current>{param.value === '' ? param.default : param.value}</TableCell>
                  <TableCell>
                    <Input
                      id={`param-${param.key}`}
                      name={`param.${param.key}`}
                      inputMode={param.kind === 'int' ? 'numeric' : 'decimal'}
                      defaultValue={param.value}
                      placeholder={param.default}
                      disabled={locked}
                    />
                  </TableCell>
                  <TableCell data-bounds>
                    {param.min} – {param.max}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>
            <h2 className="text-lg font-semibold">期权排除</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Table data-exclusions>
            <TableHeader>
              <TableRow>
                <TableHead>标的</TableHead>
                <TableHead>排除</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {payload.rows.map((row) => (
                <TableRow key={row.instrument_id} data-instrument={row.instrument_id}>
                  <TableCell>
                    <Label htmlFor={`excluded-${row.instrument_id}`}>{row.symbol}</Label>
                    <input type="hidden" name="row" value={row.instrument_id} />
                  </TableCell>
                  <TableCell>
                    <Switch
                      id={`excluded-${row.instrument_id}`}
                      name="excluded"
                      value={String(row.instrument_id)}
                      defaultChecked={row.excluded}
                      disabled={locked}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <Button type="submit" disabled={locked}>
        保存
      </Button>
    </form>
  )
}

function HistoryCard({ payload }: { payload: Payload }) {
  const { form } = payload
  return (
    <Card data-history>
      <CardHeader>
        <CardTitle>
          <h2 className="text-lg font-semibold">修改历史</h2>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>项目</TableHead>
              <TableHead>值</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {payload.history.map((row, index) => (
              <TableRow key={`${index}-${row.at}`} data-history-row>
                <TableCell>{row.at}</TableCell>
                <TableCell>{row.name}</TableCell>
                <TableCell>{row.value}</TableCell>
                <TableCell>
                  <form method="post" action="/owner-settings" data-revert>
                    {form ? <Token form={form} /> : null}
                    <RevertFields row={row} />
                    <Button type="submit" variant="outline" size="sm" disabled={form === null}>
                      使用此值
                    </Button>
                  </form>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

export function SettingsPage({ payload }: { payload: Payload }) {
  const NoticeIcon = payload.notice ? NOTICE_ICON[payload.notice.kind] : null
  return (
    <div className="min-w-0 space-y-4" data-page="settings">
      <h1 className="text-2xl font-semibold">{payload.title}</h1>
      {payload.notice && NoticeIcon ? (
        <Alert variant={payload.notice.kind === 'error' ? 'destructive' : 'default'} data-notice={payload.notice.kind}>
          <NoticeIcon aria-hidden />
          <AlertDescription>{payload.notice.text}</AlertDescription>
        </Alert>
      ) : null}
      {payload.gap ? (
        <Alert data-gap={payload.gap.reason}>
          <Info aria-hidden />
          <AlertDescription>{payload.gap.text}</AlertDescription>
        </Alert>
      ) : null}
      {payload.confirm ? <ConfirmCard payload={payload} confirm={payload.confirm} /> : <EditCard payload={payload} />}
      {payload.history.length > 0 ? <HistoryCard payload={payload} /> : null}
    </div>
  )
}

export const registration = { schema: 'settings_page', page: page<Payload>(validate, SettingsPage) }
