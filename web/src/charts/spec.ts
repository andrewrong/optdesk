// The chart a page draws, as plain data (ADR 0034, Charts): every category, number, text and colour is a field of
// the payload; a mapping function fills it and nothing is computed or formatted. `Chart` draws it with recharts.
export interface ChartPoint {
  /** The number drawn; `null` draws nothing (a gap). */
  value: number | null
  /** Python's formatted figure, shown by the tooltip instead of `value`. */
  text?: string
  /** A theme colour of this point, such as `var(--success)`. */
  color?: string
  /** A text drawn on the point (above a bar). */
  label?: string
}

export interface ChartSeries {
  name: string
  points: ChartPoint[]
  /** Series with the same stack name are stacked. */
  stack?: string
  /** A series that draws nothing (the transparent base of a floating bar). */
  invisible?: boolean
  /** A theme colour of the whole series; the default is `var(--chart-N)`. */
  color?: string
  /** Draws the line dashed and thinner (a line chart only). */
  dashed?: boolean
  /** Keeps the series out of the legend, so several lines can share one entry. */
  legendHidden?: boolean
}

export interface ChartSpec {
  kind: 'line' | 'bar'
  categories: string[]
  series: ChartSeries[]
  /** Draws a legend of the series names. */
  legend?: boolean
}
