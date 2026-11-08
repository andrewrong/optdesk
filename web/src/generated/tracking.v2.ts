// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type BuildId = string;
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
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime = string;
export type Id = string;
export type GeneratedAt = string;
/**
 * @maxItems 10
 */
export type How =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string];
export type Note = string | null;
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "tracking";
export type SchemaVersion = 2;
export type Scope = string;
export type Sentence = string;
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime1 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type StaleAfter = string;
export type Text = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Title = string;
/**
 * Rule versions, current first; older ones are later entries.
 *
 * @minItems 1
 * @maxItems 10
 */
export type Versions =
  | [RuleVersion]
  | [RuleVersion, RuleVersion]
  | [RuleVersion, RuleVersion, RuleVersion]
  | [RuleVersion, RuleVersion, RuleVersion, RuleVersion]
  | [RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion]
  | [RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion]
  | [RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion]
  | [RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion, RuleVersion]
  | [
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion
    ]
  | [
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion,
      RuleVersion
    ];
export type Label = string;
export type ExpiredText = string;
/**
 * @minItems 1
 * @maxItems 6
 */
export type Groups =
  | [RuleGroup]
  | [RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup, RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup, RuleGroup, RuleGroup, RuleGroup];
export type Id1 = string;
export type Title1 = string;
export type Summary =
  | [Tile]
  | [Tile, Tile]
  | [Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile, Tile, Tile, Tile]
  | Gap;
export type Label1 = string;
export type Text1 = string;
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
export type Note1 = string | null;
export type Assigned = string | null;
export type ExpiryText = string;
export type Outcome = string;
export type Premium = Usd | Gap;
export type Text3 = string;
export type Unit = "usd";
export type Value = number;
export type Status = string;
export type Strike = string;
export type Symbol = string;
export type UnderlyingMove = Usd | Gap;
/**
 * @maxItems 2000
 */
export type Rows = TrackingRow[];
export type CoveredCalls = CoveredCalls1 | null;
export type Title2 = string;
export type Sub = string;
/**
 * @minItems 1
 * @maxItems 6
 */
export type Groups1 =
  | [RuleGroup]
  | [RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup, RuleGroup, RuleGroup]
  | [RuleGroup, RuleGroup, RuleGroup, RuleGroup, RuleGroup, RuleGroup];
/**
 * @minItems 1
 * @maxItems 12
 */
export type Columns =
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string];
/**
 * @maxItems 2000
 */
export type Rows1 =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string][];
export type Note2 = string | null;
export type MyTrades = MyTrades1 | Gap;
export type Title3 = string;
export type Sub1 = string;
/**
 * @minItems 1
 * @maxItems 12
 */
export type Columns1 =
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string];
/**
 * @maxItems 2000
 */
export type Rows2 =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string][];
export type Note3 = string | null;

export interface Tracking {
  build_id: BuildId;
  data_times: DataTimes;
  generated_at: GeneratedAt;
  how: How;
  note?: Note;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  sentence: Sentence;
  stale: StaleLabel;
  stale_after?: Timestamp;
  title: Title;
  versions: Versions;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Id;
}
/**
 * The times and the label; the browser only compares its clock with stale_after.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text;
}
/**
 * One rule version; the first entry of versions is the current one, later entries are older and folded.
 */
export interface RuleVersion {
  label: Label;
  expired_text: ExpiredText;
  groups: Groups;
  covered_calls: CoveredCalls;
  my_trades: MyTrades;
}
/**
 * One group of a rule version: its tiles (or a Gap), a note and its evaluation rows.
 */
export interface RuleGroup {
  key: Id1;
  title: Title1;
  summary: Summary;
  note: Note1;
  rows: Rows;
}
/**
 * One summary tile of a rule group; label and text are the producer's words; metric id: assigned by compute.
 */
export interface Tile {
  label: Label1;
  text: Text1;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text2;
}
/**
 * One evaluation. ``premium`` and ``underlying_move`` are the P&L split, or the same Gap
 * (the outcome in words) when the row has no grade to split.
 */
export interface TrackingRow {
  assigned: Assigned;
  expiry_text: ExpiryText;
  outcome: Outcome;
  premium: Premium;
  status: Status;
  strike: Strike;
  symbol: Symbol;
  underlying_move: UnderlyingMove;
}
/**
 * A signed USD amount as text and number.
 */
export interface Usd {
  text: Text3;
  unit: Unit;
  value: Value;
}
/**
 * The covered-call section of a rule version.
 */
export interface CoveredCalls1 {
  title: Title2;
  sub: Sub;
  groups: Groups1;
  comparison: Table;
  note: Note2;
}
/**
 * Columns and rows of strings; the view prints them and computes nothing.
 */
export interface Table {
  columns: Columns;
  rows: Rows1;
}
/**
 * The My trades table; a Gap with its cause until a producer exists.
 */
export interface MyTrades1 {
  title: Title3;
  sub: Sub1;
  columns: Columns1;
  rows: Rows2;
  note: Note3;
}
