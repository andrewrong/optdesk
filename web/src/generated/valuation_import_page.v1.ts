// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Day = string;
export type Action = "insert" | "same" | "older" | "refused" | "profile_only";
export type Figure = string;
export type Optional = string;
export type Text = string;

/**
 * View model of the valuation import page: the upload form (view empty), the preview of the plan with the site's own check (view preview) and the outcome of the confirm (view result). Every figure is a decimal text written by the builder; the view computes nothing. There is no digest: the confirm re-parses the file.
 */
export interface ValuationImportPage {
  schema: "valuation_import_page";
  schema_version: 1;
  view: "empty" | "preview" | "result";
  post_path: "valuation-import-post";
  max_file_bytes: number;
  file: File | null;
  r: R | null;
  margin: Margin | null;
  rows: Row[];
  not_in_file: Text[];
  unvalued: Text[];
  removed: Text[];
  counts: Counts;
  flagged_note: Text | null;
  result: ResultItem[];
  problems: Problem[];
  gaps: Gap[];
  error: Text | null;
  labels?: Labels;
}
export interface File {
  name: string;
  sha256: string;
  as_of: Day;
}
export interface R {
  action: Action;
  file_us: Figure;
  file_jp: Figure;
  file_hkcn: Figure;
  stored_us: Optional;
  stored_jp: Optional;
  stored_hkcn: Optional;
  note: Optional;
  latest_yield: Optional;
  problem: Optional;
}
export interface Margin {
  action: Action;
  rows: MarginRow[];
}
export interface MarginRow {
  p_from: Figure;
  m: Figure;
  w: Figure;
  h: Figure;
  s: Figure;
}
export interface Row {
  key: Optional;
  ticker: Text;
  name: Text;
  currency: Text;
  r_market: "US" | "JP" | "HKCN";
  new_profile: boolean;
  unmatched: boolean;
  proposed_key: Optional;
  action: Action;
  reason: Optional;
  review_type: Optional;
  v: Figure;
  p: Figure;
  version_date: Day;
  file_buy_top: Optional;
  site_buy_top: Optional;
  needs_tick: boolean;
}
export interface Counts {
  insert: number;
  same: number;
  older: number;
  refused: number;
  needs_tick: number;
}
export interface ResultItem {
  key: Text;
  outcome: "inserted" | "same" | "older" | "refused" | "not_confirmed" | "failed";
  text: Optional;
}
export interface Problem {
  text: Text;
}
export interface Gap {
  kind: "gap";
  reason: "store_unavailable";
  text: Text;
}
export interface Labels {
  r: Text;
  buy_zone_top_site: Text;
  buy_zone_top_file: Text;
  general_tick: Text;
  title?: Text;
  intro?: Text;
  file?: Text;
  preview?: Text;
  not_in_file?: Text;
  unvalued?: Text;
  removed?: Text;
  count_insert?: Text;
  count_same?: Text;
  count_older?: Text;
  count_refused?: Text;
  count_needs_tick?: Text;
  stored?: Text;
  older_than_stored?: Text;
  margin_table?: Text;
  result?: Text;
  same_file_again?: Text;
  confirm?: Text;
}
