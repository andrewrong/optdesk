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
export type NullableText = Text1 | null;

/**
 * The same-morning page: one section, today, with the newest run of each market key and a row for each job that is not wired. A render of rows a producer writes; nothing is computed here.
 */
export interface OpsTodayPage {
  schema: "ops_today_page";
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
   * The session day, an ISO date.
   */
  day: string;
  stale_after?: Timestamp;
  today: TodaySection;
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
  reason: string;
  text: Text1;
}
/**
 * The one section: rows ordered by the producer.
 */
export interface TodaySection {
  rows: Row[];
}
/**
 * One run row: the newest run of a market key with its reason code, or a not_wired row (reason_code not_wired, the NAME of the missing key in detail, never a value). Every text is the producer's.
 */
export interface Row {
  dataset: Text1;
  key: Text1;
  market: Text1;
  status: Text1;
  error_code: NullableText;
  reason_code: NullableText;
  /**
   * Rows returned, as the run recorded it.
   */
  returned_n: number;
  /**
   * Rows usable, as the run recorded it.
   */
  usable_n: number;
  detail: NullableText;
}
