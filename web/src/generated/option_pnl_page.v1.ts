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
export type OptText = Text1 | null;
export type OptLine = Line | null;
export type OptGap = Gap | null;

/**
 * The options P&L page: per account and currency a book, and per short put lot its profit split into time, move cost, direction and IV re-marking. All money and percent figures are final text; nothing is computed here.
 */
export interface OptionPnlPage {
  schema: "option_pnl_page";
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
/**
 * One account: its books (one per currency), its lots, the sample note and a gap when the account could not be built.
 */
export interface Account {
  alias: string;
  books: Book[];
  lots: Lot[];
  sample_note: Text1;
  gap: OptGap;
  ledger?: Ledger | null;
  /**
   * Cash due if assigned: one line per currency, absent when the account has no open short put.
   */
  cash_due?: Line[];
}
/**
 * The sum of the lots of one currency. left_out is the line naming the lots left out of the sums, or null when none.
 */
export interface Book {
  id: string;
  currency: Text1;
  counts: Text1;
  left_out: OptText;
  headline: Line;
  lines: Line[];
}
/**
 * One labelled line of final text.
 */
export interface Line {
  id: Id;
  label: Text1;
  text: string;
  hint: OptText;
  tone: "none" | "good" | "bad" | "muted";
}
/**
 * One short put lot. gap is set when the lot could not be split, with lines empty.
 */
export interface Lot {
  id: string;
  title: Text1;
  summary: Text1;
  lines: Line[];
  days_line: Line;
  iv_line: OptLine;
  end_line: OptLine;
  recon: Line[];
  badges: Text1[];
  gap: OptGap;
}
/**
 * The ended-lot ledger of one account: one view per grouping. Optional on Account; null when the account has a gap.
 */
export interface Ledger {
  title: Text1;
  note: Text1;
  views: LedgerView[];
}
/**
 * One grouping of the ledger: by month or by company.
 */
export interface LedgerView {
  id: "by_month" | "by_company";
  label: Text1;
  groups: Group[];
}
/**
 * One month or one company of a ledger view, with its books (one per currency).
 */
export interface Group {
  id: string;
  label: Text1;
  summary: Text1;
  books: Book[];
}
