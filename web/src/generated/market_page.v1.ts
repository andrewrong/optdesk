// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp1 = string;
export type Id1 = string;
export type RunStatus = "ok" | "partial" | "failed";
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp2 = string;
export type IsStale = boolean;
export type Text = string;
export type Text1 = string;
export type Kind = "gap";
export type Reason =
  | "missing_quote"
  | "no_data"
  | "unsupported_currency"
  | "not_built"
  | "no_reader"
  | "no_news_source"
  | "no_rule"
  | "market_closed"
  | "stale"
  | "flex_time_unknown"
  | "jp_slice_missing"
  | "missing_input"
  | "stale_input";
export type Text2 = string;
export type FigureOrGap = Figure | Gap;

/**
 * Board 7: the market page. A render of numbers a producer writes; nothing is computed here.
 */
export interface MarketPage {
  schema: "market_page";
  schema_version: 1;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  data_times: SourceTime[];
  run_status: RunStatus;
  stale: StaleLabel | null;
  title: Text1;
  sentence: Text1;
  notes: Text1[];
  gaps: Gap[];
  weather: Weather;
  indices: Indices;
  term: Term | Gap;
  breadth: Breadth | Gap;
  stale_after?: Timestamp;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: Timestamp1;
  source: Id1;
}
/**
 * The times and the label. The producer sets is_stale; the browser compares no clock.
 */
export interface StaleLabel {
  data_time: Timestamp2;
  is_stale: IsStale;
  text: Text;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: Kind;
  reason: Reason;
  text: Text2;
}
export interface Weather {
  headline: Text1;
  sentence: Text1;
  chart: Chart | Gap;
}
export interface Chart {
  aria_text: Text1;
  /**
   * @minItems 1
   * @maxItems 3
   */
  series_names: [Text1] | [Text1, Text1] | [Text1, Text1, Text1];
  /**
   * @maxItems 400
   */
  points: ChartPoint[];
}
export interface ChartPoint {
  label: Text1;
  /**
   * Index close (metric id: assigned by compute)
   */
  close: number;
  /**
   * 200-day line (metric id: assigned by compute)
   */
  ma200: number;
}
export interface Indices {
  title: Text1;
  sentence: Text1;
  columns: Text1[];
  rows: IndexRow[];
  note: Text1;
}
/**
 * Each figure is written by the producer (metric id: assigned by compute); a missing one is a Gap.
 */
export interface IndexRow {
  name: Text1;
  iv: FigureOrGap;
  rv20: FigureOrGap;
  iv_minus_rv20: FigureOrGap;
  rank_3y: FigureOrGap;
}
/**
 * A number the producer wrote; the view prints text.
 */
export interface Figure {
  kind: "value";
  value: number;
  text: Text1;
  unit: string;
}
export interface Term {
  title: Text1;
  sentence: Text1;
  points: TermPoint[];
}
/**
 * Term curve values (metric id: assigned by compute).
 */
export interface TermPoint {
  label: Text1;
  today: FigureOrGap;
  five_days_ago: FigureOrGap;
}
export interface Breadth {
  title: Text1;
  rows: BreadthRow[];
}
/**
 * Breadth figure (metric id: assigned by compute).
 */
export interface BreadthRow {
  label: Text1;
  figure: FigureOrGap;
}
