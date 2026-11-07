// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type BenchmarkText = string;
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
 * @minItems 3
 * @maxItems 3
 */
export type Heads = [string, string, string];
export type Meaning = string;
export type Text1 = string;
export type Unit = "beta";
export type Value = number;
export type Name = string;
export type Text2 = string;
export type Unit1 = "USD";
export type Value1 = number;
export type Currency = "USD" | "JPY";
export type Text3 = string;
export type Tone = ("up" | "down" | "flat" | "none") | null;
export type Unit2 = "usd" | "usd_per_share" | "usd_signed";
export type Value2 = number;
/**
 * @minItems 0
 * @maxItems 50
 */
export type Rows = BetaRow[];
export type Title = string;
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
export type DataTime = string;
export type Source = string;
/**
 * @minItems 2
 * @maxItems 2
 */
export type Heads1 = [string, string];
export type Meaning1 = string;
export type Name1 = string;
/**
 * @minItems 0
 * @maxItems 50
 */
export type Rows1 = DeltaRow[];
export type Title1 = string;
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
export type GeneratedAt = string;
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
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "household_risk";
export type SchemaVersion = 1;
export type Scope = string;
export type Sentence = string;
export type AriaText = string;
/**
 * @minItems 4
 * @maxItems 4
 */
export type Columns = [string, string, string, string];
export type Meaning2 = string;
/**
 * @minItems 4
 * @maxItems 4
 */
export type Cells = [MoneyCell, MoneyCell, MoneyCell, MoneyCell];
export type Name2 = string;
/**
 * @minItems 0
 * @maxItems 50
 */
export type Rows2 = SigmaRow[];
export type Title2 = string;
export type DataTime1 = string;
export type StaleAfter = string;
export type Text4 = string;
/**
 * @minItems 1
 * @maxItems 20
 */
export type Metrics =
  | [Metric]
  | [Metric, Metric]
  | [Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric, Metric]
  | [
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric
    ]
  | [
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric
    ]
  | [
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric
    ]
  | [
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric
    ]
  | [
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric
    ]
  | [
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric,
      Metric
    ];
export type Explanation = string;
export type Label = string;
/**
 * @minItems 1
 * @maxItems 8
 */
export type Columns1 =
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string];
/**
 * @maxItems 20
 */
export type Rows3 =
  | []
  | [MetricRow]
  | [MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow]
  | [MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow, MetricRow]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ]
  | [
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow,
      MetricRow
    ];
/**
 * @minItems 1
 * @maxItems 8
 */
export type Cells1 =
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string];
export type ValueText = string;
export type Title3 = string;
export type Title4 = string;
/**
 * @minItems 2
 * @maxItems 2
 */
export type Heads2 = [string, string];
export type Meaning3 = string;
export type ExpiryText = string;
export type Text5 = string;
export type Unit3 = "usd_per_vol_point";
export type Value3 = number;
/**
 * @minItems 0
 * @maxItems 50
 */
export type Rows4 = VegaRow[];
export type VegaTitle = string;

export interface HouseholdRisk {
  beta_delta: BetaBlock;
  build_id: BuildId;
  data_times: DataTimes;
  delta: DeltaBlock;
  gaps: Gaps;
  generated_at: GeneratedAt;
  notes: Notes;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  sentence: Sentence;
  sigma: SigmaBlock;
  stale: StaleLabel;
  standard_metrics: MetricsBlock;
  title: Title4;
  vega: VegaBlock;
  vega_title: VegaTitle;
}
export interface BetaBlock {
  benchmark_text: BenchmarkText;
  gap?: Gap | null;
  heads: Heads;
  meaning: Meaning;
  rows: Rows;
  title: Title;
  total?: MoneyCell | null;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text;
}
export interface BetaRow {
  beta: Beta;
  name: Name;
  weighted_delta: MoneyCell;
}
export interface Beta {
  text: Text1;
  unit: Unit;
  value: Value;
}
/**
 * A money cell with a currency and, for JPY, an optional display-only ``approx_usd``.
 */
export interface MoneyCell {
  approx_usd?: UsdFigure | null;
  currency: Currency;
  text: Text3;
  tone?: Tone;
  unit: Unit2;
  value: Value2;
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure {
  text: Text2;
  unit: Unit1;
  value: Value1;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source;
}
export interface DeltaBlock {
  gap?: Gap | null;
  heads: Heads1;
  meaning: Meaning1;
  rows: Rows1;
  title: Title1;
  total?: MoneyCell | null;
}
export interface DeltaRow {
  delta: MoneyCell;
  name: Name1;
}
export interface SigmaBlock {
  aria_text: AriaText;
  columns: Columns;
  gap?: Gap | null;
  meaning: Meaning2;
  rows: Rows2;
  scale_high?: MoneyCell | null;
  scale_low?: MoneyCell | null;
  title: Title2;
  total?: SigmaRow | null;
}
export interface SigmaRow {
  cells: Cells;
  name: Name2;
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text4;
}
export interface MetricsBlock {
  metrics: Metrics;
  title: Title3;
}
export interface Metric {
  explanation: Explanation;
  label: Label;
  table: MetricTable | null;
  value_text: ValueText;
}
export interface MetricTable {
  columns: Columns1;
  rows: Rows3;
}
export interface MetricRow {
  cells: Cells1;
}
export interface VegaBlock {
  gap?: Gap | null;
  heads: Heads2;
  meaning: Meaning3;
  rows: Rows4;
  total?: Vega | null;
}
export interface VegaRow {
  expiry_text: ExpiryText;
  vega: Vega;
}
export interface Vega {
  text: Text5;
  unit: Unit3;
  value: Value3;
}
