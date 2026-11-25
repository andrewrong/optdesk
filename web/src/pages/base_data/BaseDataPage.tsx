// The base data page (docs/design/base-data-page.md#11): the top verdict, one table of datasets by group, the glossary
// and the thresholds, gaps in words. Every visible value is a payload field; this file computes nothing.
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ScrollHint } from '@/components/ScrollHint'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/base_data_page'
import { DatasetTable } from './DatasetTable'
import { LevelBadge } from './LevelBadge'

// The Alert has three tones (neutral, destructive, success), the Badge has one per level (LevelBadge); warn and unknown
// show as the neutral Alert because the Badge beside the text already carries their colour.
const ALERT_VARIANT = { pass: 'success', warn: 'default', fail: 'destructive', unknown: 'default' } as const

// The link reads in words; its href is still the payload's repair_path.
const REPAIR_LINK_TEXT = '到采集记录页处理'

export function BaseDataView({ payload }: { payload: Payload }) {
  const { top } = payload
  const { counts } = top
  const thresholds = payload.groups.flatMap((g) =>
    g.rows.flatMap((r) => r.checks.filter((c) => c.warn_at !== '' || c.fail_at !== '').map((c) => ({ ...c, dataset: r.dataset }))),
  )
  return (
    <div className="min-w-0 space-y-4">
      <Card className="min-w-0">
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl font-semibold">{'基础数据 (Base data)'}</h1>
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            as of {payload.as_of} (last closed session US {payload.last_sessions.XNYS}, JP {payload.last_sessions.XTKS})
          </p>
        </CardHeader>
        <CardContent className="space-y-3">
          <Alert variant={ALERT_VARIANT[top.level]} data-top={top.level}>
            <AlertTitle className="flex items-center gap-2">
              <LevelBadge level={top.level} />
              <span>
                pass {counts.pass}, warn {counts.warn}, fail {counts.fail}, unknown {counts.unknown}
              </span>
            </AlertTitle>
            <AlertDescription>{top.text_zh}</AlertDescription>
          </Alert>
          {payload.gaps.map((g, i) => (
            <Alert key={`${i}-${g.reason}`} data-gap={g.reason}>
              <AlertDescription>{g.text}</AlertDescription>
            </Alert>
          ))}
          <ScrollHint />
          <DatasetTable groups={payload.groups} />
          {payload.derived_note !== '' ? <p className="text-sm text-muted-foreground">{payload.derived_note}</p> : null}
          <a href={payload.repair_path} className="text-sm underline underline-offset-4">
            {REPAIR_LINK_TEXT}
          </a>
        </CardContent>
      </Card>
      {payload.glossary.length > 0 ? (
        <Card className="min-w-0">
          <CardHeader>
            <CardTitle>{'术语表 (Glossary)'}</CardTitle>
          </CardHeader>
          <CardContent>
            <dl data-glossary className="space-y-2 text-sm">
              {payload.glossary.map((g) => (
                <div key={g.term} data-term={g.term}>
                  <dt className="inline font-medium">{g.term}</dt> <dd className="inline text-muted-foreground">{g.text}</dd>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>
      ) : null}
      {thresholds.length > 0 ? (
        <Card className="min-w-0">
          <CardHeader>
            <CardTitle>{'阈值 (Thresholds)'}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table data-thresholds>
              <TableHeader>
                <TableRow>
                  <TableHead>{'数据集'}</TableHead>
                  <TableHead>{'规则'}</TableHead>
                  <TableHead>{'预警线'}</TableHead>
                  <TableHead>{'失败线'}</TableHead>
                  <TableHead>{'来源'}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {thresholds.map((c) => (
                  <TableRow key={`${c.dataset}/${c.rule}`}>
                    <TableCell>{c.dataset}</TableCell>
                    <TableCell>{c.rule}</TableCell>
                    <TableCell>{c.warn_at}</TableCell>
                    <TableCell>{c.fail_at}</TableCell>
                    <TableCell>{c.threshold_source}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

export const registration = { schema: 'base_data_page', page: page<Payload>(validate, BaseDataView) }
