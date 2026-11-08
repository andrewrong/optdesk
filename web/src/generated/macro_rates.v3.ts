// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Text = string;
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
export type Text1 = string;

/**
 * View model of the macro rates page, version 2: every string the page shows is in the payload.
 */
export interface MacroRatesV3 {
  schema: "macro_rates";
  schema_version: 3;
  generated_at: string;
  session_day: string;
  title: Text;
  sentence: Text;
  series: Series[];
  gaps: GapLine[];
}
export interface Series {
  id: string;
  title: Text;
  unit: "percent" | "ratio" | "ccy_amount" | "count" | "index_points";
  latest: LatestValue | Gap;
  points: Point[];
}
export interface LatestValue {
  kind: "value";
  date: string;
  value: number;
  text: Text;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: Kind;
  reason: Reason;
  text: Text1;
}
export interface Point {
  date: string;
  value: number | null;
  text: Text;
}
export interface GapLine {
  series_id: string;
  date: string;
  reason: "no_value" | "stale";
  text: Text;
}
