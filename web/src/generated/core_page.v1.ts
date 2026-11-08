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

/**
 * Board 15: core. The page gives no target price and feeds no rule.
 */
export interface CorePage {
  schema: "core_page";
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
  basis_note: Text1;
  scale: Scale;
  rows: CoreRow[];
  jp_note: Text1;
  legend: Text1[];
  snapshot_text: Text1;
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
export interface Scale {
  low_text: Text1;
  high_text: Text1;
  treasury: Treasury;
}
export interface Treasury {
  text: Text1;
  /**
   * Position of the treasury mark on the bar, 0 to 100 (metric id: assigned by compute)
   */
  marker_percent: number;
}
export interface CoreRow {
  symbol: Text1;
  price_text: Text1;
  cyclical: boolean;
  flag: Text1 | null;
  returns:
    | [ReturnPoint]
    | [ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint]
    | Gap;
}
/**
 * Return at today's price under one assumption; the text is produced.
 */
export interface ReturnPoint {
  assumption: Text1;
  text: Text1;
  /**
   * Position of the dot on the bar, 0 to 100 (metric id: assigned by compute)
   */
  marker_percent: number;
}
