# Vol edge: is a sale paid for? Implied volatility against realised and a stated forecast, per name

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 5
Status: draft, under review in the pull request that adds this file. Read on origin/main of 2026-11-12 after #1758 (the branch holds a merge of it). "[unverified]" marks a claim not checked in code, docs or a probe. Revision 2 answers the check of revision 1 (`vol-edge-review-r1.md`, outside the repository) and the adversarial pass r1 (`## Adversarial`); revision 3 the pass on 26789d0c; revision 4 the checks `vol-edge-review-r2.md` and the confirming pass on 95b5f08a; revision 5 the crew design session on db5819e3 and the advisor's cutting items.

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
  implied minus forecast; implied's percentile in its year; the past error of the forecast; and one English sentence that says
  what it does not know (the earnings and ex-dividend dates are the second cut, `## Left out`);
- name-level, and the page says so in the `method` text of the block (Interface item 4; entry 4 acceptance 4): the figures belong to
  the name, not to a strike or an expiry. They do not follow the owner's choice of strike (no skew, `## Left out`). They serve a
  covered call as well as a short put, because a call on the name is compared with the same forecast; the call page is the second
  cut (the page loads short puts only: `internal/pages/option_boards/load.go:16,30`);
- every figure carries the data day of its reading, and a missing input is named with its reason, never filled: a name with no
  `market.iv_ibkr` shows the row `iv_name` as a gap with its reason code (`no_input`) and still shows its realised figures (the
  codes are in Interface item 4);
- a name gets stored figures on a day when it has an option chain stored for that day (`listSubjects`), and the page builds the view
  from the key alone (Decision 7), so a name with no candidate row still gets a view: the newest stored figures of the last 7
  days, each with its own day, else a named gap. Who falls in the gap: (a) a name of the options group on a day when its chain
  capture failed has no figures for that day and shows the previous day's, dated, or a gap if none is within 7 days; (b) a held
  stock with no options; (c) a name that has a held option position but is outside the options group: not flagged `options`,
  paused, retired, pending or not listed (`cmd/ot/followfilter.go:19-40`; nothing adds the underlying of an option position to the
  group), so it has no chain, no `market.iv_ibkr` and no candidate row. For (b) and (c) the view is a named gap (`no_input`). Case (c)
  is the one the owner meets when a position outlives the flag. It is closed by a separate issue, "the underlying of every held
  option position is followed", written outside this design; until it lands the page shows the gap. Figures for (b) need a
  name-to-instrument reader that does not create instruments, which main does not have (Decision 5); that is the second cut;
- the forecast's past error is in this cut: the forecast made 20 sessions ago against the realised volatility of the 20 sessions
  since, per name, from bars (Decision 6);
- the figures are stored once a day from the day this lands. The windows of an earlier day are pure functions of stored bars, but
  the bars are backfilled 400 calendar days, so for the oldest sales the wait is not free (Decision 12). The grade of each sale is
  the second cut (`## Left out`, Decision 12).

No pricer, no new collection, the candidate ranking unchanged. Like with like is not claimed: `market.iv_ibkr` is IBKR's daily implied-volatility bar, whose horizon this repository does not know
(it is never mixed with the own IV30), and the realised figures are annualised with 252 sessions (`HistoricalVol`). Both are annual
fractions shown in volatility points; the page's `method` sentence says the horizon of implied is not stated, so the edge is a
difference of two numbers of possibly different tenor, not a measured premium.

## What main holds today

- Figures are rules plus a Go method: `internal/compute/rules/rules.json` (22 lines, one rule per line), `compute.Figures`
  (`internal/compute/api.go:172`; `HistoricalVol` takes any N from the rule: `internal/compute/figures/series.go`), a register
  entry per rule in `docs/conventions/register.toml`, `TestRulesMatchRegister` (`internal/compute/rules/rules_test.go:367`)
  that needs the rule row and its register entry in one pull request, and `TestHashesMatchSource` (`rules_test.go:252`) that needs the
  hash of each rule and the register `source_hash` to be the hash of its `implemented_by` source (precedent: `docs/design/market-rv20.md` issue 3).
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
  `store/api.go:587`); its schema `contracts/schemas/page.stock_option_page.v2.json` has fixed blocks. A key with no stored row gives an
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
| 4 | pages | `option_boards.BuildStock` builds `vol_view` from stored readings, by the key |
| 5 | ot | the stock page site gives the builder `rd.view` |
| 6 | web | the stock option page shows `vol_view` |

Module dependencies follow `docs/design/modules.md`: jobs and pages depend on compute and store; web on contracts; ot on all.
Entry 6 is over the cut of 3 to 5 (Decision 11, Doubt 1).

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
2. contracts, `contracts/schemas/page.stock_option_page.v3.json`: the v2 schema plus the required property `vol_view`, always an object
   (never null: with no reader or no figure its rows are gaps). The v3 schema has no null of its own: `TestGeneratedV3Types` fails a generated v3 file that holds `null` unless its schema refs a v2 page file (`internal/contracts/generated_ts_test.go:107`), so `data_day` is a string, the empty string when a row has no reading (pattern `^([0-9]{4}-[0-9]{2}-[0-9]{2})?$`). Whether the file copies v2 or refs it is decided by that test (Doubt 11). One block per page: `name` (text), `method` (the text of item 4: the forecast, the two annualisations, that implied and the forecast are not the same horizon, and that the figures are name-level), `rows`
   (an open list of `{label, text, data_day, gap}`, label a string of the pattern `^[a-z0-9_]{1,32}$`, not an enum; `text` the stored
   decimal shown in volatility points or as a share, `data_day` the `Day` of the stored reading, empty when there is none, `gap` a named gap with its reason code in
   place of `text`), `coaching` (one English sentence). There is no events block in this cut (`## Left out`). The
   builder writes the rows `iv_name`, `rv20`, `rv60`, `rv120`, `forecast`, `edge`, `ivp`, `forecast_err`, in that order. There is no
   `fit` and no per-contract field: the block is name-level. v2 and v1 stay beside it.
3. Rules, read by `jobs` and `pages` by id string only: `market.rv60` (convention `rv60_v1`, n 60), `market.rv120` (`rv120_v1`, n 120),
   `market.vol_forecast` (`vol_blend_v1`), `market.vol_edge` (`vol_diff_v1`), `market.vol_forecast_err` (`vol_diff_v1`); subject
   kind `underlying`, unit decimal, scale 6, trading days 252, max age 4 days.
4. Texts, built in Go from fixed strings in `internal/pages/option_boards/strings.go` (the package's file of texts), chosen by the
   stored edge text. `text` of a ready row is the stored decimal text unchanged; `gap` of a gap row is a reason code and the page writes
   no prose for a gap; the web shows the row label and the code as given. The codes, all of them stored reasons of `compute`: `no_input`
   (no ok reading of that id within the window, which covers a missing implied value (`iv_name` is `no_input`), a name with no stored
   chain (every row is `no_input`), a nil reader, an unknown or ambiguous ticker, and a stored missing reading whose own reason is
   `no_input`), `short_history`, `stale_input` and `bad_input` (a stored missing reading carries its own code and its `Day` as
   `data_day`). So "implied missing" is the row `iv_name` with gap `no_input` beside rows that still carry figures, and "no stored
   option chain" is eight `no_input` rows with the `coaching` text of no edge; neither is a sentence of the page.
   - `method`: "Implied is IBKR's implied volatility of the name; its horizon is not stated. The forecast is the plain mean of the
     realised volatility of the last 20, 60 and 120 sessions, each annualised with 252 sessions. Implied and the forecast are not the
     same horizon, so the edge is a difference of two numbers of possibly different tenor. The figures belong to the name, not to a
     strike or an expiry, and are read the same for a short put and a covered call."
   - the tail of every `coaching` text: "This page does not know the next earnings move, the skew, how well this forecast has fitted
     this name (see the error row), early assignment, or the horizon of implied. One past error is shown, not a track record."
   - `coaching`, edge above the dead band (stored edge above `0.010000`): "Implied volatility is above the forecast by the edge
     shown." + tail. Below (edge below `-0.010000`): "Implied volatility is below the forecast by the edge shown." + tail. Inside
     (`-0.010000` to `0.010000`, both ends inside): "Implied volatility is within one volatility point of the forecast." + tail.
     No edge: "No edge is shown because an input is missing; the rows name it." + tail.
   - no text names an action; the words sell, buy, should, recommend and advice appear in none of them.
## The method

Forecast `market.vol_forecast` = the plain mean of `market.rv20`, `market.rv60` and `market.rv120` of the same day: close-to-close
log returns of the stored `trades` closes (IBKR split-adjusted, not dividend-adjusted: `docs/design/market-bars.md`), sample standard
deviation (ddof 1), times the square root of 252, as `HistoricalVol` does. It counts stored rows, so a missing session is a two-day
return and is not detected. Implied is `market.iv_ibkr` of the day (IBKR's implied volatility, horizon not stated). `market.vol_edge` = implied minus forecast, an annual fraction
(0.03 is 3 volatility points). `market.vol_forecast_err` = the forecast the same method gives on the closes up to 20 sessions ago
minus `market.rv20` of today (the realised volatility of the 20 sessions since); positive means the forecast was too high. A missing
part makes the result missing with its reason, never a fill.

The past forecast is exact: with the stored closes numbered 0 to n-1 (n-1 is the as-of session), it is `HistoricalVol` for n 20, 60
and 120 on closes 0 to n-21, each called with as-of day = the day of close n-21 (not today's, or all three would be `stale_input`),
then `Combine` of `vol_blend_v1` over those three with the same as-of day. The error is then `Combine` of `vol_diff_v1` over the
past forecast and today's rv20, with today as as-of day. The returns of today's rv20 start after close n-21, so the two windows do
not overlap. The error needs 141 closes (121 for the past rv120 and 20 since).

Day rule of `Combine`: the result's `Day` is the as-of day; its `DataDay` is the newest `DataDay` of the parts; `Observations` is the
count of parts; the result is `stale_input` when the as-of day is more than `max_age_days` after that `DataDay`. For the error the
first part (the past forecast) is dated 20 sessions back and the second (today's rv20) is today's, so the error is dated today; the
job stores the day of each part as a parameter of the reading, which the page does not show in this cut. The page's day for a
row is the stored reading's own `Day` (`iv_history` stores no `data_day` parameter).

## Issues

Build order: every entry compiles and its own tests pass against main plus the entries it is ordered after. BASE is `git merge-base origin/main HEAD` of the pull request. A grep that "prints nothing" prints nothing and exits 1. An acceptance names a test by its function name, so that `git grep -n "func TestName"` finds it; what the test asserts is in the same item. The pull request of entry 2 carries the line `Contracts: page.stock_option_page.v3 v3 (new)` (the form `.github/workflows/checks.yml` accepts); the others, and this design's own pull request, `Contracts: unchanged`.

### 1 vol-combine (interface)

Module: compute. A judged entry that also touches `docs` (the register file: `TestRulesMatchRegister` and `TestHashesMatchSource` need the rule rows and register entries in one pull request, as `docs/design/market-rv20.md` issue 3), `jobs` (the test fake `fakeFigs`, which stops compiling when `Combine` joins `Figures`) and the golden of the contract test (`figures.json`, hand-written: it has no provenance key and `TestProvenanceHashes` covers only `indicators.json` and `iv30_independent.json`, so the generator `tools/compute-golden/gen.py` is not touched). Size: M.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `internal/compute/contracttest/contracttest_test.go`, `internal/compute/contracttest/figures.go`, `internal/compute/contracttest/golden_test.go`, `internal/compute/contracttest/testdata/figures.json`, `internal/compute/figures/combine.go`, `internal/compute/figures/combine_test.go`, `internal/compute/figures/series_test.go`, `internal/compute/rules/rules.json`, `internal/compute/rules/rules_test.go`, `internal/compute/own/testdata/stored_text.json`, `docs/contracts/compute.md`, `docs/conventions/register.toml`, `internal/jobs/computefigures/job_test.go`
Design: docs/design/vol-edge.md#interface, #the-method (Decisions 2, 3, 6)
Order: first. Entries 3 and 4 are after it.
Goal: declare `Figures.Combine`; implement `calc.Combine` for the conventions `vol_blend_v1` and `vol_diff_v1` with the day rule of `## The method`; add the five rules and five register entries; give the fakes `fakeFigures` (`api_test.go:143`, `contracttest_test.go:221`) and `fakeFigs` (`job_test.go:58`; `seriesFigs`, `hv_test.go:222`, embeds it) the method; add a `Combine` handler and a `parts` input to `contracttest/figures.go` and cases of group `Combine` to `figures.json` (an ok mean, an ok difference with a negative result, a missing part, a wrong count), written by hand with the `working` and `tolerance` of the existing cases; add `"Combine"` to the method list of `internal/compute/figures/series_test.go` `TestContract`.
Acceptance:
1. `git grep -n "Combine(id string, parts \[\]Figure, asOf contracts.Day) Figure" -- internal/compute/api.go` prints one line, and `wc -l < internal/compute/api.go` is 839.
2. `git grep -c '"id":"market.rv60"' -- internal/compute/rules/rules.json` prints 1, and so for each of `"id":"market.rv120"`, `"id":"market.vol_forecast"`, `"id":"market.vol_edge"` and `"id":"market.vol_forecast_err"`; `git grep -n '"convention":"vol_diff_v1"' -- internal/compute/rules/rules.json` prints two lines (edge and error) and `git grep -n '"convention":"vol_blend_v1"' -- internal/compute/rules/rules.json` prints one.
3. `git grep -c 'id = "market.rv60"' -- docs/conventions/register.toml` prints 1, and so for the other four ids; no new entry has `pending-recompute` in its `history`: `git grep -n -A22 'id = "<id>"' -- docs/conventions/register.toml | grep -c pending-recompute` prints 0 for each of the five ids (`TestNoPendingForGoIds` in `rules_test.go` also decides it, under Checked by CI); each new entry has `implemented_by` naming `internal/compute/figures.(*calc).Combine` (edge, error, forecast) or `internal/compute/figures.(*calc).HistoricalVol` (rv60, rv120).
4. `internal/compute/figures/combine_test.go` has `TestCombineBlendMean` (0.20, 0.30, 0.40 give `0.300000`), `TestCombineDiffSign` (0.25 minus 0.20 is `0.050000`; 0.20 minus 0.25 is `-0.050000`), `TestCombineMissingPart` (one part missing gives missing `no_input`), `TestCombineBadCount` (one part for a blend of three, or three for a diff, gives missing `bad_input`) and `TestCombineDays` (the day rule: `DataDay` is the newest part's, a newest part older than `max_age_days` gives `stale_input`).
5. `git grep -n '"group": "Combine"' -- internal/compute/contracttest/testdata/figures.json` prints four lines (the grep counts lines, so the count equals the cases only because each case carries its `"group"` key once, on its first line, as the existing cases do), `git diff BASE -- tools` prints nothing, and `internal/compute/figures/series_test.go` passes `"Combine"` to `contracttest.Figures` (`git grep -n '"Combine"' -- internal/compute/figures/series_test.go` prints a line).
6. `git grep -n "Combine" -- internal/compute/api_test.go internal/compute/contracttest/contracttest_test.go internal/jobs/computefigures/job_test.go` prints a line in each of the three files.
7. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/compute/... ./internal/jobs/computefigures -race -count=1` (`TestRulesMatchRegister` and `TestHashesMatchSource` and `TestNoPendingForGoIds`: the hash of each rule and the register `source_hash` are the hash of the `implemented_by` source), `go run ./scripts/arch --check`.

### 2 vol-page-schema (interface)

Module: contracts. Size: S.
Files: `contracts/schemas/page.stock_option_page.v3.json`, `web/src/generated/stock_option_page.v3.ts`, `docs/contracts/page-schemas.md`
Design: docs/design/vol-edge.md#interface (item 2, Decision 4)
Order: parallel with 1; before 4 and 6.
Goal: the v3 schema and its generated types, produced by `pnpm --dir web run generate:types page.stock_option_page.v3`, not by hand; the pull request carries `Contracts: page.stock_option_page.v3 v3 (new)`.
Acceptance:
1. `git grep -c '"vol_view"' -- contracts/schemas/page.stock_option_page.v3.json` prints a count of at least 2 (the property and the required list), and the schema has no `fit`: `git grep -n '"fit"' -- contracts/schemas/page.stock_option_page.v3.json` prints nothing.
2. The row label is a pattern, not an enum, and each row requires `data_day`: `git grep -n 'a-z0-9_\]{1,32}' -- contracts/schemas/page.stock_option_page.v3.json` prints a line; `python3 -c "import json;d=json.load(open('contracts/schemas/page.stock_option_page.v3.json'));print(sorted(d['\$defs']['VolRow']['required']))"` prints `['data_day', 'gap', 'label', 'text']` (the definition is named `VolRow`).
3. `git grep -n "vol_view" -- web/src/generated/stock_option_page.v3.ts` prints a line; the file is the generator's output (CI: `git diff --exit-code web/src/generated` after `pnpm --dir web run generate:types`).
4. `git grep -n "stock_option_page.v3" -- docs/contracts/page-schemas.md` prints a line that says v2 is kept.
5. `internal/contracts/carryover_test.go` is not edited: on main it lists no `pageNewFiles`; `classifyPages` takes a page file with no `carryover.sha256` line, outside the v2 and v3 lines, and an `$id` of this repository as new. So the v3 file has the `$id` `https://optdesk.invalid/schemas/page.stock_option_page.v3.json` (`git grep -n '"$id": "https://optdesk.invalid/schemas/page.stock_option_page.v3.json"' -- contracts/schemas/page.stock_option_page.v3.json` prints a line) and is in no list. The sentence of `docs/contracts/page-schemas.md` that names `pageNewFiles` (line 68) is stale and may be corrected in that file, which is in Files.
6. `data_day` is a string, never null, and `vol_view` is required and not nullable: `python3 -c "import json;d=json.load(open('contracts/schemas/page.stock_option_page.v3.json'));print('null' in json.dumps(d['\$defs']['VolRow']), d['\$defs']['VolRow']['properties']['data_day']['type'])"` prints `False string`.
7. `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: the contracts check, `go test ./internal/contracts -race -count=1` (`TestGeneratedV3Types`, `TestGeneratedV3NullRule`, the carryover classification), the generated-types step of `ci-web`.

### 3 vol-figures

Module: jobs. Size: M.
Files: `internal/jobs/computefigures/vol.go`, `internal/jobs/computefigures/vol_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/computefigures/hv_test.go`, `internal/jobs/computefigures/valuation_test.go`, `internal/jobs/computefigures/rv_test.go`, `internal/jobs/computefigures/ease_test.go`, `internal/jobs/computefigures/ivp_ibkr_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/vol-edge.md#the-method (Decisions 2, 5, 6)
Order: after 1 (parallel with 4).
Goal: for every subject of `listSubjects` (the underlyings with a stored chain), read the closes once and store `market.rv20`, `market.rv60`, `market.rv120` with `HistoricalVol`, then `market.vol_forecast` with `Combine` over the three, `market.vol_edge` with `Combine` over the stored `market.iv_ibkr` reading of the day (an ok row dated the as-of day, as `runIvStats` requires) and the forecast, and `market.vol_forecast_err` with `Combine` over the past forecast (the exact definition in `## The method`, each past window with the as-of day of the past session) and today's rv20. Routing: `Plan` emits the six keys for each subject when `hvOn()` is true and none when it is false, except that for a name of `rv20_indices` it emits no `market.rv20` key (`runRv20` plans and stores those three); `Run` sends a key of the six new ids, `market.rv20` of a non-index name included, to the vol step before the default branch of `underlyingLists.add`, which sends other keys to the IVP and IVR step. The vol step has its own `volLookback`; each stored reading carries the day of each part as a parameter. A missing part gives a stored missing reading with its reason. In `Run` the vol step comes after the HV30 step (`TestHv30ClosesAscendingLast31` in `hv_test.go` asserts that the first price read is the 70-day hv30 read, so a vol step before it would break the test; the vol step's longer read follows it).
Acceptance:
1. `git grep -n 'rv60 = "market.rv60"' -- internal/jobs/computefigures/job.toml` prints a line, and so for `rv120 = "market.rv120"`, `vol_forecast = "market.vol_forecast"`, `vol_edge = "market.vol_edge"` and `vol_forecast_err = "market.vol_forecast_err"`.
2. `git grep -n "volLookback" -- internal/jobs/computefigures/vol.go` shows a constant of at least 230 (141 closes for the error are about 215 calendar days).
3. `vol_test.go` has these tests, each on the fake price reader of `hv_test.go` and the fake clock: `TestVolStoresAllFigures` (160 closes store rv20, rv60, rv120, forecast and error ok), `TestVolShortHistory` (70 closes: rv120 missing `short_history`, forecast missing `no_input`, rv20 and rv60 ok), `TestVolNoImplied` (no `market.iv_ibkr` reading: edge missing `no_input`, forecast ok), `TestVolIndexRv20Once` (SPX gets `market.rv20` once, from `runRv20`, and the other five ids from the vol step), `TestVolRoutesRv20OfStock` (a non-index name's `market.rv20` key is stored by the vol step, not by the IVP step: no `market.ivp_own` reading is made from it), `TestVolGatedByBars` (a nil `PriceBars` plans none of the six keys), `TestVolErrorAsOf` (the error is ok on 160 closes, and its past windows are called with the day of the past session: a fake that refuses an older data day than as-of minus `max_age_days` is not hit).
4. The key counts follow the six keys, per subject and only with `hvOn()`. Unchanged, and a worker must not rewrite them: `job_test.go:131-132` (`TestPlanKeys`, no bars: 13 keys), `job_test.go:252` (`TestFirstRunOnEmptyStores`: empty stores have no subject) and `hv_test.go:174` (nil bars). Rewritten: `valuation_test.go:208` (`4+1+3+len(marketIDs)+3`, bars set, one subject: six keys more) and `job_test.go:160` (`TestRunStoresOkAndMissing`, `rvJob`: `Returned` and `Usable` 14 become 20); `git grep -n "res.Returned != 20 || res.Usable != 20" -- internal/jobs/computefigures/job_test.go` prints a line and `git grep -n "res.Returned != 14" -- internal/jobs/computefigures/job_test.go` prints nothing, and `git grep -n "market.rv60" -- internal/jobs/computefigures/valuation_test.go` prints a line. `rv_test.go`, `ease_test.go` and `ivp_ibkr_test.go` set `PriceBars` and run the planned keys, so they are in Files; they are edited only if the vol keys change their totals (a missing part counts as usable, `steps.go:252-282`, so they are expected to pass unchanged).
5. `docs/contracts/jobs/compute_figures.md` lists the six figures, the eleven keys per name (iv30, ivp, ivr, ivp_ibkr, hv30, rv20, rv60, rv120, forecast, edge, error), the order of the steps (the vol step after HV30) and the run bound of 10 minutes: `git grep -n "market.vol_forecast_err" -- docs/contracts/jobs/compute_figures.md` prints a line.
6. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/jobs/computefigures -race -count=1`, `go run ./scripts/arch --check`.

### 4 vol-view-builder

Module: pages. Size: M.
Files: `internal/pages/option_boards/stock_vol.go`, `internal/pages/option_boards/stock_vol_test.go`, `internal/pages/option_boards/page.go`, `internal/pages/option_boards/stock_build.go`, `internal/pages/option_boards/payload.go`, `internal/pages/option_boards/strings.go`, `internal/pages/option_boards/page_test.go`, `internal/pages/option_boards/quote_test.go`, `internal/pages/option_boards/stock_schema_test.go`, `internal/pages/option_boards/stock_page_test.go`, `internal/pages/option_boards/names_test.go`, `internal/pages/option_boards/testdata/stock_option_page_ok.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_open.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_no_key.golden.json`, `docs/contracts/pages.md`
Design: docs/design/vol-edge.md#interface (items 2 and 4), Decisions 4, 7, 8
Order: after 1 and 2 (not after 3: the builder reads stored readings by id string through a fake in its tests and touches no jobs file; 3 and 4 run in parallel).
Goal: `StockConfig` gets `Readings store.ReadingViewReader` (nil gives every row a named gap, never an empty block); `BuildStock` writes `Meta.Schema` `page.stock_option_page.v3` and `SchemaVersion: 3` and fills `vol_view` from the key alone: the subject is the ticker, which is `names.ticker(mine[0].UnderlyingID)` of the first matching row when the key matches a row (so an issuer key with a row gets its ticker, `stock_build.go:117`), else the key in upper case (an issuer key with no candidate row has no known ticker, so it finds no reading and every row is a gap). The readings read are the newest of `volReadDays` (one named constant in `stock_vol.go`, 7, the same as `lookbackDays` of `load.go:13`) days ending on the page's as-of day, each with its own day; one older than that is a gap. The figures are `market.iv_ibkr`, `market.rv20`, `market.rv60`, `market.rv120`, `market.vol_forecast`, `market.vol_edge`, `market.ivp_ibkr` and `market.vol_forecast_err`; each row carries the `Day` of its reading as `data_day`; a row with no ok reading is a gap with the reason of the stored missing reading, else `no_input`. A key with no candidate row, or a name with no stored figure (no chain, or an ambiguous ticker), still gets the view, every row a gap. There is no events block. The one comparison the page makes is Decision 8. No `Fits`.
Acceptance:
1. `git grep -n "Readings .*store.ReadingViewReader" -- internal/pages/option_boards/page.go` prints a line; `git grep -n 'page.stock_option_page.v3' -- internal/pages/option_boards/page.go internal/pages/option_boards/page_test.go internal/pages/option_boards/quote_test.go internal/pages/option_boards/stock_schema_test.go` prints a line in each of the four files; `git grep -n 'page.stock_option_page.v2' -- internal/pages/option_boards ':!*golden*'` prints nothing.
2. The page reads by id and computes nothing: each of `"market.iv_ibkr"`, `"market.rv20"`, `"market.rv60"`, `"market.rv120"`, `"market.vol_forecast"`, `"market.vol_edge"`, `"market.ivp_ibkr"`, `"market.vol_forecast_err"` is found by `git grep -c '<id>' -- internal/pages/option_boards/stock_vol.go` printing at least 1; `git grep -nE "math\.|big\.|strconv\.Parse(Float|Int)" -- internal/pages/option_boards/stock_vol.go` prints nothing, and `git grep -n "compareDecimal" -- internal/pages/option_boards/stock_vol.go` prints a line (the dead band); `git grep -n "volReadDays" -- internal/pages/option_boards/stock_vol.go` prints the one constant and its uses, and `git grep -n "lookbackDays\|\b7\b" -- internal/pages/option_boards/stock_vol.go` shows no second literal 7.
3. `stock_vol_test.go` has `TestVolViewNoRow` (a key with no candidate row and stored readings: eight rows), `TestVolViewBlockedRow` (a blocked row: the same), `TestVolViewNoFigures` (a name with no stored figure: eight gap rows, `data_day` empty, never a null block), `TestVolViewNoImplied` (no `market.iv_ibkr`: `iv_name` is a gap with `gap` `no_input`, `rv20`, `rv60`, `rv120` are not gaps), `TestVolViewDataDay` (each ok row has its reading's day) and `TestVolViewNilReader` (all eight rows are gaps), `TestVolViewIssuerKey` (an issuer key with a row reads the ticker's readings; with no row all eight rows are gaps), `TestVolViewWindow` (a reading 8 days old is a gap, one 3 days old is shown with its own day), `TestVolViewFollowedName` (a name with stored figures and no candidate row, as a held-option underlying has once the separate issue lands, gets ok rows with no page change), `TestVolViewMethod` (`method` contains "its horizon is not stated", "not the same horizon" and "not to a strike or an expiry"; every `coaching` text ends with the tail, including "not a track record").
4. The texts are those of Interface item 4, English, built in Go, and each `coaching` text ends with the tail that says what the page does not know: `git grep -n "does not know the next earnings move" -- internal/pages/option_boards/strings.go` prints one line (the tail is one constant that the four cases join); `git grep -n "not the same horizon" -- internal/pages/option_boards/strings.go` and `git grep -n "not to a strike or an expiry" -- internal/pages/option_boards/strings.go` print a line each. No added text contains advice words, in either file that may hold text: `git diff BASE -- internal/pages/option_boards/strings.go internal/pages/option_boards/stock_vol.go | grep '^+' | grep -v '^+[[:space:]]*//' | grep -ciE "sell|buy|should|recommend|advice"` prints 0 (comment lines are excluded: they describe the code, the rule is about the texts). `TestVolSentenceCases` asserts the exact text chosen for edges `0.030000`, `-0.030000`, `0.004000`, `-0.004000`, the two ends `0.010000` and `-0.010000` (inside) and a missing edge.
5. The built pages validate against v3: `TestStockSchemaV3` in `stock_schema_test.go` validates the three golden files and a built page; `git grep -n '"schema_version": 3' -- internal/pages/option_boards/testdata/stock_option_page_ok.golden.json` prints a line, and so for `_open` and `_no_key`.
6. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `go test ./internal/pages/... -race -count=1`, `go run ./scripts/arch --check`.

### 5 vol-view-wire

Module: ot. Size: S (a few lines).
Files: `cmd/ot/stock_option_site.go`, `cmd/ot/stock_option_site_test.go`, `docs/contracts/ot/pages/stock_option_page.md`
Design: docs/design/vol-edge.md Decisions 5, 7, 9
Order: after 4 and 6. The wiring is last so that the served page is version 3 only when the web reads it (a worker may not merge 5 first).
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
Design: docs/design/vol-edge.md Decision 10
Order: after 2 (parallel with 3 and 4; before 5, which serves the page the web must already read; the page shows real data only after 3 and 5).
Goal: render `vol_view` as a table (label, text and its data day, or the gap and its reason), the `method` text and the `coaching` sentence, with no arithmetic. `SUPPORTED_VERSIONS` becomes `[1, 2, 3]`; version 1 and 2 payloads still validate and `bad-version.json` (version 9) stays refused.
Acceptance:
1. `git grep -n "vol_view" -- web/src/pages/stockoption/Vol.tsx` prints a line; `git grep -nE "parseFloat|Number\(|toFixed|Math\." -- web/src/pages/stockoption/Vol.tsx` prints nothing.
2. `git grep -n "SUPPORTED_VERSIONS = \[1, 2, 3\]" -- web/src/types/stock_option_page.ts` prints a line.
3. `StockOptionView.test.tsx` has the cases `renders the vol view`, `shows a gap row with its reason`, `renders a vol view of gap rows` and `validates a version 3 payload` (`git grep -n "renders the vol view\|shows a gap row with its reason\|renders a vol view of gap rows\|validates a version 3 payload" -- web/test/pages/stockoption/StockOptionView.test.tsx` prints four lines).
4. No new package: `git diff --name-only BASE -- package.json pnpm-lock.yaml frontend-whitelist.json` prints nothing.
5. `git diff --name-only BASE` lists only paths of the Files line.
Checked by CI: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, the build.

## Left out

Second cut (named, with the reason):

- The grade of every sale. One population, closed early or held: premium kept, assigned or not, days held, realised volatility over the life against implied at entry, the held-to-expiry group beside the bought-back group (40 percent assignment against about 90 percent kept is a selection effect, not a verdict), never a win rate alone; the owner's habit (about 35 DTE in, out near 21 to 23 days or 90 percent kept) shown as kept or broken, without proposing another rule. Needs a read-time join of the Flex trades (the option ledger) with the readings of this cut, a page schema (`option_result_page` v2) and a builder. For history since 2025-12-01 the entry implied is `market.iv_ibkr` of the entry day, marked "name-level, not the contract's"; moneyness at entry (strike over that day's close from bars) stands in for delta. Decision 12 says why it waits.
- Figures for held shares with no chain: a reader of stock and ETF instruments by name in `store` that does not create (the model is `instrument.OpenIndexIDs`), a `Followed` list in `compute_figures`, and its wiring in `ot`; the first item of the second cut. A held option position on a name outside the options group is not this item: it is the separate issue "the underlying of every held option position is followed" (outside this design), which gives the name a chain, `iv_history` and so figures without a new reader; until it lands the view of such a name is a named gap. Bars alone would not do: a name off the list has none either (`cmd/ot/followfilter.go`).
- The events block: the stored next earnings and ex-dividend dates as stored. Not in cut one because the stored `EarningsState` of a candidate is the issuer's `listed`/`absent`/`none` state and the "inside the life of the option" verdict is a per-contract gate, and `NextExDate` is read from one contract row, so a name-level block needs its own definition of which row and what it claims. Second cut, with the call page.
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
5. Figures are stored per day, not on demand (the page computes nothing, and a stored reading per day is the entry fact of a later grade). The subjects are the underlyings with a stored chain (`listSubjects`, which is the options group: `iv_history`, option capture and candidates all follow that group, `cmd/ot/followfilter.go`). The advisor's R2 (every held name, the options group or not) is narrowed to those names, because the step that would reach the others does not exist on main: the only function that turns an identifier into an instrument id is `ResolveInstrument` (`internal/store/api.go`, called at `internal/jobs/barsrefresh/run.go:86` with `Scheme: "ibkr_conid"`), and it creates the instrument when none exists; the only read of instruments by name is `instrument.OpenIndexIDs` (`internal/store/instrument/index_ids.go`), which selects `instrument_type = 'index'` only. A reader of stock and ETF instruments by name in `store`, a `Followed` list in the job and its wiring in `ot` is the first item of the second cut. Until then a held name outside the options group shows its view as named gaps with the reason code `no_input` on every row and the no-edge `coaching` text (Interface item 4); the page writes no prose about the chain, this design does. That includes a name with a held option position that is not an active row flagged `options` (`cmd/ot/followfilter.go:19-40`: nothing adds the underlying of an option position to the group); the separate issue "the underlying of every held option position is followed" closes that case without the reader, because the name then has a chain and `iv_history`.
6. The forecast's past error is in the cut: the owner is told how wrong the number has been, per name, from the bars, not left to trust a model choice. It is one stored figure a day (forecast of 20 sessions ago minus the realised 20 sessions since), which accumulates into a series; the mean over a year is the second cut. It is computable today from the 400 days of bars, with no waiting (rv20 and the forecast of any past day are pure functions of stored closes), so the cut does not wait for days to pass.
7. The view is built from the key alone, whatever the gate status of the rows and whether the key has rows at all, because the owner picks the name first (valuation and buy price) and never traded from the old scan. The page route and the readings are by ticker. A name whose ticker is ambiguous has the figure subject `underlying:<id>` and so no reading under its ticker: it shows gaps with `no_input`, like any name with no figure; no separate reason is built.
8. The sentence is English, built in Go from the fixed texts of Interface item 4, and chosen by the stored edge text with a dead band of one volatility point (`0.010000`) around zero, both ends inside it: above the band, below it, inside it, or no edge (it names the missing input). The comparison uses `compareDecimal` of `internal/pages/option_boards/decimal.go` on the stored edge text and `0.010000`, which the package already uses for its own decimal texts and which parses nothing into a float; whether a reviewer reads that as computing is Doubt 3. The sentence never reads as trade advice: it states the difference between two stored numbers and what the model does not know (the next earnings move, skew, how well the blend forecasts this name, shown by the error beside it, early assignment, the horizon of implied, which is not stated). It is not a rank and not a score. One vol point is the stored scale of IBKR's IV bar; the owner may move the band (Doubt 4).
9. `ot` passes `rd.view`, already opened for the site; no new database role and no new opener.
10. The web renders; it holds no arithmetic and adds no package (recharts and @tanstack/react-table stay the whitelist). Versions 1 and 2 of the page stay readable.
11. Six entries, one over the cut of 3 to 5. Five modules are touched by the smallest honest path (compute, contracts, jobs, pages, web) and the sixth, `ot`, is the composition root that gives the builder its reader (`AGENTS.md`: a path belongs to one module). Moved out to hold the line: the grade, the call page, the candidates-page columns, the per-strike view and the second forecast.
12. The per-sale record waits for the second cut and is derived, not a new table. The name-level inputs (the forecast, the windows and `market.iv_ibkr` of each day) are kept per day by this cut and by `iv_history`, and the Flex rows are stored insert-only, so a grade by the name-level facts can be built at any time with nothing lost by waiting. What a table would add is the contract's own IV and delta at entry outside the capture band, and that is a collection question (late-collection storage), not a table question: a table written after the sale cannot recover a chain that was not captured. The bars are the limit: they are backfilled 400 calendar days (about 275 sessions), so the 120-session window of a sale reaches back about 176 calendar days before it; a sale older than about 224 days has no rv120 at entry from stored bars, and the oldest sales since 2025-12 lose it as days pass [unverified: how far back the real store reaches, Doubt 6]. For those sales waiting is not free for rv120 (rv20 and rv60 last longer), which is the cost of leaving the grade to the second cut. So the first cut does not build a table, and the second cut decides the contract-level entry facts together with that design.
13. JP is not in this cut. Reason: `compute_figures` runs on the XNYS calendar and the stored chains of the JP option jobs are separate; whether IBKR sends IV for OSE options is unverified; the page already says "Japan options: later design (option-candidates-jp)" (`jpNote` in `internal/pages/option_boards/strings.go:11`). A JP name has no figure unless it has option_eod rows on the US-calendar day; no JP-specific figure is added.
14. The owner's habit (about 35 DTE in, out near 21 to 23 days or 90 percent kept, held to expiry sometimes, covered calls a core strategy) guides what the sentence must not contradict; no rule is proposed and the habit as kept or broken belongs to the grade (second cut).

## Doubts

1. Entry count: six against a cut of 3 to 5 (Decision 11). No two can merge: entry 1 is already a judged entry across compute, docs and a test fake of jobs, and `ot` and `web` are other modules. If the cap is hard, dropping 5 and 6 together stores and builds the view but does not show it; dropping 3 computes at read time, which breaks "pages compute nothing".
2. Entry 1 names three modules although each entry should edit one. The register file is `docs/**`; a compute-only change fails `TestRulesMatchRegister`, a docs-first one fails because `implemented_by` must name an existing function, and the fake `fakeFigs` of jobs stops compiling when `Combine` joins `Figures`. The precedent (`docs/design/market-rv20.md` issue 3) is on main for compute and docs; the jobs test file is new here. The alternative is a separate small interface for `Combine` in jobs, which `compute/api.go` cannot hold (ceiling).
3. The page compares two decimal texts for the dead band and the sign with `compareDecimal` (`decimal.go`). The rule "the page computes nothing" is the reviewer's to read; the alternative is a stored word (`above`, `below`, `inside`) from `Combine`, which would put a threshold into a compute convention.
4. The dead band of one vol point is mine, not the owner's. It sits in the page's strings and is changed there.
5. The advisor's R2 is narrowed (Decision 5): a name gets figures only on a day with a stored chain. Three groups see a gap: held stock with no options (second cut), a name with a held option position outside the options group (the separate issue "the underlying of every held option position is followed"), and a group name on a day when its chain capture failed (the page shows the previous day's figures, dated, for up to 7 days). The second group is the one that hurts: the owner has a position and no view of it, until that issue lands.
6. History depth: rv120 needs 121 closes and the error 141; the bars job backfills 400 calendar days (about 275 sessions) at `backfill_names_per_run = 10`, so a new name needs several days before its history exists, and a name with fewer shows a named gap. How far back the real store reaches is [unverified]. Whether `deepen_names_per_run = 0` switches the 5-year deepening off is [unverified].
7. The forecast is a model choice. A blend of past windows ignores mean reversion, events and the market's own information; for a name just after earnings the 20-day window is dominated by one jump. The page says so and shows the error beside it.
8. Run size: eleven keys per name instead of five. At 100 names that is about 1,100 keys, each one read and one insert of the readings store; `run_bound = "10m"` is unchanged and the estimate (a few round trips per key) is far inside it, but the real number of names and the round-trip time are [unverified]. The contract text states the count so that a later rise is seen.
9. An index name that is also a subject (SPX with chain rows) must get `market.rv20` once, from `runRv20`; entry 3 `TestVolIndexRv20Once` tests it.
10. The close-to-close figures count stored rows: a session missing from bars becomes a two-day return and is not detected; closes are IBKR-split-adjusted and not dividend-adjusted. The sentence does not claim more.
11. How the v3 schema is composed. Copying v2 puts v2's `null` members into the generated file and `TestGeneratedV3Types` refuses a v3 file with null unless it refs a v2 page; refs to v2 may clash with `additionalProperties` in v2 [unverified]. The entry's worker decides by that test; the block itself has no null.

## Adversarial

Round 1 (on revision 1): one read-only sub-agent, given the text and the repository and not the conversation; its answer is saved as `vol-edge-adversarial-r1.md` in the designer's scratchpad `designs/` directory (outside the repository). Revision 2 follows a second check of revision 1 by the advisor (`vol-edge-review-r1.md`, outside the repository) and the advisor's requirements R1 to R6; a second adversarial pass on the pushed head is run by the coordinator. Answers to round 1:

1. Gap against the goal. Adopted: the view is name-level and says so (Goal), the figures are stored for the names with a stored chain (Decision 5; held names outside the options group wait for the second cut and a separate issue), and implied missing is a named gap beside realised figures. The magnitude bands asked for are replaced by one dead band (Decision 8), which is the owner's parameter to move.
2. Biggest unnamed risk. Adopted: `contracttest_test.go` and `contracttest/figures.go` are in entry 1; the fake `fakeFigs` of jobs is added (the check of revision 1 found it); the rule hash is checked by `TestHashesMatchSource` (entry 1 Checked by CI).
3. A simpler way. Not adopted for the forecast: the owner asked for a stated forecast and the error needs it stored. Adopted for entry 5: the reader is `rd.view`.
4. The lock-in. Adopted: the row label is a string; the ceiling of `api.go` is named in Decision 3; the stored ids carry the method (a change is a new rule version).
5. The claim most likely false. Adopted: "every name" is replaced by the subjects of Decision 5; the order of `iv_history` (close + 10 minutes) and `compute_figures` (+ 20) is settled by reading; the count of readings is corrected.
6. Likeliest slip. Adopted: the two fakes; the generator is `pnpm --dir web run generate:types`; `hvLookback` stays 70 and the new steps have `volLookback`. Doubt 2 stays the likeliest review stall.

Round 4 (the crew design session on db5819e3 and the advisor's cutting items): entry 4 no longer after 3; entry 2 shows `carryover_test.go` needs no edit; one set of gap codes and texts (Interface item 4, Goal, Decision 5); "and the job its names" out of Decision 11; exact greps for `Returned != 20`, the five pending checks and the one-case-per-line assumption; the vol step after HV30; `data_day` a string, empty when none; the wiring (entry 5) after the web (entry 6); `compareDecimal` for the dead band; comment lines out of the advice grep; the tail adds "not a track record"; `volReadDays`; `TestVolViewFollowedName`.

Round 3 (the checks on 95b5f08a, `vol-edge-review-r2.md` and the confirming pass, outside the repository): the `Contracts:` forms (B1), `gen.py` dropped (B2), the texts quoted and the advice grep over both files (B3), the key-count tests named (B4), the as-of day of the past forecast (B5), events moved to the second cut (B6), the name-level statement in `method` with a test (R1), the gap cases of R2 stated with the separate issue, the horizon statement in the block (R4), the 7-day window, the issuer-key rule and the bars limit of R7.

Round 2 (on revision 2, head 26789d0c; `vol-edge-adversarial-26789d0c.md` in the same scratchpad directory). Answers:

1. B1, the test files that count keys: adopted; `valuation_test.go` (line 208), `job_test.go` and `hv_test.go` are in entry 3 and named in its acceptance 4.
2. B2, the unverified id lookup: settled by reading and R2 narrowed (Decision 5): main has no function that gives an instrument id for a name without creating one, so `Followed` and its wiring are out of the cut and in the second cut; entry 5 is only `rd.view`.
3. `market.iv_ibkr` is not known to be a 30-day measure: adopted, the text says "horizon not stated" and no like-with-like claim is made (Goal, The method, Decision 8).
4. The generator argument is `page.stock_option_page.v3`: adopted. Routing of `market.rv20` of a non-index name, and the day rule of `Combine`: adopted (entry 3 Goal, `## The method`). `ambiguous_name`: dropped (Decision 7). Acceptance items that already pass on main were removed, `git grep -c` counts were replaced by exact greps and named tests, and BASE is defined. A `figures.json` case for `Combine` is in entry 1.
5. Not adopted: dropping `Combine` for sums in the job (the rule hash and `implemented_by` need a function in compute; Decision 3); the cut of the forecast and error from this cut (R4).

