# pages contract

## Responsibility

Build the view model of a page from stored values and serve it inside a static page. `macro_rates.Build` reads the series Reader and emits `page.macro_rates.v1` JSON; `embed.Handler` writes that JSON into the page at serve time. Pages make no request: the one place a page reads its data from is `<script id="vm" type="application/json">`.

## Interface

- `pages.Meta{Name, Schema}`: `Schema` is informational; no code reads it and it is not the JSON field `schema`.
- `pages.Builder`: `func(ctx, store.SeriesReader, contracts.Day, generatedAt time.Time) ([]byte, error)`.
- `pages.Page{Meta, Build}`.
- `macro_rates.Page()` returns `Meta{macro_rates, page.macro_rates.v1}` with the package `Build`.
- `macro_page.NewPage(c)` returns `Meta{macro_page, page.macro_page.v1}`; its `Build` uses a copy of `c` whose `Indicators` is copied at construction.
- Zone rule: no pages code names a zone; `macro_page.Config.Zone` is given by the caller.

## Inputs

- A `store.SeriesReader` (`SeriesPoints(ctx, seriesID, from, to)`), nothing else from `internal/store`.
- A `contracts.Day` (`YYYY-MM-DD`) and a `generatedAt` instant given by the caller; the builder reads no clock.
- `obs.series_point r1` rows, read for each listed series (`DGS10` in week 1) from the day minus 30 calendar days to the day.
- For the handler, a view model as JSON bytes.

## Outputs

- `macro_rates.Build`: JSON of `contracts/schemas/page.macro_rates.v1.json`; `generated_at` is `generatedAt` in UTC, `session_day` is the given day, points are in date order, a nil value is `null`.
- `gaps[]`: a nil value yields one `no_value` gap with the `series_id` and the date of the point. A series whose last point is more than 4 calendar days before the day, or that has no point in range, yields one `stale` gap with the day as `date`; a stale last point with a nil value gives that one gap only.
- `embed.Handler`: `text/html` with the view model in the vm element; `embed.Render` returns the page bytes or an error for a view model that is not JSON (the handler answers 500).

## Invariants

- The builder only formats stored values; it computes nothing, and a missing value is a gap, never 0.
- Staleness is date arithmetic on the two `YYYY-MM-DD` values; no calendar. The freshness limit (4) and the lookback (30) are package constants.
- The injected JSON is compacted and `<`, `>`, `&`, U+2028 and U+2029 are written as `\u` escapes, so a string value cannot close the script element; parsed, it equals the input.
- The page stub holds the vm element exactly once.

## Version history

- v1: `page.macro_rates.v1` builder and the embed injector.
- v2: `Meta`, `Builder` and `Page` names; `macro_rates.Page` and `macro_page.NewPage`.
- v3: `internal/pages/option_data_page`: `NewPage(Config)` and `Build` for `page.option_data_page.v1`; `Config` is `RunID`, `BuildID`, `Scope`, `Zone`, `UnderlyingID`, `Label`, `Eod` (`store.OptionEodReader`) and `Quotes` (`store.QuoteSnapshotReader`); the series reader the caller passes is ignored.

## Contract tests

- `internal/pages/api_test.go`: the interface from both sides.
- `internal/pages/api_test.go`: `TestPagesPageContract`.
- `internal/pages/macro_rates/page_test.go`: `TestPage`.
- `internal/pages/macro_page/page_test.go`: `TestNewPage`, `TestNewPageCopiesConfig`.
- `internal/pages/option_data_page/page_test.go`: `TestNewPage`, `TestNewPageCopiesConfig`.
- `internal/pages/option_data_page/builder_test.go`: `TestBuildValidates`, `TestBuildGolden`, `TestBuildSelection`, `TestBuildGaps`.
- `internal/pages/macro_rates/builder_test.go`: `TestBuildGolden`, `TestGapsAndStale`, `TestBuildDemo`, `TestBuildErrors`.
- `internal/pages/embed/inject_test.go`: `TestInjectEscapes`, `TestHandlerServesStub`, `TestRenderRejectsBadJSON`, `TestHandlerBadViewModel`.

## Jobs and slots

None. The builder and the handler run in the commands that call them (`ot page`, `ot serve`, a follow-up design); they own no slot.

## Forbidden

- Importing pgx or a store implementation package from `internal/pages`.
- Computing a value, reading a clock or making a request in a page builder.
- Writing the view model into the page anywhere but the vm element, or without escaping.
- Editing `contracts/schemas` or `contracts/fixtures` from this package.
