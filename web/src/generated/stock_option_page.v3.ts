// Code generated from contracts/, DO NOT EDIT.
/* eslint-disable */

export type BuildId = string;
export type Advice = string;
/**
 * @maxItems 30
 */
export type Amounts = string[];
export type Chosen = boolean;
export type Expiry = string;
export type InMatrix = boolean;
export type Key = string;
/**
 * @minItems 3
 * @maxItems 3
 */
export type Ladder = [Pair, Pair, Pair];
export type Label = string;
export type Text = string;
export type Liquidity = string;
export type PassTag = boolean;
export type Risk = string;
export type Strike = string;
export type Title = string;
export type YieldLine = string;
/**
 * @maxItems 2000
 */
export type Cards = ChosenCard[];
export type Basis = string;
/**
 * @minItems 11
 * @maxItems 11
 */
export type Columns = [string, string, string, string, string, string, string, string, string, string, string];
export type Empty = string | null;
/**
 * @maxItems 10
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
  | [string, string, string, string, string, string, string, string, string, string];
/**
 * @minItems 11
 * @maxItems 11
 */
export type Cells = [string, string, string, string, string, string, string, string, string, string, string];
export type Expiry1 = string;
export type ExpiryText = string;
export type AsOf = string;
/**
 * null for a close archive value, which has no tick type.
 */
export type DataType = ("live" | "frozen" | "delayed") | null;
export type OutsideBand = boolean;
export type Skew = boolean;
export type Source = "open_pull" | "refresh" | "close_archive";
/**
 * @maxItems 2000
 */
export type Rows = ContractLine[];
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
export type Source1 = string;
export type Footer = string;
export type GeneratedAt = string;
export type Account = string;
export type Detail = string;
export type Earned = string | null;
export type Headline = string;
export type Done = string;
export type Mark = string | null;
export type Note = string;
/**
 * @maxItems 50
 */
export type Held = HeldItem[];
/**
 * @maxItems 2
 */
export type Bands = [] | [MapBand] | [MapBand, MapBand];
export type High = string;
export type Kind = "pref" | "sigma";
export type Low = string;
export type Text1 = string;
export type Description = string;
export type Expiry2 = string;
export type Heading = string;
export type Kind1 = "spot" | "cap" | "ma50" | "ma200" | "breakeven" | "strike";
export type Price = string;
export type Text2 = string;
/**
 * @maxItems 200
 */
export type Marks = MapMark[];
/**
 * @maxItems 30
 */
export type Maps = PriceMap[];
export type Chosen1 = boolean;
export type Delta = string;
export type Earnings = boolean;
export type Expiry3 = string;
export type Key1 = string;
export type Out = boolean;
export type Premium = string;
export type Shade = "worth" | "marginal" | "none";
export type Star = boolean;
export type Strike1 = string;
export type Thin = boolean;
export type Volume = string;
export type YieldText = string;
/**
 * @maxItems 2000
 */
export type Cells1 = MatrixCell[];
export type Empty1 = string | null;
export type Expiry4 = string;
export type Text3 = string;
/**
 * @maxItems 30
 */
export type Expiries = MatrixExpiry[];
export type Facts = string | null;
export type Legend = string;
/**
 * @maxItems 100
 */
export type Strikes = string[];
export type NoCard = string | null;
export type Notice = string | null;
export type Enabled = boolean;
export type FreshCount = number;
export type Notice1 = string | null;
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
export type Schema = "stock_option_page";
export type SchemaVersion = 3;
export type Scope = string;
export type SharedGap = string | null;
export type DataTime1 = string;
export type StaleAfter = string;
export type Text4 = string;
export type Title1 = string;
export type DayChange = string | null;
export type DayTone = ("up" | "down" | "flat" | "none") | null;
export type Earnings1 = string | null;
export type Iv = string | null;
export type Name = string | null;
export type Policy = string;
export type Quote1 = string | null;
export type RefreshExpiry = string | null;
export type RefreshStrike = string | null;
export type Spot = string | null;
export type Symbol = string;
export type Trend = string;

export interface StockOptionPage {
  build_id: BuildId;
  cards: Cards;
  contracts: ContractsBlock;
  data_times: DataTimes;
  footer: Footer;
  generated_at: GeneratedAt;
  held: Held;
  maps: Maps;
  matrix: MatrixBlock;
  no_card: NoCard;
  notice: Notice;
  refresh: Refresh;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  shared_gap: SharedGap;
  stale: StaleLabel | null;
  title: Title1;
  top: TopBar;
  vol_view: VolView;
}
/**
 * The chosen-contract card of one cell (``key`` of the cell), or of the board's chosen contract
 * when that is no cell (``in_matrix`` false).
 */
export interface ChosenCard {
  advice: Advice;
  amounts: Amounts;
  chosen: Chosen;
  expiry: Expiry;
  in_matrix: InMatrix;
  key: Key;
  ladder: Ladder;
  liquidity: Liquidity;
  pass_tag: PassTag;
  risk: Risk;
  strike: Strike;
  title: Title;
  yield_line: YieldLine;
}
/**
 * A term and its value, both text as Python wrote them (the Bid, Mid and Ask ladder).
 */
export interface Pair {
  label: Label;
  text: Text;
}
export interface ContractsBlock {
  basis: Basis;
  columns: Columns;
  empty: Empty;
  notes: Notes;
  rows: Rows;
}
/**
 * One fetched put; ``cells`` follow the block's ``columns`` (a missing figure reads ``—``).
 */
export interface ContractLine {
  cells: Cells;
  expiry: Expiry1;
  expiry_text: ExpiryText;
  quote: Quote;
}
/**
 * The quote behind the prices of one row (docs/design/option-open-pull.md Decision 18): when it was taken (exchange-local HH:MM:SS, or a date text for a close archive value), the market data type of its tick, where it came from, and the two flags of the pull. Copied from the stored row, never computed.
 */
export interface Quote {
  as_of: AsOf;
  data_type: DataType;
  outside_band: OutsideBand;
  skew: Skew;
  source: Source;
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
  data_time: DataTime;
  source: Source1;
}
/**
 * A held put: ``headline``, ``detail`` (a line made of its parts) and the account. ``earned`` is
 * the share earned as text, absent when it cannot be said.
 */
export interface HeldItem {
  account: Account;
  detail: Detail;
  earned: Earned;
  headline: Headline;
  progress: Progress | null;
}
/**
 * The share earned on a held put as a bar: ``done`` is percent as text (``72.0``), ``mark`` the
 * close report's line in percent (absent while the threshold is unset) and ``note`` what says so.
 */
export interface Progress {
  done: Done;
  mark: Mark;
  note: Note;
}
/**
 * The map of one expiry: its marks and its bands (the expected move's edges are the ``sigma``
 * band's low and high; absent when the board has no move for the expiry).
 */
export interface PriceMap {
  bands: Bands;
  description: Description;
  expiry: Expiry2;
  heading: Heading;
  marks: Marks;
}
/**
 * A range on the axis: the preferred range (``pref``) or the expected move (``sigma``).
 */
export interface MapBand {
  high: High;
  kind: Kind;
  low: Low;
  text: Text1;
}
/**
 * A price on the map: ``price`` for the axis, ``text`` as the old page labels it.
 */
export interface MapMark {
  kind: Kind1;
  price: Price;
  text: Text2;
}
/**
 * ``facts`` is the line of what the matrix is drawn on, ``legend`` the key to its marks,
 * ``empty`` the sentence when there is no cell (then ``cells`` is empty).
 */
export interface MatrixBlock {
  cells: Cells1;
  empty: Empty1;
  expiries: Expiries;
  facts: Facts;
  legend: Legend;
  strikes: Strikes;
}
/**
 * One existing contract. ``key`` joins it to its card. ``yield_text`` and ``premium`` are what
 * the two views of the old matrix show; ``out`` is a delta outside the band, ``thin`` poor
 * liquidity, ``earnings`` an earnings date before expiry.
 */
export interface MatrixCell {
  chosen: Chosen1;
  delta: Delta;
  earnings: Earnings;
  expiry: Expiry3;
  key: Key1;
  out: Out;
  premium: Premium;
  quote: Quote;
  shade: Shade;
  star: Star;
  strike: Strike1;
  thin: Thin;
  volume: Volume;
  yield_text: YieldText;
}
export interface MatrixExpiry {
  expiry: Expiry4;
  text: Text3;
}
/**
 * The state of the refresh button (Decisions 13 to 20): copied by the builder from the route; the web computes nothing. When enabled is false the notice says why.
 */
export interface Refresh {
  enabled: Enabled;
  fresh_count: FreshCount;
  notice: Notice1;
  symbols: Symbols;
  total_count: TotalCount;
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
 * The bar above the page. ``quote`` is the quotes' time and market-data type (or the stored
 * quotes' time and reason, without a market-data type); ``iv`` and ``earnings`` are absent when the
 * board has none of their parts; ``trend`` and ``policy`` always read. ``refresh_expiry`` and
 * ``refresh_strike`` name the chosen contract the refresh link goes back to, if any.
 */
export interface TopBar {
  day_change: DayChange;
  day_tone: DayTone;
  earnings: Earnings1;
  iv: Iv;
  name: Name;
  policy: Policy;
  quote: Quote1;
  refresh_expiry: RefreshExpiry;
  refresh_strike: RefreshStrike;
  spot: Spot;
  symbol: Symbol;
  trend: Trend;
}
export interface VolView {
  coaching: string;
  method: string;
  name: string;
  /**
   * @maxItems 20
   */
  rows:
    | []
    | [VolRow]
    | [VolRow, VolRow]
    | [VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow, VolRow]
    | [
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow
      ]
    | [
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow
      ]
    | [
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow
      ]
    | [
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow
      ]
    | [
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow
      ]
    | [
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow,
        VolRow
      ];
}
/**
 * One line of the volatility view; a ready row has an empty gap, a gap row an empty text.
 */
export interface VolRow {
  data_day: string;
  gap: string;
  label: string;
  text: string;
}
