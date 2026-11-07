// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Text = string;
export type Text1 = string;
export type Unit = "percent" | "ratio" | "ccy_amount" | "count";
export type Latest = LatestValue | Gap;
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
export type Text3 = string;
export type Text4 = string;
export type Text5 = string;
export type Text6 = string;

/**
 * View model of the macro rates page, version 2: every string the page shows is in the payload.
 */
export interface MacroRates {
  schema: "macro_rates";
  schema_version: 2;
  generated_at: string;
  session_day: string;
  title: Text;
  sentence: Text1;
  series: Series[];
  gaps: GapLine[];
}
export interface Series {
  id: string;
  title: Text2;
  unit: Unit;
  latest: Latest;
  points: Point[];
}
export interface LatestValue {
  kind: "value";
  date: string;
  value: number;
  text: Text3;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: "gap";
  reason: Reason;
  text: Text4;
}
export interface Point {
  date: string;
  value: number | null;
  text: Text5;
}
export interface GapLine {
  series_id: string;
  date: string;
  reason: "no_value" | "stale";
  text: Text6;
}
