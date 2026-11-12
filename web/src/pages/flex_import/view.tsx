// The Flex import page (docs/design/flex-xml-import.md#9 web-flex-import-view): the upload form, the result of each
// file and the recent imports. Every visible value is a payload field; this file posts nothing by script and computes nothing.
import type { Alias, File, Recent } from '@/generated/flex_import.v1'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/flex_import'

type Form = NonNullable<Payload['form']>

const text = (t: string): Cell => ({ text: t })
const count = (n: number): Cell => ({ text: String(n), value: n })

const ALIAS_COLUMNS: Column[] = [
  { id: 'alias', label: 'Alias' },
  { id: 'statements', label: 'Statements', align: 'right' },
  { id: 'parsed', label: 'Rows parsed', align: 'right' },
  { id: 'new', label: 'New', align: 'right' },
  { id: 'present', label: 'Already present', align: 'right' },
]
const RECENT_COLUMNS: Column[] = [
  { id: 'at', label: 'Time' },
  { id: 'alias', label: 'Alias' },
  { id: 'status', label: 'Status' },
  { id: 'new', label: 'New', align: 'right' },
  { id: 'present', label: 'Already present', align: 'right' },
]

const aliasRow = (a: Alias) => ({
  alias: text(a.alias),
  statements: count(a.statements),
  parsed: count(a.rows_parsed),
  new: count(a.rows_new),
  present: count(a.rows_present),
})
const recentRow = (r: Recent) => ({
  at: text(r.at_text),
  alias: text(r.alias),
  status: { text: r.status, badge: true },
  new: count(r.rows_new),
  present: count(r.rows_present),
})

function UploadForm({ form }: { form: Form }) {
  return (
    <form method="post" action={form.post_path} encType="multipart/form-data" className="space-y-3">
      <input type="hidden" name="expires" value={form.expires} />
      <input type="hidden" name="mac" value={form.mac} />
      <Label htmlFor="flex-files">{form.max_file_text}</Label>
      <Input id="flex-files" type="file" name="files" multiple accept=".xml,.gz" />
      <Button type="submit">Upload</Button>
    </form>
  )
}

function FileResult({ file }: { file: File }) {
  return (
    <div data-file={file.name} className="space-y-2">
      <div className="flex items-center gap-2">
        <span className="font-medium">{file.name}</span>
        <Badge variant={file.status === 'failed' ? 'destructive' : 'secondary'}>{file.status_text}</Badge>
        <span className="text-sm text-muted-foreground">
          {file.period_first} {file.period_last}
        </span>
      </div>
      {file.reason ? (
        <Alert variant="destructive">
          <AlertDescription>{file.reason}</AlertDescription>
        </Alert>
      ) : null}
      {file.unmapped_statements > 0 ? (
        <Alert>
          <AlertDescription data-unmapped>{file.unmapped_statements}</AlertDescription>
        </Alert>
      ) : null}
      {file.aliases.length > 0 ? <DataTable columns={ALIAS_COLUMNS} rows={file.aliases.map(aliasRow)} /> : null}
    </div>
  )
}

export function FlexImportView({ payload }: { payload: Payload }) {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">{payload.title}</h1>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p>{payload.sentence}</p>
          <Alert>
            <AlertDescription>{payload.scope_note}</AlertDescription>
          </Alert>
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          {payload.form ? <UploadForm form={payload.form} /> : null}
        </CardContent>
      </Card>
      {payload.files.length > 0 ? (
        <Card>
          <CardContent className="space-y-4 pt-4">
            {payload.files.map((f, i) => (
              <FileResult key={`${i}-${f.name}`} file={f} />
            ))}
          </CardContent>
        </Card>
      ) : null}
      {payload.recent.length > 0 ? (
        <Card>
          <CardContent className="pt-4">
            <DataTable columns={RECENT_COLUMNS} rows={payload.recent.map(recentRow)} />
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

export const registration = { schema: 'flex_import', page: page<Payload>(validate, FlexImportView) }
