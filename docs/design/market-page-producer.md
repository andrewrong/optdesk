# Market page producer: `ot` serves `market_page` v1 per request, with the 3-year percentiles of VIX, VXN and RVX

Author: the optdesk designer (first draft; it replaces the shelved `index-iv` draft, `design/index-iv` commit fc00d1c, and answers its adversarial review)
Owner approved: (pending)
Revision: 1

Source: the owner's decisions of 2026-11-08: index implied-volatility history and percentiles come from FRED `VIXCLS`, `VXNCLS`, `RVXCLS` (review answer 3 a); the IBKR live index IV comes later (left out here); reports and pages are computed per request, no stored file (2b); the front end renders data and computes nothing. This is the first design of the builders wave: the pages that have a schema and a web view and no Go builder.
Read on main at eb7321d: `AGENTS.md`, `docs/design/modules.md`, `missing-pages.md` (the schema and the view), `compute-figures.md`, `risks_block` and `macro_page` (`internal/pages/`), `contracts/schemas/page.market_page.v1.json` and `common.v2.json`, `web/src/pages/market/MarketPage.tsx` and `web/test/pages/market/fixtures/`, `docs/conventions/register.toml` (the five market figures), `internal/jobs/computefigures/` (`job.toml`, `job.go`, `steps.go`), `internal/compute/api.go` (`ReadingFunc`), `internal/contracts/gen/records.go` (`ReadingRecord`), `internal/store/api.go` (`SeriesReader`), `scripts/arch`, `cmd/ot/site.go`.
Read on branches, not merged: `docs/design/report-page-producer.md` (`origin/design/report-page-producer`, pull request 500: the reader of readings for the page role), `docs/design/unified-site.md` (`origin/design/unified-site-3`, pull request 533: how `ot serve` serves pages), `docs/design/ease-v2.md` (pull request 560: shares `register.toml`).
Also read: the shelved `index-iv.md` and `index-iv-adversarial.md` of the scratchpad.

Goal: `/market` of `ot serve` and `ot page --name market_page` give a `market_page` v1 built per request from stored data. The index table shows, for SPX, NDX and RUT, the 3-year percentile of VIX, VXN and RVX; the term block shows VIX and VIX3M today and five days ago; the weather block says how far the SPX is from its 200-day line. Every other field is a named Gap.

## What main holds today

- Schema and view exist: `page.market_page.v1.json` (blocks `weather`, `indices`, `term`, `breadth`, plus the envelope), `web/src/pages/market/MarketPage.tsx` registered as `market_page` with three fixtures (`ok`, `gap`, `stale`). No Go code writes the page and no route serves it (`missing-pages.md`: "The page has no route").
- The stored data that matches the page. `compute_figures` (`internal/jobs/computefigures/job.toml`) stores, as readings of kind `market` with the FRED id (the first series of its manifest entry) as subject: `market.vix_pctile_3y` (VIXCLS), `market.vxn_pctile_3y` (VXNCLS), `market.rvx_pctile_3y` (RVXCLS), `market.sp500_pctile_3y`, `market.sp500_vs_200d` (version 2, SP500) and `market.vix_term_ratio` (VIXCLS over VXVCLS). Each is `pages = []` in `register.toml` ("no page reads it yet"). The FRED series `VIXCLS`, `VXNCLS`, `RVXCLS`, `VXVCLS`, `SP500` are stored by `fredseries` and readable through `store.SeriesReader`.
- A reading has no field for its data day. `ReadingRecord` holds `day` (the as-of session day of the run), `value` (decimal text or null), `state` (`ok` or `missing_input`), `reason_code` (a number, 1 to 5) and `parameters`. `compute_figures` writes the parameter `data_day` (a text tag, the day of the newest input point) whenever the figure has one (`steps.go`, `parameters`). The reason codes are numbers, not words: 1 `no_input`, 2 `short_history`, 3 `stale_input`, 4 `zero_range`, 5 `bad_input` (`steps.go`, `reasonCode`).
- No code of the page role can read a reading today (`ot_ro_pages` has `SELECT` on the view `rd.reading_r1` and nothing on the tables). Pull request 500 designs `store.ReadingViewReader` (`WinnerReadings`, a `compute.ReadingFunc` as a method value; one row per day, parameters filled) and its implementation entry `report-reading-view-impl`. This design reads through it and adds no reader of its own.
- Pull request 533 changes `ot serve`: one site command, no `--name` or `--site`, a route table in `cmd/ot/site.go`, stores opened once with `OT_PG_DSN_PAGES`. The market page is not an item of its bar.
- The Cboe indices are not at-the-money IV30 (glossary line 75), so the percentile is a different number from `market.ivp_own`. The percentile rule (`pctile_3y_v1`: 3 calendar years, strictly below, the day excluded, `short_history` until the stored history spans 3 years) is registered and hashed; nothing here changes it.

## Decisions

1. One new package, `internal/pages/market_page`, shaped like `report_page` of pull request 500 and `macro_page`: `Build(ctx, c Config, r store.SeriesReader, day, generatedAt) ([]byte, error)` and `NewPage(c Config) pages.Page` (`Meta{Name: "market_page", Schema: "page.market_page.v1"}`). The series reader is the argument of `pages.Builder` as for `macro_page`; the readings come from `Config.Readings`, a `compute.ReadingFunc`, so the package imports `contracts`, `store`, `compute` (the root) and `pages` only. `internal/pages/api.go` is not edited. Cells are built in this package, not in a second block package: the review of the shelved draft refused a package that nothing calls, and the cells ship with their caller. No new module row; the sentence of `modules.md` that lists the implementation packages `cmd/ot` imports gains `internal/pages/market_page` and `arch.toml` is regenerated, both in the pull request of this design (`go run ./scripts/arch --check` prints `arch: ok` with them). `.go-arch-lint.yml` reads the tree, so it can be regenerated only once the directory exists: the interface entry does it (Issues).
2. Per request, stored nowhere, no job and no schedule (owner 2b). Freshness reaches the page through `data_times`, `stale` and the Gaps. Run status is `partial` while any block is a Gap and `ok` when none is; it is never `failed`: a reader error is an error of the call (`ot` answers 500 with a fixed text), never a Gap.
3. Where each field comes from (the list the producer must fill):

   | field | source | state |
   |---|---|---|
   | envelope: `schema`, `schema_version` 1, `run_id`, `build_id`, `scope`, `generated_at` (the owner's zone, `Config.Zone`, as `macro_page`), `run_status`, `title`, `sentence`, `notes`, `gaps` | constants of `strings.go` and `Config` | filled |
   | `data_times` | one `SourceTime` per source that has a data day: the series ids `VIXCLS`, `VXNCLS`, `RVXCLS`, `VXVCLS`, `SP500` (midnight in the zone) | filled |
   | `stale`, `stale_after` | the oldest data day of the page, `is_stale` when it is more than 4 days before the request day (the limit of `macro_page`), text `数据日 MM-DD` | filled |
   | `weather.headline`, `weather.sentence` | reading `market.sp500_vs_200d` (SP500): a fixed headline and the signed percent text | filled; Gap text when no reading |
   | `weather.chart` | needs a stored 200-day line per day: none (`market.spx_close` is registered, not computed) | Gap `not_built` |
   | `indices.rows[]` SPX, NDX, RUT: `rank_3y` | readings `market.vix_pctile_3y` (VIXCLS), `market.vxn_pctile_3y` (VXNCLS), `market.rvx_pctile_3y` (RVXCLS) | filled; Gap by the reading's state |
   | `indices.rows[]` N225: `rank_3y` | no FRED series for the Nikkei volatility | Gap `no_data` |
   | `indices.rows[]` all four: `iv` | `market.index_iv` is the IBKR live value, no stored reading | Gap `not_built` |
   | `indices.rows[]` all four: `rv20`, `iv_minus_rv20` | no figure of a 20-day realised volatility of an index exists | Gap `not_built` |
   | `indices.title`, `sentence`, `columns`, `note` | constants | filled |
   | `term.points[]` `1M` (VIXCLS) and `3M` (VXVCLS): `today`, `five_days_ago` | the stored series points (Decision 6) | filled; Gap per cell |
   | `term.sentence` | reading `market.vix_term_ratio` (VIXCLS) | filled; fixed text when no reading |
   | `breadth` | no figure of the share of stocks above a moving average, and no constituent data | Gap `not_built` |

   Nothing in this table is computed by the page. Every number is a stored reading or a stored point; the page shifts a decimal text, rounds it and writes it (Decision 4), and picks a stored point by its date (Decision 6).
4. Number text and unit, by string arithmetic. A stored reading is decimal text (`0.685000`). The percentile cell is a share: unit `share_percent` of `common.v2.json` (`SharePercentFigure`: whole percent, 0 to 100, `ROUND_HALF_EVEN`), so stored `0.685000` is value `68`, text `68%`. The review of the shelved draft found `percent` wrong: in `common.v2.json` `percent` is a signed change (`+2.1%`) and the view would read a percentile as one. The weather figure is the signed change: stored ratio `0.031250` is unit `percent`, text `+3.12%` (two decimals, half-even). The term ratio is unit `ratio`, text with two decimals (`0.90`); a series point is unit `index_points`, text with two decimals. One helper, `shiftRound(text string, shift, decimals int) (string, error)`, uses `math/big` (exact decimal, no float): it shifts the text by `shift` places and rounds half-even to `decimals` places. The JSON number of a `Figure.value` is written as `json.Number` of that same text, so no `float64` appears in the package except where a series point is formatted (`strconv.FormatFloat(v, 'f', 2, 64)`, in `format.go` only). A text that is not a decimal, or a percentile outside 0 to 1, is a `missing_input` Gap (`bad_input` of the reading), never a number.
5. Readings and the data day. The package asks `Readings` for each figure over the 6 days `day-5` to `day` and takes the newest day returned (the window of 492 `Evaluate`). The row is one of four cases, by `state` and the numeric `reason_code`:
   - `ok` with a value: the figure.
   - `missing_input`, code 2 (`short_history`): Gap `no_data`, text `历史不满 3 年，不给分位` (the glossary words).
   - `missing_input`, code 3 (`stale_input`): Gap `stale_input`, text `分位数据已过期，数据日 MM-DD` (the day is the `data_day` below; without one, `分位数据已过期`).
   - `missing_input`, any other code (1, 4, 5) or an `ok` row without a value: Gap `missing_input`, text `分位暂缺：输入不全`.
   - no row: Gap `no_data`, text `分位暂无数据：还没有存储`.
   The data day of a reading is the value of its parameter `data_day` (a `value_tag` of the form `YYYY-MM-DD`), never its `day` (the as-of day of the run, which can be later than the newest input). A reading with no such parameter or a bad one has no data day: the figure is still shown, its source is left out of `data_times`, and it does not enter `stale`. The shelved draft read "the day of the newest point" from a field that does not exist; this is the one it can use. The wording of the Gaps is constants of `strings.go`.
6. Term block. `1M` is `VIXCLS` and `3M` is `VXVCLS`, stored series (`seriesUnits` `index_points`). For each, the builder reads `SeriesPoints` for the 40 days before the request day, keeps one value per observation date (the greatest `RealtimeStart` not after the day, as `macro_page`'s `latestPoint`), and picks: `today` is the newest stored point on or before the day, if it is at most 4 days old, else Gap `stale`; `five_days_ago` is the newest stored point on or before the date 5 calendar days before the `today` point, if it is at most 4 days older than that date, else Gap `no_data`. A series with no point at all gives both cells a Gap `no_data`. If all four cells are Gaps the whole `term` is one Gap `no_data`. Picking a stored point by a date is selection, as `macro_page` does; nothing is summed or compared.
7. Reading as the page role. `ot` passes `WinnerReadings` of the reader of pull request 500 as `Config.Readings`, and the series reader it already opens. No new `.env` line (`OT_PG_DSN_PAGES` only), no new grant, no migration. This design depends on `report-reading-view-impl` of pull request 500 and does not wait for its `report-page-ot`.
8. Serving. `ot page --name market_page` prints the page; `/market` of the site of pull request 533 serves it, built per request, the stores opened once for the process as 533 Decision 8 says. The market page is not added to the top bar here (Left out). The demo profile opens no store: a demo reading function returns fixed synthetic readings with the parameter `data_day`.
9. History is the owner's act, owned by a runbook item. The percentiles show a value only when the stored FRED history of the three index series spans 3 years before the day. That history is loaded by the owner with `ot series-load --series VIXCLS --series VXNCLS --series RVXCLS --from DAY --to DAY` (exists on main; the loader refuses a range over 5 years, so one run covers 3 years and a margin; `--from` is at least 3 years and 10 days before the first day a percentile is wanted, because the first stored point must be on or before day minus 3 years and that date may be a weekend); the next `compute_figures` run stores the readings (it computes the as-of day only). Until then the page says `历史不满 3 年，不给分位`, which is true. The runbook is a file of the docs entry of this design; the owner's run is not an agent step (the window runs `ot series-load` only with the owner's word).
10. The register. The `pages` line of a figure names a page that reads it; it is set only by the issue that makes a page read it. The docs entry therefore comes last: it is cut after `market-page-ot` has merged, and sets `pages` of the five figures this page reads (`market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y`, `market.vix_term_ratio`, and `market.sp500_vs_200d` version 2) to the page paths, with one `history` line each and no change of `version`, `formula` or `source_hash` (a changed formula would be a new version and a new hash; this is not one). `market.sp500_pctile_3y` stays `pages = []` (no row reads it).
11. Libraries. None new: `encoding/json`, `math/big` and `santhosh-tekuri/jsonschema/v6` (already in `go.mod`; it validates the output against the schema in the tests). Own code only where no library fits (a decimal shift and half-even rounding of a decimal text: `math/big` gives exact arithmetic, and no mainstream library does this one step more simply). The shift-and-round helper is own glue in one file (`shift.go`) so a library can replace it.

## Modules

`pages` owns the package and `docs/contracts/pages.md`; `ot` owns `cmd/ot`; `docs` owns `docs/conventions/register.toml` and `docs/deploy/`. `docs/design/modules.md` is edited by this pull request (one sentence, the list of packages `cmd/ot` may import) and `arch.toml` is regenerated by `go run ./scripts/arch`. The `pages` row keeps its depends cell (`contracts store compute`): `market_page` imports `contracts`, `store`, `compute` and `pages`.

| issue | module | paths |
|---|---|---|
| market-page-interface (interface) | pages, plus the file `.go-arch-lint.yml` of repo (the interface entry: the one licensed cross-module entry) | `internal/pages/market_page/api.go`, `internal/pages/market_page/api_test.go`, `.go-arch-lint.yml` |
| market-page-builder | pages | `internal/pages/market_page/**` (except the two files of the interface entry), `docs/contracts/pages.md` |
| market-page-ot | ot | `cmd/ot/market.go`, `cmd/ot/market_test.go`, `cmd/ot/page.go`, `cmd/ot/page_test.go`, `cmd/ot/site.go`, `cmd/ot/site_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/demo.go`, `docs/contracts/ot.md` |
| market-page-docs | docs | `docs/conventions/register.toml`, `docs/deploy/index-iv-history.md` |

## Interface

Owner `pages`, file `internal/pages/market_page/api.go` (declarations only; a new file of a new package). It carries the contract between `ot` (which fills `Config` and calls the constructor) and the builder, and the figure ids and series ids both sides use, so the builder and the register issue cannot disagree on a name.

```go
// Package market_page builds page.market_page.v1 from stored readings and series (declarations only).
package market_page

const (
	PageName   = "market_page"
	PageSchema = "page.market_page.v1"

	FigureVixPctile   = "market.vix_pctile_3y"
	FigureVxnPctile   = "market.vxn_pctile_3y"
	FigureRvxPctile   = "market.rvx_pctile_3y"
	FigureSpxVs200d   = "market.sp500_vs_200d"
	FigureVixTermRatio = "market.vix_term_ratio"

	SeriesVix, SeriesVxn, SeriesRvx, SeriesVix3m, SeriesSpx = "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS", "SP500"
)

// Config is one run of the page: its identity, the owner's zone (generated_at is written in it) and the readings.
// Readings is the method value WinnerReadings of the reader of the page role; it applies the winner rule of the report page.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location
	Readings              compute.ReadingFunc
}

// NewPageFunc is the constructor `ot` calls: the page it returns builds per request from the series reader it is given and from `Config.Readings`.
// A reader error is an error of Build, never a Gap; a nil Zone or nil Readings is an error of Build.
type NewPageFunc func(c Config) pages.Page
```

`api_test.go` tests the contract from both sides: a fake `NewPageFunc` (one side) and a caller that fills `Config` with a fake `ReadingFunc` and builds (the other); and that the constants equal the ids in `internal/compute/rules/rules.json` and `internal/jobs/computefigures/job.toml` (read as files). The schema is not changed: `Contracts: unchanged`. `internal/pages/api.go` is not edited.

## Issues

Every test calls `t.Parallel()`, uses `t.TempDir()` for any file, a fake clock where time matters (the builder reads no clock: `generatedAt` is an argument), no sleep, no fixed port, no live database. A pull request of each entry carries a `Contracts: unchanged` line from the day it is opened.

### market-page-interface (interface)

- Module: pages, plus the file `.go-arch-lint.yml` of repo. This is the interface entry: the one licensed cross-module entry, because the component of the new directory cannot be written before the directory exists (`scripts/arch` reads the tree for `.go-arch-lint.yml`). Size: S. Depends on: none.
- Order: first; before every other entry of this design. If another open pull request has regenerated `.go-arch-lint.yml`, it merges first and `go run ./scripts/arch` is run again.
- Design: docs/design/market-page-producer.md#interface
- Files: internal/pages/market_page/api.go, internal/pages/market_page/api_test.go, .go-arch-lint.yml

Goal: the declarations of `## Interface` and a test of the contract from both sides; `.go-arch-lint.yml` regenerated by `go run ./scripts/arch` so it holds the component `pages/market_page`.

Acceptance:

1. `grep -c "type Config struct\|type NewPageFunc func\|PageName\|PageSchema" internal/pages/market_page/api.go` is at least 4, and `grep -c "^func " internal/pages/market_page/api.go` prints 0 (declarations only).
2. `grep -c "^func Test" internal/pages/market_page/api_test.go` is at least 2 (a fake constructor; a caller filling `Config`), and the test reads `internal/compute/rules/rules.json` and `internal/jobs/computefigures/job.toml` and fails if a constant of `api.go` names an id that either does not hold.
3. `grep -c "pages/market_page" .go-arch-lint.yml` is at least 2 and `go run ./scripts/arch --check` prints `arch: ok`.
4. `go list -f '{{join .Imports "\n"}}' ./internal/pages/market_page | grep optdesk` lists only `internal/contracts`, `internal/store`, `internal/compute` and `internal/pages`.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- contracts web internal/pages/api.go | grep -c .` prints 0.

Checked by CI: `go test ./internal/pages/market_page -race -count=1`; ci-go (go-arch-lint, golangci-lint); `go run ./scripts/arch --check` printing `arch: ok`.

### market-page-builder

- Module: pages. Size: L (kept under the size rule by five files; if it does not fit, report it in the pull request description and do not split the file). Depends on: market-page-interface.
- Order: after the interface entry; needs no code of pull request 500 or 533 (its tests pass a fake `ReadingFunc` and a fake `SeriesReader`).
- Design: docs/design/market-page-producer.md#decisions (1 to 6, 11)
- Files: internal/pages/market_page/builder.go, internal/pages/market_page/cells.go, internal/pages/market_page/term.go, internal/pages/market_page/shift.go, internal/pages/market_page/format.go, internal/pages/market_page/strings.go, internal/pages/market_page/page.go, internal/pages/market_page/builder_test.go, internal/pages/market_page/cells_test.go, internal/pages/market_page/shift_test.go, internal/pages/market_page/term_test.go, internal/pages/market_page/testdata/ (the expected JSON of each case, new), docs/contracts/pages.md

Goal: `Build` and `NewPage` return the JSON of `page.market_page.v1` as Decisions 3 to 6 say, all fixed Chinese texts as constants in `strings.go`, `var _ NewPageFunc = NewPage` in `page.go`.

Acceptance:

1. `grep -rhE "^func (TestPercentileThreeOk|TestPercentileShortHistoryIsNoData|TestPercentileStaleInputUsesDataDay|TestPercentileOtherReasonIsMissingInput|TestPercentileNoRowIsNoData|TestPercentileNumericReasonCodes|TestPercentileDataDayNotAsOfDay|TestN225IsGap|TestIvRvCellsAreNotBuilt|TestWeatherSignedPercent|TestWeatherChartIsGap|TestTermPicksStoredPoints|TestTermStaleIsGap|TestTermAllGapIsOneGap|TestBreadthIsGap|TestStaleLabelOldestDataDay|TestGeneratedAtInZone|TestBuildMatchesSchema|TestReaderErrorIsError|TestNilZoneIsError|TestNewPageMeta)\(" internal/pages/market_page | wc -l` prints 21, and `go test ./internal/pages/market_page -race -count=1` passes.
2. `TestBuildMatchesSchema` validates every output (all readings `ok`; every percentile a Gap of each kind; no term series; a series with one point) against `contracts/schemas/page.market_page.v1.json` with `santhosh-tekuri/jsonschema/v6`; `grep -c "jsonschema" go.mod` is unchanged from main.
3. `TestPercentileNumericReasonCodes` feeds `reason_code` 1, 2, 3, 4, 5 as numbers (and a record with no code and state `missing_input`) and expects `missing_input`, `no_data`, `stale_input`, `missing_input`, `missing_input`, `missing_input` in that order; `TestPercentileDataDayNotAsOfDay` stores a reading with `day` 2026-11-06 and `data_day` 2026-11-03 and checks that the stale text and the entry of `data_times` say 11-03, and that a reading with no `data_day` leaves its source out of `data_times`.
4. `TestPercentileThreeOk` and the unit tests of the helper (`shift_test.go`) are exact. Percentile cell (shift 2, 0 decimals, unit `share_percent`, JSON number written as text): `0.685000` gives `68` and `0.675000` gives `68` (both ties; half-up gives 69 for the first), `0.125000` gives `12` and `0.135000` gives `14` (half-up gives 13 for the first), `0.684999` gives `68`, `0.685001` gives `69`, `0.000000` gives `0` and `1.000000` gives `100`; `1.000001` and `-0.1` and `abc` are errors that become the Gap `missing_input`. The same helper at one decimal (shift 2, 1 decimal) gives `0.123500` the text `12.4` and `0.124500` the text `12.4` (the second is a tie that half-up turns into `12.5`), which is the tie pair that tells half-even from half-up. Signed percent of the weather (shift 2, 2 decimals, sign shown): `0.031250` gives `+3.12%` and `0.031350` gives `+3.14%` (ties; half-up gives `+3.13%` for the first), `-0.031250` gives `-3.12%`, `-0.000040` gives `0.00%` with no sign. Ratio of the term (shift 0, 2 decimals): `0.905000` gives `0.90` and `0.915000` gives `0.92`. `grep -c '"0.123500"\|"0.124500"\|"0.685000"\|"0.675000"\|"0.031250"\|"0.905000"' internal/pages/market_page/shift_test.go` is at least 6.
5. `TestWeatherChartIsGap`, `TestBreadthIsGap`, `TestIvRvCellsAreNotBuilt` and `TestN225IsGap`: `weather.chart`, `breadth` and the `iv`, `rv20`, `iv_minus_rv20` cells of the four rows are Gaps of reason `not_built`; the `rank_3y` of N225 is a Gap of reason `no_data`; the four row names are `SPX`, `NDX`, `RUT`, `N225` in that order.
6. `TestTermPicksStoredPoints`: for points on 2026-11-06, 2026-10-30, 2026-10-29 and day 2026-11-07, `today` is the 11-06 point and `five_days_ago` the 10-30 point (the newest on or before 11-01); a series whose newest point is 5 days old gives Gap `stale` for `today`; two vintages of one observation date give one value, the newer. `TestTermAllGapIsOneGap`: no point of either series gives `term` as one Gap `no_data`.
7. `grep -rc "time.Now()\|time.Sleep\|LoadLocation\|ParseFloat" internal/pages/market_page/*.go | grep -v '_test.go' | grep -v ':0$'` prints nothing; `grep -c "float64" internal/pages/market_page/*.go | grep -v '_test.go' | grep -v ':0$'` prints only `internal/pages/market_page/format.go:`; `grep -c "t.Parallel()" internal/pages/market_page/builder_test.go` is at least `grep -c "^func Test" internal/pages/market_page/builder_test.go`, and the same holds for the other three test files.
8. `grep -rc --exclude='*_test.go' "internal/store/\|internal/jobs\|internal/notify\|internal/compute/\|internal/pages/[a-z_]*/" internal/pages/market_page | grep -v ':0$'` prints nothing (only the roots `store`, `compute`, `pages`, `contracts` are imported, and no sibling builder package); `grep -c "market.vix_pctile_3y\|market.vxn_pctile_3y\|market.rvx_pctile_3y" internal/pages/market_page/api.go` prints 3 and the builder uses the constants of `api.go`, not literals (`grep -c '"market\.' internal/pages/market_page/builder.go internal/pages/market_page/cells.go internal/pages/market_page/term.go` prints 0 for each).
9. `docs/contracts/pages.md` names `market_page.Build`, `NewPage`, `Config`, the page name `market_page`, the unit of each figure (`share_percent`, `percent`, `ratio`, `index_points`) and the Gap blocks, with a Version history line.
10. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line (`testdata/` counts as one path) and neither `internal/pages/market_page/api.go` nor `api_test.go`; `git diff $(git merge-base HEAD origin/main) -- contracts web internal/pages/api.go .go-arch-lint.yml | grep -c .` prints 0; `wc -l internal/pages/market_page/*.go` shows no file at 600 lines or more.

Checked by CI: `go test ./internal/pages/market_page -race -count=1` (the tests of acceptance 1 to 6, the schema check and the contract test of the interface entry); `go test ./internal/pages/... -race -count=1`; ci-go (go-arch-lint, golangci-lint); `go run ./scripts/arch --check` printing `arch: ok`; the size rule.

### market-page-ot

- Module: ot. Size: M. Depends on: market-page-builder; `report-reading-view-impl` of pull request 500 (the reader `WinnerReadings`); `site-ot-serve` of pull request 533 (the route table of `cmd/ot/site.go` and the stores opened once).
- Order: after market-page-builder and after both entries of the other designs have merged. `cmd/ot/wire.go`, `page.go` and `site.go` are also edited by `report-page-ot` (500) and `site-ot-serve` (533): the second to merge rebases. This entry is not cut before they have merged; if 533 has not merged when the builder lands, the dispatcher holds this entry, it does not fall back to `ot serve --name`.
- Design: docs/design/market-page-producer.md#decisions (7, 8)
- Files: cmd/ot/market.go, cmd/ot/market_test.go, cmd/ot/page.go, cmd/ot/page_test.go, cmd/ot/site.go, cmd/ot/site_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/demo.go, docs/contracts/ot.md

Goal: `ot page --name market_page [--day DAY]` prints the page, and `/market` of `ot serve` serves it, built per request from the stored readings read as `ot_ro_pages` through the reader opened once per process, and from the series store already opened for the site.

Behaviour: `market.go` holds `marketPage(ctx, a)` (the page, a closer, an error) and the handler for `/market`; the `Config` is `RunID`, `BuildID`, `Scope` as `macro_page` fills them, the profile zone, and `Readings` the method value `WinnerReadings`; a nil zone is the same error as `macro_page`. `page.go` tests the name `market_page` before `pageNamed`, as 500 does for `report_page`. `site.go` gains the route `/market` (GET and HEAD; a reader error is 500 with the fixed text and no error text; a path below `/market` is 404). `wire.go` gains the `deps` fields `marketPage` (real value `market_page.NewPage`); only `wire.go` imports the implementation package. `docs/design/modules.md` already lists `internal/pages/market_page` for `cmd/ot`. `demo.go` gains a demo `ReadingFunc` returning fixed readings with `data_day` for the five figures; with the demo profile no store is opened.

Acceptance:

1. `TestPageMarketFlags` (fake `deps`): `--name market_page` builds and prints, `--day x` is a bad-day error, and `--name macro_rates` and `--name macro_page` give the bytes they gave before.
2. `TestSiteMarketRoute`: `/market` answers 200 with a vm whose `schema` is `market_page`, built once per request for the requested day (a fake page counts its calls); two requests with a fake reader whose rows change return two different `indices` blocks; `/market/x` is 404, POST is 405, a reader error is 500 with a fixed text and no error text.
3. `TestMarketOpensPagesRoleOnly`: the fake opener sees the DSN of the line `OT_PG_DSN_PAGES` and no other DSN name, once for the process; a missing line fails before any read with the variable name in the error and no DSN text; `grep -c "OT_PG_DSN\|DSNEnv" cmd/ot/market.go` prints at most 1 and that use is `store.DSNEnvPages`.
4. `TestMarketRankFilled` (a fake reader returning a `market.vix_pctile_3y` row of subject `VIXCLS`, value `0.685000`, parameter `data_day`, and a real `market_page` builder): the JSON of `ot page --name market_page --day <fixed>` has `indices.rows[0].rank_3y` of kind `value`, value 68, text `68%`, unit `share_percent`; the same row without the parameter still shows the value but `data_times` has no `VIXCLS`; the output validates against `contracts/schemas/page.market_page.v1.json`, so a mapping that drops the parameters fails.
5. `TestMarketDemoOpensNoStore`: with the demo profile and an opener that fails the test when called, the page builds and its `rank_3y` cells are values.
6. `git diff cmd/ot/wire.go` adds no import other than `market_page` (and `reading` if not yet imported); `docs/contracts/ot.md` names `market_page`, the route `/market`, `ot page --name market_page` and `OT_PG_DSN_PAGES`; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file is under 600 lines; `go run ./scripts/arch --check` prints `arch: ok`.

Checked by CI: `go build ./...`; `go test ./cmd/ot/... -race -count=1`; ci-go (go-arch-lint, golangci-lint).

### market-page-docs

- Module: docs. Size: S. Depends on: market-page-ot (merged).
- Order: last, and cut only after `market-page-ot` has merged: the register `pages` line is set only by the issue that makes a page read the figure, and until `/market` is served no page does. `docs/conventions/register.toml` is also edited by `ease-register` of `ease-v2` (pull request 560): the second to merge rebases; no entry waits for it.
- Design: docs/design/market-page-producer.md#decisions (9, 10)
- Files: docs/conventions/register.toml, docs/deploy/index-iv-history.md

Goal: in the entries `market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y`: `pages = ["market_page.indices.rows.rank_3y"]`; `market.vix_term_ratio`: `pages = ["market_page.term.sentence"]`; `market.sp500_vs_200d` version 2: `pages = ["market_page.weather.sentence"]` (version 1 is not touched); one more line in `history` of each ("<date> read by market_page through pages/market_page; formula, inputs and source_hash unchanged"); and a runbook of one page, new, under 80 lines: the command of Decision 9 with the three ids, the rule for `--from`, that the run is the owner's (the window does not run it without the owner's word), the check afterwards (the next `compute_figures` run stores an `ok` reading for each of the three ids, or `short_history` when `--from` was too late, and the page then shows `历史不满 3 年，不给分位`), and that the IBKR live index value is not part of it. No key, account id or DSN in the file.

Acceptance:

1. `grep -c 'pages = \["market_page.indices.rows.rank_3y"\]' docs/conventions/register.toml` prints 3, and `grep -c '"market_page.term.sentence"\|"market_page.weather.sentence"' docs/conventions/register.toml` prints 2.
2. `git diff $(git merge-base HEAD origin/main) -- docs/conventions/register.toml | grep '^[-+]' | grep -c 'source_hash\|^[-+]version\|^[-+]formula'` prints 0.
3. `grep -c "ot series-load" docs/deploy/index-iv-history.md` is at least 1; `grep -c "VIXCLS" docs/deploy/index-iv-history.md`, `grep -c "VXNCLS" ...` and `grep -c "RVXCLS" ...` are each at least 1; `grep -c "short_history" docs/deploy/index-iv-history.md` is at least 1.
4. `grep -c "[0-9]\{6,\}:[A-Za-z0-9_-]\{20,\}\|OT_PG_DSN.*=" docs/deploy/index-iv-history.md` prints 0; `wc -l < docs/deploy/index-iv-history.md` prints at most 80.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: `go test ./internal/compute/rules ./internal/compute/alertrules -race -count=1` (`TestRegisterEntriesComplete` and the `source_hash` comparison against the source of each rule); the register check of `docs/conventions`; `go run ./scripts/hygiene` printing `hygiene: ok`.

Runbook item owned by this entry: the owner's `ot series-load` of the three series is done from the runbook after it merges. It is not an agent step and not a test; the page tells the truth before it (`short_history`) and after it (a value from the next `compute_figures` run).

## Order

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | market-page-interface (interface) | pages, plus `.go-arch-lint.yml` (the interface entry) | S | first |
| 2 | market-page-builder | pages | L | after 1 |
| 3 | market-page-ot | ot | M | after 2, and after `report-reading-view-impl` (pull request 500) and `site-ot-serve` (pull request 533) |
| 4 | market-page-docs | docs | S | after 3 has merged |

Entries 1 and 2 start the day this design merges. Entry 3 waits for pull requests 500 and 533. Pull request 560 shares `register.toml` with entry 4 only as a file to rebase.

## Left out

- Rejected: a block package `internal/pages/index_iv` apart from the page (the shelved draft). Nothing would call it until a producer existed, and its register line would have named a reader that did not exist. The cells now ship with their caller.
- Rejected: a new register figure "index IV percentile". `market.vix_pctile_3y`, `market.vxn_pctile_3y` and `market.rvx_pctile_3y` are it, with their hashes; the rule, the window and the hash are unchanged.
- Rejected: unit `percent` for the percentile cell (it is a signed change in `common.v2.json`; the percentile is a share, `share_percent`).
- Rejected: computing, summing or comparing in the page or in `web` (a ratio to percent, a sign to a weather word, a 200-day mean): only a shift and a rounding of a stored text, and a pick of a stored point by its date. The weather headline therefore says "与 200 日线相差 +3.12%", not "above" or "below".
- Rejected: a percentile over 252 sessions (`ivp_v1`) for the Cboe indices: that is the figure of a stock IV30, and the glossary keeps the two apart.
- Rejected: a bootstrap of the history from IBKR or option-trader (the owner chose FRED; no splice of sources).
- Rejected: one `market_page` build per job, stored in a table (owner 2b). No dataset, role, migration or schedule.
- Rejected: reading the series for the percentile in the page (a 3-year sort in the builder): `compute_figures` already stores it.
- Deferred, left out: the IBKR live index IV (`market.index_iv`, the `iv` cell of the four rows) after a CBOE subscription probe (owner decision 3a); `rv20` and `iv_minus_rv20` (no figure of a 20-day realised volatility of an index: a figure in `register.toml` and the compute rules first, its own design); the N225 percentile (no FRED series for the Nikkei volatility; it needs the IBKR `N225_IV` history); the `weather.chart` (a stored 200-day line per day: `market.spx_close` is registered and not computed; the chart needs that series of closes and 200-day values, which no figure stores); the `breadth` block (a figure of the share of stocks above a moving average and constituent data); the history or trend of the IV level and of its percentile on the page (the schema has no field for a series of percentiles: a v2 of the schema); a percentile on the home card (`VolIndexItem` has no field: a `market_summary` v4); the SP500 percentile (`market.sp500_pctile_3y`: no row reads it); a top-bar item for `/market` (the bar is `TopNav.tsx` of pull request 533, a web entry of its own; the page is reached at `/market`); the window's own `ot series-load` run (the owner's act, the runbook of the docs entry).
- Not verified: that FRED still carries each of `VIXCLS`, `VXNCLS`, `RVXCLS` back to the first day wanted (the loader fetches and the owner sees the count); that `compute_figures` writes the parameter `data_day` on a `stale_input` row (read in `steps.go` and `figures.go`: the parameter is written whenever `DataDay` is set, and `checkAge` leaves it set; not run); that the three readings are `ok` in production (they depend on the history load and a later `compute_figures` run). No FRED call and no database was used while drafting.

## Open questions for the owner

None. The unit (`share_percent`, whole percent), the Gap wording, the route `/market` without a bar item, and the order of the register entry after the page is served are the designer's choices within the owner's decisions of 2026-11-08.
