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
 * One kind of candidate. gap is non-null exactly when main and other are both null.
 */
export type Group = (
  | {
      main?: null;
      other?: null;
      gap?: Gap;
      [k: string]: unknown;
    }
  | ((
      | {
          main?: Table;
          [k: string]: unknown;
        }
      | {
          other?: Table;
          [k: string]: unknown;
        }
    ) & {
      gap?: null;
      [k: string]: unknown;
    })
) & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
} & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
} & {
  kind: "short_put" | "covered_call" | "held_call_guard";
  title: Text1;
  /**
   * Date text of the rows of this group, or null.
   */
  day: Text1 | null;
  /**
   * True when day is before the build day.
   */
  stale: boolean;
  counts: Counts;
  main: null | Table;
  other: null | Table;
  gap: Gap | null;
};
export type Cell = ValueCell | Gap;

/**
 * The option candidates page: the stored short puts, covered calls and guard rows of the last run. A render of rows a producer writes; nothing is computed here.
 */
export interface OptionCandidatesPage {
  schema: "option_candidates_page";
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
   * Date text of the option chain day, or null.
   */
  chain_day: Text1 | null;
  /**
   * Time the run was computed, text in the owner zone, or null.
   */
  computed_at: Text1 | null;
  rule_version: Text1 | null;
  estimate_note: string;
  /**
   * One to three groups in the order short_put, covered_call, held_call_guard.
   *
   * @minItems 1
   * @maxItems 3
   */
  groups: [Group] | [Group, Group] | [Group, Group, Group];
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
export interface Table {
  columns: Column[];
  rows: Row[];
}
export interface Column {
  id: Id;
  label: Text1;
  align: "left" | "right";
  hint: Text1 | null;
}
export interface Row {
  id: string;
  /**
   * Column id to Cell.
   */
  cells: {
    [k: string]: Cell;
  };
  /**
   * Only on other rows; empty on main rows.
   */
  reasons: Reason1[];
}
export interface ValueCell {
  text: string;
  value: number | null;
  badges: Text1[];
  tone: "up" | "down" | "flat" | "none";
}
/**
 * Why a row is blocked or unknown. gate and code are patterns, not enums, so a new gate needs no schema change.
 */
export interface Reason1 {
  gate: string;
  status: "block" | "unknown";
  code: string;
  text: Text1;
}
/**
 * The stored row counts of the group.
 */
export interface Counts {
  ok: number;
  blocked: number;
  unknown: number;
}
