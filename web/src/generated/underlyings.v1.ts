// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Text = string;
export type Status = "pending" | "active" | "paused" | "retired" | "unresolved";
export type Optional = string;
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
export type Text1 = string;

/**
 * View model of the underlyings page: the followed names with their switches, the form, the confirm step and the history. Every visible string is a field.
 */
export interface Underlyings {
  schema: "underlyings";
  schema_version: 1;
  generated_at: string;
  session_day: string;
  title: Text;
  sentence: Text;
  rows: Row[];
  form: Form | null;
  confirm: Confirm | null;
  notice: string;
  /**
   * @maxItems 20
   */
  history:
    | []
    | [HistoryLine]
    | [HistoryLine, HistoryLine]
    | [HistoryLine, HistoryLine, HistoryLine]
    | [HistoryLine, HistoryLine, HistoryLine, HistoryLine]
    | [HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine]
    | [HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine]
    | [HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine]
    | [HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine, HistoryLine]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ]
    | [
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine,
        HistoryLine
      ];
  gaps: Gap[];
}
export interface Row {
  key: string;
  symbol: string;
  market: "US" | "JP";
  kind: "stock" | "etf";
  name: Text;
  status: Status;
  status_text: Text;
  reason: Optional;
  issuer: Optional;
  con_id: number | null;
  options: boolean;
  fundamentals: boolean;
  held: boolean;
  buy_price: Optional;
  sale_price: Optional;
  report_date_override: Optional;
  note: Optional;
  followed_by: ("bars" | "dividends" | "options" | "fundamentals")[];
  followed_text: Text;
  set_at: Text;
  editable: Text[];
}
export interface Form {
  post_path: "/owner-underlyings";
  expires: Text;
  mac: Text;
}
export interface Confirm {
  changes: Change[];
  stops: Text[];
}
export interface Change {
  label: Text;
  old: string;
  new: string;
}
export interface HistoryLine {
  at: Text;
  key: Text;
  text: Text;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: Kind;
  reason: Reason;
  text: Text1;
}
