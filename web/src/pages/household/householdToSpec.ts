// Maps blocks of the household_page payload to chart specs (ADR 0034, Charts). A pure mapping: every point,
// label and text is a field of the payload; nothing is computed or formatted here.
import type { ChartSpec } from '@/charts/spec'
import type { Payload } from '@/types/household_page'

type MoneyBlock = Payload['income']

type TrendSeries = NonNullable<Payload['trend']['series']>[number]

// The scopes in one chart. When any row carries a percent change, the y axis is the percent since each series' first
// point, so every line starts at 0, and a series without percent points has no line. When no row of any series
// carries one, each series with rows is a line of the rows' own net value. The x axis holds the union of the days; a
// line has a point only on the days its series has (no filling). A point's tooltip text is the payload's text.
// Returns null when no series has a row.
export function trendToSpec(series: readonly TrendSeries[]): ChartSpec | null {
  const percent = series.some((s) => s.rows.some((row) => row.change))
  const lines = series.filter((s) => (percent ? s.rows.some((row) => row.change) : s.rows.length > 0))
  if (lines.length === 0) return null
  const days = [...new Set(lines.flatMap((s) => s.rows.map((row) => row.day)))].sort()
  return {
    kind: 'line',
    legend: true,
    categories: days,
    series: lines.map((s) => ({
      name: s.label,
      hole: true,
      points: days.map((day) => {
        const row = s.rows.find((r) => r.day === day)
        if (!row) return { value: null }
        if (!percent) return { value: row.value.value, text: row.value.text }
        return row.change
          ? { value: row.change.value, text: `${row.change.text} ${row.value.text}` }
          : { value: null }
      }),
    })),
  }
}

// One chart per currency: amounts of different currencies are never drawn on one axis.
export function incomeToSpec(income: MoneyBlock, currency: 'USD' | 'JPY'): ChartSpec {
  const rows = income.rows.filter((row) => row.amount.currency === currency)
  return {
    kind: 'bar',
    categories: rows.map((row) => row.name),
    series: [
      {
        name: income.title,
        points: rows.map((row) => ({ value: row.amount.value, text: row.amount.text })),
      },
    ],
  }
}

// A floating waterfall (issue #1760): a transparent base series stacked under the visible height series, both sent
// by Python; the bar's label is the step's figure and its tooltip is the step's own sentence. Colours are the theme
// tokens of gain and loss.
export function monthChangeToSpec(block: Payload['month_change']): ChartSpec {
  // The colour follows the tone Python put on the change cell; the opening and closing levels have none.
  const color = (row: Payload['month_change']['rows'][number]) => {
    if (row.change?.tone === 'up') return 'var(--success)'
    if (row.change?.tone === 'down') return 'var(--destructive)'
    return 'var(--muted-foreground)'
  }
  return {
    kind: 'bar',
    categories: block.rows.map((row) => row.name),
    series: [
      {
        name: 'base',
        stack: 'waterfall',
        invisible: true,
        points: block.rows.map((row) => ({ value: row.base.value })),
      },
      {
        name: block.title,
        stack: 'waterfall',
        points: block.rows.map((row) => ({
          value: row.height.value,
          text: row.sentence,
          color: color(row),
          label: (row.change ?? row.level).text,
        })),
      },
    ],
  }
}
