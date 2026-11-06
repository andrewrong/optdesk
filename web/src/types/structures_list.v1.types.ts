// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
export type BuildId = string
export type DataTime = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
export type GeneratedAt = string
/**
 * @minItems 2
 * @maxItems 2
 */
export type Groups = [StructureGroup, StructureGroup]
export type CapturedParam = string
export type CashAfter = (string | Gap)
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text = string
export type CloseCost = (string | Gap)
export type ExitDteParam = string
export type RecentCommission = (string | Gap)
export type RollNet = (string | Gap)
export type Sentence = (string | null)
export type Distance = (string | Gap)
export type Earned = (string | Gap)
export type ExDividend = (string | Gap | null)
export type ExpiryText = string
export type Href = string
export type KindText = string
export type MaxGain = (Usd | Gap)
export type Text1 = string
export type Unit = "usd"
export type Value = number
export type MaxLoss = (Usd | Gap)
export type Moneyness = (string | Gap)
export type Pnl = (Usd | Gap)
export type Symbol = string
/**
 * @maxItems 200
 */
export type Rows = StructureRow[]
export type Title = string
export type Note = (string | null)
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "structures_list"
export type SchemaVersion = 1
export type Scope = string
export type Sentence1 = string
export type DataTime1 = string
export type StaleAfter = string
export type Text2 = string
export type Title1 = string

export interface StructuresList {
build_id: BuildId
data_times: DataTimes
generated_at: GeneratedAt
groups: Groups
note?: Note
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope
sentence: Sentence1
stale: StaleLabel
title: Title1
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
data_time: DataTime
source: Source
}
/**
 * One card of the page: its title and its rows in order.
 */
export interface StructureGroup {
rows: Rows
title: Title
}
/**
 * One structure. ``symbol`` is the first cell and leads to ``href``, the detail route of its key.
 */
export interface StructureRow {
compare: (Compare | null)
distance: Distance
earned: Earned
ex_dividend: ExDividend
expiry_text: ExpiryText
href: Href
kind_text: KindText
max_gain: MaxGain
max_loss: MaxLoss
moneyness: Moneyness
pnl: Pnl
symbol: Symbol
}
/**
 * What the roll decision needs, as ready texts (gross: no fee is deducted). ``cash_after`` is the
 * account's cash less all its puts expiring on or before this structure's expiry, not this structure
 * alone. ``sentence`` is facts beside the two owner values (``captured_param``, ``exit_dte_param``),
 * ``None`` when both are set and neither is reached; it never tells the owner to act.
 */
export interface Compare {
captured_param: CapturedParam
cash_after: CashAfter
close_cost: CloseCost
exit_dte_param: ExitDteParam
recent_commission: RecentCommission
roll_net: RollNet
sentence: Sentence
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap {
reason: Reason
text: Text
}
export interface Usd {
text: Text1
unit: Unit
value: Value
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text2
}
