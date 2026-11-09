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
export type NullableDay = Day | null;
export type Day = string;
export type NullableText = Text1 | null;

/**
 * The option notices page: one block per account alias with its notices, cash by date and cushion text. A render of rows a producer writes; nothing is computed here.
 */
export interface OptionNoticesPage {
  schema: "option_notices_page";
  schema_version: 1;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  data_times: SourceTime[];
  run_status: RunStatus;
  stale: StaleLabel | null;
  title: Text1;
  notes: Text1[];
  gaps: Gap[];
  /**
   * The build day, an ISO date.
   */
  day: string;
  /**
   * One block per alias, the main alias first.
   */
  accounts: AliasBlock[];
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
 * A named data gap, used in place of a value that cannot be shown. Never a zero.
 */
export interface Gap {
  kind: "gap";
  reason:
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
    | "stale_input"
    | "no_rows"
    | "no_run"
    | "unreadable";
  text: Text1;
}
/**
 * The notices of one account alias. status is ok, no_capture (no stored capture, note says why) or stale (the data day is older than the build day).
 */
export interface AliasBlock {
  alias: Text1;
  main: boolean;
  status: "ok" | "no_capture" | "stale";
  data_day: NullableDay;
  notices: NoticeRow[];
  cash_by_date: CashRow[];
  cushion_text: NullableText;
  note: NullableText;
}
/**
 * One notice. code is a pattern, not an enum, so a new code needs no schema change.
 */
export interface NoticeRow {
  id: string;
  code: string;
  severity: "info" | "warn" | "act";
  title: Text1;
  contract: Text1;
  expiry: NullableDay;
  facts: Fact[];
  reasons: Text1[];
}
export interface Fact {
  label: Text1;
  text: Text1;
}
export interface CashRow {
  day: Day;
  currency: Text1;
  amount_text: Text1;
}
