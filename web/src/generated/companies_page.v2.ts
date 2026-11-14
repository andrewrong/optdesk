// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type BuildId = string;
/**
 * @maxItems 100
 */
export type Lines = string[];
export type Note = string | null;
export type Sentence = string | null;
export type Title = string;
export type Text = string;
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
export type DataTime = string;
export type Source = string;
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
export type Note1 = string | null;
/**
 * @minItems 1
 * @maxItems 8
 */
export type Tiles =
  | [Tile]
  | [Tile, Tile]
  | [Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile, Tile, Tile]
  | [Tile, Tile, Tile, Tile, Tile, Tile, Tile, Tile];
export type Label = string;
export type Text2 = string;
export type Title1 = string;
export type DataDay = string;
export type HeldCount = number | null;
export type Line = string | null;
export type Title2 = string;
/**
 * @minItems 1
 * @maxItems 12
 */
export type Columns =
  | [Column]
  | [Column, Column]
  | [Column, Column, Column]
  | [Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column];
export type Numeric = boolean;
export type Text3 = string;
/**
 * @minItems 1
 * @maxItems 12
 */
export type Cells =
  | [Cell]
  | [Cell, Cell]
  | [Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell];
export type Text4 = string;
export type Tone = "up" | "down" | "flat" | "none";
/**
 * @minItems 1
 * @maxItems 20
 */
export type Moat =
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
 * @maxItems 200
 */
export type Rows = HoldingRow[];
export type VolatilityNote = string | null;
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
export type Day = string;
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "companies_page";
export type SchemaVersion = 2;
export type Scope = string;
/**
 * An RFC 3339 time with any offset.
 */
export type DataTime1 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type StaleAfter = string;
export type Text5 = string;
/**
 * An RFC 3339 time with any offset.
 */
export type Timestamp = string;
export type Title3 = string;
export type Unvalued = string | null;
/**
 * @maxItems 12
 */
export type Columns1 =
  | []
  | [Column]
  | [Column, Column]
  | [Column, Column, Column]
  | [Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column];
/**
 * @minItems 1
 * @maxItems 12
 */
export type Cells1 =
  | [Cell]
  | [Cell, Cell]
  | [Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell]
  | [Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell, Cell];
/**
 * @maxItems 200
 */
export type Rows1 = WatchRow[];
export type Sentence1 = string | null;
export type Title4 = string;

/**
 * companies_page v1 plus core, diagnosis and questions, each the block or a Gap (docs/design/valuation.md#Decisions 12).
 */
export interface CompaniesPageV2 {
  build_id: BuildId;
  changes: ChangesBlock | null;
  core: CoreBlock | Gap;
  data_times: DataTimes;
  diagnosis: DiagnosisBlock | Gap;
  gaps: Gaps;
  generated_at: GeneratedAt;
  glance: GlanceBlock | null;
  header: Header;
  holdings: HoldingsBlock | null;
  notes: Notes;
  /**
   * Optional: PE and PB of each company from stored filings and bars, or a Gap.
   */
  pepb?: PepbBlock | Gap;
  questions: QuestionsBlock | Gap;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  stale: StaleLabel;
  stale_after?: Timestamp;
  title: Title3;
  unvalued: Unvalued;
  watch_list: WatchBlock;
}
/**
 * ``lines`` are the changes; ``sentence`` is the one state sentence (近 7 天没有变化, or the
 * reading is not usable); ``note`` is ``数据日 月-日`` when the reading is of an earlier day.
 */
export interface ChangesBlock {
  lines: Lines;
  note?: Note;
  sentence?: Sentence;
  title: Title;
}
export interface CoreBlock {
  basis_note: Text;
  low_text: Text;
  high_text: Text;
  /**
   * The newest DGS10 point within 7 days of today, or a Gap.
   */
  treasury: Treasury | Gap;
  /**
   * A Japanese name has no rate to compare with: always a Gap.
   */
  jp_rate: Gap;
  /**
   * @maxItems 200
   */
  rows: CoreRow[];
  /**
   * @maxItems 20
   */
  legend:
    | []
    | [Text]
    | [Text, Text]
    | [Text, Text, Text]
    | [Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text
      ];
  snapshot_text: Text;
}
export interface Treasury {
  text: Text;
  /**
   * Position of the treasury mark on the bar, 0 to 100 (metric id: assigned by compute)
   */
  marker_percent: number;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text1;
}
/**
 * One company at one cost (the bare strike of a shown offer): the returns at that cost, no rank and no verdict.
 */
export interface CoreRow {
  symbol: Text;
  price_text: Text;
  cyclical: boolean;
  flag: Text | null;
  returns:
    | [ReturnPoint]
    | [ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint]
    | [ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint, ReturnPoint]
    | Gap;
}
/**
 * Return at the cost under one assumption; the text is produced.
 */
export interface ReturnPoint {
  assumption: Text;
  text: Text;
  /**
   * Position of the dot on the bar, 0 to 100 (metric id: assigned by compute)
   */
  marker_percent: number;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source;
}
export interface DiagnosisBlock {
  basis_note: Text;
  /**
   * @maxItems 10
   */
  groups:
    | []
    | [DiagGroup]
    | [DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup]
    | [DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup, DiagGroup];
}
export interface DiagGroup {
  /**
   * Class named by the producer; the view orders groups by array position.
   */
  key: string;
  title: Text;
  /**
   * @maxItems 12
   */
  columns:
    | []
    | [Text]
    | [Text, Text]
    | [Text, Text, Text]
    | [Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text];
  /**
   * @maxItems 200
   */
  rows: (DiagRow | Gap)[];
  /**
   * @maxItems 20
   */
  legend:
    | []
    | [Text]
    | [Text, Text]
    | [Text, Text, Text]
    | [Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text, Text]
    | [
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text,
        Text
      ];
}
/**
 * Ten-year ratios and ranks as produced text (metric id: assigned by compute).
 */
export interface DiagRow {
  symbol: Text;
  latest_fy: Text;
  latest: Text;
  rank_text: Text;
  extra: Text;
  usual: Text;
  /**
   * @maxItems 12
   */
  years:
    | []
    | [YearRow]
    | [YearRow, YearRow]
    | [YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow]
    | [YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow, YearRow];
}
export interface YearRow {
  fy: Text;
  revenue: Text;
  margin: Text;
  extra: Text;
  /**
   * Width of the mini bar, 0 to 100 (metric id: assigned by compute)
   */
  bar_percent: number;
}
/**
 * ``note`` is the price-lines state sentence, present only when the lines are not usable.
 */
export interface GlanceBlock {
  note?: Note1;
  tiles: Tiles;
  title: Title1;
}
export interface Tile {
  label: Label;
  text: Text2;
}
/**
 * ``line`` is ``月-日 · 家庭持有 N 家``; ``held_count`` is N, None without a held reading.
 */
export interface Header {
  data_day: DataDay;
  held_count?: HeldCount;
  line?: Line;
  title: Title2;
}
/**
 * ``volatility_note`` is the label line of the 波动 column (the own series and the data day);
 * absent on a payload made before the column.
 */
export interface HoldingsBlock {
  columns: Columns;
  rows: Rows;
  volatility_note?: VolatilityNote;
}
/**
 * A column head; ``numeric`` columns are right-aligned by the browser.
 */
export interface Column {
  numeric: Numeric;
  text: Text3;
}
/**
 * One held company: a cell per column, and its moat lines (the owner's record as written, or
 * ``护城河记录未填``).
 */
export interface HoldingRow {
  cells: Cells;
  moat: Moat;
}
/**
 * A table cell. A price line cell may carry price_state and hand_price.
 */
export interface Cell {
  text: Text4;
  tone?: Tone;
  /**
   * The state of a price line cell: the band, the owner's own price, the band not yet approved, or the approval older than 90 days.
   */
  price_state?: "band" | "owner override" | "awaiting approval" | "expired";
  /**
   * The owner's hand-entered buy price as text, shown beside the band; never changed.
   */
  hand_price?: string;
}
export interface PepbBlock {
  basis_note: Text;
  /**
   * @maxItems 200
   */
  rows: PepbRow[];
}
/**
 * One company: PE and PB each a line or a Gap, and the forward Gap.
 */
export interface PepbRow {
  symbol: Text;
  pe: PepbLine | Gap;
  pb: PepbLine | Gap;
  /**
   * Forward PE has no estimates source: always a Gap.
   */
  forward: Gap;
}
/**
 * One ratio of one company on the newest stored day, as produced text and figures; no verdict.
 */
export interface PepbLine {
  text: Text;
  ratio: Figure & {
    unit?: "ratio";
    [k: string]: unknown;
  };
  day: Day;
  basis_end: Day;
  percentile: Figure | Gap;
  /**
   * @maxItems 2
   */
  at_percentile: [] | [PepbAtPercentile] | [PepbAtPercentile, PepbAtPercentile];
  shares: Text;
  shares_filed: Day;
  window_days: number;
  window_from: Day;
  /**
   * @maxItems 120
   */
  history: PepbHistoryPoint[];
}
/**
 * A number the producer wrote; the view prints text.
 */
export interface Figure {
  value: number;
  text: Text;
  unit: string;
}
/**
 * The ratio at a rank of the window and the price that puts today's ratio there; history, not a buy price.
 */
export interface PepbAtPercentile {
  percentile: 20 | 50;
  ratio: Figure & {
    unit?: "ratio";
    [k: string]: unknown;
  };
  price: Figure & {
    unit?: "price";
    [k: string]: unknown;
  };
  text: Text;
}
/**
 * One stored day of the chart.
 */
export interface PepbHistoryPoint {
  day: Day;
  value: number;
  text: Text;
}
export interface QuestionsBlock {
  basis_note: Text;
  /**
   * @maxItems 200
   */
  rows: QuestionRow[];
}
/**
 * Q1 to Q4 as figures from filings or a Gap; Q5 has no figure.
 */
export interface QuestionRow {
  symbol: Text;
  /**
   * Return on tangible capital.
   */
  q1: Figure | Gap;
  /**
   * Growth cost.
   */
  q2: Figure | Gap;
  /**
   * Pricing power.
   */
  q3: Figure | Gap;
  /**
   * Cash behind profit.
   */
  q4: Figure | Gap;
  /**
   * The owner's own text for the fifth question, or a Gap when none is entered.
   */
  fifth: Text | Gap;
}
/**
 * The times and the label; the browser only compares its clock with stale_after.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text5;
}
/**
 * Rows, or one state sentence naming the file (not filled / unreadable) and no rows.
 */
export interface WatchBlock {
  columns: Columns1;
  rows: Rows1;
  sentence?: Sentence1;
  title: Title4;
}
export interface WatchRow {
  cells: Cells1;
}
