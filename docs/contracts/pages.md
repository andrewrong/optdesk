# pages contract

## Responsibility

Build the view model of a page from stored values and serve it inside a static page. `macro_rates.Build` reads the series Reader and emits `page.macro_rates.v3` JSON; `embed.Handler` writes that JSON into the built page at serve time and `embed.Assets` serves the bundle files. Pages make no request: the one place a page reads its data from is `<script id="vm" type="application/json">`.

## Interface

- `pages.Meta{Name, Schema}`: `Schema` is informational; no code reads it and it is not the JSON field `schema`.
- `pages.Builder`: `func(ctx, store.SeriesReader, contracts.Day, generatedAt time.Time) ([]byte, error)`.
- `pages.Page{Meta, Build}`.
- `macro_rates.Page()` returns `Meta{macro_rates, page.macro_rates.v3}` with the package `Build`.
- `embed.Handler(viewModel) http.Handler` and `embed.Assets() http.Handler`: the package embeds `all:dist` (`internal/pages/embed/dist`, written by `pnpm --dir web build`, not committed; only `dist/.gitignore` is). `Handler` writes the view model into `dist/index.html`; `Assets` answers request paths `/assets/*` from `dist/assets`: another path, an unclean path or a missing file is 404, a method other than GET and HEAD is 405. When `index.html` is absent `Handler` answers 500 with the text `web bundle not built: run pnpm --dir web build` (no path in the body).
- `macro_page.NewPage(c)` returns `Meta{macro_page, page.macro_page.v1}`; its `Build` uses a copy of `c` whose `Indicators` is copied at construction.
- `market_page.Build(ctx, c Config, r store.SeriesReader, day, generatedAt)` and `market_page.NewPage(c Config)` (page name `market_page`, schema `page.market_page.v1`): `Config` is `RunID`, `BuildID`, `Scope`, `Zone` (`generated_at` is written in it) and `Readings`, a `compute.ReadingFunc`; a nil `Zone` or `Readings`, a nil series reader, a bad day or id is an error of `Build`, and so is a reader error (never a Gap).
- Zone rule: no pages code names a zone; `macro_page.Config.Zone` is given by the caller.

## Inputs

- A `store.SeriesReader` (`SeriesPoints(ctx, seriesID, from, to)`), nothing else from `internal/store`.
- A `contracts.Day` (`YYYY-MM-DD`) and a `generatedAt` instant given by the caller; the builder reads no clock.
- `obs.series_point r1` rows, read for each of the seven series (`DGS10`, `DGS3MO`, `SP500`, `VIXCLS`, `VXNCLS`, `RVXCLS`, `VXVCLS`, in this order) from the day minus 30 calendar days to the day.
- For the handler, a view model as JSON bytes.

## Outputs

- `macro_rates.Build`: JSON of `contracts/schemas/page.macro_rates.v3.json` (`schema_version` 3; `index_points` text has two decimals and no suffix) (texts in English, written by `text.go`; `latest` is the last point or a gap); `generated_at` is `generatedAt` in UTC, `session_day` is the given day, points are in date order, a nil value is `null`.
- `gaps[]`: a nil value yields one `no_value` gap with the `series_id` and the date of the point. A series whose last point is more than 4 calendar days before the day, or that has no point in range, yields one `stale` gap with the day as `date`; a stale last point with a nil value gives that one gap only.
- `embed.Handler`: `text/html` with the view model in the vm element; `embed.Render` returns the page bytes or an error for a view model that is not JSON (the handler answers 500).
- `report_page.Build(ctx, c Config, day, generatedAt)` and `report_page.NewPage(c Config)` (page name `report_page`, schema `page.report_page.v2`): `Config` is `Market` (`US` or `JP`), `Slot` (`open` or `close`), `Zone` (`generated_at` is written in it) and `Risks`, a `RiskBlockFunc` (`func(ctx, day, generatedAt) ([]byte, error)`). The `risks` block is the bytes `Risks` returns, embedded unchanged; an error of `Risks`, a market or slot outside the lists, or a nil `Zone` or `Risks` is an error of `Build`. The other four blocks (`actions`, `account_moves`, `market_watch`, `global_assets`) are gap-only: state `gap`, one `Gap` of reason `not_built`, each required figure field a `Gap` of reason `not_built`. No optional block is written. The package computes no figure; only `ot` imports it.
- `market_page.Build` output: the weather headline and sentence from `market.sp500_vs_200d` (signed `percent`, `+3.12%`, half-even); `rank_3y` of SPX, NDX and RUT from the three percentile readings, unit `share_percent` (whole percent, `68%`, JSON number from the exact decimal text; N225 is a `no_data` Gap); the term cells from stored `VIXCLS` and `VXVCLS` points, unit `index_points` (two decimals; the ratio of `market.vix_term_ratio` in the term sentence has unit `ratio`, two decimals). `data_times` has one entry per series, `stale` is the oldest data day, `run_status` is `ok` or `partial` (never `failed`; permanent Gaps do not count) and the envelope `gaps` copy the Gap blocks in page order. The Gap blocks `weather.chart`, `breadth` and the `iv`, `rv20`, `iv_minus_rv20` cells are `not_built`. Numbers are shifted and rounded half-even as decimal text (`math/big`), never as floats.

## Invariants

- The builder only formats stored values; it computes nothing, and a missing value is a gap, never 0.
- Staleness is date arithmetic on the two `YYYY-MM-DD` values; no calendar. The freshness limit (4) and the lookback (30) are package constants.
- The injected JSON is compacted and `<`, `>`, `&`, U+2028 and U+2029 are written as `\u` escapes, so a string value cannot close the script element; parsed, it equals the input.
- `dist/index.html` holds the vm element exactly once.

## Version history

- v1: `page.macro_rates.v1` builder and the embed injector.
- v2: `Meta`, `Builder` and `Page` names; `macro_rates.Page` and `macro_page.NewPage`.
- v4: `macro_rates` writes `page.macro_rates.v3` for seven series; `embed` embeds the built bundle (`all:dist`), adds `Assets` and the 500 text; the stub page is gone.
- v3: `internal/pages/option_data_page`: `NewPage(Config)` and `Build` for `page.option_data_page.v1`; `Config` is `RunID`, `BuildID`, `Scope`, `Zone`, `UnderlyingID`, `Label`, `Eod` (`store.OptionEodReader`) and `Quotes` (`store.QuoteSnapshotReader`) and `Notes` (texts of the page notes, at most 20); the series reader the caller passes is ignored. v3 also fills three fields: `Choices` (`[]Choice`, the first 50 become `underlyings`; a `Href` outside `^/[A-Za-z0-9/_-]{1,100}$` is an error of `Build`), `Terms` (`store.OptionTermsReader`, one call with the kept rows' ids; fills `expiry`, `right` and `strike`; a reader error is returned) and `Symbol` (`underlying.symbol`); zero values keep the output unchanged.
- v5: `internal/pages/report_page`: `Build` and `NewPage` for `page.report_page.v2`, with `Config` and `RiskBlockFunc`; the `risks` block is given bytes and four blocks are gap-only.
- v6: `internal/pages/market_page`: `Build`, `NewPage` and `Config` write `page.market_page.v1` from stored readings and series points.
- v7: `internal/pages/option_candidates_page`: `Build` writes `page.option_candidates_page.v1` from `Config.Candidates` (`store.CandidateReader`), once per kind: the build day, else the last day with rows up to 7 days back (a group day before the build day is stale and its rows carry the badge `过期`); `ok` rows go to the main table and `blocked` and `unknown` rows to the second table with their reasons, in the stored order; numeric cells carry `value` as the stored decimal text (`json.Number`); the yield text is shifted two places as text; a kind without rows is the group gap `no_rows`, no stored run the page gap `no_run`, a nil reader `no_reader`. No float, sort, clock or notify.
- v8: `internal/pages/option_notices_page`: `Build` writes `page.option_notices_page.v1` from `Config.Account` (`store.AccountReader`), `Config.Candidates` and `Config.Earnings` through `Config.Rule`: per alias (`Main` first, then `Aliases`) the latest day with positions up to 7 days back and the one before it, the `held_call_guard` rows of that day and the next listed earnings date of each held option symbol, converted to `compute.NoticeInput` and evaluated with `Config.Params`; one block per alias in the rule's order (status `ok`, `stale` when the data day is older than the build day, or `no_capture` with one note); a missing capture of `Main` is the page gap `no_capture`, a nil reader the gap `no_reader`; a nil `Zone` or `Rule` is an error. Amounts are decimal text with the currency, the cushion text is `Cushion ` plus the rule's text. No float, sort, clock or notify.

## Contract tests

- `internal/pages/api_test.go`: the interface from both sides.
- `internal/pages/api_test.go`: `TestPagesPageContract`.
- `internal/pages/macro_rates/page_test.go`: `TestPage`.
- `internal/pages/macro_page/page_test.go`: `TestNewPage`, `TestNewPageCopiesConfig`.
- `internal/pages/market_page/page_test.go`: `TestNewPageMeta`, `TestConfigContract`.
- `internal/pages/option_candidates_page/page_test.go`: `TestNewPageMeta`, `TestConfigContract`.
- `internal/pages/option_notices_page/page_test.go`: `TestNewPageMeta`, `TestConfigContract`.
- `internal/pages/option_data_page/page_test.go`: `TestNewPage`, `TestNewPageCopiesConfig`, `TestNewPageCopiesNotes`.
- `internal/pages/option_data_page/builder_test.go`: `TestBuildValidates`, `TestBuildGolden`, `TestBuildSelection`, `TestBuildGaps`.
- `internal/pages/option_data_page/choices_test.go`: `TestBuildChoicesGolden` (new), `TestBuildNoNewFields` (new), `TestBuildChoices` (new).
- `internal/pages/option_data_page/terms_test.go`: `TestBuildTermsGolden` (new), `TestBuildTerms` (new).
- `internal/pages/macro_rates/builder_test.go`: `TestBuildGolden`, `TestText`, `TestGapsAndStale`, `TestBuildFixtures`, `TestBuildDemo`, `TestBuildSevenSeries`, `TestBuildErrors`.
- `internal/pages/embed/inject_test.go`: `TestInjectEscapes`, `TestRenderRejectsBadJSON`, `TestHandlerServesIndex`, `TestHandlerBadViewModel`, `TestHandlerNoBundle500`.
- `internal/pages/embed/assets_test.go`: `TestAssets`.

## Jobs and slots

None. The builder and the handler run in the commands that call them (`ot page`, `ot serve`, a follow-up design); they own no slot.

## Forbidden

- Importing pgx or a store implementation package from `internal/pages`.
- Computing a value, reading a clock or making a request in a page builder.
- Writing the view model into the page anywhere but the vm element, or without escaping.
- Editing `contracts/schemas` or `contracts/fixtures` from this package.
