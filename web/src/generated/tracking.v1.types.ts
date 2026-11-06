// TEMPORARY placeholder, copied from the old repository's generated types until the week1-schemas issue generates them.
export type BuildId = string
export type DataTime = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
export type GeneratedAt = string
/**
 * @maxItems 10
 */
export type How = []|[string]|[string, string]|[string, string, string]|[string, string, string, string]|[string, string, string, string, string]|[string, string, string, string, string, string]|[string, string, string, string, string, string, string]|[string, string, string, string, string, string, string, string]|[string, string, string, string, string, string, string, string, string]|[string, string, string, string, string, string, string, string, string, string]
export type Note = (string | null)
export type Pending = string
export type Assigned = (string | null)
export type ExpiryText = string
export type Outcome = string
export type Premium = (Usd | Gap)
export type Text = string
export type Unit = "usd"
export type Value = number
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text1 = string
export type Status = string
export type Strike = string
export type Symbol = string
export type UnderlyingMove = (Usd | Gap)
/**
 * @maxItems 2000
 */
export type Rows = TrackingRow[]
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "tracking"
export type SchemaVersion = 1
export type Scope = string
export type Sentence = string
export type DataTime1 = string
export type StaleAfter = string
export type Text2 = string
export type Title = string

export interface Tracking {
build_id: BuildId
data_times: DataTimes
generated_at: GeneratedAt
how: How
note?: Note
pending: Pending
rows: Rows
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope
sentence: Sentence
stale: StaleLabel
title: Title
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
data_time: DataTime
source: Source
}
/**
 * One evaluation. ``premium`` and ``underlying_move`` are the P&L split, or the same Gap
 * (the outcome in words) when the row has no grade to split.
 */
export interface TrackingRow {
assigned: Assigned
expiry_text: ExpiryText
outcome: Outcome
premium: Premium
status: Status
strike: Strike
symbol: Symbol
underlying_move: UnderlyingMove
}
export interface Usd {
text: Text
unit: Unit
value: Value
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
reason: Reason
text: Text1
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text2
}
