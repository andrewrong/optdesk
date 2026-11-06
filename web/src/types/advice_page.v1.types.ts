// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
export type BuildId = string
export type DataTime = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
export type Day = string
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text = string
export type GeneratedAt = string
export type Badge = (boolean | null)
export type Source1 = (string | null)
export type Text1 = string
export type Value = string
export type AfterValue = (FigureCell | Gap)
export type Text2 = string
export type Value1 = number
export type Delta = (FigureCell | Gap)
export type RefValue = (FigureCell | Gap)
/**
 * @maxItems 500
 */
export type Rows = AdviceRow[]
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "advice_page"
export type SchemaVersion = 1
export type Scope = string
export type Sentence = string
export type Title = string

export interface AdvicePage {
build_id: BuildId
data_times: DataTimes
day: Day
gap?: (Gap | null)
generated_at: GeneratedAt
rows: Rows
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope
sentence: Sentence
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
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
reason: Reason
text: Text
}
export interface AdviceRow {
account: TextCell
acted: TextCell
after_value: AfterValue
delta: Delta
ref_value: RefValue
sentence: TextCell
}
/**
 * A text cell; ``value`` is what a click on the column header sorts by. ``source`` is a label
 * shown in a Badge beside the text (the urgent mark); ``badge`` shows the text itself in a Badge.
 */
export interface TextCell {
badge?: Badge
source?: Source1
text: Text1
value: Value
}
/**
 * A per-share figure (GLOSSARY "Prices and premiums"): two decimals in ``value`` and ``text``.
 */
export interface FigureCell {
text: Text2
value: Value1
}
