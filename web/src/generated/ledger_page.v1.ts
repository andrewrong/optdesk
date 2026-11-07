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
 * @maxItems 10
 */
export type Footer =
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
export type FxNote = string | null;
export type Through = string;
export type Title = string;
/**
 * @maxItems 20
 */
export type Notes =
  | []
  | [Note]
  | [Note, Note]
  | [Note, Note, Note]
  | [Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note, Note]
  | [
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note,
      Note
    ];
export type Term = string;
export type Text1 = string;
export type NothingClosed = string | null;
export type Count = string;
export type Key = string;
export type Title1 = string;
export type Text2 = string;
export type Tone = "gain" | "loss" | "none";
/**
 * @maxItems 60
 */
export type Overview = OverviewRow[];
export type RunId = string;
export type RunStatus = "ok" | "partial" | "failed";
export type Schema = "ledger_page";
export type SchemaVersion = 1;
export type Scope = string;
/**
 * @minItems 2
 * @maxItems 2000
 */
export type Points = [ChartPoint, ChartPoint, ...ChartPoint[]];
export type Day = string;
export type Text3 = string;
export type Value = number;
export type Summary = string;
export type Key1 = string;
export type Caption = string;
/**
 * @maxItems 500
 */
export type More = string[];
/**
 * @maxItems 40
 */
export type Detail = string[];
export type Label = string;
export type Small = string;
/**
 * @maxItems 2000
 */
export type Open = OpenRow[];
/**
 * @maxItems 2
 */
export type OpenColumns = [] | [string] | [string, string];
export type OpenTitle = string | null;
/**
 * @maxItems 20
 */
export type Strategies =
  | []
  | [StrategyRow]
  | [StrategyRow, StrategyRow]
  | [StrategyRow, StrategyRow, StrategyRow]
  | [StrategyRow, StrategyRow, StrategyRow, StrategyRow]
  | [StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow]
  | [StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow]
  | [StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow]
  | [StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow, StrategyRow]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ]
  | [
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow,
      StrategyRow
    ];
/**
 * @minItems 4
 * @maxItems 4
 */
export type Cells = [Cell, Cell, Cell, Cell];
/**
 * @minItems 3
 * @maxItems 3
 */
export type Cells1 = [Cell, Cell, Cell];
/**
 * @maxItems 40
 */
export type Detail1 = string[];
/**
 * @maxItems 2000
 */
export type Closed = ClosedRow[];
export type Delivered = boolean;
export type Text4 = string;
export type Value1 = number;
/**
 * @maxItems 2000
 */
export type Dots = Dot[];
/**
 * @maxItems 20
 */
export type Lines =
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
 * @maxItems 4
 */
export type StrategyColumns =
  [] | [string] | [string, string] | [string, string, string] | [string, string, string, string];
export type Title2 = string;
export type TotalsDiffer = string | null;
/**
 * @maxItems 60
 */
export type Sections = Section[];
export type Key2 = string;
export type Lead1 = string | null;
export type Note1 = string | null;
/**
 * @minItems 3
 * @maxItems 3
 */
export type Cells2 = [Cell, Cell, Cell];
/**
 * @maxItems 500
 */
export type Rows = ShareRow[];
/**
 * @minItems 4
 * @maxItems 4
 */
export type Cells3 = [Cell, Cell, Cell, Cell];
/**
 * @minItems 1
 * @maxItems 200
 */
export type Detail2 = [string, ...string[]];
/**
 * @maxItems 500
 */
export type Sales = SaleRow[];
export type Title3 = string;
/**
 * @maxItems 60
 */
export type Shares = SharesSection[];
export type DataTime1 = string;
export type StaleAfter = string;
export type Text5 = string;
export type Title4 = string;

export interface LedgerPage {
  build_id: BuildId;
  data_times: DataTimes;
  footer: Footer;
  gaps: Gaps;
  generated_at: GeneratedAt;
  header: Header;
  notes: Notes;
  nothing_closed: NothingClosed;
  overview: Overview;
  run_id: RunId;
  run_status: RunStatus;
  schema: Schema;
  schema_version: SchemaVersion;
  scope: Scope;
  sections: Sections;
  shares: Shares;
  stale: StaleLabel;
  title: Title4;
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
/**
 * ``through`` is ``Realized · 截至 <day>``; ``fx_note`` names the USD.JPY rate, or says there is
 * none within 7 days; it is absent on a page without a JPY section.
 */
export interface Header {
  fx_note?: FxNote;
  through: Through;
  title: Title;
}
export interface Note {
  term: Term;
  text: Text1;
}
/**
 * ``total`` is the closed total (``—`` when none is closed; absent on the Shares row);
 * ``count`` is ``N open`` or, on the Shares row, ``N held``.
 */
export interface OverviewRow {
  count: Count;
  key: Key;
  title: Title1;
  total: Cell | null;
}
export interface Cell {
  text: Text2;
  tone?: Tone;
}
/**
 * One account and contract currency. ``lead`` is absent when ``nothing_closed`` of the page
 * already says no position is closed; ``totals_differ`` is present only when the chains and the
 * option books disagree; ``more`` are the lines of the folded More block.
 */
export interface Section {
  chart: Chart | null;
  key: Key1;
  lead: Lead | null;
  more: More;
  open: Open;
  open_columns: OpenColumns;
  open_title: OpenTitle;
  strategies: Strategies;
  strategy_columns: StrategyColumns;
  title: Title2;
  totals_differ: TotalsDiffer;
}
/**
 * The cumulative line of a section (six or more closed positions): the points in day order,
 * the first one on the first of the first month at 0, and the sentence that stands for the chart.
 */
export interface Chart {
  points: Points;
  summary: Summary;
}
/**
 * One step of the cumulative line: the day, the running total as text and as the number the
 * chart draws.
 */
export interface ChartPoint {
  day: Day;
  text: Text3;
  value: Value;
}
/**
 * The headline total of a section and its caption (``Realized P&L · 2 closed``), or no total
 * and the no-closed sentence.
 */
export interface Lead {
  caption: Caption;
  total: Cell | null;
}
/**
 * An open position: its contract, the small line under it (strategy, DTE, status), the premium
 * (unsigned, as the old page prints it) and the facts one tap down.
 */
export interface OpenRow {
  detail: Detail;
  label: Label;
  premium: Cell;
  small: Small;
}
/**
 * ``cells`` are Strategy, Closed, Realized, Ann.; ``lines`` the sub lines; ``dots`` the strip
 * (two or more closed positions); ``closed`` newest first.
 */
export interface StrategyRow {
  cells: Cells;
  closed: Closed;
  dots: Dots;
  lines: Lines;
}
/**
 * ``cells`` are the contract, the realized result (with its tone) and the end mark (may be
 * empty); ``detail`` are the facts one tap down.
 */
export interface ClosedRow {
  cells: Cells1;
  detail: Detail1;
}
/**
 * One closed position of a strategy on its dot strip; ``delivered`` is drawn as a ring.
 */
export interface Dot {
  delivered: Delivered;
  text: Text4;
  value: Value1;
}
/**
 * The stock side of one account and currency. When the baseline was refused ``note`` holds the
 * reason and there are no rows.
 */
export interface SharesSection {
  key: Key2;
  lead: Lead1;
  note?: Note1;
  rows: Rows;
  sales: Sales;
  title: Title3;
}
/**
 * A held or unreconciled row: symbol, shares, average cost (or the ``◦`` mark).
 */
export interface ShareRow {
  cells: Cells2;
}
/**
 * A sale summary: four cells (shares, how and at what price, average cost, result) and its
 * detail lines (the day, then the inflows).
 */
export interface SaleRow {
  cells: Cells3;
  detail: Detail2;
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
  data_time: DataTime1;
  stale_after: StaleAfter;
  text: Text5;
}
