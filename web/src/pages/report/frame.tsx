// The header of the page and the frame every block shares: title, sub, say, note, state, gaps, gap_text,
// the folded data_times and the link. All text is a field of the payload.
import { Info } from 'lucide-react'
import type { ReactNode } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Gap, Link, ReportPage } from '@/generated/report_page.v2'
import { DataTimesFold } from '@/pages/home/DataTimesFold'
import { StaleAlert } from '@/pages/home/StaleAlert'

export interface FrameBlock {
  key: string
  title: string
  sub: string
  say: string
  note?: string | null
  state: 'ok' | 'partial' | 'gap'
  gaps: readonly Gap[]
  gap_text?: string
  data_times: string
  link?: Link | null
}

// The badge words present the closed enum `state`; they are shell text.
const STATE_BADGE = {
  ok: { word: '正常', variant: 'success' },
  partial: { word: '延迟', variant: 'warning' },
  gap: { word: '缺', variant: 'down' },
} as const

export function LinkButton({ link }: { link: Link }) {
  return (
    <Button asChild variant="link" size="sm">
      <a href={link.href}>{link.label}</a>
    </Button>
  )
}

export function Frame({ block, children }: { block: FrameBlock; children?: ReactNode }) {
  const degraded = block.state !== 'ok'
  return (
    <Card data-block-key={block.key} data-state={block.state}>
      <CardHeader>
        <CardTitle>{block.title}</CardTitle>
        <CardDescription>{block.sub}</CardDescription>
        <Badge variant={STATE_BADGE[block.state].variant}>
          {STATE_BADGE[block.state].word}
        </Badge>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm">{block.say}</p>
        {block.note ? <p className="text-xs text-muted-foreground">{block.note}</p> : null}
        {degraded ? (
          <Alert data-gaps>
            <Info aria-hidden />
            <AlertDescription>
              {block.gap_text ? <p>{block.gap_text}</p> : null}
              <ul className="list-disc pl-4">
                {block.gaps.map((gap, at) => (
                  <li key={`${at}-${gap.text}`}>{gap.text}</li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        ) : null}
        {children}
        <DataTimesFold text={block.data_times} />
        {block.link ? <LinkButton link={block.link} /> : null}
      </CardContent>
    </Card>
  )
}

export function Header({ payload }: { payload: ReportPage }) {
  return (
    <header className="space-y-2" data-report-header>
      <h1 className="text-xl font-semibold">{payload.title}</h1>
      <p className="text-sm">{payload.say}</p>
      <div className="flex justify-between gap-4 text-sm text-muted-foreground">
        <span>{payload.banner.left}</span>
        <span>{payload.banner.right}</span>
      </div>
      {payload.chips && payload.chips.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {payload.chips.map((chip, at) => (
            <Badge key={`${at}-${chip.text}`} variant="secondary">
              {chip.text}
            </Badge>
          ))}
        </div>
      ) : null}
      {payload.stale ? <StaleAlert stale={payload.stale} generatedAt={payload.generated_at} /> : null}
      <dl className="text-xs" data-status-lines>
        {payload.status_lines.map((line, at) => (
          <div key={`${at}-${line.label}`} className="flex gap-2">
            <dt className="text-muted-foreground">{line.label}</dt>
            <dd>{line.text}</dd>
          </div>
        ))}
      </dl>
    </header>
  )
}
