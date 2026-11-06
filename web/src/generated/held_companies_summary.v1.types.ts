// TEMPORARY placeholder, copied from the old repository's generated types until the week1-schemas issue generates them.
export type BuildId = string
/**
 * @minItems 4
 * @maxItems 4
 */
export type Columns = [Column, Column, Column, Column]
export type Id = ("company" | "earnings" | "day_change" | "five_day_change")
export type Label = string
export type DataTimesText = string
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text = string
/**
 * @maxItems 20
 */
export type Gaps = Gap[]
/**
 * @maxItems 20
 */
export type Notes = string[]
export type Rows = (Row[] | Gap)
export type Currency = ("USD" | "JPY")
export type DayChange = (PercentFigure | Gap)
export type Text1 = string
export type Tone = (("up" | "down" | "flat" | "none") | null)
export type Unit = "percent"
export type Value = number
export type Earnings = (EarningsCell | Gap)
export type Date = string
export type Text2 = string
export type FiveDayChange = (PercentFigure | Gap)
export type Symbol = string
export type DataDay = string
export type Hv30 = (PercentFigure | Gap)
export type Iv30 = (PercentFigure | Gap)
export type Ivp = (PercentFigure | Gap)
export type Label1 = string
export type Reconciliation = string
export type Source = string
export type Say = string
export type DataTime = string
export type StaleAfter = string
export type Text3 = string
export type Subtitle = string
export type Title = string
export type DataTime1 = string
export type Source1 = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
export type GeneratedAt = string
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "held_companies_summary"
export type SchemaVersion = 1
export type Scope = string

/**
 * Envelope (ADR 0034) and card. No envelope currency: the names carry their own.
 */
export interface HeldCompaniesSummary {
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
columns: Columns
data_times_text: DataTimesText
gaps: Gaps
notes: Notes
rows: Rows
say: Say
stale: StaleLabel
subtitle: Subtitle
title: Title
}
export interface Column {
id: Id
label: Label
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
reason: Reason
text: Text
}
export interface Row {
currency: Currency
day_change: DayChange
earnings: Earnings
five_day_change: FiveDayChange
symbol: Symbol
volatility: VolatilityLine
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
text: Text1
tone?: Tone
unit: Unit
value: Value
}
/**
 * A results date: ``date`` is the ISO day, ``text`` what is shown (``10-28`` or ``已公布 10-20``).
 */
export interface EarningsCell {
date: Date
text: Text2
}
/**
 * The tool's own HV30, IV30 and IV percentile of a name (ADR 0033 change 9). ``source`` is the short
 * label ("own"), ``label`` names the conventions and the data day; the figures are percents without a
 * sign or tone, a part that is not an ``ok`` reading of the day is a ``Gap``, and one of an earlier day
 * carries that day in its ``text``. ``reconciliation`` is the status the readings carry against IBKR's
 * series; no IBKR value is shown.
 */
export interface VolatilityLine {
data_day: DataDay
hv30: Hv30
iv30: Iv30
ivp: Ivp
label: Label1
reconciliation: Reconciliation
source: Source
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
source: Source1
}
