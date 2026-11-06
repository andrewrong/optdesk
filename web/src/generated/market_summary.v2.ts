// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
export type AriaText = string;
/**
 * @minItems 2
 * @maxItems 2
 */
export type EndLabels = [string, string];
export type BandHigh = number | null;
export type BandLow = number | null;
export type Close = number | null;
export type Label = string;
export type Ma200 = number | null;
/**
 * @maxItems 120
 */
export type Points = ChartPoint[];
/**
 * @minItems 3
 * @maxItems 3
 */
export type SeriesNames = [string, string, string];
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
 * @minItems 4
 * @maxItems 4
 */
export type Columns = [string, string, string, string];
/**
 * @maxItems 10
 */
export type Rows =
  | []
  | [IndexRow]
  | [IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow]
  | [IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow, IndexRow];
export type Iv = VolPointsFigure | Gap;
export type Source = string | null;
export type Text1 = string;
export type Tone = ("up" | "down" | "flat" | "none") | null;
export type Unit = "vol_points";
export type Value = number;
export type IvMinusRv20 = VolPointsFigure | Gap;
export type Name = string;
export type Rv20 = VolPointsFigure | Gap;
export type SourceText = string | null;
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
export type Say = string;
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
export type Title = string;
export type VolIndices = [VolIndexItem, VolIndexItem, VolIndexItem] | null;
export type Day = string;
export type Level = IndexLevelFigure | Gap;
export type Text3 = string;
export type Unit1 = "index_points";
export type Value1 = number;
export type Name1 = string;
export type SourceText1 = string;
export type VolIndicesNote = string | null;
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
export type Schema = "market_summary";
export type SchemaVersion = 2;

/**
 * Envelope (ADR 0034) and card.
 */
export interface MarketSummary {
  build_id: Id;
  card: Card;
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
  chart: Chart;
  data_times_text: DataTimesText;
  gaps: Gaps;
  index_table: IndexTable;
  notes: Notes;
  say: Say;
  stale: StaleLabel;
  subtitle: Subtitle;
  title: Title;
  vol_indices?: VolIndices;
  vol_indices_note?: VolIndicesNote;
}
export interface Chart {
  aria_text: AriaText;
  end_labels: EndLabels;
  points: Points;
  series_names: SeriesNames;
}
/**
 * One day: every level is a plain number or ``None`` (null) for a missing day.
 */
export interface ChartPoint {
  band_high: BandHigh;
  band_low: BandLow;
  close: Close;
  label: Label;
  ma200: Ma200;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text;
}
export interface IndexTable {
  columns: Columns;
  rows: Rows;
}
export interface IndexRow {
  iv: Iv;
  iv_minus_rv20: IvMinusRv20;
  name: Name;
  rv20: Rv20;
  source_text?: SourceText;
}
/**
 * Vol points (GLOSSARY "Vol point"), signed (``+2.1``).
 */
export interface VolPointsFigure {
  source?: Source;
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
 * One Cboe volatility index (VIX, VXN, RVX): its level in index points as Cboe publishes it
 * (18.20 is 18.20 points, not a fraction and not a percent), the data day ``mm-dd`` and the
 * source series. Never part of the IV cell of the index table: a Cboe index is a methodology of
 * its own.
 */
export interface VolIndexItem {
  day: Day;
  level: Level;
  name: Name1;
  source_text: SourceText1;
}
/**
 * The level of a published market index in index points as its publisher states it (a Cboe
 * volatility index: 18.20 is 18.20 points). Not the 0 to 100 ease scale of ``IndexPointsFigure``:
 * the level has no upper bound in practice, the cap here only rejects a corrupt value.
 */
export interface IndexLevelFigure {
  text: Text3;
  unit: Unit1;
  value: Value1;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime1;
  source: Id1;
}
