// The 市场概览 card of the home page (market_summary v3). Every visible string and figure is a field of the
// payload; this file formats, compares and computes nothing. `value` is used only by the table's user-started sort.
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import { MissingValue } from '@/components/data-table/missing-value'
import { DataTable, type Cell, type Column } from '@/components/data-table/data-table'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { page } from '@/shell/page'
import { validate, type Payload } from '@/types/market_summary'
import { CardNotes } from './CardNotes'
import { DataTimesFold } from './DataTimesFold'
import { StaleNotice } from './StaleNotice'
import { TitleLink } from './OverviewCard'

type Chart_ = Payload['card']['chart']

function specOf(chart: Chart_): ChartSpec {
  const [close, average, band] = chart.series_names
  const line = (name: string, key: 'close' | 'ma200') => ({
    name,
    points: chart.points.map((point) => ({ value: point[key] ?? null })),
  })
  const end = (key: 'band_low' | 'band_high') => chart.points.map((point) => ({ value: point[key] ?? null }))
  return {
    kind: 'line',
    legend: true,
    categories: chart.points.map((point) => point.label),
    series: [line(close, 'close'), line(average, 'ma200')],
    band: { name: band, low: end('band_low'), high: end('band_high') },
  }
}

// Python sends the column names in order: name, iv, rv20, difference.
function columnsOf(names: Payload['card']['index_table']['columns']): Column[] {
  const [name, iv, rv20, diff] = names
  return [
    { id: 'name', label: name },
    { id: 'iv', label: iv, align: 'right' },
    { id: 'rv20', label: rv20, align: 'right' },
    { id: 'diff', label: diff, align: 'right' },
  ]
}

type Slot = Payload['card']['index_table']['rows'][number]['iv']

// A figure slot is a value or a gap, told apart by its kind tag; a gap has no value and no tone.
function cellOf(slot: Slot): Cell {
  if (slot.kind === 'gap') return { reason: slot.reason, text: slot.text }
  return { text: slot.text, value: slot.value, tone: slot.tone }
}

export function MarketSummaryCard({ payload, titleHref }: { payload: Payload; titleHref?: string }) {
  const { card } = payload
  const rows = card.index_table.rows.map((row) => ({
    name: {
      text: row.name,
      value: row.name,
      note: row.source_text ?? null,
    } as Cell,
    iv: cellOf(row.iv),
    rv20: cellOf(row.rv20),
    diff: cellOf(row.iv_minus_rv20),
  }))
  const [first, last] = card.chart.end_labels
  return (
    <Card data-card="market-summary">
      <CardHeader>
        <CardTitle>
          <TitleLink href={titleHref}>{card.title}</TitleLink>
        </CardTitle>
        <CardDescription>{card.say}</CardDescription>
        <p className="text-xs text-muted-foreground" data-subtitle>
          {card.subtitle}
        </p>
      </CardHeader>
      <CardContent className="space-y-3">
        <StaleNotice text={card.stale.text} isStale={card.stale.is_stale} />
        <div data-chart-block>
          <Chart spec={specOf(card.chart)} ariaText={card.chart.aria_text} name="market-summary" />
          <div className="flex justify-between text-xs text-muted-foreground" data-end-labels>
            <span>{first}</span>
            <span>{last}</span>
          </div>
        </div>
        <div data-index-table>
          <DataTable columns={columnsOf(card.index_table.columns)} rows={rows} />
        </div>
        {card.vol_indices ? (
          <ul className="flex flex-wrap gap-2" data-vol-indices>
            {card.vol_indices.map((item) => (
              <li key={item.name} className="flex items-center gap-1 text-sm">
                <span>{item.name}</span>
                {item.level.kind === 'gap' ? <MissingValue text={item.level.text} /> : <span>{item.level.text}</span>}
                <span className="text-xs text-muted-foreground">
                  {item.day} {item.source_text}
                </span>
              </li>
            ))}
          </ul>
        ) : null}
        {card.vol_indices_note ? (
          <p className="text-xs text-muted-foreground" data-vol-indices-note>
            {card.vol_indices_note}
          </p>
        ) : null}
        <CardNotes notes={[...card.notes]} gaps={[...card.gaps]} />
        <DataTimesFold text={card.data_times_text} />
      </CardContent>
    </Card>
  )
}

export const registration = {
  schema: 'market_summary',
  page: page<Payload>(validate, MarketSummaryCard),
}
