// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Text = string;
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
export type Status = "stored" | "present" | "nothing_found" | "unconfigured" | "failed";
export type Reason1 = "not_report" | "too_large" | "too_many_files" | "bad_gzip" | "store_error" | "";
export type Optional = string;
export type Count = number;

/**
 * View model of the Flex import page: the upload form, the results of one post and the recent imports. Every visible string is a field.
 */
export interface FlexImport {
  schema: "flex_import";
  schema_version: 1;
  title: Text;
  sentence: Text;
  scope_note: Text;
  form: Form | null;
  gaps: Gap[];
  files: File[];
  recent: Recent[];
}
export interface Form {
  post_path: "/owner-flex-import";
  expires: Text;
  mac: Text;
  max_files: number;
  max_file_text: Text;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: Kind;
  reason: Reason;
  text: Text1;
}
export interface File {
  name: string;
  status: Status;
  reason: Reason1;
  status_text: Text;
  period_first: Optional;
  period_last: Optional;
  aliases: Alias[];
  unmapped_statements: Count;
}
export interface Alias {
  alias: Text;
  statements: Count;
  rows_parsed: Count;
  rows_new: Count;
  rows_present: Count;
}
export interface Recent {
  at_text: Text;
  alias: Optional;
  status: Status;
  reason: Reason1;
  rows_parsed: Count;
  rows_new: Count;
  rows_present: Count;
  unmapped_statements: Count;
}
