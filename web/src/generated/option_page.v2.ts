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
export type GeneratedAt = string;
/**
 * @minItems 1
 * @maxItems 2
 */
export type Markets = [MarketPane] | [MarketPane, MarketPane];
export type Target = string;
export type Text1 = string;
/**
 * @minItems 1
 * @maxItems 20
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
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column, Column]
  | [
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column
    ]
  | [
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column
    ]
  | [
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column
    ]
  | [
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column
    ]
  | [
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column
    ]
  | [
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column,
      Column
    ];
export type Align = "left" | "right";
export type Hint = string | null;
export type Id = string;
export type Label = string;
/**
 * @minItems 1
 * @maxItems 20
 */
export type Cells =
  | [CellAt]
  | [CellAt, CellAt]
  | [CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt, CellAt]
  | [
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt
    ]
  | [
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt
    ]
  | [
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt
    ]
  | [
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt
    ]
  | [
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt
    ]
  | [
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt,
      CellAt
    ];
export type Cell = UsdAmount | JpyAmount | PercentAmount | VolAmount | CountAmount | TextCell | Gap;
export type Source1 = string | null;
export type Text2 = string;
export type Unit = "usd";
export type Value = number;
export type Text3 = string;
export type Unit1 = "jpy";
export type Value1 = number;
export type Text4 = string;
export type Unit2 = "percent";
export type Value2 = number;
export type Text5 = string;
export type Unit3 = "vol_points";
export type Value3 = number;
export type Text6 = string;
export type Unit4 = "contracts";
export type Value4 = number;
export type Badge = boolean | null;
export type Kind = "text";
export type Text7 = string;
export type Value5 = number | string | null;
export type Column1 = string;
export type Id1 = string;
export type AsOf = string;
/**
 * null for a close archive value, which has no tick type.
 */
export type DataType = ("live" | "frozen" | "delayed") | null;
export type OutsideBand = boolean;
export type Skew = boolean;
export type Source2 = "open_pull" | "refresh" | "close_archive";
/**
 * @maxItems 200
 */
export type Rows = Row[];
export type CandidatesTitle = string;
/**
 * @maxItems 20
 */
export type Cards =
  | []
  | [Opportunity]
  | [Opportunity, Opportunity]
  | [Opportunity, Opportunity, Opportunity]
  | [Opportunity, Opportunity, Opportunity, Opportunity]
  | [Opportunity, Opportunity, Opportunity, Opportunity, Opportunity]
  | [Opportunity, Opportunity, Opportunity, Opportunity, Opportunity, Opportunity]
  | [Opportunity, Opportunity, Opportunity, Opportunity, Opportunity, Opportunity, Opportunity]
  | [Opportunity, Opportunity, Opportunity, Opportunity, Opportunity, Opportunity, Opportunity, Opportunity]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ]
  | [
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity,
      Opportunity
    ];
export type Annualised = PercentAmount | Gap;
export type AnnualisedText = string;
export type ContractLine = string;
export type Id2 = string;
export type IvLine = string;
export type LiquidityLine = string;
export type PremiumLine = string;
export type Reason1 = string;
export type SizeLine = string;
export type Symbol = string;
/**
 * @maxItems 10
 */
export type Tags =
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
  | [string, string, string, string, string, string, string, string, string, string];
export type Key = "worth" | "marginal" | "pending" | "skip";
export type Label1 = string;
export type Tone = "good" | "warn" | "neutral" | "muted";
export type CardsTitle = string;
export type Summary = string;
export type Title = string;
export type Currency = "USD" | "JPY";
export type Headline = string;
export type Label2 = string;
export type Market = "US" | "JP";
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
export type Enabled = boolean;
export type FreshCount = number;
export type Notice = string | null;
/**
 * @maxItems 8
 */
export type Symbols =
  | []
  | [string]
  | [string, string]
  | [string, string, string]
  | [string, string, string, string]
  | [string, string, string, string, string]
  | [string, string, string, string, string, string]
  | [string, string, string, string, string, string, string]
  | [string, string, string, string, string, string, string, string];
export type TotalCount = number;
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "option_page";
export type SchemaVersion = 2;
export type Scope = string;
export type DataTime1 = string;
export type StaleAfter = string;
export type Text8 = string;
export type Title1 = string;

/**
 * Envelope (ADR 0034) and the page.
 */
export interface OptionPage {
  build_id: BuildId;
  data_times: DataTimes;
  gaps: Gaps;
  generated_at: GeneratedAt;
  markets: Markets;
  notes: Notes;
  refresh: Refresh;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  stale: StaleLabel;
  title: Title1;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source;
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
  reason: Reason;
  text: Text;
}
export interface MarketPane {
  all_link: Link;
  candidates: Table;
  candidates_title: CandidatesTitle;
  cards: Cards;
  cards_title: CardsTitle;
  chain?: Chain | null;
  currency: Currency;
  headline: Headline;
  label: Label2;
  market: Market;
}
/**
 * A link to a section of the same page: ``target`` is the id of that section.
 */
export interface Link {
  target: Target;
  text: Text1;
}
export interface Table {
  columns: Columns;
  rows: Rows;
}
export interface Column {
  align: Align;
  hint?: Hint;
  id: Id;
  label: Label;
}
export interface Row {
  cells: Cells;
  id: Id1;
  quote: Quote;
}
/**
 * The cell of one column of a row (a list, not a map: the writer walks lists and dataclasses).
 */
export interface CellAt {
  cell: Cell;
  column: Column1;
}
export interface UsdAmount {
  source?: Source1;
  text: Text2;
  unit: Unit;
  value: Value;
}
/**
 * A price in Japanese yen (GLOSSARY: JPY with ≈USD). ``approx_usd`` is the display-only
 * figure Python made from the stored JPY per USD rate; without a stored rate it is absent.
 */
export interface JpyAmount {
  approx_usd?: UsdAmount | null;
  text: Text3;
  unit: Unit1;
  value: Value1;
}
export interface PercentAmount {
  text: Text4;
  unit: Unit2;
  value: Value2;
}
export interface VolAmount {
  text: Text5;
  unit: Unit3;
  value: Value3;
}
export interface CountAmount {
  text: Text6;
  unit: Unit4;
  value: Value4;
}
/**
 * A text cell; ``value`` is the sort key (a rank for a verdict, the symbol for a name).
 */
export interface TextCell {
  badge?: Badge;
  kind: Kind;
  text: Text7;
  value?: Value5;
}
/**
 * The quote behind the prices of one row (docs/design/option-open-pull.md Decision 18): when it was taken (exchange-local HH:MM:SS, or a date text for a close archive value), the market data type of its tick, where it came from, and the two flags of the pull. Copied from the stored row, never computed.
 */
export interface Quote {
  as_of: AsOf;
  data_type: DataType;
  outside_band: OutsideBand;
  skew: Skew;
  source: Source2;
}
/**
 * One 今日机会 card.
 */
export interface Opportunity {
  annualised: Annualised;
  annualised_text: AnnualisedText;
  contract_line: ContractLine;
  id: Id2;
  iv_line: IvLine;
  liquidity_line: LiquidityLine;
  premium_line: PremiumLine;
  reason: Reason1;
  size_line: SizeLine;
  size_usd_approx?: UsdAmount | null;
  symbol: Symbol;
  tags: Tags;
  verdict: Verdict;
}
/**
 * Written by Python; the browser never derives it from the figures.
 */
export interface Verdict {
  key: Key;
  label: Label1;
  tone: Tone;
}
/**
 * One option chain: strike by expiry, the Mid of each contract; the chosen one is marked.
 */
export interface Chain {
  summary: Summary;
  table: Table;
  title: Title;
}
/**
 * The state of the refresh button (Decisions 13 to 20): copied by the builder from the route; the web computes nothing. When enabled is false the notice says why.
 */
export interface Refresh {
  enabled: Enabled;
  fresh_count: FreshCount;
  notice: Notice;
  symbols: Symbols;
  total_count: TotalCount;
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text8;
}
