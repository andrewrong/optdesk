// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Figure = string;
export type Day = string;
export type Optional = string;
export type Text = string;

/**
 * View model of the market parameters page of the valuation: the newest R version, the proposal of R for the latest 10-year yield, the two parameter tables and the R history. Every figure is a decimal text written by the builder; the view computes nothing.
 */
export interface ValuationParamsPage {
  schema: "valuation_params_page";
  schema_version: 1;
  run_id: Id;
  built_at: Timestamp;
  latest_yield: Yield | null;
  current_r: RVersion | null;
  proposal: Proposal | null;
  r_table: RTable | null;
  margin_table: MarginTable | null;
  r_history: RVersion[];
  gaps: Gap[];
  error: Optional;
  form: Form | null;
}
export interface Yield {
  value: Figure;
  day: Day;
}
export interface RVersion {
  effective_date: Day;
  yield_used: Optional;
  r: RValues;
  ai_switch: boolean;
  ai_expiry: Optional;
  note: Optional;
  step_warnings: Text[];
}
export interface RValues {
  us: Figure;
  jp: Figure;
  hkcn: Figure;
}
export interface Proposal {
  r: RValues;
  step_warnings: Text[];
}
export interface RTable {
  effective_date: Day;
  rows: RTableRow[];
}
export interface RTableRow {
  yield_up_to: Optional;
  us: Figure;
  jp: Figure;
  hkcn: Figure;
}
export interface MarginTable {
  effective_date: Day;
  rows: MarginRow[];
}
export interface MarginRow {
  p_from: Figure;
  m: Figure;
  w: Figure;
  h: Figure;
  s: Figure;
}
export interface Gap {
  kind: "gap";
  reason: "yield_unavailable" | "store_unavailable";
  text: string;
}
export interface Form {
  post_path: string;
  expires: string;
  mac: string;
}
