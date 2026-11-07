// The figures of the report page: a Cell, a MoneyCell or a Gap is drawn as its text, in the tone it carries.
// Nothing is formatted or computed here; a numeric `value` is only handed to a table (for sorting), a meter or a chart.
import { DataTable, TONE_CLASS, type Cell, type Column } from '@/components/data-table/data-table'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import type { Gap, Meter, Meter1 } from '@/generated/report_page.v2'

type Tone = 'up' | 'down' | 'flat' | 'none'
// Every figure of the schema (Cell, MoneyCell, the *Figure types) has this shape; a Gap has `reason`.
export type Figure = { text: string; tone?: Tone | null; value?: number | null }
export type Shown = string | Figure | Gap

// A Gap stands in place of a figure, a table or a list; no payload object other than a Gap has a `reason`.
export function isGap(value: unknown): value is Gap {
  return typeof value === 'object' && value !== null && 'reason' in value
}

export function toCell(shown: Shown): Cell {
  if (typeof shown === 'string') return { text: shown }
  if (isGap(shown)) return { reason: shown.reason, text: shown.text }
  return { text: shown.text, tone: shown.tone ?? 'none', value: shown.value }
}

export function ToneText({ text, tone }: { text: string; tone?: Tone | null }) {
  const className = tone && tone !== 'none' ? TONE_CLASS[tone] : undefined
  return <span className={className}>{text}</span>
}

export function GapMark({ gap }: { gap: Gap }) {
  return (
    <Badge variant="secondary" className="h-auto whitespace-normal text-left" data-gap={gap.reason}>
      {gap.text}
    </Badge>
  )
}

export function Fig({ shown }: { shown: Shown }) {
  if (isGap(shown)) return <GapMark gap={shown} />
  if (typeof shown === 'string') return <span>{shown}</span>
  return <ToneText text={shown.text} tone={shown.tone} />
}

// `labels` are the column headings the payload sends; a column it does not label has an empty heading.
export function Tabular({ labels, rows }: { labels: readonly string[]; rows: readonly (readonly Shown[])[] }) {
  let width = labels.length
  for (const row of rows) if (row.length > width) width = row.length
  const columns: Column[] = Array.from({ length: width }, (_, at) => ({
    id: `c${at}`,
    label: labels[at] ?? '',
  }))
  const data = rows.map((row) => Object.fromEntries(row.map((shown, at) => [`c${at}`, toCell(shown)])))
  return <DataTable columns={columns} rows={data} />
}

// A meter of the open slot: `percent` is the share of the limit used as the payload states it.
export function Meters({ meters }: { meters: readonly Meter1[] | null | undefined }) {
  if (!meters || meters.length === 0) return null
  return (
    <ul className="space-y-2" data-meters>
      {meters.map((meter) => (
        <li key={meter.label} className="space-y-1">
          <div className="flex justify-between gap-4 text-sm">
            <span>{meter.label}</span>
            <Fig shown={meter.value} />
          </div>
          {isGap(meter.percent) ? (
            <GapMark gap={meter.percent} />
          ) : (
            <Progress value={meter.percent.value} aria-label={meter.label} />
          )}
        </li>
      ))}
    </ul>
  )
}

// A meter of the close slot: the cash left after an assignment and the margin buffer of one account.
export function AccountMeters({ meters }: { meters: readonly Meter[] | null | undefined }) {
  if (!meters || meters.length === 0) return null
  return (
    <ul className="space-y-1 text-sm" data-meters>
      {meters.map((meter) => (
        <li key={meter.account} className="flex flex-wrap gap-x-4">
          <span>{meter.account}</span>
          <Fig shown={meter.cash_after_assignment} />
          <Fig shown={meter.margin_buffer} />
        </li>
      ))}
    </ul>
  )
}
