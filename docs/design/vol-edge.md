# Vol edge: is a sale paid for? Implied volatility against realised and a stated forecast, per name

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 2
Status: draft, under review in the pull request that adds this file. Read on origin/main at 7e82ebb0 (a merge of main into the branch is the base of revision 2). "[unverified]" marks a claim not checked in code, docs or a probe. Revision 2 answers the check of revision 1 (`vol-edge-review-r1.md`, outside the repository) and the adversarial pass r1 (`## Adversarial`).

Source: owner ask 2979, 2026-11-12: "Think ahead about option portfolio strategy. option-trader did it badly; learn from it. Make it
more professional, following Natenberg's Option Volatility and Pricing, and give me coaching and help." The owner sells options (US
short puts and covered calls at delta 0.15 to 0.30, DTE 20 to 45, and JP), works on the web page only, and the front end computes
nothing. Inputs: the advisor's research and survey of main (`strategy-research-2979.md`, `strategy-survey.md`), option-trader's
code and record, and the aggregates of the owner's real trades (197 short lots since 2025-12: short puts 117 and covered calls 51,
both at a median of 35 DTE; puts bought back at a median of about 23 days with 86 percent of the credit kept; 13 rolls;
assignment on lots held to expiry 40 percent of short puts and 32 percent of covered calls; 0 of 867 old-scan offers traded; delta,
moneyness and IV at entry cannot be rebuilt from the old stores). Those files are outside the repository and are not re-checked here.

## Goal

One question per name, in Natenberg's terms for a seller of premium (short volatility): is the implied volatility high enough
against the volatility I expect? The first cut answers it for a name the owner watches or holds, on the one-stock option page
(`stock_option_page`), as a name-level volatility view:

- implied: `market.iv_ibkr`, IBKR's 30-day implied volatility of the name; realised: 20, 60 and 120 sessions; a stated forecast;
  implied minus forecast; implied's percentile in its year; the past error of the forecast; the next earnings and ex-dividend dates
  as stored; and one English sentence that says what it does not know;
- name-level, and the page says so: the figures belong to the name, not to a strike or an expiry. They do not follow the owner's
  choice of strike (no skew, `## Left out`). They serve a covered call as well as a short put, because a call on the name is
  compared with the same forecast; the call page is the second cut (the page loads short puts only: `internal/pages/option_boards/load.go:17,30`);
- every figure carries the data day of its reading, and a missing input is named with its reason, never filled: a name with no
  `market.iv_ibkr` shows "implied missing" with the reason and still shows its realised figures;
- every followed name gets the view, the options group or not: the figures are stored for the names with a stored option chain and
  for the names the bars job follows (Decision 5), and the page builds the view from the key alone, so a held name with no candidate
  row still gets it (Decision 7);
- the forecast's past error is in this cut: the forecast made 20 sessions ago against the realised volatility of the 20 sessions
  since, per name, from bars (Decision 6);
- the figures are stored once a day, so the entry facts of any later grade (the forecast and the windows of the day of a sale)
  exist from the day this lands. The grade of each sale is the second cut (`## Left out`, Decision 13).

No pricer, no new collection, the candidate ranking unchanged. Like with like: `market.iv_ibkr` is a 30-calendar-day annualised
implied; the realised figures are annualised with 252 sessions (`HistoricalVol`); both are annual fractions shown in volatility
points. The two annualisations differ a little; the page's `method` sentence says so.

## What main holds today

- Figures are rules plus a Go method: `internal/compute/rules/rules.json` (22 lines, one rule per line), `compute.Figures`
  (`internal/compute/api.go:172`; `HistoricalVol` takes any N from the rule: `internal/compute/figures/series.go`), a register
  entry per rule in `docs/conventions/register.toml`, and `TestRulesMatchRegister` (`internal/compute/rules/rules_test.go:367`)
  that needs the rule row and its register entry in one pull request (precedent: `docs/design/market-rv20.md` issue 3).
- `compute_figures` (`internal/jobs/computefigures`) stores, per underlying that has `obs.option_eod` rows of the day
  (`listSubjects`, `subjects.go:40`), five readings: `market.iv30_own`, `market.ivp_own`, `market.ivr_own`, `market.ivp_ibkr` and, when
  `PriceBars` is set, `market.hv30_own` (`runHv30`, `hv.go`, gated by `hvOn`). `market.rv20` is stored only for SPX, NDX and RUT
  (`job.toml` `rv20_indices`, `runRv20` in `rv.go`); `Run` sends an rv20 key of an index to `runRv20` and any other underlying key to
  `underlyingLists.add` (`job.go`). `market.iv_ibkr` is the IBKR 30-day IV bar series stored by `iv_history`
  (`internal/jobs/ivhistory`) for the names of the options group, 400 days; `iv_history` runs at session close + 10 minutes
  (`internal/jobs/ivhistory/job.toml`) and `compute_figures` at + 20 minutes (`computefigures/job.toml`); `runIvStats`
  (`steps.go`) already reads it by `LatestReadings` and accepts only an ok row of the as-of day.
- Bars: `bars_refresh` stores `trades` closes for the names the owner follows (group `bars` of `cmd/ot/followfilter.go`: every active
  name, and a paused or retired name that is held), 400 days of backfill. The options group (`options` flag) alone has a chain,
  `iv_history`, candidates and an option page row (`cmd/ot/candidates.go:77`).
- `hvLookback = 70` calendar days of closes (`hv.go:13`): enough for 20 and 30 sessions, not for 120 (121 closes are about 176 calendar days).
- `compute/api.go` has 836 lines and a ceiling of 840 in `docs/design/ratchet.json`; an interface change there may add 4 lines.
- Implementers of `compute.Figures`: `calc` (`internal/compute/figures`), `fakeFigures` (`internal/compute/contracttest/contracttest_test.go`,
  with the dispatch by method name in `contracttest/figures.go`) and `fakeFigs` (`internal/jobs/computefigures/job_test.go:24`,
  which `seriesFigs` embeds: `hv_test.go:222`).
- The stock page `option_boards.BuildStock` renders stored `store.Candidate` rows of kind `short_put` only
  (`internal/store/api.go:561`: `Iv`, `NextEarnings`, `EarningsState`, `NextExDate`, `ExAmount`; `Fits` is one word per account alias,
  `store/api.go:549`); its schema `contracts/schemas/page.stock_option_page.v2.json` has fixed blocks. A key with no stored row gives an
  empty page with a gap text, status 200 (`docs/contracts/ot/pages/stock_option_page.md`). The page does not use `Fits`.
- `store.ReadingViewReader` (`internal/store/api.go:491`) is how pages read stored figures; `cmd/ot/site.go:114` holds one as
  `siteReaders.view`, opened at `site.go:237`; `StockConfig` has no reading reader today (`option_boards/page.go`).
- The page types are generated by `pnpm --dir web run generate:types` (`web/package.json`), checked by `git diff --exit-code web/src/generated`
  in `.github/workflows/ci-web.yml`; `docs/contracts/page-schemas.md` says a new schema is added that way. `SUPPORTED_VERSIONS = [1, 2]`
  (`web/src/types/stock_option_page.ts:7`), and `web/test/pages/stockoption/StockOptionView.test.tsx:129` asserts version 2 validates.
- No stored per-sale record exists; the option ledger is derived at read time from the Flex rows (`docs/design/option-ledger.md`
  Decision 9), insert-only.

## Modules

| Entry | Module | What it adds |
|---|---|---|
| 1 | compute | the method `Figures.Combine`, five rules (`market.rv60`, `market.rv120`, `market.vol_forecast`, `market.vol_edge`, `market.vol_forecast_err`), their register entries; the two fakes of other modules that the method breaks |
| 2 | contracts | schema `page.stock_option_page.v3` with a `vol_view` block, and its generated types |
| 3 | jobs | `compute_figures` stores rv20, rv60, rv120, the forecast, the edge and the forecast error for every subject, from bars |
| 4 | pages | `option_boards.BuildStock` builds `vol_view` from stored readings and the page's rows |
| 5 | ot | the stock page site gives the builder `rd.view`; `compute_figures` is given the followed names |
| 6 | web | the stock option page shows `vol_view` |

Module dependencies follow `docs/design/modules.md`: jobs and pages depend on compute and store; web on contracts; ot on all.
Entry 6 is over the cut of 3 to 5 (Decision 12, Doubt 1).

## Interface

Owning modules: compute (entry 1) and contracts (entry 2). Everything else uses them by name.

1. compute, `internal/compute/api.go`, inside `Figures`:

```go
	// Combine joins the figures of one subject and day by the convention of the rule: vol_blend_v1 is the mean of
	// the parts, vol_diff_v1 is parts[0] minus parts[1]. A part that is not ok makes the result missing (no_input).
	Combine(id string, parts []Figure, asOf contracts.Day) Figure
```

   Three lines; the file ends at 839 of 840. The implementation `calc.Combine` is in `internal/compute/figures` in the same entry,
   because adding the method to `Figures` breaks `calc` and the fakes until each has it.
2. contracts, `contracts/schemas/page.stock_option_page.v3.json`: the v2 schema plus the required property `vol_view`, an object
   or null. One block per page: `name` (text), `method` (the sentence that names the forecast and the two annualisations), `rows`
   (an open list of `{label, text, data_day, gap}`, label a string of the pattern `^[a-z0-9_]{1,32}$`, not an enum; `text` the stored
   decimal shown in volatility points or as a share, `data_day` the day of the reading, `gap` a named gap with its reason code in
   place of `text`), `events` (`{kind: earnings|ex_dividend, day, state}` as stored), `coaching` (one English sentence). The
   builder writes the rows `iv_name`, `rv20`, `rv60`, `rv120`, `forecast`, `edge`, `ivp`, `forecast_err`, in that order. There is no
   `fit` and no per-contract field: the block is name-level. v2 and v1 stay beside it.
3. Rules, read by `jobs` and `pages` by id string only: `market.rv60` (convention `rv60_v1`, n 60), `market.rv120` (`rv120_v1`, n 120),
   `market.vol_forecast` (`vol_blend_v1`), `market.vol_edge` (`vol_diff_v1`), `market.vol_forecast_err` (`vol_diff_v1`); subject
   kind `underlying`, unit decimal, scale 6, trading days 252, max age 4 days.

## The method

Forecast `market.vol_forecast` = the plain mean of `market.rv20`, `market.rv60` and `market.rv120` of the same day: close-to-close
log returns of the stored `trades` closes (IBKR split-adjusted, not dividend-adjusted: `docs/design/market-bars.md`), sample standard
deviation (ddof 1), times the square root of 252, as `HistoricalVol` does. It counts stored rows, so a missing session is a two-day
return and is not detected. Implied is `market.iv_ibkr` of the day. `market.vol_edge` = implied minus forecast, an annual fraction
(0.03 is 3 volatility points). `market.vol_forecast_err` = the forecast the same method gives on the closes up to 20 sessions ago
minus `market.rv20` of today (the realised volatility of the 20 sessions since); positive means the forecast was too high. A missing
part makes the result missing with its reason, never a fill.

## Issues

Build order: every entry compiles and its own tests pass against main plus the entries it is ordered after.
Where an acceptance says a grep prints nothing, the command prints nothing and exits 1.
The pull request of entry 2 carries the line `Contracts: changed (page.stock_option_page.v3 v3 (new))`; the others `Contracts: unchanged`.

### 1 vol-combine (interface)

Module: compute. A judged entry that also touches `docs` (the register file: `TestRulesMatchRegister` needs the rule rows and register entries in one pull request, as `docs/design/market-rv20.md` issue 3) and `jobs` (the test fake `fakeFigs`, which stops compiling when `Combine` joins `Figures`). Size: M.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `internal/compute/contracttest/contracttest_test.go`, `internal/compute/contracttest/figures.go`, `internal/compute/figures/combine.go`, `internal/compute/figures/combine_test.go`, `internal/compute/rules/rules.json`, `internal/compute/rules/rules_test.go`, `internal/compute/own/testdata/stored_text.json`, `docs/contracts/compute.md`, `docs/conventions/register.toml`, `internal/jobs/computefigures/job_test.go`
Design: docs/design/vol-edge.md#interface, #the-method (Decisions 2, 3, 6)
Order: first. Entries 3 and 4 are after it.
Goal: declare `Figures.Combine`, implement `calc.Combine` for the conventions `vol_blend_v1` and `vol_diff_v1`, add the five rules and five register entries, give `fakeFigures` and `fakeFigs` the method (`seriesFigs`, `hv_test.go:222`, embeds `fakeFigs` and inherits it).
Acceptance:
1. `git grep -c "Combine(id string, parts \[\]Figure, asOf contracts.Day) Figure" -- internal/compute/api.go` prints 1 and `wc -l < internal/compute/api.go` is at most 840.
2. `git grep -c '"id":"market.rv60".*"convention":"rv60_v1".*"n":60\|"id":"market.rv120".*"convention":"rv120_v1".*"n":120\|"id":"market.vol_forecast".*"convention":"vol_blend_v1"\|"id":"market.vol_edge".*"convention":"vol_diff_v1"\|"id":"market.vol_forecast_err".*"convention":"vol_diff_v1"' -- internal/compute/rules/rules.json` prints 5.
3. `git grep -c 'id = "market.rv60"\|id = "market.rv120"\|id = "market.vol_forecast"\|id = "market.vol_edge"\|id = "market.vol_forecast_err"' -- docs/conventions/register.toml` prints 5; each new entry has `implemented_by` naming `internal/compute/figures.(*calc).Combine` or `.HistoricalVol`; `git grep -c "pending-recompute" -- docs/conventions/register.toml` does not exceed its count on the base. The register entry of `market.rv20` is reworded only if its text says "index" where the rule now serves a name (its `source_hash` is unchanged because the numbers are).
4. `internal/compute/figures/combine_test.go` has a test for each case: the mean of 0.20, 0.30, 0.40 is `0.300000`; 0.25 minus 0.20 is `0.050000`; a negative difference keeps its sign; one part missing gives missing `no_input`; a wrong count of parts gives missing `bad_input`: `git grep -c "^func Test" -- internal/compute/figures/combine_test.go` is at least 5.
5. `git grep -c "Combine" -- internal/compute/api_test.go internal/compute/contracttest/contracttest_test.go internal/compute/contracttest/figures.go internal/jobs/computefigures/job_test.go` prints a count of at least 1 for each of the four files.
6. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/compute/... ./internal/jobs/computefigures -race -count=1` (the hash of each rule is the one `TestRulesMatchRegister` expects and the register `source_hash` is the same value; no `pending-recompute`), `go run ./scripts/arch --check`.

### 2 vol-page-schema (interface)

Module: contracts. Size: S.
Files: `contracts/schemas/page.stock_option_page.v3.json`, `web/src/generated/stock_option_page.v3.ts`, `docs/contracts/page-schemas.md`
Design: docs/design/vol-edge.md#interface (item 2, Decision 4)
Order: parallel with 1; before 4 and 6.
Goal: the v3 schema and its generated types, produced by `pnpm --dir web run generate:types page.stock_option_page`, not by hand; the pull request carries `Contracts: changed (page.stock_option_page.v3 v3 (new))`.
Acceptance:
1. `git grep -c '"vol_view"' -- contracts/schemas/page.stock_option_page.v3.json` is at least 2 (property and required list); `git grep -c '"vol_view"' -- contracts/schemas/page.stock_option_page.v2.json` prints nothing.
2. The row label is a string with the pattern `^[a-z0-9_]{1,32}$`, not an enum, and the block has no `fit`: `git grep -c "a-z0-9_]{1,32}" -- contracts/schemas/page.stock_option_page.v3.json` is at least 1 and `git grep -c '"fit"' -- contracts/schemas/page.stock_option_page.v3.json` prints nothing; each row requires `data_day`: `git grep -c '"data_day"' -- contracts/schemas/page.stock_option_page.v3.json` is at least 2.
3. The generated file is `web/src/generated/stock_option_page.v3.ts` and names the block: `git grep -c "vol_view" -- web/src/generated/stock_option_page.v3.ts` is at least 1; the CI step `git diff --exit-code web/src/generated` after the generator is the check that it is the generator's output.
4. `docs/contracts/page-schemas.md` lists v3 and says v2 is kept: `git grep -c "stock_option_page.v3" -- docs/contracts/page-schemas.md` is at least 1.
5. `git diff --name-only BASE` lists exactly the Files line; `contracts/schemas/carryover.sha256` and `contracts/schemas/page.stock_option_page.v2.json` are untouched.
Checked by CI: the contracts check, `go test ./internal/contracts -race -count=1`, the generated-types step of `ci-web`.

### 3 vol-figures

Module: jobs. Size: M.
Files: `internal/jobs/computefigures/vol.go`, `internal/jobs/computefigures/vol_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/types.go`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/computefigures/hv_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/vol-edge.md#the-method (Decisions 2, 5, 6, 9)
Order: after 1.
Goal: for every vol subject, read the closes once, store `market.rv20` (not for the names of `rv20_indices`, which `runRv20` stores), `market.rv60`, `market.rv120` with `HistoricalVol`, then `market.vol_forecast` with `Combine` over the three, `market.vol_edge` with `Combine` over the stored `market.iv_ibkr` reading of the day (an ok row dated the as-of day, as `runIvStats` requires) and the forecast, and `market.vol_forecast_err` with `Combine` over the forecast computed on the closes up to 20 sessions ago and today's rv20. The vol subjects are the `listSubjects` underlyings and the optional `ComputeStores.Followed` names (declared in `types.go` as a slice of `{Name string; InstrumentID int64}`), joined by instrument id. The new steps are gated by `hvOn()` (a nil `PriceBars` plans none of the new keys), use their own `volLookback`, and `Run` routes a key of the new ids to the vol step, not to `underlyingLists.add`. A part that is missing gives a stored missing reading with its reason.
Acceptance:
1. The manifest names the ids: `git grep -c 'rv60 = "market.rv60"\|rv120 = "market.rv120"\|vol_forecast = "market.vol_forecast"\|vol_edge = "market.vol_edge"\|vol_forecast_err = "market.vol_forecast_err"' -- internal/jobs/computefigures/job.toml` prints 5; `Followed` is declared: `git grep -c "Followed " -- internal/jobs/computefigures/types.go` is at least 1.
2. `market.hv30_own` keeps its 70 days and the new figures their own: `git grep -n "hvLookback = 70" -- internal/jobs/computefigures/hv.go` prints a line and `git grep -n "volLookback" -- internal/jobs/computefigures/vol.go` shows a value of at least 230 (141 closes for the error, about 215 calendar days).
3. `vol_test.go` has tests on a fake price reader and a fake clock for: 160 closes store all of rv20, rv60, rv120, forecast and error ok; 70 closes store rv120 missing `short_history`, the forecast missing `no_input` and rv20 and rv60 ok; no `market.iv_ibkr` reading stores the edge missing `no_input` and the forecast ok; a name only in `Followed` gets its figures and one in both lists gets each key once; SPX, which is in `rv20_indices`, gets `market.rv20` once and the other four figures: `git grep -c "^func Test" -- internal/jobs/computefigures/vol_test.go` is at least 5.
4. The key counts of the existing tests follow the gate: with `PriceBars` nil the counts of `job_test.go` are unchanged (`git grep -n "4+3+len(marketIDs)" -- internal/jobs/computefigures/job_test.go` still prints a line) and with it set `job_test.go` and `hv_test.go` assert the new totals (`git grep -c "vol" -- internal/jobs/computefigures/hv_test.go` is at least 1).
5. The contract text lists the six figures, the eleven keys per name (iv30, ivp, ivr, ivp_ibkr, hv30, rv20, rv60, rv120, forecast, edge, error), the order of the steps and that the run bound stays 10 minutes: `git grep -c "market.vol_forecast_err" -- docs/contracts/jobs/compute_figures.md` is at least 1.
6. `git diff --name-only BASE` lists only paths of the Files line; `internal/jobs/computefigures/job.toml` still has `run_bound = "10m"`.
Checked by CI: `go test ./internal/jobs/computefigures -race -count=1`, `go run ./scripts/arch --check`.

### 4 vol-view-builder

Module: pages. Size: M.
Files: `internal/pages/option_boards/stock_vol.go`, `internal/pages/option_boards/stock_vol_test.go`, `internal/pages/option_boards/page.go`, `internal/pages/option_boards/stock_build.go`, `internal/pages/option_boards/payload.go`, `internal/pages/option_boards/strings.go`, `internal/pages/option_boards/page_test.go`, `internal/pages/option_boards/quote_test.go`, `internal/pages/option_boards/stock_schema_test.go`, `internal/pages/option_boards/stock_page_test.go`, `internal/pages/option_boards/names_test.go`, `internal/pages/option_boards/testdata/stock_option_page_ok.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_open.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_no_key.golden.json`, `docs/contracts/pages.md`
Design: docs/design/vol-edge.md#interface (item 2), Decisions 4, 7, 8, 9
Order: after 1, 2 and 3.
Goal: `StockConfig` gets `Readings store.ReadingViewReader` (nil gives every row a named gap, never an empty block); `BuildStock` writes `Meta.Schema` `page.stock_option_page.v3` and `SchemaVersion: 3` and fills `vol_view` from the key alone: the ticker of the key (upper case) is the subject of the seven stored figures (`market.iv_ibkr`, `market.rv20`, `market.rv60`, `market.rv120`, `market.vol_forecast`, `market.vol_edge`, `market.ivp_ibkr`) and of `market.vol_forecast_err`; each row carries the `data_day` of its reading; a name with no reading in a row gets that row's gap with the reason of the stored missing reading, else `no_input`. The view is built whether the key has candidate rows or not (a key with no row still gets its view). Events are the stored `NextEarnings`, `EarningsState`, `NextExDate` of the first row of the name, or absent with a gap when the name has no row. The page reads and shows; the one comparison it makes is Decision 8. No `Fits`.
Acceptance:
1. `git grep -c "Readings .*store.ReadingViewReader" -- internal/pages/option_boards/page.go` prints 1; `git grep -c 'page.stock_option_page.v3' -- internal/pages/option_boards/page.go` prints 1; `git grep -c 'page.stock_option_page.v2' -- internal/pages/option_boards/page.go internal/pages/option_boards/page_test.go internal/pages/option_boards/quote_test.go internal/pages/option_boards/stock_schema_test.go` prints nothing.
2. The page reads by id and computes nothing: `git grep -c '"market.rv60"\|"market.rv120"\|"market.vol_forecast"\|"market.vol_edge"\|"market.vol_forecast_err"\|"market.iv_ibkr"\|"market.ivp_ibkr"\|"market.rv20"' -- internal/pages/option_boards/stock_vol.go` prints 8 or more, and `git grep -cE "math\.|big\.|strconv\.Parse(Float|Int)" -- internal/pages/option_boards/stock_vol.go` prints nothing.
3. A key with no candidate row still gets a view: `stock_vol_test.go` has a test of a key with no row and stored readings that asserts `vol_view` is not null and has eight rows; a test of a blocked row asserts the same (`git grep -c "no row\|blocked" -- internal/pages/option_boards/stock_vol_test.go` is at least 2).
4. Every row has its day and a missing figure is a named gap, never a zero: tests assert `data_day` on each ok row; a test with no `market.iv_ibkr` reading asserts the `iv_name` row is a gap with a reason and the `rv20`, `rv60`, `rv120` rows are not gaps; a test with a nil reader asserts all eight rows are gaps (`git grep -c "^func Test" -- internal/pages/option_boards/stock_vol_test.go` is at least 5).
5. The sentence is English, built in Go, and says what it does not know: `git grep -c "does not know" -- internal/pages/option_boards/strings.go` is at least 3 (one per case: implied above the forecast, implied below it, inside the dead band; a fourth text for a missing edge says which input is missing); no text of `strings.go` contains the words "sell", "buy", "should" or "recommend": `git grep -ciE "sell|buy|should|recommend" -- internal/pages/option_boards/strings.go` is not larger than its count on the base; a test asserts the chosen text per case.
6. The golden files and the built pages validate against v3: `git grep -c "stock_option_page.v3" -- internal/pages/option_boards/stock_schema_test.go internal/pages/option_boards/quote_test.go` is at least 1 for each file; `git grep -c '"schema_version": 3' -- internal/pages/option_boards/testdata` is at least 3.
7. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/pages/... -race -count=1`, `go run ./scripts/arch --check`.

### 5 vol-view-wire

Module: ot. Size: S.
Files: `cmd/ot/stock_option_site.go`, `cmd/ot/stock_option_site_test.go`, `cmd/ot/wire_computefigures.go`, `cmd/ot/wire_computefigures_test.go`, `docs/contracts/ot/pages/stock_option_page.md`
Design: docs/design/vol-edge.md Decisions 5, 7, 10
Order: after 3 and 4.
Goal: `stockBuild` passes `rd.view` (the `siteReaders.view store.ReadingViewReader` that `cmd/ot/site.go:114` already opens and closes) to `StockConfig.Readings`; a nil `view` passes nil. `compute_figures` is given `ComputeStores.Followed`: the names of the bars group of the stored list (the same list `f.listed(groupBars, ...)` gives the bars job; a held paused name included), each with the instrument id the bars job gave it, got through the resolver of the existing wiring by the `ibkr_conid` key of the name, never creating an instrument [unverified: the handle and the call that gives an id without creating]; a name with no id is left out with one log line.
Acceptance:
1. `git grep -c "Readings: *rd.view" -- cmd/ot/stock_option_site.go` prints 1; `git grep -c "openReadingView" -- cmd/ot/stock_option_site.go` prints nothing.
2. `stock_option_site_test.go` has a test with a fake reader whose readings reach the page JSON (`git grep -c "vol_view" -- cmd/ot/stock_option_site_test.go` is at least 1) and one with no reader that still builds.
3. `git grep -c "Followed" -- cmd/ot/wire_computefigures.go` is at least 1, and `wire_computefigures_test.go` has a test that a held paused name is in `Followed` and a name with no id is not (`git grep -c "Followed" -- cmd/ot/wire_computefigures_test.go` is at least 2).
4. `docs/contracts/ot/pages/stock_option_page.md` names schema v3 and the view: `git grep -c "stock_option_page.v3" -- docs/contracts/ot/pages/stock_option_page.md` is at least 1.
5. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`, `go run ./scripts/arch --check`.

### 6 vol-view-web

Module: web. Size: S.
Files: `web/src/pages/stockoption/Vol.tsx`, `web/src/pages/stockoption/view.tsx`, `web/src/types/stock_option_page.ts`, `web/test/pages/stockoption/StockOptionView.test.tsx`, `web/test/pages/stockoption/fixtures/ok.ts`, `web/test/pages/stockoption/fixtures/gap.ts`
Design: docs/design/vol-edge.md Decision 11
Order: after 2 (parallel with 3 to 5; the page shows real data only after 5).
Goal: render `vol_view` as a table (label, text and its data day, or the gap and its reason), the events and the sentence, with no arithmetic. `SUPPORTED_VERSIONS` becomes `[1, 2, 3]`; version 2 and 1 payloads still validate and `bad-version.json` (version 9) stays refused.
Acceptance:
1. `git grep -c "vol_view" -- web/src/pages/stockoption/Vol.tsx` is at least 1; `git grep -cE "parseFloat|Number\(|toFixed|Math\." -- web/src/pages/stockoption/Vol.tsx` prints nothing.
2. `git grep -n "SUPPORTED_VERSIONS = \[1, 2, 3\]" -- web/src/types/stock_option_page.ts` prints a line, and `git grep -c "serves as schema_version 2\|served as schema_version 2" -- web/test/pages/stockoption/StockOptionView.test.tsx` is still at least 1.
3. The view test has a case for an ok view, one gap row with its reason, a null `vol_view`, and a version-3 payload: `git grep -c "vol_view" -- web/test/pages/stockoption/StockOptionView.test.tsx` is at least 3; `web/test/pages/stockoption/fixtures/bad-version.json` is untouched.
4. No new package: `git diff --name-only BASE -- package.json pnpm-lock.yaml frontend-whitelist.json` prints nothing.
5. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, the build.

## Left out

Second cut (named, with the reason):

- The grade of every sale. One population, closed early or held: premium kept, assigned or not, days held, realised volatility over the life against implied at entry, the held-to-expiry group beside the bought-back group (40 percent assignment against about 90 percent kept is a selection effect, not a verdict), never a win rate alone; the owner's habit (about 35 DTE in, out near 21 to 23 days or 90 percent kept) shown as kept or broken, without proposing another rule. Needs a read-time join of the Flex trades (the option ledger) with the readings of this cut, a page schema (`option_result_page` v2) and a builder. For history since 2025-12-01 the entry implied is `market.iv_ibkr` of the entry day, marked "name-level, not the contract's"; moneyness at entry (strike over that day's close from bars) stands in for delta. Decision 13 says why it waits.
- A covered-call page and calls on the stock page: the loader reads `short_put` only; reading `covered_call` rows (matrix, contracts table, schema) is larger than a block.
- The view per strike and expiry (the contract's own IV against the name's forecast, skew): needs the owner's choice of strike on the page and the chain history.
- The vol columns of the candidates page (generic `colDef` columns: no schema change).
- Per-account fit of a sale (`Candidate.Fits` is one word per alias; accounts are never pooled): on the account side of the second cut.
- Portfolio greeks in money (dollar delta, gamma per 1 percent, money theta and vega per account), the spot x volatility x time grid with the worst cell against the cushion, and the pricer decision (a library such as QuantLib or py_vollib, or a greeks approximation with its error shown).
- Skew and term structure (the chain history is days old and the capture narrow), beta-weighted delta, what-if margin, structures beyond single legs and hedging.
- A mean of the forecast error over a year (an aggregation of the stored daily errors) and a second forecast method (EWMA, mean reversion): after the owner has seen the first error.
- Any backtest on synthetic prices, win rate as a measure, yield as the only rank: not carried over from option-trader.

## Decisions

1. Scope is a name-level volatility view, not a strategy engine. The candidate ranking is unchanged; whether the forecast may later change the rank is the owner's word, asked after he has used the view.
2. The forecast is the plain mean of realised 20, 60 and 120 sessions, named `vol_blend_v1`. It has no parameter to tune, uses only stored bars, is checkable by hand, and each window is a figure the owner sees beside it. Library: the arithmetic is a mean of three stored figures and the rest of `calc` is already on gonum; own implementation, because a three-number mean needs no library. EWMA (RiskMetrics lambda 0.94) is the likely second method; no mainstream Go library supplies it, so it would also be own code.
3. One method `Combine` with the operation chosen by the rule's convention (`vol_blend_v1`, `vol_diff_v1`), not several methods: `compute/api.go` has 4 lines of ceiling left (836 of 840) and a ceiling is never raised by an agent. After entry 1 it has 1 line left, so the next compute interface change needs the owner's ratchet edit.
4. A new page version v3 and not an edit of v2: v1 and v2 are both kept in `contracts/schemas` and `web` keeps reading them. The block is one per page and name-level; the row label is a string, so a row added later is not a new version.
5. Figures are stored per day, not on demand (the page computes nothing, and a stored reading per day is the entry fact of a later grade). The subjects are the underlyings with a stored chain (`listSubjects`) and the followed names the wiring gives (`ComputeStores.Followed`), so a held name outside the options group, whose bars exist, gets rv20, rv60, rv120, the forecast and the error. Its implied is missing (no `iv_ibkr`, the options group only), so its edge and percentile are named gaps; the page shows "implied missing" with the reason and still the realised figures. The followed list is built when the job is built, as `iv_history`'s is; a name added later is seen after the next start [unverified: whether the owner's page edit rebuilds the jobs].
6. The forecast's past error is in the cut: the owner is told how wrong the number has been, per name, from the bars, not left to trust a model choice. It is one stored figure a day (forecast of 20 sessions ago minus the realised 20 sessions since), which accumulates into a series; the mean over a year is the second cut. It is computable today from the 400 days of bars, with no waiting (rv20 and the forecast of any past day are pure functions of stored closes), so the cut does not wait for days to pass.
7. The view is built from the key alone, whatever the gate status of the rows and whether the key has rows at all, because the owner picks the name first (valuation and buy price) and never traded from the old scan. The page route and the readings are by ticker. A name whose ticker is ambiguous (the figure subject is `underlying:<id>`) shows a gap `ambiguous_name`.
8. The sentence is English, built in Go from fixed texts, and chosen by the stored edge text with a dead band of one volatility point (`0.010000`) around zero: above the band, below it, inside it, or no edge (it names the missing input). The comparison is of the sign and of two fixed-width decimal texts, which the page does by text and not by number; whether a reviewer reads that as computing is Doubt 3. The sentence never reads as trade advice: it states the difference between two stored numbers and what the model does not know (the next earnings move, skew, how well the blend forecasts this name, shown by the error beside it, early assignment, the basis of the two annualisations). It is not a rank and not a score. One vol point is the stored scale of IBKR's IV bar; the owner may move the band (Doubt 4).
9. Events: the stored `NextEarnings`, `EarningsState` and `NextExDate` as stored; the "inside the life of the option" verdict is the stored earnings state. For ex-dividend the page shows the date and gives no verdict (a verdict needs the expiry, which is per contract; the call page is the second cut).
10. `ot` passes `rd.view`, already opened for the site; no new database role and no new opener. The followed names are wired by the same `ot` entry.
11. The web renders; it holds no arithmetic and adds no package (recharts and @tanstack/react-table stay the whitelist). Versions 1 and 2 of the page stay readable.
12. Six entries, one over the cut of 3 to 5. Five modules are touched by the smallest honest path (compute, contracts, jobs, pages, web) and the sixth, `ot`, is the composition root that gives the builder its reader and the job its names (`AGENTS.md`: a path belongs to one module). Moved out to hold the line: the grade, the call page, the candidates-page columns, the per-strike view and the second forecast.
13. The per-sale record waits for the second cut and is derived, not a new table. The name-level inputs (the forecast, the windows and `market.iv_ibkr` of each day) are kept per day by this cut and by `iv_history`, and the Flex rows are stored insert-only, so a grade by the name-level facts can be built at any time with nothing lost by waiting. What a table would add is the contract's own IV and delta at entry outside the capture band, and that is a collection question (late-collection storage), not a table question: a table written after the sale cannot recover a chain that was not captured. So the first cut does not build a table, and the second cut decides the contract-level entry facts together with that design.
14. JP is not in this cut. Reason: `compute_figures` runs on the XNYS calendar and the stored chains of the JP option jobs are separate; whether IBKR sends IV for OSE options is unverified; the page already says "Japan options: later design (option-candidates-jp)" (`jpNote` in `internal/pages/option_boards/strings.go:11`). A JP name with bars and no US-calendar chain gets its realised figures only if the followed list includes it, and then shows implied missing; no JP-specific figure is added.
15. The owner's habit (about 35 DTE in, out near 21 to 23 days or 90 percent kept, held to expiry sometimes, covered calls a core strategy) guides what the sentence must not contradict; no rule is proposed and the habit as kept or broken belongs to the grade (second cut).

## Doubts

1. Entry count: six against a cut of 3 to 5 (Decision 12). No two can merge: entry 1 is already a judged entry across compute, docs and a test fake of jobs, and `ot` and `web` are other modules. If the cap is hard, dropping 5 and 6 together stores and builds the view but does not show it; dropping 3 computes at read time, which breaks "pages compute nothing".
2. Entry 1 names three modules although each entry should edit one. The register file is `docs/**`; a compute-only change fails `TestRulesMatchRegister`, a docs-first one fails because `implemented_by` must name an existing function, and the fake `fakeFigs` of jobs stops compiling when `Combine` joins `Figures`. The precedent (`docs/design/market-rv20.md` issue 3) is on main for compute and docs; the jobs test file is new here. The alternative is a separate small interface for `Combine` in jobs, which `compute/api.go` cannot hold (ceiling).
3. The page compares two fixed-width decimal texts for the dead band and the sign. The rule "the page computes nothing" is the reviewer's to read; the alternative is a stored word (`above`, `below`, `inside`) from `Combine`, which would put a threshold into a compute convention.
4. The dead band of one vol point is mine, not the owner's. It sits in the page's strings and is changed there.
5. The followed list gives names through the existing resolver without creating an instrument; whether the wiring has a handle that cannot create is [unverified] (entry 5). If it creates, a name without bars would get a stray instrument; the entry then leaves such a name out.
6. History depth: rv120 needs 121 closes and the error 141; the bars job backfills 400 calendar days (about 275 sessions) at `backfill_names_per_run = 10`, so a new name needs several days before its history exists, and a name with fewer shows a named gap. How far back the real store reaches is [unverified]. Whether `deepen_names_per_run = 0` switches the 5-year deepening off is [unverified].
7. The forecast is a model choice. A blend of past windows ignores mean reversion, events and the market's own information; for a name just after earnings the 20-day window is dominated by one jump. The page says so and shows the error beside it.
8. Run size: eleven keys per name instead of five. At 100 names that is about 1,100 keys, each one read and one insert of the readings store; `run_bound = "10m"` is unchanged and the estimate (a few round trips per key) is far inside it, but the real number of names and the round-trip time are [unverified]. The contract text states the count so that a later rise is seen.
9. A name that is a subject in both lists, or an index name that is also a subject, must get each key once; entry 3 acceptance 3 tests it.
10. The close-to-close figures count stored rows: a session missing from bars becomes a two-day return and is not detected; closes are IBKR-split-adjusted and not dividend-adjusted. The sentence does not claim more.

## Adversarial

Round 1 (on revision 1): one read-only sub-agent, given the text and the repository and not the conversation; its answer is saved as `vol-edge-adversarial-r1.md` in the designer's scratchpad `designs/` directory (outside the repository). Revision 2 follows a second check of revision 1 by the advisor (`vol-edge-review-r1.md`, outside the repository) and the advisor's requirements R1 to R6; a second adversarial pass on the pushed head is run by the coordinator. Answers to round 1:

1. Gap against the goal. Adopted: the view is name-level and says so (Goal), the figures are stored for the followed names and not only for the watch list, and implied missing is a named gap beside realised figures. The magnitude bands asked for are replaced by one dead band (Decision 8), which is the owner's parameter to move.
2. Biggest unnamed risk. Adopted: `contracttest_test.go` and `contracttest/figures.go` are in entry 1; the fake `fakeFigs` of jobs is added (the check of revision 1 found it); the way to get the rule hash is stated in entry 1.
3. A simpler way. Not adopted for the forecast: the owner asked for a stated forecast and the error needs it stored. Adopted for entry 5: the reader is `rd.view`.
4. The lock-in. Adopted: the row label is a string; the ceiling of `api.go` is named in Decision 3; the stored ids carry the method (a change is a new rule version).
5. The claim most likely false. Adopted: "every name" is replaced by the subjects of Decision 5; the order of `iv_history` (close + 10 minutes) and `compute_figures` (+ 20) is settled by reading; the count of readings is corrected.
6. Likeliest slip. Adopted: the two fakes; the generator is `pnpm --dir web run generate:types`; `hvLookback` stays 70 and the new steps have `volLookback`. Doubt 2 stays the likeliest review stall.
