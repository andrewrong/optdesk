// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type AsOf = string;
export type BuildId = string;
export type Expires = string;
export type Mac = string;
export type PostPath = "/owner-settings";
export type Label = string;
export type New = string;
export type Old = string;
/**
 * @maxItems 1000
 */
export type Rows = ConfirmRow[];
/**
 * @maxItems 20
 */
export type DataTimes =
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
export type DataTime = string;
export type Source = string;
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
export type Text = string;
export type GeneratedAt = string;
/**
 * @maxItems 20
 */
export type History =
  | []
  | [HistoryRow]
  | [HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow]
  | [HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow, HistoryRow]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ]
  | [
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow,
      HistoryRow
    ];
export type At = string;
export type Excluded = boolean | null;
export type InstrumentId = number | null;
export type Key = string | null;
export type Kind = "param" | "exclusion";
export type Name = string;
export type Value = string;
export type Kind1 = "saved" | "nothing_changed" | "error";
export type Text1 = string;
export type Default = string;
export type Key1 = string;
export type Kind2 = "int" | "decimal";
export type Label1 = string;
export type Max = string;
export type Min = string;
export type Unit = string;
export type Value1 = string;
/**
 * @maxItems 200
 */
export type Params = ParamRow[];
export type Excluded1 = boolean;
export type InstrumentId1 = number;
export type Symbol = string;
/**
 * @maxItems 1000
 */
export type Rows1 = OptionExclusionRow[];
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "settings_page";
export type SchemaVersion = 2;
export type Scope = string;
export type Title = string;

/**
 * The owner settings page: parameters, option exclusions, the preview of a change and its history. Read-only when form is null.
 */
export interface SettingsPage {
  as_of: AsOf;
  build_id: BuildId;
  confirm: SettingsConfirm | null;
  data_times: DataTimes;
  form: SettingsForm | null;
  gap?: Gap | null;
  generated_at: GeneratedAt;
  history: History;
  notice: SettingsNotice | null;
  params: Params;
  rows: Rows1;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  title: Title;
}
/**
 * The preview answer: the rows to confirm and the form whose token carries the confirm=1 post.
 */
export interface SettingsConfirm {
  form: SettingsForm;
  rows: Rows;
}
/**
 * Where the form posts and the token it carries: UTC epoch seconds as digits and the lower-case hex MAC.
 */
export interface SettingsForm {
  expires: Expires;
  mac: Mac;
  post_path: PostPath;
}
/**
 * One change of the preview: the label and the old and new value as text.
 */
export interface ConfirmRow {
  label: Label;
  new: New;
  old: Old;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text;
}
/**
 * One past change, newest first. key or instrument_id and value let the view post a revert.
 */
export interface HistoryRow {
  at: At;
  excluded: Excluded;
  instrument_id: InstrumentId;
  key: Key;
  kind: Kind;
  name: Name;
  value: Value;
}
/**
 * The result line after a post.
 */
export interface SettingsNotice {
  kind: Kind1;
  text: Text1;
}
/**
 * One owner parameter, all text. An empty value means the default is in force.
 */
export interface ParamRow {
  default: Default;
  key: Key1;
  kind: Kind2;
  label: Label1;
  max: Max;
  min: Min;
  unit: Unit;
  value: Value1;
}
/**
 * ``instrument_id`` is the master id the form posts; ``excluded`` the current flag.
 */
export interface OptionExclusionRow {
  excluded: Excluded1;
  instrument_id: InstrumentId1;
  symbol: Symbol;
}
