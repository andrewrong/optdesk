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
 * Board 14: the price lines the owner keeps. The page posts a form to a constant path and computes nothing.
 */
export interface PriceLinesPage {
  schema: "price_lines_page";
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
  intro: Text1;
  saved: Text1 | null;
  error: Text1 | null;
  form: PriceLineForm;
  rows: PriceLineRow[];
  add: AddBlock;
  unset: UnsetRow[];
  footnote: Text1;
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
/**
 * Where the form posts and the token it carries: UTC epoch seconds as digits and the lower-case hex MAC.
 */
export interface PriceLineForm {
  post_path: "/plan/price-lines";
  expires: string;
  mac: string;
}
export interface PriceLineRow {
  symbol: Text1;
  market: "US" | "JP";
  price: Text1 | null;
  note: Text1 | null;
}
export interface AddBlock {
  /**
   * @minItems 1
   * @maxItems 2
   */
  markets: ["US" | "JP"] | ["US" | "JP", "US" | "JP"];
  hint: Text1;
}
/**
 * A held company with no price line; its add form is prefilled.
 */
export interface UnsetRow {
  symbol: Text1;
  market: "US" | "JP";
}
