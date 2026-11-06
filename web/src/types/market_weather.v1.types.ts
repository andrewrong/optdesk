// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
export type Closed = (string | null)
export type Footnote = string
export type Headline = string
export type AsOf = string
export type ExpiredText = string
export type ExpiresAt = string
export type Footnote1 = string
/**
 * @maxItems 2
 */
export type Carried = []|[IntradayCarried]|[IntradayCarried, IntradayCarried]
export type CloseDate = string
export type Label = "收盘"
export type Vote = string
export type Headline1 = string
export type Label1 = "盘中试算"
export type RatioText = string
export type State = ("good" | "mixed" | "bad" | "unknown")
/**
 * @maxItems 3
 */
export type Rows = []|[IntradayRow]|[IntradayRow, IntradayRow]|[IntradayRow, IntradayRow, IntradayRow]
export type AsOf1 = string
export type BaselineThrough = (string | null)
export type DataType = ("live" | "delayed")
export type Detail = (string | null)
export type Label2 = ("盘中" | "盘中·延迟")
export type Series = ("VIX" | "VIX3M" | "SPX")
export type SourceTime = (string | null)
export type DataTime = string
export type StaleAfter = string
export type Text = string
export type Value = string
export type Status = ("ok" | "stale" | "busy" | "unavailable")
export type StatusLine = (string | null)
export type Title = string
export type TrendLine = (string | null)
export type Day = string
export type Text1 = string
export type Unit = "index_points"
export type Value1 = number
export type Kind = ("beyond_tolerance" | "band_flip")
/**
 * @maxItems 60
 */
export type Mismatches = MismatchLine[]
/**
 * @maxItems 3
 */
export type Rows1 = []|[VoteRow]|[VoteRow, VoteRow]|[VoteRow, VoteRow, VoteRow]
export type FigureText = string
export type FirstIbkrDate = (string | null)
export type Kind1 = ("volatility" | "trend" | "credit")
export type Observed = string
export type Source = ("IBKR" | "FRED")
export type Verdict = ("bad" | "ok" | "unknown")
export type Schema = "market_weather"
export type SchemaVersion = 1
export type State1 = ("good" | "mixed" | "bad" | "unknown")

/**
 * The block; ``schema`` and ``schema_version`` are the part of the envelope it carries alone.
 */
export interface MarketWeather {
closed?: Closed
footnote: Footnote
headline: Headline
intraday?: (IntradaySection | null)
mismatches: Mismatches
rows: Rows1
schema: Schema
schema_version: SchemaVersion
stale: (StaleLabel | null)
state: State1
}
/**
 * The 盘中 section beside the close gauge (issue #1224). ``gauge`` is None when no gauge could
 * be made; ``rows`` then show what is valid or stale, and ``status_line`` says why (it carries no
 * deadline). ``as_of`` is the time of the newest counted live quote and never implies the carried
 * daily votes were refreshed. The browser hides the section from ``expires_at`` on (the one
 * comparison it makes) and shows ``expired_text`` in its place.
 */
export interface IntradaySection {
as_of: AsOf
expired_text: ExpiredText
expires_at: ExpiresAt
footnote: Footnote1
gauge: (IntradayGaugeBlock | null)
rows: Rows
status: Status
status_line: StatusLine
title: Title
trend_line?: TrendLine
}
/**
 * The 盘中试算 itself: made only when both live legs counted.
 */
export interface IntradayGaugeBlock {
carried: Carried
headline: Headline1
label: Label1
ratio_text: RatioText
state: State
}
/**
 * A daily vote the 盘中试算 carries from the close, with the close date it was made on.
 */
export interface IntradayCarried {
close_date: CloseDate
label: Label
vote: Vote
}
/**
 * One live figure of the 盘中试算 (issue #1224): VIX or VIX3M with the time the exchange gave
 * it, its data type and its label. ``stale`` is a label naming the age of a quote too old to
 * count; the browser only compares its clock with ``stale_after``.
 * 
 * The SPX row (issue #1225) is the live index point feeding the trend vote: ``source_time`` is the
 * exchange's time with its zone, ``baseline_through`` the session ending the 200-close mean and
 * ``detail`` the ratio against that mean with the verdict, written by Python.
 */
export interface IntradayRow {
as_of: AsOf1
baseline_through?: BaselineThrough
data_type: DataType
detail?: Detail
label: Label2
series: Series
source_time?: SourceTime
stale: (StaleLabel | null)
value: Value
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime
stale_after: StaleAfter
text: Text
}
/**
 * A day on which IBKR and FRED differ: by more than the tolerance, or enough to put the two
 * values on different sides of a band. ``diff`` is the absolute difference.
 */
export interface MismatchLine {
day: Day
diff: MismatchFigure
fred: MismatchFigure
ibkr: MismatchFigure
kind: Kind
}
/**
 * A level of one source, or the absolute difference of the two, in the units of the series.
 */
export interface MismatchFigure {
text: Text1
unit: Unit
value: Value1
}
/**
 * One vote: its verdict, the figure it rests on (text written by Python), where the figure is
 * from and its date. ``first_ibkr_date`` is the first day IBKR has for the series, ``None`` when
 * unknown.
 */
export interface VoteRow {
figure_text: FigureText
first_ibkr_date: FirstIbkrDate
kind: Kind1
observed: Observed
source: Source
verdict: Verdict
}
