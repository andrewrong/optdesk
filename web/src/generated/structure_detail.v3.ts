// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

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
export type ActionText = string;
export type Fee = Usd | Gap;
export type Text = string;
export type Unit = "usd";
export type Value = number;
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
export type Text1 = string;
export type LegText = string;
export type Quantity = number;
export type TimeText = string;
/**
 * @maxItems 50
 */
export type Fills = Fill[];
export type FillsTitle = string;
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
export type Greeks = GreeksBlock | Gap;
export type ModelText = string;
/**
 * @minItems 2
 * @maxItems 9
 */
export type Rows =
  | [GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow, GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow]
  | [GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow, GreekRow];
/**
 * @minItems 4
 * @maxItems 4
 */
export type Cells = [GreekFigure | Gap, GreekFigure | Gap, GreekFigure | Gap, GreekFigure | Gap];
export type Text2 = string;
export type Unit1 = "greek";
export type Value1 = number;
export type Label = string;
export type GreeksTitle = string;
/**
 * @maxItems 20
 */
export type KeyLevels =
  | []
  | [Level]
  | [Level, Level]
  | [Level, Level, Level]
  | [Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level, Level]
  | [
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level
    ]
  | [
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level
    ]
  | [
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level
    ]
  | [
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level,
      Level
    ];
export type Distance = string | Gap;
export type Name = string;
export type Note = string;
/**
 * @minItems 1
 * @maxItems 8
 */
export type Legs =
  | [Leg | Gap]
  | [Leg | Gap, Leg | Gap]
  | [Leg | Gap, Leg | Gap, Leg | Gap]
  | [Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap]
  | [Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap]
  | [Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap]
  | [Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap]
  | [Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap, Leg | Gap];
export type ExpiryText = string;
export type Label1 = string;
export type Mark = Usd | Gap;
export type Pnl = Usd | Gap;
export type Quantity1 = number;
export type RightText = string;
export type SideText = string;
export type LegsTitle = string;
export type Label2 = string;
export type Pnl1 = Usd | Gap;
export type LevelsTitle = string;
export type Payoff = Payoff1 | Gap;
export type AriaText = string;
export type High = number;
export type Low = number;
export type Text3 = string;
export type PnlUnit = "usd";
/**
 * @minItems 2
 * @maxItems 200
 */
export type Points = [PayoffPoint, PayoffPoint, ...PayoffPoint[]];
export type Pnl2 = number;
export type Price = number;
export type PriceUnit = "usd";
export type PayoffTitle = string;
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "structure_detail";
export type SchemaVersion = 3;
export type Scope = string;
export type Sentence = string;
export type DataTime1 = string;
export type StaleAfter = string;
export type Text4 = string;
export type Subline = string;
export type Caption = string;
export type Figure = Usd | Gap;
export type Label3 = string;
/**
 * @maxItems 20
 */
export type Timeline =
  | []
  | [Stage]
  | [Stage, Stage]
  | [Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage, Stage]
  | [
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage
    ]
  | [
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage
    ]
  | [
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage
    ]
  | [
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage,
      Stage
    ];
export type DateText = string;
export type DaysText = string;
export type Text5 = string;
export type Unit2 = "share_percent";
export type Value2 = number;
export type Name1 = string;
export type TimelineTitle = string;
export type Title = string;

export interface StructureDetail {
  build_id: BuildId;
  data_times: DataTimes;
  fills: Fills;
  fills_title: FillsTitle;
  gaps: Gaps;
  generated_at: GeneratedAt;
  greeks: Greeks;
  greeks_title: GreeksTitle;
  key_levels: KeyLevels;
  legs: Legs;
  legs_title: LegsTitle;
  legs_total: LegsTotal;
  levels_title: LevelsTitle;
  payoff: Payoff;
  payoff_title: PayoffTitle;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  sentence: Sentence;
  stale: StaleLabel;
  subline: Subline;
  tiles: Tiles;
  timeline: Timeline;
  timeline_title: TimelineTitle;
  title: Title;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source;
}
export interface Fill {
  action_text: ActionText;
  fee: Fee;
  leg_text: LegText;
  price: Usd;
  quantity: Quantity;
  time_text: TimeText;
}
export interface Usd {
  text: Text;
  unit: Unit;
  value: Value;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text1;
}
export interface GreeksBlock {
  model_text: ModelText;
  rows: Rows;
}
/**
 * One leg's Greeks (or the 合计 row): ``cells`` are Delta, Gamma, Theta, Vega in this order.
 */
export interface GreekRow {
  cells: Cells;
  label: Label;
}
export interface GreekFigure {
  text: Text2;
  unit: Unit1;
  value: Value1;
}
export interface Level {
  distance: Distance;
  name: Name;
  note: Note;
  price: Usd;
}
export interface Leg {
  expiry_text: ExpiryText;
  label: Label1;
  mark: Mark;
  pnl: Pnl;
  price: Usd;
  quantity: Quantity1;
  right_text: RightText;
  side_text: SideText;
  strike: Usd;
}
/**
 * The 合计 row of the legs table: the sum of the legs' ``pnl``, a Gap when any is a Gap.
 */
export interface LegsTotal {
  label: Label2;
  pnl: Pnl1;
}
export interface Payoff1 {
  aria_text: AriaText;
  band: Band;
  break_even: Usd;
  max_gain: Usd;
  max_loss: Usd;
  pnl_unit: PnlUnit;
  points: Points;
  price_unit: PriceUnit;
  spot: Usd;
}
/**
 * The one-standard-deviation range of the underlying until expiry, with Python's wording.
 */
export interface Band {
  high: High;
  low: Low;
  text: Text3;
}
export interface PayoffPoint {
  pnl: Pnl2;
  price: Price;
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text4;
}
/**
 * The four tiles of board 24, in this order.
 */
export interface Tiles {
  break_even: Tile;
  max_gain: Tile;
  max_loss: Tile;
  pnl: Tile;
}
export interface Tile {
  caption: Caption;
  figure: Figure;
  label: Label3;
}
export interface Stage {
  date_text: DateText;
  days_text: DaysText;
  elapsed: SharePercent;
  name: Name1;
}
export interface SharePercent {
  text: Text5;
  unit: Unit2;
  value: Value2;
}
