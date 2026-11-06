// TEMPORARY placeholder, copied from the old repository's generated types until the week1-schemas issue generates them.
export type Href = string
export type Label = string
export type BuildId = string
export type DataTime = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text = string
/**
 * @maxItems 20
 */
export type Gaps = Gap[]
export type GeneratedAt = string
/**
 * @maxItems 20
 */
export type Notes = string[]
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "full_scan"
export type SchemaVersion = 1
export type Scope = string
/**
 * @minItems 3
 * @maxItems 3
 */
export type Sections = [ScanSection, ScanSection, ScanSection]
export type Liquidity = (string | null)
export type Note = (string | null)
export type Strategy = ("short_put" | "covered_call" | "put_credit_spread")
export type Sub = (string | null)
/**
 * @minItems 1
 * @maxItems 20
 */
export type Columns = [Column, ...(Column)[]]
export type Align = ("left" | "right")
export type Hint = (string | null)
export type Id = string
export type Label1 = string
/**
 * @minItems 1
 * @maxItems 20
 */
export type Cells = [CellAt, ...(CellAt)[]]
export type Cell = (UsdAmount | JpyAmount | PercentAmount | VolAmount | CountAmount | TextCell | Gap)
export type Source1 = (string | null)
export type Text1 = string
export type Unit = "usd"
export type Value = number
export type Text2 = string
export type Unit1 = "jpy"
export type Value1 = number
export type Text3 = string
export type Unit2 = "percent"
export type Value2 = number
export type Text4 = string
export type Unit3 = "vol_points"
export type Value3 = number
export type Text5 = string
export type Unit4 = "contracts"
export type Value4 = number
export type Badge = (boolean | null)
export type Kind = "text"
export type Text6 = string
export type Value5 = (number | string | null)
export type Column1 = string
export type Id1 = string
/**
 * @maxItems 2000
 */
export type Rows = Row[]
export type Title = string
export type DataTime1 = string
export type StaleAfter = string
export type Text7 = string
export type Title1 = string

/**
 * Envelope (ADR 0034) and the page.
 */
export interface FullScan {
back: Link
build_id: BuildId
data_times: DataTimes
gaps: Gaps
generated_at: GeneratedAt
notes: Notes
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope
sections: Sections
stale: (StaleLabel | null)
title: Title1
}
export interface Link {
href: Href
label: Label
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
data_time: DataTime
source: Source
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
reason: Reason
text: Text
}
/**
 * One strategy: a table with its sub line (and liquidity sentence), or the Gap and no table.
 */
export interface ScanSection {
gap?: (Gap | null)
liquidity?: Liquidity
note?: Note
strategy: Strategy
sub?: Sub
table?: (ScanTable | null)
title: Title
}
export interface ScanTable {
columns: Columns
rows: Rows
}
export interface Column {
align: Align
hint?: Hint
id: Id
label: Label1
}
export interface Row {
cells: Cells
id: Id1
}
/**
 * The cell of one column of a row (a list, not a map: the writer walks lists and dataclasses).
 */
export interface CellAt {
cell: Cell
column: Column1
}
export interface UsdAmount {
source?: Source1
text: Text1
unit: Unit
value: Value
}
/**
 * A price in Japanese yen (GLOSSARY: JPY with ≈USD). ``approx_usd`` is the display-only
 * figure Python made from the stored JPY per USD rate; without a stored rate it is absent.
 */
export interface JpyAmount {
approx_usd?: (UsdAmount | null)
text: Text2
unit: Unit1
value: Value1
}
export interface PercentAmount {
text: Text3
unit: Unit2
value: Value2
}
export interface VolAmount {
text: Text4
unit: Unit3
value: Value3
}
export interface CountAmount {
text: Text5
unit: Unit4
value: Value4
}
/**
 * A text cell; ``value`` is the sort key (a rank for a verdict, the symbol for a name).
 */
export interface TextCell {
badge?: Badge
kind: Kind
text: Text6
value?: Value5
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text7
}
