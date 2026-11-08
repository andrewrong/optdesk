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
export type Cell = Figure | Gap;

export interface OptionDataPage {
  schema: "option_data_page";
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
  chain_title: Text1;
  series_title: Text1;
  notes_title: Text1;
  notes: Text1[];
  gaps: Gap[];
  underlying: Underlying;
  chain: Chain;
  /**
   * @maxItems 3
   */
  quote_series: [] | [QuoteSeries] | [QuoteSeries, QuoteSeries] | [QuoteSeries, QuoteSeries, QuoteSeries];
  stale_after?: Timestamp;
  underlyings?: UnderlyingChoice[];
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
export interface Underlying {
  id: number;
  symbol: string | null;
  label: Text1;
  session_day: string | null;
  price: Cell;
  currency: string;
  /**
   * The vol line shown in the header; metric id: assigned by compute.
   */
  vol_line?: string;
  earnings_text?: Text1;
  earnings_tone?: "in_term" | "outside" | "none";
}
export interface Figure {
  kind: "value";
  value: number;
  text: Text1;
  unit: string;
  approx_usd_text?: Text1;
}
export interface Chain {
  session_day: string | null;
  rows: ChainRow[];
}
export interface ChainRow {
  instrument_id: number;
  expiry: string | null;
  right: "call" | "put" | null;
  strike: Cell;
  status: Status;
  bid: Cell;
  ask: Cell;
  last: Cell;
  volume: Cell;
  open_interest: Cell;
  iv: Cell;
  delta: Cell;
  gamma: Cell;
  theta: Cell;
  vega: Cell;
}
export interface Status {
  code: number;
  text: Text1;
}
export interface QuoteSeries {
  instrument_id: number;
  caption: Text1;
  points: QuotePoint[];
}
export interface QuotePoint {
  session_day: string;
  slot: Text1;
  bid: Cell;
  ask: Cell;
  last: Cell;
}
export interface UnderlyingChoice {
  id: number;
  label: Text1;
  href: string;
  current: boolean;
}
