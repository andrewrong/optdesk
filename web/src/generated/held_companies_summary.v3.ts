// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * @minItems 5
 * @maxItems 5
 */
export type Columns = [Column, Column, Column, Column, Column];
export type Id1 = "company" | "price" | "earnings" | "day_change" | "five_day_change";
export type Label = string;
export type DataTimesText = string;
/**
 * @maxItems 20
 */
export type Gaps =
  | []
  | [Gap]
  | [Gap, Gap]
  | [Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap]
  | [Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap, Gap];
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
export type Text = string;
/**
 * @maxItems 20
 */
export type Notes =
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
  | [string, string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string, string, string, string, string, string, string]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ]
  | [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string
    ];
export type Currency = "USD" | "JPY";
export type DayChange = PercentFigure | Gap;
export type Kind1 = "value";
export type Text1 = string;
export type Tone = "up" | "down" | "flat" | "none";
export type Unit = "percent";
export type Value = number;
export type DaySharp = SharpMark | NoMark;
export type Kind2 = "sharp";
export type Text2 = string;
export type Kind3 = "none";
export type Earnings = EarningsCell | Gap;
export type Date = string;
export type Kind4 = "value";
export type Text3 = string;
export type FiveDayChange = PercentFigure | Gap;
export type FiveDaySharp = SharpMark | NoMark;
export type Price = PriceFigure | Gap;
export type Kind5 = "value";
export type Text4 = string;
export type Unit1 = "price";
export type Value1 = number;
export type Symbol = string;
export type DataDay = string;
export type Hv30 = PercentFigure | Gap;
export type Iv30 = PercentFigure | Gap;
export type Ivp = PercentFigure | Gap;
export type Label1 = string;
export type Reconciliation = string;
export type Source = string;
/**
 * @maxItems 30
 */
export type Rows = Row[];
export type Say = string;
export type SharpNote = string;
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime = string;
export type IsStale = boolean;
export type Text5 = string;
export type Subtitle = string;
export type Title = string;
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
export type DataTime1 = string;
export type Id2 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "held_companies_summary";
export type SchemaVersion = 3;

/**
 * Envelope (ADR 0034) and card. No envelope currency: the names carry their own.
 */
export interface HeldCompaniesSummary {
  build_id: Id;
  card: Card;
  data_times: DataTimes;
  generated_at: Timestamp;
  run_id: Id;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Id;
  stale_after: Timestamp;
}
export interface Card {
  columns: Columns;
  data_times_text: DataTimesText;
  gaps: Gaps;
  notes: Notes;
  rows: Rows;
  say: Say;
  sharp_note: SharpNote;
  stale: StaleLabel;
  subtitle: Subtitle;
  title: Title;
}
export interface Column {
  id: Id1;
  label: Label;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  kind: Kind;
  reason: Reason;
  text: Text;
}
export interface Row {
  currency: Currency;
  day_change: DayChange;
  day_sharp: DaySharp;
  earnings: Earnings;
  five_day_change: FiveDayChange;
  five_day_sharp: FiveDaySharp;
  price: Price;
  symbol: Symbol;
  volatility: VolatilityLine;
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
  kind: Kind1;
  text: Text1;
  tone?: Tone;
  unit: Unit;
  value: Value;
}
/**
 * A sharp move mark: ``text`` is the label shown beside the change.
 */
export interface SharpMark {
  kind: Kind2;
  text: Text2;
}
/**
 * No mark: the move is not sharp.
 */
export interface NoMark {
  kind: Kind3;
}
/**
 * A results date: ``date`` is the ISO day, ``text`` what is shown (``10-28`` or ``已公布 10-20``).
 */
export interface EarningsCell {
  date: Date;
  kind: Kind4;
  text: Text3;
}
/**
 * A quote price: ``value`` is the number, ``text`` what is shown.
 */
export interface PriceFigure {
  kind: Kind5;
  text: Text4;
  unit: Unit1;
  value: Value1;
}
/**
 * The tool's own HV30, IV30 and IV percentile of a name (ADR 0033 change 9). ``source`` is the short
 * label ("own"), ``label`` names the conventions and the data day; the figures are percents without a
 * sign or tone, a part that is not an ``ok`` reading of the day is a ``Gap``, and one of an earlier day
 * carries that day in its ``text``. ``reconciliation`` is the status the readings carry against IBKR's
 * series; no IBKR value is shown.
 */
export interface VolatilityLine {
  data_day: DataDay;
  hv30: Hv30;
  iv30: Iv30;
  ivp: Ivp;
  label: Label1;
  reconciliation: Reconciliation;
  source: Source;
}
/**
 * The times and the label. The producer sets is_stale; the browser compares no clock.
 */
export interface StaleLabel {
  data_time: DataTime;
  is_stale: IsStale;
  text: Text5;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime1;
  source: Id2;
}
