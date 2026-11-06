// TEMPORARY placeholder, copied from the old repository's generated types until the week1-schemas issue generates them.
/**
 * @minItems 6
 * @maxItems 6
 */
export type Columns = [string, string, string, string, string, string]
export type Meaning = string
export type Text = string
export type Unit = "USD"
export type Value = number
export type Currency = ("USD" | "JPY")
export type Text1 = string
export type Tone = (("up" | "down" | "flat" | "none") | null)
export type Unit1 = ("usd" | "usd_per_share" | "usd_signed")
export type Value1 = number
export type Gap = (string | null)
export type DataDay = (string | null)
export type Name = string
export type Scope = string
export type Whose = string
/**
 * @minItems 0
 * @maxItems 20
 */
export type Rows = AccountRow[]
export type Title = string
export type Reason = ("missing_quote" | "no_data" | "unsupported_currency" | "not_built" | "no_reader" | "no_news_source" | "no_rule" | "market_closed" | "stale" | "flex_time_unknown" | "jp_slice_missing" | "missing_input" | "stale_input")
export type Text2 = string
export type Heads = ([(string | null), (string | null), (string | null), (string | null), (string | null)] | null)
export type Meaning1 = string
export type Text3 = string
export type Tone1 = (("up" | "down" | "flat" | "none") | null)
export type Unit2 = "percentage_points"
export type Value2 = number
export type Name1 = string
export type Text4 = string
export type Unit3 = "share_percent"
export type Value3 = number
export type Text5 = string
export type Unit4 = "share_percent"
export type Value4 = number
/**
 * @minItems 0
 * @maxItems 60
 */
export type Rows1 = AllocationRow[]
export type Title1 = string
/**
 * @maxItems 2
 */
export type Totals = []|[MoneyCell]|[MoneyCell, MoneyCell]
export type BuildId = string
export type ApproxUsdLabel = (string | null)
export type Columns1 = ([string, string, string, string] | null)
export type Heads1 = ([(string | null), (string | null), (string | null)] | null)
export type Meaning2 = string
export type Result = (string | null)
export type Accounts = (AccountTag[] | null)
export type Label = string
export type Whose1 = string
export type Due = (string | null)
export type Items = (IncomeItem[] | null)
export type Item = string
export type Part = string
export type Name2 = string
export type Note = (string | null)
export type OpenPrice = (OpenPrice1 | Gap1 | null)
export type Currency1 = ("USD" | "JPY")
export type Text6 = string
export type Value5 = number
export type Scope1 = (string | null)
export type Summary = (string | null)
/**
 * @minItems 0
 * @maxItems 60
 */
export type Rows2 = MoneyRow[]
export type ScopeGaps = ([]|[ScopeGap]|[ScopeGap, ScopeGap]|[ScopeGap, ScopeGap, ScopeGap]|[ScopeGap, ScopeGap, ScopeGap, ScopeGap] | null)
export type Scope2 = string
export type Title2 = string
export type TotalLabels = ([]|[string]|[string, string] | null)
/**
 * @maxItems 2
 */
export type Totals1 = []|[MoneyCell]|[MoneyCell, MoneyCell]
export type DataTime = string
export type Source = string
/**
 * @maxItems 20
 */
export type DataTimes = SourceTime[]
/**
 * @maxItems 20
 */
export type Gaps = Gap1[]
export type GeneratedAt = string
export type Meaning3 = string
export type Result1 = (string | null)
export type Kind = ("start" | "step" | "end")
export type Name3 = string
export type Sentence = string
/**
 * @minItems 0
 * @maxItems 60
 */
export type Rows3 = WaterfallStep[]
export type Title3 = string
export type Columns2 = ([string, string, string, string] | null)
export type Meaning4 = string
export type Current = (PriceCell | Gap1)
export type Currency2 = ("USD" | "JPY")
export type Text7 = string
export type Value6 = number
export type Name4 = string
export type Note1 = (string | null)
export type Previous = (PriceCell | Gap1)
export type Scope3 = string
/**
 * @minItems 0
 * @maxItems 60
 */
export type Options = MoverRow[]
/**
 * @minItems 0
 * @maxItems 60
 */
export type Rows4 = MoneyRow[]
export type ScopeGaps1 = ([]|[ScopeGap]|[ScopeGap, ScopeGap]|[ScopeGap, ScopeGap, ScopeGap]|[ScopeGap, ScopeGap, ScopeGap, ScopeGap] | null)
/**
 * @minItems 0
 * @maxItems 60
 */
export type Stocks = MoverRow[]
export type Title4 = string
/**
 * @maxItems 2
 */
export type Totals2 = []|[MoneyCell]|[MoneyCell, MoneyCell]
/**
 * @maxItems 20
 */
export type Notes = string[]
export type PositionsLink = (string | null)
export type RiskLink = (string | null)
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "household_page"
export type SchemaVersion = 1
export type Scope4 = string
/**
 * @maxItems 4
 */
export type Scopes = []|[AccountScope]|[AccountScope, AccountScope]|[AccountScope, AccountScope, AccountScope]|[AccountScope, AccountScope, AccountScope, AccountScope]
/**
 * @minItems 1
 * @maxItems 20
 */
export type Accounts1 = [AccountTag, ...(AccountTag)[]]
export type Key = string
export type Label1 = string
export type Sentence1 = string
export type DataTime1 = string
export type StaleAfter = string
export type Text8 = string
export type Title5 = string
export type Text9 = string
export type Tone2 = (("up" | "down" | "flat" | "none") | null)
export type Unit5 = "percent"
export type Value7 = number
export type Meaning5 = string
export type Day = string
/**
 * @minItems 0
 * @maxItems 400
 */
export type Rows5 = TrendPoint[]
/**
 * @maxItems 4
 */
export type Series = []|[TrendSeries]|[TrendSeries, TrendSeries]|[TrendSeries, TrendSeries, TrendSeries]|[TrendSeries, TrendSeries, TrendSeries, TrendSeries]
export type Label2 = string
/**
 * @minItems 0
 * @maxItems 400
 */
export type Rows6 = TrendPoint[]
export type Scope5 = string
export type Window = (string | null)
export type Title6 = string
export type Window1 = (string | null)

export interface HouseholdPage {
accounts?: (AccountsBlock | null)
allocation: AllocationBlock
build_id: BuildId
cash_after: MoneyBlock
composition: MoneyBlock
concentration: MoneyBlock
data_times: DataTimes
gaps: Gaps
generated_at: GeneratedAt
income: MoneyBlock
long_term: MoneyBlock
month_change: WaterfallBlock
movers: MoversBlock
notes: Notes
obligations: MoneyBlock
overview: MoneyBlock
positions_link?: PositionsLink
risk_link?: RiskLink
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope4
scopes: Scopes
sentence: Sentence1
stale: StaleLabel
stress: MoneyBlock
title: Title5
trend: TrendBlock
}
/**
 * The account table: ``columns`` are the heads written by Python, six, in the order of the
 * cells of a row: the name first, 数据日 last. No row is not a gap.
 */
export interface AccountsBlock {
columns: Columns
meaning: Meaning
rows: Rows
title: Title
}
/**
 * The line of a Flex-only member (issue #1588; #1572). Named by a label, never an alias;
 * ``stock`` may read ``至少 …`` when a name of hers is left out; ``data_day`` is her own.
 * ``scope`` is the key of the scope the row sits under and ``whose`` the account's whose word
 * (issue #1680).
 */
export interface AccountRow {
cash: AccountFigure
data_day?: DataDay
name: Name
net_liquidation: AccountFigure
options: AccountFigure
scope: Scope
stock: AccountFigure
whose: Whose
}
/**
 * One cell of the account table: the figure, or None with the words for why it is missing.
 */
export interface AccountFigure {
cell: (MoneyCell | null)
gap?: Gap
}
/**
 * A money cell with a currency and, for JPY, an optional display-only ``approx_usd``.
 */
export interface MoneyCell {
approx_usd?: (UsdFigure | null)
currency: Currency
text: Text1
tone?: Tone
unit: Unit1
value: Value1
}
/**
 * A displayed US-dollar amount (two decimals in ``value``, whole dollars in ``text``).
 */
export interface UsdFigure {
text: Text
unit: Unit
value: Value
}
export interface AllocationBlock {
approx_usd_total?: (UsdFigure | null)
gap?: (Gap1 | null)
heads?: Heads
meaning: Meaning1
rows: Rows1
title: Title1
totals: Totals
}
/**
 * A named data gap, used in place of a figure that cannot be computed. Never a zero.
 */
export interface Gap1 {
reason: Reason
text: Text2
}
export interface AllocationRow {
amount: MoneyCell
difference?: (PercentagePointsFigure | null)
name: Name1
share?: (NetSharePercentFigure | null)
target?: (SharePercentFigure | null)
}
/**
 * A difference of two percents, signed.
 */
export interface PercentagePointsFigure {
text: Text3
tone?: Tone1
unit: Unit2
value: Value2
}
/**
 * A share of the household net liquidation in whole percent (issue #1632): negative for a
 * negative cash, and over 100 is possible, so it is not a ``SharePercentFigure``.
 */
export interface NetSharePercentFigure {
text: Text4
unit: Unit3
value: Value3
}
/**
 * A share in whole percent, 0 to 100.
 */
export interface SharePercentFigure {
text: Text5
unit: Unit4
value: Value4
}
/**
 * Rows of money. ``totals`` has one cell per currency present, ``approx_usd_total`` the
 * display-only total of all rows in ≈USD (None without a JPY row, or without a rate).
 * 
 * ``columns`` are the heads of the table, written by Python: for the obligations block four,
 * the name, the expiry (``MoneyRow.due``), the open price (``MoneyRow.open_price``) and the
 * amount, in that order; None in the other
 * blocks. ``total_labels`` has one label per cell of ``totals``, in the same order, and
 * ``approx_usd_label`` is the label of ``approx_usd_total``; both None where the block has no
 * labels (the table then names a total row by the block's title).
 */
export interface MoneyBlock {
approx_usd_label?: ApproxUsdLabel
approx_usd_total?: (UsdFigure | null)
columns?: Columns1
gap?: (Gap1 | null)
heads?: Heads1
meaning: Meaning2
result?: Result
rows: Rows2
scope_gaps?: ScopeGaps
title: Title2
total_labels?: TotalLabels
totals: Totals1
}
export interface MoneyRow {
accounts?: Accounts
amount: MoneyCell
due?: Due
items?: Items
name: Name2
note?: Note
open_price?: OpenPrice
scope?: Scope1
share?: (SharePercentFigure | null)
summary?: Summary
}
/**
 * One account, named without its alias (issue #1680): ``label`` (``账户（本人）``, ``Flex
 * 账户（配偶）``) and ``whose`` (``本人``, ``配偶``, ``女儿``), both written by Python from the
 * account's group. In the concentration table ``amount`` is what the account holds of the name.
 */
export interface AccountTag {
amount?: (MoneyCell | null)
label: Label
whose: Whose1
}
/**
 * One item of a month of 每月收入 (issue #1682): ``part`` (``期权``, ``股息``, ``利息``), ``item``
 * (``AAA 11-20 P250 平仓``, ``BBB 股息``, ``利息``) and the account's tag, all written by Python.
 */
export interface IncomeItem {
account: AccountTag
amount: MoneyCell
item: Item
part: Part
}
/**
 * The open price per share in the row's own currency, 4 places, credit + (GLOSSARY "Open
 * structure money line"); ``text`` is written by Python (``+1.5000``).
 */
export interface OpenPrice1 {
currency: Currency1
text: Text6
value: Value5
}
/**
 * The one line of a scope that has no rows in a table: why, never a zero.
 */
export interface ScopeGap {
gap: Gap1
scope: Scope2
}
/**
 * The data time of one source behind the card.
 */
export interface SourceTime {
data_time: DataTime
source: Source
}
/**
 * Month-over-month steps in one currency; ``total`` is the closing level.
 */
export interface WaterfallBlock {
gap?: (Gap1 | null)
meaning: Meaning3
result?: Result1
rows: Rows3
title: Title3
total?: (MoneyCell | null)
}
export interface WaterfallStep {
base: MoneyCell
change: (MoneyCell | null)
height: MoneyCell
kind: Kind
level: MoneyCell
name: Name3
sentence: Sentence
}
/**
 * 谁带动了今天 (issue #1681): ``stocks`` and ``options`` apart, each line under its scope; ``rows``
 * are the 其他 line (the part of the household's change that is no listed position), once, in the
 * household scope. ``totals`` is the household scope's change: its lines and the 其他 line, the
 * daughter's lines are in no total. ``scope_gaps`` says in one line that a scope has an account
 * without intraday prices (``无盘中价格（Flex）``).
 */
export interface MoversBlock {
approx_usd_total?: (UsdFigure | null)
columns?: Columns2
gap?: (Gap1 | null)
meaning: Meaning4
options?: Options
rows: Rows4
scope_gaps?: ScopeGaps1
stocks?: Stocks
title: Title4
totals: Totals2
}
/**
 * One line of the 股票 or the 期权 table of 谁带动了今天 (issue #1681). ``name`` is written by
 * Python (``AAA``, ``AAA 11-20 P250``, ``AAA 11-20 P250/240 价差 ×1``); ``previous`` and
 * ``current`` are the marks of the previous snapshot and of the day, or the gap words when one is
 * missing, never 0; ``amount`` is the day's money in USD, None when a price or a rate is missing
 * (``note`` then says which rate). ``scope`` is the key of the scope the line sits under and
 * ``account`` the tag of its account.
 */
export interface MoverRow {
account: AccountTag
amount: (MoneyCell | null)
current: Current
name: Name4
note?: Note1
previous: Previous
scope: Scope3
}
/**
 * A price per share in the row's own currency, 4 places half-even (issue #1681); ``text`` is
 * written by Python (``48.0000``, ``−0.5000``).
 */
export interface PriceCell {
currency: Currency2
text: Text7
value: Value6
}
/**
 * A frame the accounts are counted in (issue #1680): ``household`` (the groups the group file
 * names, me and spouse together) or one scope for each other group (the daughter, apart).
 * Nothing is summed across scopes.
 */
export interface AccountScope {
accounts: Accounts1
key: Key
label: Label1
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text8
}
/**
 * ``rows``, ``change`` and ``gap`` are those of the household scope; ``series`` holds one
 * series for each scope, the household first.
 */
export interface TrendBlock {
change?: (PercentFigure | null)
gap?: (Gap1 | null)
meaning: Meaning5
rows: Rows5
series?: Series
title: Title6
window?: Window1
}
/**
 * A fraction times 100, signed (``+2.1%``).
 */
export interface PercentFigure {
text: Text9
tone?: Tone2
unit: Unit5
value: Value7
}
export interface TrendPoint {
change?: (PercentFigure | null)
day: Day
value: UsdFigure
}
/**
 * The trend of one scope (issue #1680): ``change`` says what it is (``−4.3%：最后一点比第一点``)
 * and ``window`` names the days it runs over, both written by Python.
 */
export interface TrendSeries {
change?: (PercentFigure | null)
gap?: (Gap1 | null)
label: Label2
rows: Rows6
scope: Scope5
window?: Window
}
