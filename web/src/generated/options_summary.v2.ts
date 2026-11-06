// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
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
export type Reason = "missing_quote" | "no_data" | "unsupported_currency";
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
/**
 * @minItems 6
 * @maxItems 6
 */
export type PremiumSeries6M = [SeriesPoint, SeriesPoint, SeriesPoint, SeriesPoint, SeriesPoint, SeriesPoint];
export type Closed = UsdSignedFigure | Gap;
export type Text1 = string;
export type Tone = ("up" | "down" | "flat" | "none") | null;
export type Unit = "usd_signed";
export type Value = number;
export type Label = string;
/**
 * Calendar month as YYYY-MM.
 */
export type Month = string;
export type Premium = UsdSignedFigure | Gap;
export type PremiumSeriesCaption = string;
export type PremiumSeriesText = string;
export type Say = string | null;
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime = string;
/**
 * An RFC 3339 time with any offset.
 */
export type StaleAfter = string;
export type Text2 = string;
export type Subtitle = string;
export type Caption = string | null;
export type Figure = UsdFigure | Gap;
export type Text3 = string;
export type Unit1 = "USD";
export type Value1 = number;
export type Label1 = string;
export type Caption1 = string | null;
export type Figure1 = UsdSignedFigure | Gap;
export type Label2 = string;
export type Caption2 = string | null;
export type Figure2 = CountFigure | Gap;
export type Text4 = string;
export type Unit2 = "structures";
export type Value2 = number;
export type Label3 = string;
export type Title = string;
export type Currency = "USD";
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
export type Id1 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "options_summary";
export type SchemaVersion = 2;

/**
 * Envelope (ADR 0034) and card.
 */
export interface OptionsSummary {
  build_id: Id;
  card: Card;
  currency: Currency;
  data_times: DataTimes;
  generated_at: Timestamp;
  run_id: Id;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Id;
  stale_after?: Timestamp;
}
export interface Card {
  gaps: Gaps;
  notes: Notes;
  premium_series_6m: PremiumSeries6M;
  premium_series_caption: PremiumSeriesCaption;
  premium_series_text: PremiumSeriesText;
  say?: Say;
  stale: StaleLabel;
  subtitle: Subtitle;
  tiles: Tiles;
  title: Title;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text;
}
export interface SeriesPoint {
  closed: Closed;
  label: Label;
  month: Month;
  premium: Premium;
}
/**
 * A signed US-dollar amount; ``text`` starts with ``+`` or ``−`` (U+2212).
 */
export interface UsdSignedFigure {
  text: Text1;
  tone?: Tone;
  unit: Unit;
  value: Value;
}
/**
 * The times and the label; the browser only compares its clock with stale_after.
 */
export interface StaleLabel {
  data_time: DataTime;
  stale_after: StaleAfter;
  text: Text2;
}
/**
 * The four tiles, under fixed keys.
 */
export interface Tiles {
  max_loss_usd: UsdTile;
  open_premium_usd: UsdTile;
  premium_month_usd: SignedUsdTile;
  sold_count_month: CountTile;
}
export interface UsdTile {
  caption?: Caption;
  figure: Figure;
  label: Label1;
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure {
  text: Text3;
  unit: Unit1;
  value: Value1;
}
export interface SignedUsdTile {
  caption?: Caption1;
  figure: Figure1;
  label: Label2;
}
export interface CountTile {
  caption?: Caption2;
  figure: Figure2;
  label: Label3;
}
/**
 * A displayed count of option structures (a short put, a spread: one structure each).
 */
export interface CountFigure {
  text: Text4;
  unit: Unit2;
  value: Value2;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime1;
  source: Id1;
}
