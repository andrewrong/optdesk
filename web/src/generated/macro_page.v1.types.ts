// TEMPORARY placeholder, copied from the old repository's generated types until the week1-schemas issue generates them.
export type BuildId = string
export type DataTime = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
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
export type EaseTitle = string
export type Date = string
export type NameText = string
export type Previous = (string | Gap)
export type WhenText = string
/**
 * @maxItems 20
 */
export type Events = Event[]
export type EventsTitle = string
/**
 * @maxItems 20
 */
export type Gaps = Gap[]
export type GeneratedAt = string
export type DetailHref = (string | null)
export type Label = string
export type Latest = (PercentFigure | Gap)
export type Text3 = string
export type Tone1 = (("up" | "down" | "flat" | "none") | null)
export type Unit2 = "percent"
export type Value2 = number
export type Name = string
export type SourceText = string
export type Status = ("good" | "late" | "missing")
/**
 * @maxItems 20
 */
export type Indicators = Indicator[]
export type IndicatorsTitle = string
/**
 * @maxItems 20
 */
export type Notes = string[]
export type NotesTitle = string
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "macro_page"
export type SchemaVersion = 1
export type Scope = string
export type Sentence = string
export type DataTime1 = string
export type StaleAfter = string
export type Text4 = string
export type Title = string

export interface MacroPage {
build_id: BuildId
data_times: DataTimes
ease: Ease
ease_title: EaseTitle
events: Events
events_title: EventsTitle
gaps: Gaps
generated_at: GeneratedAt
indicators: Indicators
indicators_title: IndicatorsTitle
notes: Notes
notes_title: NotesTitle
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
export interface Indicator {
detail_href: DetailHref
label: Label
latest: Latest
name: Name
source_text: SourceText
status: Status
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
text: Text3
tone?: Tone1
unit: Unit2
value: Value2
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text4
}
