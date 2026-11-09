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
 * One account. gap is non-null exactly when open, recent and recon are all null.
 */
export type Account = (
  | {
      open?: null;
      recent?: null;
      recon?: null;
      gap?: Gap;
      [k: string]: unknown;
    }
  | ((
      | {
          open?: PositionTable;
          [k: string]: unknown;
        }
      | {
          recent?: PositionTable;
          [k: string]: unknown;
        }
      | {
          recon?: CheckTable;
          [k: string]: unknown;
        }
    ) & {
      gap?: null;
      [k: string]: unknown;
    })
) & {
  alias: string;
  title: Text1;
  mark_day: Text1 | null;
  guard_day: Text1 | null;
  long_legs_left_out: number;
  open: PositionTable | null;
  recent: PositionTable | null;
  recon: CheckTable | null;
  gap: Gap | null;
} & {
  alias: string;
  title: Text1;
  mark_day: Text1 | null;
  guard_day: Text1 | null;
  long_legs_left_out: number;
  open: PositionTable | null;
  recent: PositionTable | null;
  recon: CheckTable | null;
  gap: Gap | null;
} & {
  alias: string;
  title: Text1;
  mark_day: Text1 | null;
  guard_day: Text1 | null;
  long_legs_left_out: number;
  open: PositionTable | null;
  recent: PositionTable | null;
  recon: CheckTable | null;
  gap: Gap | null;
} & {
  alias: string;
  title: Text1;
  mark_day: Text1 | null;
  guard_day: Text1 | null;
  long_legs_left_out: number;
  open: PositionTable | null;
  recent: PositionTable | null;
  recon: CheckTable | null;
  gap: Gap | null;
};
export type Cell = ValueCell | Gap;

/**
 * The option tracking page: per account the open short positions, the recent closes and the reconciliation checks of the ledger. A render of rows a producer writes; nothing is computed here.
 */
export interface OptionTrackingPage {
  schema: "option_tracking_page";
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
   * Date text of the page.
   */
  as_of: string;
  mark_note: string;
  /**
   * One to four accounts.
   *
   * @minItems 1
   * @maxItems 4
   */
  accounts: [Account] | [Account, Account] | [Account, Account, Account] | [Account, Account, Account, Account];
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
export interface PositionTable {
  columns: Column[];
  rows: PositionRow[];
}
export interface Column {
  id: Id;
  label: Text1;
  align: "left" | "right";
  hint: Text1 | null;
}
export interface PositionRow {
  id: string;
  /**
   * Column id to Cell.
   */
  cells: {
    [k: string]: Cell;
  };
}
export interface ValueCell {
  text: string;
  value: number | null;
  badges: Text1[];
  tone: "up" | "down" | "flat" | "none";
}
export interface CheckTable {
  columns: Column[];
  rows: CheckRow[];
}
export interface CheckRow {
  id: string;
  /**
   * Column id to Cell.
   */
  cells: {
    [k: string]: Cell;
  };
}
