// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type Id = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp1 = string;
export type Id1 = string;
export type RunStatus = "ok" | "partial" | "failed";
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp2 = string;
export type IsStale = boolean;
export type Text = string;
export type Text1 = string;
export type FigureOrGap = Figure | Gap1;
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
export type Text2 = string;
export type Day = string;
export type LongText = string;
export type RankOrGap =
  | {
      at: number;
      of: number;
    }
  | Gap1;

/**
 * Where the index, VIX, high-yield spread and the sector funds stand today: level and percentile only, each market row with a trend chart. A render of text and numbers a producer wrote; nothing is computed here.
 */
export interface MarketStatePage {
  schema: "market_state_page";
  schema_version: 1;
  run_id: Id;
  build_id: Id;
  scope: Id;
  generated_at: Timestamp;
  data_times: SourceTime[];
  run_status: RunStatus;
  stale: StaleLabel | null;
  title: Text1;
  notes: Text1[];
  gaps: Gap[];
  /**
   * The page day, an ISO date.
   */
  day: string;
  /**
   * What the figures are and how old each source is; the producer's text.
   */
  freshness_text: string;
  market: MarketBlock;
  sectors: SectorBlock;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: Timestamp1;
  source: Id1;
}
/**
 * The times and the label. The producer sets is_stale; the browser compares no clock.
 */
export interface StaleLabel {
  data_time: Timestamp2;
  is_stale: IsStale;
  text: Text;
}
/**
 * A named data gap of the page, used in place of a value that cannot be shown. Never a zero.
 */
export interface Gap {
  kind: "gap";
  reason: string;
  text: Text1;
}
export interface MarketBlock {
  title: Text1;
  /**
   * @minItems 5
   * @maxItems 5
   */
  rows: [MarketRow, MarketRow, MarketRow, MarketRow, MarketRow];
}
/**
 * One market row. Level and percentile only: no verdict word.
 */
export interface MarketRow {
  id: "sp500_vs_200d" | "sp500_drawdown" | "vix" | "vix_term" | "hy_spread";
  label: Text1;
  figure: FigureOrGap;
  /**
   * What the figure is measured over, in words.
   */
  window: string;
  /**
   * The data day of the figure, or the Gap when the figure is a gap.
   */
  day: Day | Gap1;
  percentile?: Percentile;
  history: History | Gap1;
}
/**
 * A number the producer wrote; the view prints text.
 */
export interface Figure {
  kind: "value";
  value: number;
  text: Text1;
  unit: string;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap1 {
  kind: Kind;
  reason: Reason;
  text: Text2;
}
/**
 * A percentile text and the words saying what it covers.
 */
export interface Percentile {
  text: Text1;
  window: Text1;
}
/**
 * The daily points of a row, ascending; the data of the chart. The caption is its visible text.
 */
export interface History {
  /**
   * The visible text of the chart: first day and number of points.
   */
  caption: string;
  aria_text: LongText;
  unit: string;
  /**
   * @maxItems 800
   */
  points: HistoryPoint[];
}
export interface HistoryPoint {
  day: Day;
  value: number;
  /**
   * The display text of the point, as the tooltip shows it.
   */
  text: string;
}
export interface SectorBlock {
  caption: LongText;
  /**
   * The newest close day of the benchmark, or a Gap when it has none.
   */
  end_day: Day | Gap1;
  bars_caption: LongText;
  rotation_axes: RotationAxes;
  /**
   * @maxItems 12
   */
  rows:
    | []
    | [SectorRow]
    | [SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow]
    | [SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow, SectorRow]
    | [
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow
      ]
    | [
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow,
        SectorRow
      ];
}
export interface RotationAxes {
  x_caption: Text1;
  y_caption: Text1;
  /**
   * Names both windows and the tail offsets 5, 10, 15 and 20 sessions.
   */
  note: string;
  /**
   * @minItems 4
   * @maxItems 4
   */
  quadrants: [Quadrant, Quadrant, Quadrant, Quadrant];
}
export interface Quadrant {
  id: "leading" | "weakening" | "lagging" | "improving";
  text: Text1;
}
export interface SectorRow {
  symbol: Text1;
  label: Text1;
  gap_21: FigureOrGap;
  gap_63: FigureOrGap;
  rank_21: RankOrGap;
  rank_63: RankOrGap;
  group: "sector" | "industry";
  rotation:
    | [RotationPoint]
    | [RotationPoint, RotationPoint]
    | [RotationPoint, RotationPoint, RotationPoint]
    | [RotationPoint, RotationPoint, RotationPoint, RotationPoint]
    | [RotationPoint, RotationPoint, RotationPoint, RotationPoint, RotationPoint]
    | Gap1;
}
export interface RotationPoint {
  day: Day;
  x: number;
  y: number;
  /**
   * Starts with the point's own day.
   */
  text: string;
}
