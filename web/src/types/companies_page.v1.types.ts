// TEMPORARY hand-written placeholder (not generated; web/src/generated stays the stub). Types copied from the old repository until the week1-schemas issue generates them.
export type BuildId = string
/**
 * @maxItems 100
 */
export type Lines = string[]
export type Note = (string | null)
export type Sentence = (string | null)
export type Title = string
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
export type Note1 = (string | null)
/**
 * @minItems 1
 * @maxItems 8
 */
export type Tiles = [Tile]|[Tile, Tile]|[Tile, Tile, Tile]|[Tile, Tile, Tile, Tile]|[Tile, Tile, Tile, Tile, Tile]|[Tile, Tile, Tile, Tile, Tile, Tile]|[Tile, Tile, Tile, Tile, Tile, Tile, Tile]|[Tile, Tile, Tile, Tile, Tile, Tile, Tile, Tile]
export type Label = string
export type Text1 = string
export type Title1 = string
export type DataDay = string
export type HeldCount = (number | null)
export type Line = (string | null)
export type Title2 = string
/**
 * @minItems 1
 * @maxItems 12
 */
export type Columns = [Column, ...(Column)[]]
export type Numeric = boolean
export type Text2 = string
/**
 * @minItems 1
 * @maxItems 12
 */
export type Cells = [Cell, ...(Cell)[]]
export type Text3 = string
export type Tone = ("up" | "down" | "flat" | "none")
/**
 * @minItems 1
 * @maxItems 20
 */
export type Moat = [string, ...(string)[]]
/**
 * @maxItems 200
 */
export type Rows = HoldingRow[]
export type VolatilityNote = (string | null)
/**
 * @maxItems 20
 */
export type Notes = string[]
export type RunId = string
export type RunStatus = ("ok" | "partial" | "failed")
export type Schema = "companies_page"
export type SchemaVersion = 1
export type Scope = string
export type DataTime1 = string
export type StaleAfter = string
export type Text4 = string
export type Title3 = string
export type Unvalued = (string | null)
/**
 * @maxItems 12
 */
export type Columns1 = Column[]
/**
 * @minItems 1
 * @maxItems 12
 */
export type Cells1 = [Cell, ...(Cell)[]]
/**
 * @maxItems 200
 */
export type Rows1 = WatchRow[]
export type Sentence1 = (string | null)
export type Title4 = string

export interface CompaniesPage {
build_id: BuildId
changes: (ChangesBlock | null)
data_times: DataTimes
gaps: Gaps
generated_at: GeneratedAt
glance: (GlanceBlock | null)
header: Header
holdings: (HoldingsBlock | null)
notes: Notes
run_id: RunId
run_status: RunStatus
schema: Schema
schema_version: SchemaVersion
scope: Scope
stale: StaleLabel
title: Title3
unvalued: Unvalued
watch_list: WatchBlock
}
/**
 * ``lines`` are the changes; ``sentence`` is the one state sentence (近 7 天没有变化, or the
 * reading is not usable); ``note`` is ``数据日 月-日`` when the reading is of an earlier day.
 */
export interface ChangesBlock {
lines: Lines
note?: Note
sentence?: Sentence
title: Title
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
 * ``note`` is the price-lines state sentence, present only when the lines are not usable.
 */
export interface GlanceBlock {
note?: Note1
tiles: Tiles
title: Title1
}
export interface Tile {
label: Label
text: Text1
}
/**
 * ``line`` is ``月-日 · 家庭持有 N 家``; ``held_count`` is N, None without a held reading.
 */
export interface Header {
data_day: DataDay
held_count?: HeldCount
line?: Line
title: Title2
}
/**
 * ``volatility_note`` is the label line of the 波动 column (the own series and the data day);
 * absent on a payload made before the column.
 */
export interface HoldingsBlock {
columns: Columns
rows: Rows
volatility_note?: VolatilityNote
}
/**
 * A column head; ``numeric`` columns are right-aligned by the browser.
 */
export interface Column {
numeric: Numeric
text: Text2
}
/**
 * One held company: a cell per column, and its moat lines (the owner's record as written, or
 * ``护城河记录未填``).
 */
export interface HoldingRow {
cells: Cells
moat: Moat
}
export interface Cell {
text: Text3
tone?: Tone
}
/**
 * Python sends the times and the label; the browser only compares its clock with ``stale_after``.
 */
export interface StaleLabel {
data_time: DataTime1
stale_after: StaleAfter
text: Text4
}
/**
 * Rows, or one state sentence naming the file (not filled / unreadable) and no rows.
 */
export interface WatchBlock {
columns: Columns1
rows: Rows1
sentence?: Sentence1
title: Title4
}
export interface WatchRow {
cells: Cells1
}
