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
 * A figure as text, or a gap with its reason. Never a zero for a missing figure.
 */
export type Cell = {
  kind: "value" | "gap";
  text: Text1;
  reason?: string;
};
/**
 * The next earnings date and the days to it, or a gap with its reason.
 */
export type EarningsCell = {
  kind: "date" | "gap";
  date?: Text1;
  days_to?: number;
  text: Text1;
  reason?: "no_row" | "unsupported" | "row_error" | "no_reader" | "no_underlying" | "no_issuer" | "past_date";
};

/**
 * The held companies of the family: each company with the accounts that hold it, its market value, its share of the family total and its next earnings date. A render of text a producer wrote; nothing is computed here. Not the home card held_companies_summary.
 */
export interface HeldFamilyPage {
  schema: "held_family_page";
  schema_version: 1;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  /**
   * @maxItems 20
   */
  data_times:
    | []
    | [SourceTime]
    | [SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
    | [SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime, SourceTime]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ]
    | [
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime,
        SourceTime
      ];
  run_status: RunStatus;
  stale: StaleLabel | null;
  title: Text1;
  /**
   * @maxItems 20
   */
  notes:
    | []
    | [Text1]
    | [Text1, Text1]
    | [Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ];
  /**
   * @maxItems 20
   */
  gaps:
    | []
    | [Gap]
    | [Gap, Gap]
    | [Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
    | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap];
  total: Total;
  /**
   * @minItems 5
   * @maxItems 5
   */
  columns: [Column, Column, Column, Column, Column];
  /**
   * @maxItems 200
   */
  rows: Row[];
  rows_sum: Cell;
  /**
   * @maxItems 20
   */
  lead_notes:
    | []
    | [Text1]
    | [Text1, Text1]
    | [Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1, Text1]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ]
    | [
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1,
        Text1
      ];
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
/**
 * The family total the shares are taken of.
 */
export interface Total {
  text: Text1;
  currency: Text1;
  day: Text1;
  accounts_text: Text1;
}
export interface Column {
  id: Text1;
  label: Text1;
  hint: Text1;
}
export interface Row {
  company: Text1;
  /**
   * @minItems 1
   * @maxItems 20
   */
  accounts:
    | [Account]
    | [Account, Account]
    | [Account, Account, Account]
    | [Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account, Account, Account, Account, Account, Account]
    | [Account, Account, Account, Account, Account, Account, Account, Account, Account, Account, Account, Account]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ]
    | [
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account,
        Account
      ];
  value: Cell;
  share: Cell;
  earnings: EarningsCell;
}
/**
 * One holding account, named by its alias word.
 */
export interface Account {
  alias: Text1;
  label: Text1;
}
