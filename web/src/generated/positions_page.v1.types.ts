// TEMPORARY placeholder, copied from the old repository's generated types until the week1-schemas issue generates them.
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
export type Schema = "positions_page"
export type SchemaVersion = 1
export type Scope = string
export type DataTime1 = string
export type StaleAfter = string
export type Text1 = string
export type Title = string
/**
 * @minItems 3
 * @maxItems 3
 */
export type Views = [PositionsView, PositionsView, PositionsView]
export type Key = ("me" | "spouse" | "household")
export type Label = ("我" | "配偶" | "家庭")
/**
 * @minItems 8
 * @maxItems 8
 */
export type Columns = [Column, Column, Column, Column, Column, Column, Column, Column]
export type Align = ("left" | "right")
export type Id = ("code" | "name" | "role" | "quantity" | "cost" | "price" | "day" | "unrealised")
export type Label1 = string
export type Text2 = string
export type Value = string
export type Text3 = string
export type Unit = "USD"
export type Value1 = number
export type Currency = ("USD" | "JPY")
export type Text4 = string
export type Tone = (("up" | "down" | "flat" | "none") | null)
export type Unit1 = ("usd" | "usd_per_share" | "usd_signed")
export type Value2 = number
export type Day = (MoneyCell | Gap)
export type Price = (MoneyCell | Gap)
export type Text5 = string
export type Unit2 = ("shares" | "contracts")
export type Value3 = number
export type Text6 = ("我" | "配偶")
export type Value4 = ("我" | "配偶")
export type Unrealised = (MoneyCell | Gap)
/**
 * @minItems 0
 * @maxItems 200
 */
export type Rows = PositionRow[]
export type Title1 = string
export type Sentence = string
/**
 * @minItems 6
 * @maxItems 6
 */
export type Columns1 = [TradeColumn, TradeColumn, TradeColumn, TradeColumn, TradeColumn, TradeColumn]
export type Align1 = ("left" | "right")
export type Id1 = ("time" | "account" | "action" | "instrument" | "quantity" | "price")
export type Label2 = string
export type Account = (Cell | Gap)
export type Text7 = string
export type Tone1 = ("up" | "down" | "flat" | "none")
export type Unit3 = (("USD" | "usd_signed" | "percent" | "share_percent" | "index_points" | "percent_of_track" | "vol_points" | "percentage_points") | null)
export type Value5 = (number | null)
export type Action = (Cell | Gap)
export type Instrument = (Cell | Gap)
export type Price1 = (Cell | Gap)
export type Quantity = (Cell | Gap)
export type Time = (Cell | Gap)
/**
 * @minItems 0
 * @maxItems 200
 */
export type Rows1 = TradeRow[]
export type Title2 = string

export interface PositionsPage {
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
stale: StaleLabel
title: Title
views: Views
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
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text1
}
export interface PositionsView {
key: Key
label: Label
options: PositionsTable
sentence: Sentence
stocks: PositionsTable
trades?: (TradesTable | null)
}
export interface PositionsTable {
columns: Columns
gap?: (Gap | null)
rows: Rows
title: Title1
}
export interface Column {
align: Align
id: Id
label: Label1
}
export interface PositionRow {
code: TextCell
cost: MoneyCell
day: Day
name: TextCell
price: Price
quantity: QuantityCell
role: RoleCell
unrealised: Unrealised
}
export interface TextCell {
text: Text2
value: Value
}
/**
 * A money cell with a currency and, for JPY, an optional display-only ``approx_usd``.
 */
export interface MoneyCell {
approx_usd?: (UsdFigure | null)
currency: Currency
text: Text4
tone?: Tone
unit: Unit1
value: Value2
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure {
text: Text3
unit: Unit
value: Value1
}
export interface QuantityCell {
text: Text5
unit: Unit2
value: Value3
}
/**
 * Closed role vocabulary: neither aliases nor account identifiers are accepted.
 */
export interface RoleCell {
text: Text6
value: Value4
}
/**
 * Today's fills (issue #1445), the cells and wording of ``report_actions.TradeRow``.
 */
export interface TradesTable {
columns: Columns1
gap?: (Gap | null)
rows: Rows1
title: Title2
}
export interface TradeColumn {
align: Align1
id: Id1
label: Label2
}
export interface TradeRow {
account: Account
action: Action
instrument: Instrument
price: Price1
quantity: Quantity
time: Time
}
/**
 * One displayed table cell.
 */
export interface Cell {
text: Text7
tone: Tone1
unit?: Unit3
value?: Value5
}
