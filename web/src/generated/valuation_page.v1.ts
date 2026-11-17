// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Text = string;
export type Optional = string;
export type OptFigure = string;
export type OptDay = string;
export type Zone = "" | "core" | "clear" | "trim" | "fair" | "heavy" | "buy" | "between";
export type Market = "US" | "JP" | "HKCN";
export type Day = string;
export type Long = string;
export type Figure = string;
export type ZoneNote = "" | "no_close" | "no_zone_market";

/**
 * View model of the valuation pages: the table of every name, the page of one company and the entry form, told apart by view. Every figure is a decimal text written by the builder from the function's result; the view computes nothing.
 */
export interface ValuationPage {
  schema: "valuation_page";
  schema_version: 1;
  run_id: Id;
  built_at: Timestamp;
  view: "table" | "company" | "form";
  rows: Row[] | null;
  unvalued_count: number;
  company: Company | null;
  current: Current | null;
  lines: Lines | null;
  band: Band | null;
  close: Close | null;
  history: HistoryRow[] | null;
  required: Text[];
  problems: Problem[];
  error: Optional;
  saved: Optional;
  gaps: Gap[];
  form: Form | null;
}
export interface Row {
  company_key: Text;
  name: Text;
  valued: boolean;
  not_entered: "" | "not entered";
  entry_link: Optional;
  v: OptFigure;
  version_date: OptDay;
  r: OptFigure;
  c: OptFigure;
  heavy: OptFigure;
  buy_zone: Optional;
  fair_zone: Optional;
  trim: OptFigure;
  clear: OptFigure;
  core: OptFigure;
  close: OptFigure;
  zone: Zone;
  flags: Flags | null;
  p: OptFigure;
  buy_price: OptFigure;
  status: "" | "draft" | "effective";
}
export interface Flags {
  at_heavy: boolean;
  in_buy: boolean;
  in_fair: boolean;
  at_trim: boolean;
  at_clear: boolean;
  at_core: boolean;
}
export interface Company {
  company_key: Text;
  name: Text;
  symbol: Optional;
  currency: Text;
  r_market: Market;
  pool: "in_pool" | "watch" | "removed";
  pool_date: OptDay;
  note: Optional;
}
export interface Current {
  review_id: Text;
  type: "first" | "post_earnings" | "event" | "backfill";
  version_date: Day;
  status: "draft" | "effective" | "void";
  v: OptFigure;
  p: OptFigure;
  trigger: "" | "a" | "b" | "c" | "d";
  change_memo: Long;
  detail: Detail;
}
export interface Detail {
  source: "owner" | "program";
  fiscal_period_end: OptDay;
  report_date: OptDay;
  earnings_base: OptFigure;
  base_basis: Optional;
  forward_factor: OptFigure;
  locked_multiple: OptFigure;
  /**
   * @maxItems 3
   */
  method_results: [] | [OptFigure] | [OptFigure, OptFigure] | [OptFigure, OptFigure, OptFigure];
  adjustments: Adjustment[];
  scenarios: Scenario[];
  reaction: Long;
  moat: Optional;
  understand: Optional;
  management: Optional;
  financial: Optional;
  reasons: Reasons;
  report_text: Long;
  entered_at: Timestamp;
  effective_at: Timestamp | null;
}
export interface Adjustment {
  percent: OptFigure;
  reason: Optional;
}
export interface Scenario {
  label: "bear" | "base" | "bull" | "tail";
  probability: OptFigure;
  eps: OptFigure;
  multiple: OptFigure;
}
export interface Reasons {
  p: Long;
  moat: Long;
  understand: Long;
  management: Long;
  financial: Long;
}
export interface Lines {
  c: Figure;
  r: Figure;
  heavy: Figure;
  buy_low: Figure;
  buy_high: Figure;
  fair_low: Figure;
  fair_high: Figure;
  trim: Figure;
  clear: Figure;
  core: Figure;
  m: Figure;
  w: Figure;
  h: Figure;
  s: Figure;
  zone: Zone;
  zone_note: ZoneNote;
  flags: Flags | null;
}
export interface Band {
  min: Figure;
  max: Figure;
  marks: Mark[];
}
export interface Mark {
  name: Text;
  value: Figure;
  pos: Figure;
}
export interface Close {
  value: Figure;
  day: Day;
}
export interface HistoryRow {
  review_id: Text;
  date: Day;
  type: "first" | "post_earnings" | "event" | "backfill";
  v: OptFigure;
  v_from: OptFigure;
  v_to: OptFigure;
  change_percent: OptFigure;
  trigger: "" | "a" | "b" | "c" | "d";
  memo: Long;
  status: "draft" | "effective" | "void";
  void_reason: Optional;
  detail: Detail;
}
export interface Problem {
  field: Optional;
  text: Text;
}
export interface Gap {
  kind: "gap";
  reason: "store_unavailable" | "tables_missing" | "close_unavailable";
  text: string;
}
export interface Form {
  post_path: string;
  expires: string;
  mac: string;
}
