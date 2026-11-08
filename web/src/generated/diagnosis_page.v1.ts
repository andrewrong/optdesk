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
 * Board 16: diagnosis over ten years. The page ranks nothing and gives no price.
 */
export interface DiagnosisPage {
  schema: "diagnosis_page";
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
  groups: DiagGroup[];
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
export interface DiagGroup {
  /**
   * Class named by the producer; the view orders groups by array position.
   */
  key: string;
  title: Text1;
  columns: Text1[];
  rows: (DiagRow | Gap)[];
  legend: Text1[];
}
/**
 * Ten-year ratios and ranks as produced text (metric id: assigned by compute).
 */
export interface DiagRow {
  symbol: Text1;
  latest_fy: Text1;
  latest: Text1;
  rank_text: Text1;
  extra: Text1;
  usual: Text1;
  /**
   * @maxItems 12
   */
  years:
    | []
    | [YearRow]
    | [YearRow, YearRow]
    | [YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow];
}
export interface YearRow {
  fy: Text1;
  revenue: Text1;
  margin: Text1;
  extra: Text1;
  /**
   * Width of the mini bar, 0 to 100 (metric id: assigned by compute)
   */
  bar_percent: number;
}
