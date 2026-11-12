# Vol edge: is a sale paid for? Implied volatility against realised and a stated forecast, per name

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 3
Status: draft, under review in the pull request that adds this file. Read on origin/main of 2026-11-12 after #1758 (the branch holds a merge of it). "[unverified]" marks a claim not checked in code, docs or a probe. Revision 2 answers the check of revision 1 (`vol-edge-review-r1.md`, outside the repository) and the adversarial pass r1 (`## Adversarial`).

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

- implied: `market.iv_ibkr`, IBKR's implied volatility of the name (its horizon is not stated: `docs/GLOSSARY.md:79`, `docs/conventions/register.toml:1750`); realised: 20, 60 and 120 sessions; a stated forecast;
  implied minus forecast; implied's percentile in its year; the past error of the forecast; the next earnings and ex-dividend dates
  as stored; and one English sentence that says what it does not know;
- name-level, and the page says so: the figures belong to the name, not to a strike or an expiry. They do not follow the owner's
  choice of strike (no skew, `## Left out`). They serve a covered call as well as a short put, because a call on the name is
  compared with the same forecast; the call page is the second cut (the page loads short puts only: `internal/pages/option_boards/load.go:17,30`);
- every figure carries the data day of its reading, and a missing input is named with its reason, never filled: a name with no
  `market.iv_ibkr` shows "implied missing" with the reason and still shows its realised figures;
- every name with a stored option chain gets stored figures, and the page builds the view from the key alone (Decision 7), so a
  name with no candidate row still gets a view: realised figures where the figures exist, else a named gap. A held name outside the
  options group has no chain and so no stored figures in this cut: its view is a named gap (`no_input`). Giving it figures needs
  a name-to-instrument reader that does not create instruments, which main does not have (Decision 5); it is the first item of the second cut;
- the forecast's past error is in this cut: the forecast made 20 sessions ago against the realised volatility of the 20 sessions
  since, per name, from bars (Decision 6);
- the figures are stored once a day, so the entry facts of any later grade (the forecast and the windows of the day of a sale)
  exist from the day this lands. The grade of each sale is the second cut (`## Left out`, Decision 13).

No pricer, no new collection, the candidate ranking unchanged. Like with like is not claimed: `market.iv_ibkr` is IBKR's daily implied-volatility bar, whose horizon this repository does not know
(it is never mixed with the own IV30), and the realised figures are annualised with 252 sessions (`HistoricalVol`). Both are annual
fractions shown in volatility points; the page's `method` sentence says the horizon of implied is not stated, so the edge is a
difference of two numbers of possibly different tenor, not a measured premium.

## What main holds today

- Figures are rules plus a Go method: `internal/compute/rules/rules.json` (22 lines, one rule per line), `compute.Figures`
  (`internal/compute/api.go:172`; `HistoricalVol` takes any N from the rule: `internal/compute/figures/series.go`), a register
  entry per rule in `docs/conventions/register.toml`, and `TestRulesMatchRegister` (`internal/compute/rules/rules_test.go:367`)
  that needs the rule row and its register entry in one pull request (precedent: `docs/design/market-rv20.md` issue 3).
- `compute_figures` (`internal/jobs/computefigures`) stores, per underlying that has `obs.option_eod` rows of the day
  (`listSubjects`, `subjects.go:40`), five readings: `market.iv30_own`, `market.ivp_own`, `market.ivr_own`, `market.ivp_ibkr` and, when
  `PriceBars` is set, `market.hv30_own` (`runHv30`, `hv.go`, gated by `hvOn`). `market.rv20` is stored only for SPX, NDX and RUT
  (`job.toml` `rv20_indices`, `runRv20` in `rv.go`); `Run` sends an rv20 key of an index to `runRv20` and any other underlying key to
  `underlyingLists.add` (`job.go`). `market.iv_ibkr` is the IBKR implied-volatility bar series (horizon not stated) stored by `iv_history`
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
| 3 | jobs | `compute_figures` stores rv20, rv60, rv120, the forecast, the edge and the forecast error for every subject with a stored chain, from bars |
| 4 | pages | `option_boards.BuildStock` builds `vol_view` from stored readings and the page's rows |
| 5 | ot | the stock page site gives the builder `rd.view` |
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
return and is not detected. Implied is `market.iv_ibkr` of the day (IBKR's implied volatility, horizon not stated). `market.vol_edge` = implied minus forecast, an annual fraction
(0.03 is 3 volatility points). `market.vol_forecast_err` = the forecast the same method gives on the closes up to 20 sessions ago
minus `market.rv20` of today (the realised volatility of the 20 sessions since); positive means the forecast was too high. A missing
part makes the result missing with its reason, never a fill.

Day rule of `Combine`: the result's `Day` is the as-of day; its `DataDay` is the newest `DataDay` of the parts; `Observations` is the
count of parts; the result is `stale_input` when the as-of day is more than `max_age_days` after that `DataDay`. For the error the
first part (the past forecast) is dated 20 sessions back and the second (today's rv20) is today's, so the error is dated today; the
job stores the day of each part as a parameter of the reading, which the page does not show in this cut.

## Issues

Build order: every entry compiles and its own tests pass against main plus the entries it is ordered after. BASE is the merge base of the pull request with origin/main. A grep that "prints nothing" prints nothing and exits 1. An acceptance names a test by its function name, so that `git grep -n "func TestName"` finds it; what the test asserts is in the same item. The pull request of entry 2 carries the line `Contracts: changed (page.stock_option_page.v3 v3 (new))`; the others `Contracts: unchanged`.

### 1 vol-combine (interface)

Module: compute. A judged entry that also touches `docs` (the register file: `TestRulesMatchRegister` needs the rule rows and register entries in one pull request, as `docs/design/market-rv20.md` issue 3), `jobs` (the test fake `fakeFigs`, which stops compiling when `Combine` joins `Figures`) and the golden of the contract test (`figures.json` and the generator `tools/compute-golden/gen.py`, both in the compute row). Size: M.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `internal/compute/contracttest/contracttest_test.go`, `internal/compute/contracttest/figures.go`, `internal/compute/contracttest/golden_test.go`, `internal/compute/contracttest/testdata/figures.json`, `tools/compute-golden/gen.py`, `internal/compute/figures/combine.go`, `internal/compute/figures/combine_test.go`, `internal/compute/figures/series_test.go`, `internal/compute/rules/rules.json`, `internal/compute/rules/rules_test.go`, `internal/compute/own/testdata/stored_text.json`, `docs/contracts/compute.md`, `docs/conventions/register.toml`, `internal/jobs/computefigures/job_test.go`
Design: docs/design/vol-edge.md#interface, #the-method (Decisions 2, 3, 6)
Order: first. Entries 3 and 4 are after it.
Goal: declare `Figures.Combine`; implement `calc.Combine` for the conventions `vol_blend_v1` and `vol_diff_v1` with the day rule of `## The method`; add the five rules and five register entries; give the fakes `fakeFigures` (`api_test.go:143`, `contracttest_test.go:221`) and `fakeFigs` (`job_test.go:58`; `seriesFigs`, `hv_test.go:222`, embeds it) the method; add a `Combine` handler and a `parts` input to `contracttest/figures.go` and cases of group `Combine` to `figures.json` (an ok mean, an ok difference with a negative result, a missing part, a wrong count), with the generator and its provenance brought along (`TestProvenanceHashes`); add `"Combine"` to the method list of `internal/compute/figures/series_test.go` `TestContract`.
Acceptance:
1. `git grep -n "Combine(id string, parts \[\]Figure, asOf contracts.Day) Figure" -- internal/compute/api.go` prints one line, and `wc -l < internal/compute/api.go` is 839.
2. `git grep -c '"id":"market.rv60"' -- internal/compute/rules/rules.json` prints 1, and so for each of `"id":"market.rv120"`, `"id":"market.vol_forecast"`, `"id":"market.vol_edge"` and `"id":"market.vol_forecast_err"`; `git grep -n '"convention":"vol_diff_v1"' -- internal/compute/rules/rules.json` prints two lines (edge and error) and `git grep -n '"convention":"vol_blend_v1"' -- internal/compute/rules/rules.json` prints one.
3. `git grep -c 'id = "market.rv60"' -- docs/conventions/register.toml` prints 1, and so for the other four ids; no new entry has `pending-recompute` in its `history`: `git grep -n -A22 'id = "market.vol_edge"' -- docs/conventions/register.toml | grep -c pending-recompute` prints 0; each new entry has `implemented_by` naming `internal/compute/figures.(*calc).Combine` (edge, error, forecast) or `internal/compute/figures.(*calc).HistoricalVol` (rv60, rv120).
4. `internal/compute/figures/combine_test.go` has `TestCombineBlendMean` (0.20, 0.30, 0.40 give `0.300000`), `TestCombineDiffSign` (0.25 minus 0.20 is `0.050000`; 0.20 minus 0.25 is `-0.050000`), `TestCombineMissingPart` (one part missing gives missing `no_input`), `TestCombineBadCount` (one part for a blend of three, or three for a diff, gives missing `bad_input`) and `TestCombineDays` (the day rule: `DataDay` is the newest part's, a newest part older than `max_age_days` gives `stale_input`).
5. `git grep -c '"group": "Combine"' -- internal/compute/contracttest/testdata/figures.json` prints 4, and `internal/compute/figures/series_test.go` passes `"Combine"` to `contracttest.Figures` (`git grep -n '"Combine"' -- internal/compute/figures/series_test.go` prints a line).
6. `git grep -n "Combine" -- internal/compute/api_test.go internal/compute/contracttest/contracttest_test.go internal/jobs/computefigures/job_test.go` prints a line in each of the three files.
7. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/compute/... ./internal/jobs/computefigures -race -count=1` (the hash of each rule is the one `TestRulesMatchRegister` expects and the register `source_hash` is that value), `go run ./scripts/arch --check`.

### 2 vol-page-schema (interface)

Module: contracts. Size: S.
Files: `contracts/schemas/page.stock_option_page.v3.json`, `web/src/generated/stock_option_page.v3.ts`, `docs/contracts/page-schemas.md`
Design: docs/design/vol-edge.md#interface (item 2, Decision 4)
Order: parallel with 1; before 4 and 6.
Goal: the v3 schema and its generated types, produced by `pnpm --dir web run generate:types page.stock_option_page.v3`, not by hand; the pull request carries `Contracts: changed (page.stock_option_page.v3 v3 (new))`.
Acceptance:
1. `git grep -c '"vol_view"' -- contracts/schemas/page.stock_option_page.v3.json` prints a count of at least 2 (the property and the required list), and the schema has no `fit`: `git grep -n '"fit"' -- contracts/schemas/page.stock_option_page.v3.json` prints nothing.
2. The row label is a pattern, not an enum, and each row requires `data_day`: `git grep -n 'a-z0-9_\]{1,32}' -- contracts/schemas/page.stock_option_page.v3.json` prints a line; `python3 -c "import json;d=json.load(open('contracts/schemas/page.stock_option_page.v3.json'));print(sorted(d['\$defs']['VolRow']['required']))"` prints `['data_day', 'gap', 'label', 'text']` (the definition is named `VolRow`).
3. `git grep -n "vol_view" -- web/src/generated/stock_option_page.v3.ts` prints a line; the file is the generator's output (CI: `git diff --exit-code web/src/generated` after `pnpm --dir web run generate:types`).
4. `git grep -n "stock_option_page.v3" -- docs/contracts/page-schemas.md` prints a line that says v2 is kept.
5. `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: the contracts check, `go test ./internal/contracts -race -count=1`, the generated-types step of `ci-web`.

### 3 vol-figures

Module: jobs. Size: M.
Files: `internal/jobs/computefigures/vol.go`, `internal/jobs/computefigures/vol_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/computefigures/hv_test.go`, `internal/jobs/computefigures/valuation_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/vol-edge.md#the-method (Decisions 2, 5, 6)
Order: after 1.
Goal: for every subject of `listSubjects` (the underlyings with a stored chain), read the closes once and store `market.rv20`, `market.rv60`, `market.rv120` with `HistoricalVol`, then `market.vol_forecast` with `Combine` over the three, `market.vol_edge` with `Combine` over the stored `market.iv_ibkr` reading of the day (an ok row dated the as-of day, as `runIvStats` requires) and the forecast, and `market.vol_forecast_err` with `Combine` over the forecast computed on the closes up to 20 sessions before and today's rv20. Routing: `Plan` emits the six keys for each subject when `hvOn()` is true and none when it is false, except that for a name of `rv20_indices` it emits no `market.rv20` key (`runRv20` plans and stores those three); `Run` sends a key of the six new ids, `market.rv20` of a non-index name included, to the vol step before the default branch of `underlyingLists.add`, which sends other keys to the IVP and IVR step. The vol step has its own `volLookback`; each stored reading carries the day of each part as a parameter. A missing part gives a stored missing reading with its reason.
Acceptance:
1. `git grep -n 'rv60 = "market.rv60"' -- internal/jobs/computefigures/job.toml` prints a line, and so for `rv120 = "market.rv120"`, `vol_forecast = "market.vol_forecast"`, `vol_edge = "market.vol_edge"` and `vol_forecast_err = "market.vol_forecast_err"`.
2. `git grep -n "volLookback" -- internal/jobs/computefigures/vol.go` shows a constant of at least 230 (141 closes for the error are about 215 calendar days), and `hvLookback` is still the 70 that `hv_test.go` expects for `market.hv30_own`.
3. `vol_test.go` has these tests, each on the fake price reader of `hv_test.go` and the fake clock: `TestVolStoresAllFigures` (160 closes store rv20, rv60, rv120, forecast and error ok), `TestVolShortHistory` (70 closes: rv120 missing `short_history`, forecast missing `no_input`, rv20 and rv60 ok), `TestVolNoImplied` (no `market.iv_ibkr` reading: edge missing `no_input`, forecast ok), `TestVolIndexRv20Once` (SPX gets `market.rv20` once, from `runRv20`, and the other five ids from the vol step), `TestVolRoutesRv20OfStock` (a non-index name's `market.rv20` key is stored by the vol step, not by the IVP step: no `market.ivp_own` reading is made from it), `TestVolGatedByBars` (a nil `PriceBars` plans none of the six keys).
4. The key counts follow the six keys: `job_test.go:131-132` expects `4+3+len(marketIDs)` keys with bars nil and the new total with bars set (the line that says "13" is rewritten); `job_test.go:252`, `hv_test.go:174` (rv20 keys) and `valuation_test.go:208` (`4+1+3+len(marketIDs)+3`) are rewritten for the new total; `git grep -n "market.rv60" -- internal/jobs/computefigures/valuation_test.go internal/jobs/computefigures/hv_test.go` prints a line in each of the two files.
5. `docs/contracts/jobs/compute_figures.md` lists the six figures, the eleven keys per name (iv30, ivp, ivr, ivp_ibkr, hv30, rv20, rv60, rv120, forecast, edge, error), the order of the steps and the run bound of 10 minutes: `git grep -n "market.vol_forecast_err" -- docs/contracts/jobs/compute_figures.md` prints a line.
6. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/jobs/computefigures -race -count=1`, `go run ./scripts/arch --check`.

### 4 vol-view-builder

Module: pages. Size: M.
Files: `internal/pages/option_boards/stock_vol.go`, `internal/pages/option_boards/stock_vol_test.go`, `internal/pages/option_boards/page.go`, `internal/pages/option_boards/stock_build.go`, `internal/pages/option_boards/payload.go`, `internal/pages/option_boards/strings.go`, `internal/pages/option_boards/page_test.go`, `internal/pages/option_boards/quote_test.go`, `internal/pages/option_boards/stock_schema_test.go`, `internal/pages/option_boards/stock_page_test.go`, `internal/pages/option_boards/names_test.go`, `internal/pages/option_boards/testdata/stock_option_page_ok.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_open.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_no_key.golden.json`, `docs/contracts/pages.md`
Design: docs/design/vol-edge.md#interface (item 2), Decisions 4, 7, 8, 9
Order: after 1, 2 and 3.
Goal: `StockConfig` gets `Readings store.ReadingViewReader` (nil gives every row a named gap, never an empty block); `BuildStock` writes `Meta.Schema` `page.stock_option_page.v3` and `SchemaVersion: 3` and fills `vol_view` from the key alone: the ticker of the key (upper case) is the subject of the stored figures `market.iv_ibkr`, `market.rv20`, `market.rv60`, `market.rv120`, `market.vol_forecast`, `market.vol_edge`, `market.ivp_ibkr` and `market.vol_forecast_err`; each row carries the `data_day` of its reading; a row with no ok reading is a gap with the reason of the stored missing reading, else `no_input`. A key with no candidate row, or a name with no stored figure (no chain, or an ambiguous ticker), still gets the view, every row a gap. Events are the stored `NextEarnings`, `EarningsState`, `NextExDate` of the first row of the name, or absent. The one comparison the page makes is Decision 8. No `Fits`.
Acceptance:
1. `git grep -n "Readings .*store.ReadingViewReader" -- internal/pages/option_boards/page.go` prints a line; `git grep -n 'page.stock_option_page.v3' -- internal/pages/option_boards/page.go internal/pages/option_boards/page_test.go internal/pages/option_boards/quote_test.go internal/pages/option_boards/stock_schema_test.go` prints a line in each of the four files; `git grep -n 'page.stock_option_page.v2' -- internal/pages/option_boards ':!*golden*'` prints nothing.
2. The page reads by id and computes nothing: each of `"market.iv_ibkr"`, `"market.rv20"`, `"market.rv60"`, `"market.rv120"`, `"market.vol_forecast"`, `"market.vol_edge"`, `"market.ivp_ibkr"`, `"market.vol_forecast_err"` is found by `git grep -c '<id>' -- internal/pages/option_boards/stock_vol.go` printing at least 1; `git grep -nE "math\.|big\.|strconv\.Parse(Float|Int)" -- internal/pages/option_boards/stock_vol.go` prints nothing.
3. `stock_vol_test.go` has `TestVolViewNoRow` (a key with no candidate row and stored readings: `vol_view` not null, eight rows), `TestVolViewBlockedRow` (a blocked row: the same), `TestVolViewNoFigures` (a name with no stored figure: eight gap rows, not null), `TestVolViewNoImplied` (no `market.iv_ibkr`: `iv_name` is a gap with a reason, `rv20`, `rv60`, `rv120` are not gaps), `TestVolViewDataDay` (each ok row has its reading's day) and `TestVolViewNilReader` (all eight rows are gaps).
4. The sentence is English, built in Go, and says what it does not know: `strings.go` holds one text per case (above the dead band, below it, inside it, no edge) and each contains the words "does not know": `git grep -c "does not know" -- internal/pages/option_boards/strings.go` prints at least 4 on lines of its own; no new text contains advice words: the added lines of `git diff BASE -- internal/pages/option_boards/strings.go` contain none of "sell", "buy", "should", "recommend" (`git diff BASE -- internal/pages/option_boards/strings.go | grep '^+' | grep -ciE "sell|buy|should|recommend"` prints 0); `TestVolSentenceCases` asserts the text chosen for edges `0.030000`, `-0.030000`, `0.004000`, `-0.004000` and a missing edge.
5. The built pages validate against v3: `TestStockSchemaV3` in `stock_schema_test.go` validates the three golden files and a built page; `git grep -n '"schema_version": 3' -- internal/pages/option_boards/testdata/stock_option_page_ok.golden.json` prints a line, and so for `_open` and `_no_key`.
6. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/pages/... -race -count=1`, `go run ./scripts/arch --check`.

### 5 vol-view-wire

Module: ot. Size: S (a few lines).
Files: `cmd/ot/stock_option_site.go`, `cmd/ot/stock_option_site_test.go`, `docs/contracts/ot/pages/stock_option_page.md`
Design: docs/design/vol-edge.md Decisions 5, 7, 10
Order: after 4.
Goal: `stockBuild` passes `rd.view` (the `siteReaders.view store.ReadingViewReader` that `cmd/ot/site.go:114` already opens at `site.go:237` and closes) to `StockConfig.Readings`; a nil `view` passes nil. Nothing is opened anew and `compute_figures` is not wired here (Decision 5).
Acceptance:
1. `git grep -n "Readings: *rd.view" -- cmd/ot/stock_option_site.go` prints a line.
2. `cmd/ot/stock_option_site_test.go` has `TestStockPageVolView` (a fake reader whose readings reach the page JSON as a non-null `vol_view` with eight rows) and `TestStockPageNoViewReader` (a nil view still builds, and `vol_view` rows are gaps).
3. `git grep -n "stock_option_page.v3" -- docs/contracts/ot/pages/stock_option_page.md` prints a line.
4. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`, `go run ./scripts/arch --check`.

### 6 vol-view-web

Module: web. Size: S.
Files: `web/src/pages/stockoption/Vol.tsx`, `web/src/pages/stockoption/view.tsx`, `web/src/types/stock_option_page.ts`, `web/test/pages/stockoption/StockOptionView.test.tsx`, `web/test/pages/stockoption/fixtures/ok.ts`, `web/test/pages/stockoption/fixtures/gap.ts`
Design: docs/design/vol-edge.md Decision 11
Order: after 2 (parallel with 3 to 5; the page shows real data only after 5).
Goal: render `vol_view` as a table (label, text and its data day, or the gap and its reason), the events and the sentence, with no arithmetic. `SUPPORTED_VERSIONS` becomes `[1, 2, 3]`; version 1 and 2 payloads still validate and `bad-version.json` (version 9) stays refused.
Acceptance:
1. `git grep -n "vol_view" -- web/src/pages/stockoption/Vol.tsx` prints a line; `git grep -nE "parseFloat|Number\(|toFixed|Math\." -- web/src/pages/stockoption/Vol.tsx` prints nothing.
2. `git grep -n "SUPPORTED_VERSIONS = \[1, 2, 3\]" -- web/src/types/stock_option_page.ts` prints a line.
3. `StockOptionView.test.tsx` has the cases `renders the vol view`, `shows a gap row with its reason`, `renders with a null vol_view` and `validates a version 3 payload` (`git grep -n "renders the vol view\|shows a gap row with its reason\|renders with a null vol_view\|validates a version 3 payload" -- web/test/pages/stockoption/StockOptionView.test.tsx` prints four lines).
4. No new package: `git diff --name-only BASE -- package.json pnpm-lock.yaml frontend-whitelist.json` prints nothing.
5. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, the build.

## Left out

Second cut (named, with the reason):

- The grade of every sale. One population, closed early or held: premium kept, assigned or not, days held, realised volatility over the life against implied at entry, the held-to-expiry group beside the bought-back group (40 percent assignment against about 90 percent kept is a selection effect, not a verdict), never a win rate alone; the owner's habit (about 35 DTE in, out near 21 to 23 days or 90 percent kept) shown as kept or broken, without proposing another rule. Needs a read-time join of the Flex trades (the option ledger) with the readings of this cut, a page schema (`option_result_page` v2) and a builder. For history since 2025-12-01 the entry implied is `market.iv_ibkr` of the entry day, marked "name-level, not the contract's"; moneyness at entry (strike over that day's close from bars) stands in for delta. Decision 13 says why it waits.
- Figures for followed names outside the options group (held shares with no chain): a reader of stock and ETF instruments by name in `store` that does not create (the model is `instrument.OpenIndexIDs`), a `Followed` list in `compute_figures`, and its wiring in `ot`; the first item of the second cut.
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
5. Figures are stored per day, not on demand (the page computes nothing, and a stored reading per day is the entry fact of a later grade). The subjects are the underlyings with a stored chain (`listSubjects`, which is the options group: `iv_history`, option capture and candidates all follow that group, `cmd/ot/followfilter.go`). The advisor's R2 (every held name, the options group or not) is narrowed to those names, because the step that would reach the others does not exist on main: the only function that turns an identifier into an instrument id is `ResolveInstrument` (`internal/store/api.go`, called at `internal/jobs/barsrefresh/run.go:86` with `Scheme: "ibkr_conid"`), and it creates the instrument when none exists; the only read of instruments by name is `instrument.OpenIndexIDs` (`internal/store/instrument/index_ids.go`), which selects `instrument_type = 'index'` only. A reader of stock and ETF instruments by name in `store`, a `Followed` list in the job and its wiring in `ot` is the first item of the second cut. Until then a held name outside the options group shows its view as named gaps with the reason `no_input`, and says why: no stored option chain.
6. The forecast's past error is in the cut: the owner is told how wrong the number has been, per name, from the bars, not left to trust a model choice. It is one stored figure a day (forecast of 20 sessions ago minus the realised 20 sessions since), which accumulates into a series; the mean over a year is the second cut. It is computable today from the 400 days of bars, with no waiting (rv20 and the forecast of any past day are pure functions of stored closes), so the cut does not wait for days to pass.
7. The view is built from the key alone, whatever the gate status of the rows and whether the key has rows at all, because the owner picks the name first (valuation and buy price) and never traded from the old scan. The page route and the readings are by ticker. A name whose ticker is ambiguous has the figure subject `underlying:<id>` and so no reading under its ticker: it shows gaps with `no_input`, like any name with no figure; no separate reason is built.
8. The sentence is English, built in Go from fixed texts, and chosen by the stored edge text with a dead band of one volatility point (`0.010000`) around zero: above the band, below it, inside it, or no edge (it names the missing input). The comparison is of the sign and of two fixed-width decimal texts, which the page does by text and not by number; whether a reviewer reads that as computing is Doubt 3. The sentence never reads as trade advice: it states the difference between two stored numbers and what the model does not know (the next earnings move, skew, how well the blend forecasts this name, shown by the error beside it, early assignment, the horizon of implied, which is not stated). It is not a rank and not a score. One vol point is the stored scale of IBKR's IV bar; the owner may move the band (Doubt 4).
9. Events: the stored `NextEarnings`, `EarningsState` and `NextExDate` as stored; the "inside the life of the option" verdict is the stored earnings state. For ex-dividend the page shows the date and gives no verdict (a verdict needs the expiry, which is per contract; the call page is the second cut).
10. `ot` passes `rd.view`, already opened for the site; no new database role and no new opener.
11. The web renders; it holds no arithmetic and adds no package (recharts and @tanstack/react-table stay the whitelist). Versions 1 and 2 of the page stay readable.
12. Six entries, one over the cut of 3 to 5. Five modules are touched by the smallest honest path (compute, contracts, jobs, pages, web) and the sixth, `ot`, is the composition root that gives the builder its reader and the job its names (`AGENTS.md`: a path belongs to one module). Moved out to hold the line: the grade, the call page, the candidates-page columns, the per-strike view and the second forecast.
13. The per-sale record waits for the second cut and is derived, not a new table. The name-level inputs (the forecast, the windows and `market.iv_ibkr` of each day) are kept per day by this cut and by `iv_history`, and the Flex rows are stored insert-only, so a grade by the name-level facts can be built at any time with nothing lost by waiting. What a table would add is the contract's own IV and delta at entry outside the capture band, and that is a collection question (late-collection storage), not a table question: a table written after the sale cannot recover a chain that was not captured. So the first cut does not build a table, and the second cut decides the contract-level entry facts together with that design.
14. JP is not in this cut. Reason: `compute_figures` runs on the XNYS calendar and the stored chains of the JP option jobs are separate; whether IBKR sends IV for OSE options is unverified; the page already says "Japan options: later design (option-candidates-jp)" (`jpNote` in `internal/pages/option_boards/strings.go:11`). A JP name has no figure unless it has option_eod rows on the US-calendar day; no JP-specific figure is added.
15. The owner's habit (about 35 DTE in, out near 21 to 23 days or 90 percent kept, held to expiry sometimes, covered calls a core strategy) guides what the sentence must not contradict; no rule is proposed and the habit as kept or broken belongs to the grade (second cut).

## Doubts

1. Entry count: six against a cut of 3 to 5 (Decision 12). No two can merge: entry 1 is already a judged entry across compute, docs and a test fake of jobs, and `ot` and `web` are other modules. If the cap is hard, dropping 5 and 6 together stores and builds the view but does not show it; dropping 3 computes at read time, which breaks "pages compute nothing".
2. Entry 1 names three modules although each entry should edit one. The register file is `docs/**`; a compute-only change fails `TestRulesMatchRegister`, a docs-first one fails because `implemented_by` must name an existing function, and the fake `fakeFigs` of jobs stops compiling when `Combine` joins `Figures`. The precedent (`docs/design/market-rv20.md` issue 3) is on main for compute and docs; the jobs test file is new here. The alternative is a separate small interface for `Combine` in jobs, which `compute/api.go` cannot hold (ceiling).
3. The page compares two fixed-width decimal texts for the dead band and the sign. The rule "the page computes nothing" is the reviewer's to read; the alternative is a stored word (`above`, `below`, `inside`) from `Combine`, which would put a threshold into a compute convention.
4. The dead band of one vol point is mine, not the owner's. It sits in the page's strings and is changed there.
5. The advisor's R2 is narrowed (Decision 5): a held name outside the options group gets no figures in this cut. Names the owner holds and does not sell options on wait for the second cut.
6. History depth: rv120 needs 121 closes and the error 141; the bars job backfills 400 calendar days (about 275 sessions) at `backfill_names_per_run = 10`, so a new name needs several days before its history exists, and a name with fewer shows a named gap. How far back the real store reaches is [unverified]. Whether `deepen_names_per_run = 0` switches the 5-year deepening off is [unverified].
7. The forecast is a model choice. A blend of past windows ignores mean reversion, events and the market's own information; for a name just after earnings the 20-day window is dominated by one jump. The page says so and shows the error beside it.
8. Run size: eleven keys per name instead of five. At 100 names that is about 1,100 keys, each one read and one insert of the readings store; `run_bound = "10m"` is unchanged and the estimate (a few round trips per key) is far inside it, but the real number of names and the round-trip time are [unverified]. The contract text states the count so that a later rise is seen.
9. An index name that is also a subject (SPX with chain rows) must get `market.rv20` once, from `runRv20`; entry 3 `TestVolIndexRv20Once` tests it.
10. The close-to-close figures count stored rows: a session missing from bars becomes a two-day return and is not detected; closes are IBKR-split-adjusted and not dividend-adjusted. The sentence does not claim more.

## Adversarial

Round 1 (on revision 1): one read-only sub-agent, given the text and the repository and not the conversation; its answer is saved as `vol-edge-adversarial-r1.md` in the designer's scratchpad `designs/` directory (outside the repository). Revision 2 follows a second check of revision 1 by the advisor (`vol-edge-review-r1.md`, outside the repository) and the advisor's requirements R1 to R6; a second adversarial pass on the pushed head is run by the coordinator. Answers to round 1:

1. Gap against the goal. Adopted: the view is name-level and says so (Goal), the figures are stored for the followed names and not only for the watch list, and implied missing is a named gap beside realised figures. The magnitude bands asked for are replaced by one dead band (Decision 8), which is the owner's parameter to move.
2. Biggest unnamed risk. Adopted: `contracttest_test.go` and `contracttest/figures.go` are in entry 1; the fake `fakeFigs` of jobs is added (the check of revision 1 found it); the way to get the rule hash is stated in entry 1.
3. A simpler way. Not adopted for the forecast: the owner asked for a stated forecast and the error needs it stored. Adopted for entry 5: the reader is `rd.view`.
4. The lock-in. Adopted: the row label is a string; the ceiling of `api.go` is named in Decision 3; the stored ids carry the method (a change is a new rule version).
5. The claim most likely false. Adopted: "every name" is replaced by the subjects of Decision 5; the order of `iv_history` (close + 10 minutes) and `compute_figures` (+ 20) is settled by reading; the count of readings is corrected.
6. Likeliest slip. Adopted: the two fakes; the generator is `pnpm --dir web run generate:types`; `hvLookback` stays 70 and the new steps have `volLookback`. Doubt 2 stays the likeliest review stall.

Round 2 (on revision 2, head 26789d0c; `vol-edge-adversarial-26789d0c.md` in the same scratchpad directory). Answers:

1. B1, the test files that count keys: adopted; `valuation_test.go` (line 208), `job_test.go` and `hv_test.go` are in entry 3 and named in its acceptance 4.
2. B2, the unverified id lookup: settled by reading and R2 narrowed (Decision 5): main has no function that gives an instrument id for a name without creating one, so `Followed` and its wiring are out of the cut and in the second cut; entry 5 is only `rd.view`.
3. `market.iv_ibkr` is not known to be a 30-day measure: adopted, the text says "horizon not stated" and no like-with-like claim is made (Goal, The method, Decision 8).
4. The generator argument is `page.stock_option_page.v3`: adopted. Routing of `market.rv20` of a non-index name, and the day rule of `Combine`: adopted (entry 3 Goal, `## The method`). `ambiguous_name`: dropped (Decision 7). Acceptance items that already pass on main were removed, `git grep -c` counts were replaced by exact greps and named tests, and BASE is defined. A `figures.json` case for `Combine` is in entry 1.
5. Not adopted: dropping `Combine` for sums in the job (the rule hash and `implemented_by` need a function in compute; Decision 3); the cut of the forecast and error from this cut (R4).

