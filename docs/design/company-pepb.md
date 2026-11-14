# company PE and PB history: the company card says whether today's price is cheap against this company's own past

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 1

Source: owner valuation doc v1 (2026-11-11: the buy price sets the strike; the card must say what today's price is against the company's own history), advisor fact notes of 2026-11-14 (SEC fact tags, bar depth), `docs/design/valuation.md` (the band beside which the figure sits).

Goal: on the company card of the Companies page, beside the FCF buy band, the owner reads one line per company and per ratio: "PE 18.2 (TTM to 2026-09-30, filed 2026-10-28), 34th percentile of 1210 stored days since 2021-11-15", with a small line chart of the ratio over those days, and the same for PB. The ratio is the stored close of a day times the share count the company filed, over the net income of the trailing twelve months (PE) or the stockholders' equity (PB), each as filed on or before that day, so no figure uses a fact that was not yet public. A figure that cannot be formed is a named gap with its reason ("loss, no PE"), never a zero. Japanese names (no filings source) and forward PE are named gaps. The front end renders what the page gives and computes nothing. No migration, no new source, no new job: it only reads facts and bars that are stored.

Checked on origin/main (e123986d), the three points the advisor left open, and one correction:
- Splits in `obs.price_bar`: `trades` bars are split-adjusted by IBKR (`docs/design/market-bars.md` Decision 3) and `barsrefresh` rewrites the stored days when a pulled day differs by more than `restate_tolerance` (`run.go` `collect`, test `TestRestatedRule` case "split"). But the bar rows carry no usable trace of it: migration 0041 set `adjust_epoch` to 1 on every row and no code writes it, and `UpsertPriceBars` replaces a differing row in place and never sets `received_at` (`internal/store/barpg/store.go:141`, `internal/store/bar/api.go:34`). So after a split every stored close is on the new share basis, while a cover-page count filed before the split is on the old one, and nothing in the bars says when. The guard therefore uses the facts only (Decision 4).
- `companies_page.v2` is not in `carryover.sha256` (only the v1 file is; v2 is in `pageV2Files` of `internal/contracts/common_test.go`, which the carry-over test skips). The root and every block set `additionalProperties: false`, but a new property that is not in `required` keeps every stored v2 document valid, and the web `validate` of `web/src/types/companies_page.ts` requires only `core`, `diagnosis` and `questions`. So an optional `pepb` property fits v2 with no new version and no manifest line.
- A read of "facts filed on or before D" exists: `store.FactReader.FactsFiledBy(ctx, source, subject, cutoff)` (`internal/store/api.go:516`, index `(subject, filed)` of migration 0017) returns every row filed up to the cutoff and picks no winner. One call at the page day gives every fact for every earlier day, and the compute filters `filed <= D` per day. No new store method, so no wait for #2329.
- Depth of stored facts: the daily `fundamentals` job fetches only the filings filed in `[day - lookback_days, day]`, the manifest value is 14 and the code refuses more than 31 (`internal/jobs/fundamentals/plan.go:76`; `run.go:115` passes `From: day - lookback_days`). So stored facts reach back to the first day the job ran (plus the comparative columns each filing carries: a 10-K holds three years of income, a 10-Q holds the prior-year year-to-date), not five years. Days earlier than the first filing that gives A, Y and P have no figure (`ttm_piece_missing`), the window may stay under 250 days, and then the card shows the ratio with no percentile and `too_few_days`. Decision 12 removes the bound.
- Correction of the advisor frame (c): deepening is already on. `deepen_names_per_run = 0` means no cap (`internal/jobs/barsrefresh/manifest.go:62`, test `TestManifestDeepenZeroIsNoCap`), so every name is deepened toward `deepen_days = 1825` once its 400-day backfill is complete, one name-step a week (`deepen.go`). The window of the percentile is therefore all stored days (400 days now, up to five years later), printed on the card; there is no manifest entry in this design.

## Modules

| module | change |
|---|---|
| compute | entry cell gains `internal/compute/pepbapi` (new declarations-only package; `internal/compute/pepb` is its implementation, inside the existing `internal/compute/**` paths). `internal/compute/api.go` (839 of 840) and `internal/store/api.go` (800) are not touched. |
| contracts | paths unchanged; `page.companies_page.v2.json` gains the optional `pepb`, with its generated type, one new fixture and its line in `contracts/fixtures/MANIFEST.synthetic-v1.json`. |
| pages | paths unchanged; `internal/pages/companies_page` gains `pepb.go` and two optional Config fields. |
| web | paths unchanged; `web/src/pages/companies` gains `PepbBlock.tsx`. |
| jobs | paths unchanged; `internal/jobs/fundamentals` edits `run.go`, `run_test.go` and `docs/contracts/jobs/fundamentals.md` (entry 7). No new job, no new file. |
| ot | paths unchanged; `cmd/ot` gains `companies_pepb.go` and `companies_pepb_test.go`, and edits `companies_inputs.go`, `companies_inputs_test.go` and `companies_site.go` (entry 6); no job registration or console row changes. |

The only line of `docs/design/modules.md` that changes is the `compute` row, entry cell: `internal/compute/pepbapi` after `internal/compute/valuationapi`. No depends cell changes: pages already depends on compute and store, ot on both, compute on contracts only (`pepb` imports the root `compute` for `Stats`, which a sub-package may).

## Interface

### Decisions

1. Market cap, PE, PB. Market cap of day D is the `trades` close of D times the latest `EntityCommonStockSharesOutstanding` (taxonomy `dei`) filed on or before D. PE is market cap over trailing twelve months `NetIncomeLoss`; PB is market cap over `StockholdersEquity`. The SEC mapping keeps no EPS tag (`internal/sources/sec/mapping.go:18-49`), so net income over the cover count is the only path. Only facts in USD count (unit `USD`); a fact in another unit gives no figure with reason `non_usd`, because the close is in dollars. Equity is the `StockholdersEquity` row with the latest `period_end` among the rows filed on or before D (ties: the latest filed), not the latest filed row, which may carry a prior-year balance. Both ratios print with one decimal and the day they are for. A new test `TestNonUSDGivesNoFigure` and a new test `TestEquityTakesLatestPeriodEnd` cover the two rules.
2. No look-ahead. For day D only rows with `filed <= D` count, and of several rows for one period the latest filed wins (a restatement is used from its own filing day).
3. Trailing twelve months (US GAAP duration facts, `period_start_key`, `period_end`, `form`). Forms: A comes from a `10-K` or `10-K/A` row; Y and P from `10-Q` or `10-Q/A` rows; of several rows of one concept and period the one with the latest filed day on or before D is used (so a restated value is used from its filing day). With no annual fact A at all the reason is `no_annual` (state `missing_input`), not `ttm_piece_missing`. Take the latest annual fact A (a period of 364 to 371 days, so 52 and 53 week years count). If no 10-Q year-to-date fact starts 1 to 7 days after A ends, TTM is A. Otherwise TTM is A plus the latest year-to-date fact Y (starts 1 to 7 days after A ends) minus the prior-year year-to-date fact P (starts at A's start, ends 364 to 371 days before Y's end). A missing Y or P gives no figure with reason `ttm_piece_missing`; the three-quarter sum is never used. TTM at or below zero gives no PE with reason `loss` (card text "loss, no PE"). Equity at or below zero gives no PB with reason `negative_equity`. The card names the period end and the filed day of the newest piece.
4. Share-count jump guard (splits). Stored bars are all on the newest share basis after a split and the bars carry no trace of it (Checked bullet 1), while every count filed before the split is on the old basis. From the facts alone: when two consecutive filed share counts differ by more than 20 percent, every day before the later count's filed day gives no figure with reason `share_count_jump`, back to the first stored bar, so the window of the percentile starts at the later count's filing. A buyback or an issue over 20 percent is not told from a split and gives a gap, not a wrong figure. The newest count has no later one to compare, so a split since the newest filing is not detected (`barsrefresh` records no day of a restatement, Checked bullet 1, so the guard cannot use it either); to make that visible every line prints the share count and its filed day ("on 1,520M shares filed 2026-10-28").
5. Fact depth and the card on day one. The daily job stores only filings filed since it first ran (Checked bullet 4). A 10-Q holds no annual figure, so on day one before that run PE has no figure for a name (`no_annual`, text "PE: no 10-K stored") until its next 10-K, PB and the share count appear from the first stored filing, and the percentile and price points are gaps (`too_few_days`, text "no percentile: 40 days since 2026-10-05"). After the first daily run of the `fundamentals` job changed by entry 7 (or an owner re-run of a stored day with force from its existing console row) the facts reach as far back as SEC's history, so the window is limited by the bars: about 400 days at once and up to five years as deepening proceeds (Checked bullet 5); PE and PB then have a percentile at once. A day has a ratio only when its own as-of facts give every piece. Under `MinDays` the percentile and the price points are gaps with reason `too_few_days`; the card still prints the ratio, the window days and the window start.
6. Percentile. The values are the ratios of every stored day that has one; a bar whose close is empty, unparsable or not above zero is skipped and its day is not counted. The rank is the share of those days strictly below today's ratio (`compute.Below`), computed by the existing `compute.Stats.PercentileRank` (gonum behind `internal/compute/gonumstats`), times 100 and rounded half up to an integer 0 to 100 (the schema carries no `x-rounding` for these figures; the rounding is this rule's, in compute). Under 250 such days (`MinDays`) the ratio is shown and the percentile is a gap with reason `too_few_days`. "Today" is the latest stored bar day on or before the page day; a bar older than 5 days gives no figure with reason `price_stale`. The card prints the number of days and the first day of the window. With the 400 calendar days of bars now stored (about 275 sessions) and days lost before the first filing that gives every piece or before a share jump, a name can have fewer than `MinDays`: the card then prints the ratio, "no percentile: N days since D" and no price points, until deepening adds days.
7. The page and the card. `companies_page.v2` gets one optional property `pepb`: a block with a basis note and one row per company, or a Gap. A row has the symbol, `pe`, `pb` (each a line or a Gap) and `forward`, always a Gap ("forward PE: no estimates source"). A Japanese name's `pe` and `pb` are Gaps `jp_slice_missing`. A line carries the sentence of the card, the ratio as a Figure, the day, the period end of its basis, the percentile (a Figure or a Gap), the window, the price points of Decision 10, and at most 120 history points chosen in compute (every Nth stored day) for the chart. The web draws them as given and sorts, sums and rounds nothing.
8. Order and switching. The web entry accepts documents with and without `pepb`; the page entry is additive (a nil input gives today's page byte for byte); the `ot` entry is the one step that switches the block on. Entries 1 and 3 land first (interfaces), 2 after 1, 4 after 1 and 3, 5 after 3, 6 last. The fact-depth change (entry 7) is independent of the card entries and merges whenever its Order allows; it has no ot entry, the job is already registered.
9. Libraries. Decimal arithmetic uses `github.com/shopspring/decimal` (already a direct dependency of compute) and the percentile uses gonum through `compute.Stats`; no new dependency. The chart is `recharts` through `web/src/charts/Chart.tsx`. The own glue is the TTM and as-of rule of `internal/compute/pepb`; it sits behind `pepbapi.BuildFunc`, so a library or a stored figure can replace it. Own implementation, because no library reads SEC facts by filed day.

10. Price at a percentile (the owner's buy price sets the strike). Compute also gives, for the 20th and the 50th percentile of the window, the ratio at that rank (nearest rank over the sorted window ratios) and the price at which today's ratio would sit there: for PE the ratio times today's TTM over today's share count, for PB the ratio times today's equity over today's share count. Both are gaps with `too_few_days` under `MinDays`. Each point carries a sentence made in compute that names the window and says it is history: "PE at the 20th percentile of 300 days since 2025-09-01: price 41.20 (history, not a buy price)". The web renders it as given.

11. Share count against the filing's own diluted count. When the cover count of a filing differs from `WeightedAverageNumberOfDilutedSharesOutstanding` of the same filing (same `doc_id`; the rows with the latest `period_end`, and among them the shortest duration, so the 3-month row of a 10-Q, never the year-to-date one, and the full-year row of a 10-K) by more than 25 percent [T, a trial value], days priced with that count give no figure with reason `share_count_mismatch`; this catches a dual-class company whose store keeps one class count. A filing with no diluted count, or one that has only the `WeightedAverageNumberOfShareOutstandingBasicAndDiluted` tag, is not checked. A new test `TestShareCountMismatchGivesNoFigure` covers it.
12. Fact depth: the daily `fundamentals` job fetches with no `from` bound. `lookback_days` (14) exists only to filter which rows of the `companyfacts` document are kept (`mapCompanyFacts`, `internal/sources/sec/mapping.go:115`): the client already downloads the whole document of a company (one request per name, `internal/sources/sec/client.go:73`), so the bound saves no request and no download. Dropping it from `Run` (`From` empty, `To` the run day) is safe because the store keys a fact by its filing and `UpsertFacts` (`internal/store/fundamental/upsert.go`) writes a row only when currency, value, form, filed or `raw_sha256` differs, so facts already stored are left alone and only a company whose document changed since the last fetch is rewritten. The first run after the change loads the whole history of every watch-list name, a name added later gets it on its next daily run, and an owner re-run of a stored day with force from the existing console row of `fundamentals` (`Force: true`) does the same. No second job, no new console row, no registration. The manifest key `lookback_days` and the plan hash are left as they are (removing the key changes every plan hash); `Run` no longer reads it. Pace is the existing SEC client's; SEC's published request-rate rule is not verified here, and the request count per run does not change.

### Types

Owner: compute, `internal/compute/pepbapi/api.go`, declarations only, decimal text, no clock, no I/O.

```go
package pepbapi

// MinDays is the fewest stored days with a ratio under which no percentile is given.
const MinDays = 250

// StalePriceDays is how old the newest bar may be before no figure is given.
const StalePriceDays = 5

// The states of a Reading.
const (
	StateOK      = "ok"
	StateMissing = "missing_input" // Reason says which input
	StateLoss    = "loss"          // TTM net income at or below zero (PE) or equity at or below zero (PB)
)

// Reason codes, stable strings: no_price, price_stale, no_shares, no_income, no_annual, ttm_piece_missing,
// loss, no_equity, negative_equity, non_usd, share_count_jump, share_count_mismatch; of a percentile and the price points: too_few_days.

// Point is one stored day of the chart: Text one decimal, Value the same number for the chart.
type Point struct {
	Day   contracts.Day
	Text  string
	Value float64
}

// AtPercentile is the ratio at a rank of the window and the price that puts today's ratio there.
type AtPercentile struct {
	Percentile int    // 20 or 50
	Ratio      string // one decimal
	Price      string // two decimals
	Text       string // names the window, ends "(history, not a buy price)"
}

// Reading is one ratio of one company on the newest stored day.
type Reading struct {
	Kind, State, Reason string        // Kind is "pe" or "pb"
	Day                 contracts.Day // the bar day the ratio is for
	Ratio               string        // one decimal; empty unless StateOK
	BasisEnd            contracts.Day // period end of the newest piece of the ratio's basis
	BasisFiled          contracts.Day // filed day of that piece
	Shares              string        // the share count used, whole shares
	SharesFiled         contracts.Day // its filed day
	Percentile          string        // integer text, empty when PercentileReason is set
	PercentileReason    string
	At                  []AtPercentile // empty when PercentileReason is set
	WindowDays          int           // stored days that had a ratio
	WindowFrom          contracts.Day // first of them
	History             []Point       // at most 120, oldest first
	Text                string        // the sentence of the card, or the reason in words
}

// Input holds what is stored: every fact of the company filed on or before Today and its
// `trades` bars, oldest first.
type Input struct {
	Facts []contracts.FundamentalFact
	Bars  []contracts.PriceBar
	Today contracts.Day
}

// Result is the PE and the PB reading.
type Result struct{ PE, PB Reading }

// BuildFunc forms both readings. Implementation: internal/compute/pepb (New takes a compute.Stats).
type BuildFunc func(in Input) Result
```

Page schema (owner: contracts, `contracts/schemas/page.companies_page.v2.json`): optional top-level `pepb`, `anyOf` PepbBlock or `common.v1.json#/$defs/Gap`; `PepbBlock {basis_note, rows[<=200]}`; `PepbRow {symbol, pe, pb, forward}` with `pe`, `pb` each PepbLine or Gap and `forward` a Gap; `PepbLine {text, ratio: Figure (unit "ratio"), day, basis_end, percentile: Figure or Gap, at_percentile[<=2] of {percentile, ratio, price: Figure (unit "price"), text}, shares (Text), shares_filed (day), window_days, window_from, history[<=120] of {day, value, text}}`. Gap reasons come from the existing enum (`missing_input`, `stale_input`, `no_data`, `jp_slice_missing`, `not_built`, `no_reader`); the code of Decisions 3 to 6 is in the Gap text.

Pages (owner: pages, `internal/pages/companies_page`): two optional Config fields, `Pepb pepbapi.BuildFunc` and `Bars func(ctx context.Context, symbol string, from, to contracts.Day) ([]contracts.PriceBar, error)`. Either nil leaves the property out. Facts come from the existing `Config.Facts`.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. No migration, nothing holds the auto-deploy. Each entry carries its own `Contracts:` line: `companies_page v2 additive` in entry 3, `unchanged` in the others. Test runs are in the pull requests' CI, not in acceptance.

### 1 company-pepb-api (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/pepbapi/api.go`, `internal/compute/pepbapi/api_test.go`
Design: docs/design/company-pepb.md#Types
Contracts: unchanged
Acceptance:
1. `api.go` declares `MinDays`, `StalePriceDays`, the three states, `Point`, `AtPercentile`, `Reading`, `Input`, `Result` and `BuildFunc` as in the Types section, with docs and no function body, and is the only non-test `.go` file of the package.
2. `grep -c 'internal/compute/pepbapi' docs/design/modules.md` prints 1 (written by this design's pull request); `git diff --name-only BASE -- internal/compute/api.go internal/store/api.go docs/design/ratchet.json` prints nothing.
3. `api_test.go` has a new contract test `TestBuildFuncContract` (`t.Parallel()`): a fake `BuildFunc` is called by a caller that reads `Result.PE.State`, `Ratio` and `History`.

### 2 company-pepb-compute

Module: compute. Size: L.
Order: after 1.
Files: `internal/compute/pepb/build.go`, `internal/compute/pepb/ttm.go`, `internal/compute/pepb/series.go`, `internal/compute/pepb/build_test.go`, `internal/compute/pepb/ttm_test.go`, `internal/compute/pepb/guard_test.go`, `internal/compute/pepb/rules_test.go`
Design: docs/design/company-pepb.md#Decisions (Decisions 1 to 6, 9, 10, 11)
Contracts: unchanged
Acceptance:
1. `build.go` declares `New(stats compute.Stats) pepbapi.BuildFunc` and imports `github.com/shopspring/decimal`, `internal/compute` (the root) and of the other internal packages only `contracts` and `pepbapi`; no file of the package imports `net/http`, `database/sql` or `os`, and none calls `time.Now`.
2. A new test `TestNoLookAhead` gives a 10-K filed after day D and shows the ratio of D does not use it.
3. A new test `TestTTMAnnualPlusYTDMinusPriorYTD` shows TTM equals A plus Y minus P for a 10-Q year-to-date case, and A alone right after a 10-K.
4. A new test `TestMissingPriorYTDGivesNoFigure` shows a missing P (and a missing Y) gives state `missing_input` with reason `ttm_piece_missing` and no ratio, and that no three-quarter sum is formed.
5. A new test `TestLossGivesNoPE` shows TTM at or below zero gives state `loss`, reason `loss` and Text "loss, no PE" while PB is still formed; a new test `TestNegativeEquityGivesNoPB` shows the same for equity.
6. `guard_test.go` has a new test `TestShareCountJumpGivesNoFigure` as Decision 4 states it (counts 100, 100, 400, 400: days priced with either 100 give reason `share_count_jump`, days from the first 400 on have figures, and the window starts there), a new test `TestNewestCountHasNoGuard` (the newest count is never compared, Decision 4), and a new test `TestLineCarriesSharesAndFiledDay` (`Shares` and `SharesFiled` are the count used).
7. A new test `TestTTMToleranceAcceptsFiftyThreeWeekYear` shows an annual fact of 371 days and a prior-year year-to-date ending 371 days before Y's end form a TTM, and one of 372 days does not.
8. A new test `TestPercentile` shows with 300 stored days of which 34 percent are below today's ratio Percentile "34", `WindowDays` 300 and `WindowFrom` the first day; a new test `TestUnderMinDaysHasNoPercentile` shows 249 days give `PercentileReason` `too_few_days` and an empty Percentile; a new test `TestStaleBarGivesNoFigure` shows a newest bar 6 days old gives reason `price_stale`.
9. `rules_test.go` has new tests of the rules of Decisions 1, 3 and 11: `TestNonUSDGivesNoFigure`, `TestEquityTakesLatestPeriodEnd`, `TestAmendmentAndLatestFiledRowWin` (a 10-K/A value filed before D replaces the 10-K value, one filed after does not), `TestNoAnnualFactGivesNoAnnualReason` and `TestShareCountMismatchGivesNoFigure` (the 3-month diluted row is the one compared, not the year-to-date one, and a filing with only the BasicAndDiluted tag is not checked).
10. A new test `TestPercentileRoundsHalfUpAndSkipsBadCloses` shows 1 of 8 days below gives "13" (12.5 rounded up) and a bar with an empty or zero close is not counted in `WindowDays`; a new test `TestPricePointsAtPercentile` shows the 20th and 50th percentile ratios and their prices for a PE and a PB case, that under 250 days `At` is empty and `Text` reads "no percentile: N days since D", and that every `Text` names the window and ends "(history, not a buy price)".
11. A new test `TestHistoryIsAtMost120Points` shows `History` has at most 120 points, oldest first, and ends on today's day.

### 3 company-pepb-contract (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.companies_page.v2.json`, `web/src/generated/companies_page.v2.ts`, `contracts/fixtures/page.companies_page.v2.pepb.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/companies_page_v2_test.go`
Design: docs/design/company-pepb.md#Types
Contracts: companies_page v2 additive
Acceptance:
1. The schema has a top-level property `pepb` that is not in the root `required` list, and the `$defs` `PepbBlock`, `PepbRow` and `PepbLine` of the Types section; `schema_version` and every other property are unchanged.
2. `web/src/generated/companies_page.v2.ts` contains `pepb?`, `PepbBlock`, `PepbRow`, `PepbLine` and `at_percentile`, and `git diff --name-only BASE` lists the schema and this file together (the file is never edited by hand).
3. `contracts/fixtures/page.companies_page.v2.pepb.json` is new and has a row with a full line (with `at_percentile`), a row with a loss gap and a Japanese row; the three existing v2 fixtures are not in `git diff --name-only BASE`.
4. `internal/contracts/companies_page_v2_test.go` has a new test `TestCompaniesPageV2PepbFixture` that validates the new fixture, validates the existing `ok` and `gap` fixtures (no `pepb`), and refuses a `pepb` row with a verdict field.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. `contracts/fixtures/MANIFEST.synthetic-v1.json` lists `page.companies_page.v2.pepb.json` with a sha256 entry, and its entries for the existing `companies_page.v2` fixtures are unchanged.

### 4 company-pepb-page

Module: pages. Size: M.
Order: after 1 and 3. Additive: with `Pepb` or `Bars` nil the page is the one of main.
Files: `internal/pages/companies_page/pepb.go`, `internal/pages/companies_page/pepb_test.go`, `internal/pages/companies_page/builder.go`
Design: docs/design/company-pepb.md#Decisions (Decisions 7 and 8)
Contracts: unchanged
Acceptance:
1. `Config` in `builder.go` has the two fields `Pepb pepbapi.BuildFunc` and `Bars` of the Types section, and the page struct has `Pepb` with `json:"pepb,omitempty"`.
2. `pepb.go` calls `c.Facts.FactsFiledBy` once per company at the page day, reads bars through `c.Bars` for the whole stored span, calls `c.Pepb` and copies its fields into the block; it contains no arithmetic on ratios (`grep -c 'big\.\|decimal\.\|ParseFloat' internal/pages/companies_page/pepb.go` prints 0), and `grep -c 'pepbapi' internal/pages/companies_page/pepb.go` is at least 1.
3. A new test `TestPepbNilGivesTheOldPage` shows with both fields nil the built JSON has no `pepb` key and equals the output of the same Config on BASE (the existing fixtures of `builder_test.go` are unchanged).
4. A new test `TestPepbRowsAndGaps` shows a company with a full result gives a line with the sentence, ratio, percentile and history and `at_percentile` as the fake returned them, a loss result gives a Gap `missing_input` whose text holds "loss, no PE", an `edinet:` issuer gives Gaps `jp_slice_missing` for `pe` and `pb`, and every row has the `forward` Gap.
5. A new test `TestPepbValidatesAgainstSchema` validates a built page with the block against `contracts/schemas/page.companies_page.v2.json`.
6. `git diff --name-only BASE` lists no file outside the Files line.

### 5 company-pepb-web

Module: web. Size: M.
Order: after 3. It accepts documents with and without `pepb`, so it merges before any producer writes the block.
Files: `web/src/pages/companies/PepbBlock.tsx`, `web/src/pages/companies/CompaniesPage.tsx`, `web/test/pages/companies/pepb.test.tsx`
Design: docs/design/company-pepb.md#Decisions (Decision 7)
Contracts: unchanged
Acceptance:
1. `PepbBlock.tsx` takes its type from `@/generated/companies_page.v2`, and `CompaniesPage.tsx` renders it only when the property is present; `web/src/types/companies_page.ts` is not edited (`git diff --name-only BASE` lists no file outside the Files line).
2. `grep -c "Math\.\|\.sort(\|\.reduce(\|parseFloat\|Number(\|toFixed" web/src/pages/companies/PepbBlock.tsx` prints 0, and `git diff BASE -- package.json pnpm-lock.yaml frontend-whitelist.json | grep -c .` prints 0.
3. The component prints the `text` of each line, the percentile, the `text` of each `at_percentile` row as given, the share count and its filed day, and the Gap `text` for a Gap, one card per company; a row's `forward` Gap is shown as one muted line.
4. `pepb.test.tsx` is new and has new tests: a page without `pepb` renders as the existing `v2.test.tsx` fixture does; a page with the fixture of entry 3 shows the "34th percentile" sentence and the loss text; a Gap block shows its text and no empty table.
5. 按 frontend-stack：用 shadcn card (`components/ui/card.tsx`) 做 the card of a company beside the band, and shadcn alert (`components/ui/alert.tsx`) 做 the Gap line。
6. 按 frontend-stack：用 recharts through `web/src/charts/Chart.tsx` 做 the history line of PE and PB (the points as given)。
7. 按 frontend-stack：用 lucide-react (`Info`) 做 the icon of the forward-PE gap line。

### 6 company-pepb-wire

Module: ot. Size: S.
Order: after 1, 2 and 4. This is the switching step: it sets the two Config fields.
Files: `cmd/ot/companies_pepb.go`, `cmd/ot/companies_pepb_test.go`, `cmd/ot/companies_inputs.go`, `cmd/ot/companies_inputs_test.go`, `cmd/ot/companies_site.go`
Design: docs/design/company-pepb.md#Decisions (Decision 8)
Contracts: unchanged
Acceptance:
1. `companies_pepb.go` is new, builds `pepb.New(gonumstats.New())` and a `Bars` function that finds the instrument id with the existing `symbolIDs` resolver and reads `siteReaders.bars` (`PriceBars`, basis `trades`) over the whole stored span; `companiesInputs` has a `Bars` field and `newCompaniesPage` sets `cfg.Pepb` and `cfg.Bars`.
2. A symbol with no instrument id or a nil bar reader gives no bars for that symbol and the page shows a Gap `no_data`; `companies_site.go` changes only to pass the bar reader into `companiesInputs`.
3. `companies_pepb_test.go` is new and has a new test `TestCompaniesPepbWiring`: the page built with fake readers has the `pepb` property for a company with bars and no `pepb` property when the bar reader is nil; `companies_inputs_test.go` gains the `Bars` field in the fake inputs it builds.
4. `git diff --name-only BASE` lists no file outside the Files line and `cmd/ot/wire.go` is not edited.

### 7 company-pepb-fact-depth

Module: jobs. Size: S.
Order: first (independent of the card entries).
Files: `internal/jobs/fundamentals/run.go`, `internal/jobs/fundamentals/run_test.go`, `docs/contracts/jobs/fundamentals.md`
Design: docs/design/company-pepb.md#Decisions (Decision 12)
Contracts: unchanged
Pull request description (not acceptance): the requests go through the same SEC client as before, so pacing and `User-Agent` apply and the request count does not change; the first run writes the whole history of every name in one `UpsertFacts` call, and the description states the number of facts of one company's document and that SEC's own rate rule was not verified.
Acceptance:
1. `Run` in `run.go` calls `Fetch` with `FactPlan{Subjects, To: day}` and no `From`; `plan.go`, `job.toml` and the daily constructor are not edited (`git diff --name-only BASE` lists no `plan.go` or `job.toml`), and `docs/contracts/jobs/fundamentals.md` says in its `Run` line that no `From` is sent and that `lookback_days` is now only in the plan hash.
2. A new test `TestRunFetchesWithNoFromBound` shows with a fake source that the `FactPlan` the source receives has an empty `From` and `To` equal to the run day.
3. A new test `TestRunTwiceSameRowsNoDuplicate` shows with a fake writer keyed as the store keys a fact that a second run of the same day leaves the same rows and no duplicate.
4. `TestEmbeddedManifest` of `run_test.go` still passes unchanged (`lookback_days` stays 14), and `git diff --name-only BASE` lists no file outside the Files line.

## Left out

- Forward PE: no estimates source; a named gap on every row.
- Japanese names: no filings source (`edinet` rows hold no facts); a named gap. A source for them is a separate design.
- EPS-based and diluted-share PE: the mapping keeps no EPS tag and the cover count is the share count of record; the weighted diluted count is a period average and is not used.
- Dual-class companies: only one class count is stored (`InsertFacts` keys a fact without a class, `internal/store/api.go:507`); the check of Decision 11 gives a gap when it differs from the diluted count by more than 25 percent [T], but a dual-class company whose counts agree within that, or a filing with no diluted count, can still show a wrong figure.
- `lookback_days` stays in the manifest, the plan hash and the comment of `plan.go` though `Run` no longer reads it; removing it changes the plan hash and is a separate change.
- Split since the newest filing: `barsrefresh` records no day of a restatement, so the card can read too cheap (PE and PB too low by the split ratio) after a split until the next filing; the printed share count and its filed day are the only sign.
- A split ratio is not stored or guessed.
- Deepening: already on (see the findings); the owner sets `deepen_names_per_run` and `deepen_days`, this design does not touch the manifest.
- A cheap or dear verdict, a threshold or a colour: the card gives the ratio and its percentile and nothing more; the owner's buy price sets the strike.
- Industry or index PE comparisons, a stored PE series, a figure on the underlyings page: not asked.
- Names with no option_eod rows have no instrument id for the `symbolIDs` resolver and show the Gap `no_data` until a resolver by symbol is wired (more than a small change in the ot entry: the resolver is shared by Spot and Offers).

## Adversarial

Round 1 (adversarial read): the six points are adopted as before, except where round 2 changed an answer: the split guard (point 2) now flags every day before the later count (Decision 4), the multi-class point (3) is answered by Decision 11 instead of a Left out line only, and fact depth (4) is answered by Decisions 5 and 12 and entry 7. Not adopted: a fix of the id resolver inside the ot entry (shared by Spot and Offers; Left out).

Round 2 (advisor review), all adopted:
1. Split guard left earlier days wrong: Decision 4 and entry 2 item 6 (counts 100, 100, 400, 400); the line prints the count and its filed day; a buyback over 20 percent gives a gap.
2. Empty card on day one: Decision 5 states the card before and after; entry 7 drops the `from` bound of the daily job; SEC's rate rule is named unverified.
3. Dual class: Decision 11, `share_count_mismatch`, 25 percent marked [T], Left out updated.
4. Contracts line: entry 3 says `Contracts: companies_page v2 additive`.
5. Unit, equity row, forms and the no-annual reason: Decisions 1 and 3 and the tests of entry 2 item 9.
Also: the price points carry the window and "history, not a buy price"; entry 2 cites Decisions 10 and 11; the liquidity-view citation is dropped.

Round 3 (advisor read of 2964876a): the separate backfill job is removed (a registered job is scheduled daily; Decision 12 and entry 7 now change the existing `fundamentals` job, no entry 8); the 3-month diluted row and the BasicAndDiluted case are named (Decision 11); a split after the newest filing is a Left out line because `barsrefresh` keeps no restatement day; entry 6 lists `cmd/ot/companies_inputs_test.go`; every entry has a `Contracts:` line; the card under `MinDays` is stated (Decision 6).
