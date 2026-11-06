// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
export type BuildId = string
export type DataTimesText = string
export type Ease = (Ease1 | Gap)
export type AriaText = string
export type Text = string
export type Tone = (("up" | "down" | "flat" | "none") | null)
export type Unit = "index_points"
export type Value = number
export type Text1 = string
export type Unit1 = "percent_of_track"
export type Value1 = number
export type ReadingText = string
/**
 * @minItems 5
 * @maxItems 5
 */
export type Ticks = [string, string, string, string, string]
export type WeekChange = (IndexPointsFigure | Gap)
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text2 = string
/**
 * @maxItems 8
 */
export type Events = []|[Event]|[Event, Event]|[Event, Event, Event]|[Event, Event, Event, Event]|[Event, Event, Event, Event, Event]|[Event, Event, Event, Event, Event, Event]|[Event, Event, Event, Event, Event, Event, Event]|[Event, Event, Event, Event, Event, Event, Event, Event]
export type Date = string
export type NameText = string
export type Previous = (string | Gap)
export type WhenText = string
/**
 * @maxItems 20
 */
export type Gaps = Gap[]
/**
 * @maxItems 20
 */
export type Notes = string[]
export type Say = string
export type DataTime = string
export type StaleAfter = string
export type Text3 = string
export type Title = string
export type DataTime1 = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
export type GeneratedAt = string
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "macro_summary"
export type SchemaVersion = 1
export type Scope = string

/**
 * Envelope (ADR 0034) and card.
 */
export interface MacroSummary {
build_id: BuildId
card: Card
data_times: DataTimes
generated_at: GeneratedAt
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope
}
export interface Card {
data_times_text: DataTimesText
ease: Ease
events: Events
gaps: Gaps
notes: Notes
say: Say
stale: StaleLabel
title: Title
}
export interface Ease1 {
aria_text: AriaText
figure: IndexPointsFigure
marker_percent: PercentOfTrackFigure
neutral_band: NeutralBand
reading_text: ReadingText
ticks: Ticks
week_change: WeekChange
}
/**
 * A position on the stored 0 to 100 ease scale, or a change in it.
 */
export interface IndexPointsFigure {
text: Text
tone?: Tone
unit: Unit
value: Value
}
/**
 * A marker position on its track, 0 to 100.
 */
export interface PercentOfTrackFigure {
text: Text1
unit: Unit1
value: Value1
}
/**
 * The ends of the neutral band on the track, ``percent_of_track``.
 */
export interface NeutralBand {
high: PercentOfTrackFigure
low: PercentOfTrackFigure
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
reason: Reason
text: Text2
}
export interface Event {
date: Date
name_text: NameText
previous: Previous
when_text: WhenText
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime
stale_after: StaleAfter
text: Text3
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
data_time: DataTime1
source: Source
}
