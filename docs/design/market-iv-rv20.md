# market iv rv20: the implied minus realised cell of SPX, NDX and RUT shows FRED's VIX, VXN or RVX less the stored 20-day realised volatility

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: owner 2026-11-11 "4. a" (the implied-minus-realised cell of the market page is built); owner 2026-11-13 "advisor settles the definitions first". The definitions are section A of the advisor's proposal of 2026-11-13; this design takes them as they are. Where the code or the coordinator's word differs from one, Doubts says so (the age limit, Doubt 1).

Goal: the `Implied minus realised` cell of SPX, NDX and RUT on the market page and in the home market card shows a signed number of volatility points, or a Gap that says why. The front end computes nothing.

Existing design: `market-rv20.md` stored `market.rv20` and left the three cells `not_built`; `vol-edge.md` added the convention `vol_diff_v1` and `Figures.Combine`; `market-page-producer.md` and `home-market.md` built the cells as `not_built`. None builds this figure.

## Today

All paths at origin/main.

- `internal/compute/api.go` is 839 lines (ratchet ceiling 840); no entry edits it. `Figures.Combine` (`internal/compute/figures/combine.go:15`) does `vol_diff_v1` as parts[0] minus parts[1], takes `DataDay` as the newest of the parts, answers `no_input` for a part that is not ok and `stale_input` when the as-of day is more than `MaxAgeDays` after `DataDay`. It does not check that the two parts carry the same day; the job must.
- `internal/jobs/computefigures` stores `market.rv20` for `rv20_indices` (SPX, NDX, RUT; `rv.go:27` `runRv20`, which stores a reading whose `data_day` parameter is the day of the newest close, `hv.go` `hvParams`) and the market figures on FRED series (`steps.go:116` `runMarket`: a figure whose newest FRED point is older than the newest session day is stored as it is and the key fails `Retryable`; an empty series is skipped). `ComputeStores` (`types.go:16`) holds `Series`, `Readings` and `SessionDays`, all required (`job.go:100`). `vol.go:153` `impliedOf` is the model of reading a stored figure as a `compute.Figure`; it checks the row's `day` only.
- The runner runs the keys of one job one at a time, in `SubjectKey` order (`internal/runner/reconcile/reconcile.go:194-198`). `market.iv_rv20|underlying|SPX` sorts before `market.rv20|underlying|SPX`, so at the run after the close of D the iv_rv20 key runs when the rv20 of D is not yet stored.
- The retry window is the job's `run_bound` of 10 minutes (`job.toml`, `[schedule]`). A retryable run before the close of the window is not counted and the key runs again; after it the key is an `expired` verdict (`reconcile.go:37`, `:385-420`). FRED publishes D on the morning of D+1, outside that window.
- `market.iv_ibkr` is empty for an index, so no figure of the index row reads implied volatility today. `VIXCLS`, `VXNCLS` and `RVXCLS` are stored by `fredseries` and read by `compute_figures` (percentile tables) and by `home_market` (`volSpecs`).
- `internal/compute/rules/rules.json` has no `market.iv_rv20`; `git grep -n iv_rv20 -- internal/compute/rules/rules.json docs/conventions/register.toml` prints nothing, which is what `docs/design/market-rv20.md` issue 3 item 1 (line 211, "prints 0") pins. That item belongs to a merged issue and is history; this design does not edit it (`docs/design/` is owner-only). After entry 1 the grep prints lines for both files.
- `market_page/cells.go:260-296` `buildIndices` gives every row `IVMinusRV20 = mkGap("not_built", name + " implied minus realised: not built yet")` (`:268`) and never overwrites it. `home_market/member.go:270` (`buildRow`, `:265`) does the same with `notBuiltCell`, and `collectGaps` (`member.go:406`, the branch at `:416-421`) folds the not-built cells into one gap `Implied minus realised not built yet for SPX, NDX, RUT`.
- Tests that pin the text: `market_page/cells_test.go:78-96` `TestN225RvStaysNotBuilt` (all four rows `not_built`), `cells_test.go:99-103` (the indices sentence says the column is not built), `home_market/member_test.go:350` `TestIndexGapsOnePerFeature`, `home_market/testdata/ok.golden.json` (lines 95, 116, 137, 186-187; the test compares it and has no update flag, so it is edited by hand). No function `TestIvRvCellsAreNotBuilt` exists on main; the name is in the history of `market-page-producer.md`.
- Both pages read the readings of the last 5 calendar days (`market_page/cells.go:75` `d - 5`, `home_market/member.go:38` `readingDays = 5`) and show the newest rv20 reading with no age check. `market_page/cells.go:104` `failure()` already maps a stored `missing_input` with reason code 3 to "stale". `market_page` parses the `data_day` parameter (`cells.go:90-97`); `home_market` has no such parser. `internal/pages/report_blocks/market_watch.go:15-16` allows 5 calendar days (`staleDays`).
- Schemas: `iv_minus_rv20` is `FigureOrGap` (market_page v1) and `VolPointsFigure | Gap` (market_summary v3, `value` from -1000 to 1000, one decimal); the Gap reasons include `stale_input`, `missing_input`, `no_data`, `not_built`. A signed value and the new texts need no schema change.

## Decisions

Decisions 1 to 8 are the advisor's definitions A1 to A7 and A9, in his order; 9 to 12 are the build.

1. Reading `market.iv_rv20`, version 1, convention `vol_diff_v1`, subjects `market.iv_rv20|underlying|SPX`, `NDX` and `RUT`; made by the existing `Figures.Combine` and stored by `compute_figures`. Its rule has unit `decimal`, scale 6, `trading_days` 252 and `max_age_days` 5 (Doubt 1).
2. Formula: `iv_rv20(D) = series(D) / 100 - market.rv20(D)`; series is `VIXCLS` for SPX, `VXNCLS` for NDX, `RVXCLS` for RUT (index points: 18.5 is 18.5 percent). `market.rv20` is taken as stored, not recomputed (21 unadjusted `trades` closes, log returns, sample standard deviation, times sqrt(252), a decimal fraction of scale 6). The result is a signed decimal fraction of scale 6: 0.032000 is 3.2 volatility points, positive means implied above realised.
3. Same day only. The job takes the newest day D' not after the run's day D on which the series has a point and a stored `market.rv20` reading exists. The rv20 reading of D' is the newest rev of that day (`LatestReadings` returns the newest rev of each day, whatever its state, with its parameters; `WinnerReadings`, the page reader, prefers the highest ok rev, which the job does not use); it counts as a part only if its state is ok and its `data_day` parameter equals D' (an rv20 whose newest close is of an earlier day is the figure of that day and is not subtracted from the implied value of D'); otherwise the rv20 part is missing and Combine answers `no_input`. A FRED `no_value` day has no point (`FromSeriesPoints` keeps only present points) and so gives no figure for that day. The reading's `day` is the run's day D, its `data_day` the day of the inputs.
4. Missing: `missing_input` with reason `no_input` (code 1) when either input of D' is absent or unusable (Decision 3), or no common day exists; with `stale_input` (code 3) when D' is more than 5 calendar days before D. Never a zero, never a carried value.
5. Cell (market page `iv_minus_rv20`, home market `IVMinusRV20`): the newest stored `market.iv_rv20` reading of the index (the page's own reading function, newest day then rev). If its state is `missing_input` with code 3, the cell is Gap `stale_input`, naming the `data_day` when there is one (the pattern of `failure()`); any other missing state, or an ok reading without a usable `data_day`, is Gap `missing_input`; no reading in the window is Gap `no_data`. An ok reading whose `data_day` is more than 5 calendar days before the page's day is Gap `stale_input` naming that day. Otherwise the value is the stored fraction times 100 rounded half-even to one place (`shiftRound(v, 2, 1)`), unit `vol_points`, `text` signed (`+3.2`, `-1.4`, `0.0`; the `value` number has no plus, `3.2`). When `data_day` is not the page's day the text names it (`+3.2 (11-12)`), because through the US night the page shows the figure of D-1. "The page's day" is the day Build is called for (Doubt 2). Each page parses `data_day` itself; `home_market` gets its own parser.
6. No rich or cheap threshold, no colour, no word: plan v2.5 gives none and no document in the repo does. The cell is descriptive.
7. Named limits (Left out and the register `excludes`): the implied side looks 30 calendar days forward, the realised side 20 sessions back; the figure is not a variance premium and not IBKR's index implied volatility (`market.index_iv` and `market.iv_ibkr` stay as they are). N225 stays Gap `not_built`.
8. The `iv` cell of the same row (it reads `market.iv_ibkr`, empty for an index) is not changed here.
9. Where: computed and stored in `compute_figures` in a new step in a new file; the pages read the stored reading and compute nothing. The manifest gains `iv_rv20 = "market.iv_rv20"` and a table of the three name and series pairs (names and FRED ids only). `Plan` appends the three keys after the rv20 keys. `Run` gives the key `(market.iv_rv20, underlying, <name>)` its own `case` before `case known && kind == kindUnderlying` (`job.go:264`), the way `isRv20Key` has one, so it is not looked up among the option-chain underlyings (where it would fail `Retryable` with "has no option rows"). The step reads the series points, the stored rv20 readings of the name over `lookbackDays(rule)` days and the newest session day S not after D (its own lookup, as `runMarket` does; `steps.go` is not edited); it builds the two parts as `compute.Figure` (ok, `DataDay` D', the series value divided by 100 and the stored rv20 value) and calls `Combine("market.iv_rv20", parts, D)`; no part, or no common day, is `r.missing(t, compute.NoInput, "")`. It stores through `r.store` with the hash of the input lines (series points, and the rv20 value, state and `data_day`), the inputs `obs.series_point` and `readings.reading`, and `volParams` (`data_day`, `observations`). The step does not depend on the rv20 key of the same run, because keys run alone.
10. Retry: the key ends `Retryable` whenever S is known and the stored figure's `data_day` is before S, with or without a series point and with or without an rv20 reading (the key cannot know whether the rv20 key has run). Within the 10-minute window the key runs again; FRED publishes after it, so normally the key is `expired` and the figure of D is stored by the run of D+1, whose D' is D: the cell lags one session and names the day (Doubt 5). A whole-job run of a world with no FRED point of S therefore returns a `Retryable` error; the tests that expect another first error or none are updated (entry 2).
11. Rule and register content (entry 1): the `market.iv_rv20` entry of `docs/conventions/register.toml` (formula of Decision 2, inputs, unit decimal scale 6, `excludes` of Decision 7, `gap_text` of Decision 4, `implemented_by` `internal/compute/figures.(*calc).Combine`, `source_hash` the value `TestHashesMatchSource` reports), its line in `internal/compute/rules/rules.json` with the same hash, and a case in `internal/compute/own/testdata/stored_text.json` (`stored_text_test.go:69` fails for a rule without a case). The three files and the test land in one pull request (`TestRulesMatchRegister`, `TestNoPendingForGoIds`).
12. There is no owner step: rules table, register and stored-text case are ordinary files of an ordinary entry, as in `market-rv20.md` issue 3 and `vol-edge.md` entry 1.

## Modules

No row of `docs/design/modules.md` changes. Entry 1 is a judged two-module entry, compute plus docs (`internal/compute/**` for `rules.json`, `rules_test.go` and `own/testdata/stored_text.json`; `docs/**` for `register.toml`), because the register entry and its rule row must be one pull request. Entry 2 is jobs (`internal/jobs/**`, `docs/contracts/jobs/**`); entries 3 and 4 are pages (`internal/pages/**`, `docs/contracts/pages.md`). `cmd/ot` is not edited (the manifest is embedded; `wire.go` at 793 of 800 is untouched).

## Interface

No interface changes. `compute.Figures.Combine` (`internal/compute/api.go`), `computefigures.ComputeStores`, `internal/jobs/api.go` and `internal/pages/api.go` keep their declarations. No entry is marked (interface).

`Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes; the pages already declare `iv_minus_rv20` as a figure or a gap).

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entries 2 and 3 read the files of entry 1 (`computefigures` finds the rule in `findRules`; `ids_test.go` reads `rules.json` and `job.toml`), so they come after it; entry 3 also reads `job.toml`, so it comes after 2.

### 1 market-iv-rv20-rule

Modules: compute and docs (one entry: the register entry and the rule row are one pull request). Size: S.
Order: first.
Files: `internal/compute/rules/rules.json`, `internal/compute/rules/rules_test.go`, `internal/compute/own/testdata/stored_text.json`, `docs/conventions/register.toml`
Design: docs/design/market-iv-rv20.md#decisions (1, 2, 4, 7, 11)
Acceptance:
1. `git grep -n '"id":"market.iv_rv20"' -- internal/compute/rules/rules.json` prints one line holding `"convention":"vol_diff_v1"`, `"unit":"decimal"`, `"scale":6`, `"trading_days":252` and `"max_age_days":5`; `git grep -n 'id = "market.iv_rv20"' -- docs/conventions/register.toml` prints one line, and `git grep -n -A22 'id = "market.iv_rv20"' -- docs/conventions/register.toml | grep -c pending-recompute` prints 0.
2. The register entry's `implemented_by` is `internal/compute/figures.(*calc).Combine`, its `excludes` names IBKR's index implied volatility, a variance premium, the 30-day against 20-session horizons and N225, and its `gap_text` names `no_input` and `stale_input` as Decision 4 does; the `source_hash` of the entry equals the `hash` of the rule line (`git grep -c` of the one hash value over the two files prints 1 for each).
3. `rules_test.go` names `TestIvRv20RuleNumbers` (convention `vol_diff_v1`, unit `decimal`, scale 6, trading days 252, max age 5, through `New().Rule`), and `TestVolRuleNumbers` and `TestHashesMatchSource` still exist; `stored_text.json` holds a case for `market.iv_rv20` (`git grep -c 'market.iv_rv20' -- internal/compute/own/testdata/stored_text.json` prints 1).
4. `git diff BASE -- internal/compute/api.go internal/compute/figures` is empty; `git diff --name-only BASE` lists only paths of the Files line.

### 2 market-iv-rv20-job

Module: jobs. Size: M.
Order: after 1.
Files: `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/ivrv.go`, `internal/jobs/computefigures/ivrv_test.go`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/computefigures/valuation_test.go`, `internal/jobs/computefigures/vol_test.go`, `internal/jobs/computefigures/hv_test.go`, `internal/jobs/computefigures/rv_test.go`, `internal/jobs/computefigures/ease_test.go`, `internal/jobs/computefigures/ivp_ibkr_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/market-iv-rv20.md#decisions (3, 4, 9, 10)
Acceptance:
1. `git grep -n 'iv_rv20' -- internal/jobs/computefigures/job.toml` prints the id line and three name and series lines (`SPX` with `VIXCLS`, `NDX` with `VXNCLS`, `RUT` with `RVXCLS`); `git diff BASE -- internal/compute internal/jobs/api.go cmd` is empty.
2. Routing and plan: `job.go` has a `case` for the iv_rv20 key placed before `case known && kind == kindUnderlying` (`git grep -n 'case known && kind == kindUnderlying' -- internal/jobs/computefigures/job.go` prints a line after the line of the new case), and `Plan` appends the three keys; `ivrv_test.go` names `TestPlanHasIvRv20Keys` (the three keys once each, in sorted order) and `TestIvRv20KeyIsNotAnUnderlyingKey` (running the SPX key alone, with the chain world of `job_test.go`, stores a reading of `market.iv_rv20` and no error mentions "option rows").
3. `ivrv_test.go` names `TestIvRv20SameDay` (its literals: `VIXCLS` 20.5, rv20 `0.162060` of data day D, expected stored value `0.042940` with `data_day` D; NDX and RUT with `VXNCLS` and `RVXCLS`), `TestIvRv20WaitsForFred` (series points to D-1 only, rv20 stored for D-1 and D: the reading uses rv20 of D-1, the run error is `Retryable` and names D; after the point of D a second run stores rev 2), `TestIvRv20Rv20OfDayAbsent` (the key runs alone with no rv20 of D and no point of D: it stores the figure of D-1 and returns `Retryable`; this is the order the runner uses), `TestIvRv20Rv20DataDayMustMatch` (an rv20 of day D' with `data_day` D'-1 gives a stored `missing_input`, code 1, not a subtraction), `TestIvRv20NoValueDay`, `TestIvRv20Rv20MissingIsMissing` (code 1, no value), `TestIvRv20StaleIsMissing` (common day 6 days before D gives code 3; 5 days gives a value) and `TestIvRv20RerunIsNoOp` (a second run over unchanged inputs inserts no row); each file `git grep -L 't.Parallel()'` check prints nothing for `ivrv_test.go`.
4. The whole-job tests that run every planned key are updated for the three new keys and for Decision 10: `job_test.go` (key counts at `:140` and `:264`, `len(marketIDs)+3`, and the run counts at `:169`), `valuation_test.go:208`, `vol_test.go:100` (twelve ids for SPX), `hv_test.go` (`:93` and `:171`, the expected first error), `rv_test.go` (`wantRvFailure`, the failed counts and kinds), `ease_test.go` (`:121`, `:203`, `:299`) and `ivp_ibkr_test.go` (`:39`); each test whose world holds no point of the session day either stores one for the three series and the rv20 readings, or expects the `Retryable` error of the iv_rv20 keys, and says which in a comment. `ivinput_test.go` and `types_test.go` run single keys or a stub and are not edited.
5. `docs/contracts/jobs/compute_figures.md` has a bullet that names `market.iv_rv20`, its three keys, the same-day and `data_day` rule and the `Retryable` rule (`git grep -n 'market.iv_rv20' -- docs/contracts/jobs/compute_figures.md` prints a line).
6. `git diff --name-only BASE` lists only paths of the Files line.

### 3 market-iv-rv20-market-page

Module: pages. Size: M.
Order: after 2.
Files: `internal/pages/market_page/ids.go`, `internal/pages/market_page/ids_test.go`, `internal/pages/market_page/cells.go`, `internal/pages/market_page/ivrv.go`, `internal/pages/market_page/ivrv_test.go`, `internal/pages/market_page/strings.go`, `internal/pages/market_page/cells_test.go`, `internal/pages/market_page/builder_test.go`, `docs/contracts/pages.md`
Design: docs/design/market-iv-rv20.md#decisions (5, 6, 7)
Acceptance:
1. `ids.go` declares `figIvRv20 = "market.iv_rv20"`; `ids_test.go` includes it in the ids it finds in the rules table and in the `compute_figures` manifest (`git grep -n 'figIvRv20' -- internal/pages/market_page/ids_test.go` prints a line).
2. `buildIndices` sets `IVMinusRV20` of SPX, NDX and RUT from the reading of `market.iv_rv20` (kind `underlying`, the row name) through `ivRvCell` in `ivrv.go`; the N225 row keeps `mkGap(reasonNotBuilt, "N225 implied minus realised: not built yet")`; `git diff BASE -- internal/pages/market_page/builder.go` is empty; `purity_test.go` is unchanged.
3. `ivrv_test.go` names `TestIvRvCellSigned` (literals: stored `0.032000` gives value `3.2`, text `+3.2`, unit `vol_points`; `-0.014000` gives text `-1.4`; `0.000000` gives text `0.0`), `TestIvRvCellNamesDayWhenNotPageDay` (`data_day` D-1 gives text `+3.2 (11-12)` for page day 11-13; `data_day` equal to the page's day gives no suffix), `TestIvRvCellStaleBeyondFiveDays` (6 days before the page's day: Gap `stale_input` naming the day; 5 days: a value), `TestIvRvCellStoredCode3IsStale` (state `missing_input`, reason code 3, with and without `data_day`: Gap `stale_input`), `TestIvRvCellNoRowIsNoData`, `TestIvRvCellMissingIsMissingInput` (state `missing_input` with another code, or ok without `data_day`) and `TestIvRvCellHasNoThreshold` (values -5.0, 0.0 and +9.0 all give a plain figure with no tone or colour field); each calls `t.Parallel()`.
4. `cells_test.go`: `TestN225RvStaysNotBuilt` checks `iv_minus_rv20` not_built for row 3 (N225) only and a figure for rows 0 to 2 of `allOK()`; the test at `:99-103` expects the new `indicesSentence` of `strings.go` (the column is built for the three Cboe indices and not for the Nikkei); `pageSentence` no longer lists implied minus realised among the unbuilt blocks; `ivRvNotBuilt` is used for N225 only.
5. `builder_test.go`: `allOK()` stores a `market.iv_rv20` reading for the three names with a `data_day`; `pageGaps` and `TestEnvelopeGapsEqualBlockGaps` hold with the new cells; the N225 iv cell expectation near `:578` is unchanged; a Gap of this cell does not change `run_status` (it is not counted in `failed`), and `TestBuildMatchesSchema` has a case with a negative value.
6. `docs/contracts/pages.md` states the cell in the `market_page.Build` bullet (reading, unit `vol_points`, signed, day in the text, 5-day rule, N225 `not_built`; `git grep -n 'market.iv_rv20' -- docs/contracts/pages.md` prints a line).
7. `git diff --name-only BASE` lists only paths of the Files line.

### 4 market-iv-rv20-home-market

Module: pages. Size: S.
Order: after 1 (no file shared with 2 or 3; it may run beside them).
Files: `internal/pages/home_market/member.go`, `internal/pages/home_market/ivrv.go`, `internal/pages/home_market/ivrv_test.go`, `internal/pages/home_market/strings.go`, `internal/pages/home_market/member_test.go`, `internal/pages/home_market/testdata/ok.golden.json`
Design: docs/design/market-iv-rv20.md#decisions (5, 6, 7)
Acceptance:
1. `member.go` declares `figIvRv20 = "market.iv_rv20"` and `buildRow` sets `IVMinusRV20` through `ivRvCell` in `ivrv.go`, which reads through `newestReading` and parses the `data_day` parameter with its own function in `ivrv.go` (`git grep -n 'data_day' -- internal/pages/home_market/ivrv.go` prints a line); the age limit is a constant of 5 declared in `ivrv.go`, not `freshDays` (4, the limit of the Cboe level cells).
2. `git grep -n 'notBuiltCell\|diffNotBuilt\|notBuiltFor' -- internal/pages/home_market` prints nothing in non-test files; `collectGaps` copies the Gap of an `IVMinusRV20` cell like it does for `IV` and `RV20`, one per cell, and `git grep -n 'not built yet for' -- internal/pages/home_market` prints nothing.
3. `ivrv_test.go` names `TestIvRvCellSigned`, `TestIvRvCellNamesDayWhenNotPageDay`, `TestIvRvCellStaleBeyondFiveDays`, `TestIvRvCellStoredCode3IsStale`, `TestIvRvCellNoRowIsNoData` and `TestIvRvCellMissingIsMissingInput` with the literals of entry 3 item 3 (Gap texts in the card's capitalisation, `strings.go`); each calls `t.Parallel()`.
4. `member_test.go`: `TestIndexGapsOnePerFeature` is replaced by a test that gives the three rows a reading and finds no `not_built` Gap among the card's `gaps`, and one that gives no reading and finds three `no_data` gaps; `testdata/ok.golden.json` is edited by hand to hold a value in `iv_minus_rv20` of the three rows and no `not_built` entry in `gaps`; a test validates the card against `page.market_summary.v3.json` with a negative value.
5. A Gap of this cell does not change `run_status` (the `ok` and `partial` cases of `member_test.go` keep their expectations); `purity_test.go` is unchanged.
6. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/compute/... ./internal/jobs/... ./internal/pages/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). The review runs no test. These runs decide the behaviour the named tests pin: the stored value `0.042940` for `VIXCLS` 20.5 against rv20 `0.162060`; the `Retryable` error of a key that runs alone before the rv20 of its day; a mismatched rv20 `data_day` stored as `missing_input`; `TestHashesMatchSource`, `TestRulesMatchRegister`, `TestNoPendingForGoIds` and the stored-text test for entry 1; the page tests and the schema validation for entries 3 and 4. Size: the files touched stay under their ceilings (`job.go` 344, `cells.go` 307, `member.go` 514 lines; the new files are far under 600; no file is in `docs/design/ratchet.json`).

## Left out

- N225: the cell stays `not_built` (`market.iv_rv20_n225` of `docs/GLOSSARY.md`, by the owner's word later). A glossary entry for the three indices is not in any Files line (`docs/GLOSSARY.md` is a docs file the register entry makes optional).
- The `iv` cell of an index row (Decision 8): advice only, a later change shows the FRED value there so the row shows both terms of the difference.
- A rich or cheap threshold, a colour, a word; a variance premium, IBKR's index implied volatility, an at-the-money figure.
- Any carry of an older day's figure into the cell, and any mixing of an implied day with a different realised day.
- Ageing the rv20 cell the same way (Doubt 1); editing the item at `docs/design/market-rv20.md:211` (history of a merged issue).

## Doubts

1. Age limit. The advisor's point 5 says 4 calendar days and "the same age rule as rv20". The code has no age rule for rv20 (both pages show any rv20 reading of the 5-day window), and the coordinator asked for 5 calendar days from the figure's data day, as `report_blocks/market_watch.go:15-16` allows, which also covers a Monday holiday (the Friday figure seen on Wednesday is 5 days old). So the rule's `max_age_days`, the job's stale code 3 and both cells use 5. `freshDays` (4) is left to the Cboe level cells. Overrule: set 4 in entry 1 item 1 and the five items that say "5".
2. "Page's newest session day": neither page knows the session calendar; both get the day Build is called for. The day suffix appears when `data_day` differs from it, so a weekend page shows it for the Friday figure.
3. The advisor called `register.toml` and `rules.json` the owner's files. The project declaration lists as owner-only only `.github/`, `contracts/proto/`, `deploy/`, pyside `transport.py`, `docs/design/ratchet.json` and `docs/design/`; so they are ordinary files, in entry 1, as the adversarial review and the precedents (`market-rv20.md` issue 3, `vol-edge.md` entry 1) say.
4. Whole-job tests: Decision 10 makes every world without a point of the session day end `Retryable`, unlike `runMarket`, which skips an empty series. The simpler way (store the newest common day and never fail) was not taken; the coordinator's review wants the wait. Entry 2 item 4 lists the tests that follow; if a test file not listed breaks, the issue reports it.
5. Retry window: the wait repeats only within the 10-minute `run_bound`; FRED publishes after it, so on a normal day the cell shows the figure of D-1 until the run of D+1, and the `Retryable` mostly gives the `expired` reason. The cell names the day, so this is shown, not hidden. A console re-run after FRED publishes stores the figure of D.
6. The figure's `day` is the run's day D and its `data_day` the common input day; a rev that is `missing_input` hides an earlier ok of the same day behind the page's newest-rev rule (`steps.go` already warns of it).
7. Plan keys grow by three, so the plan hash of a past day changes; a re-run of an old day after deploy plans the three keys (additive, as rv20 did).
8. `ok.golden.json` has no update flag; entry 4 edits it by hand.

## Adversarial

Source: the author's adversarial notes (scratchpad) and the advisor's review r1 (scratchpad), both on revision 1.

1. Retryable only when rv20 of S exists (advisor B1): adopted. Keys run alone in `SubjectKey` order and iv_rv20 sorts first (`reconcile.go:194-198`); Decision 10 now fails the key whenever the stored `data_day` is before S, and `TestIvRv20Rv20OfDayAbsent` pins it.
2. rv20 `data_day` never checked (advisor B2, adversarial 2): adopted. Decision 3 requires the newest-rev rv20 row of D' to be ok with `data_day` = D'; `LatestReadings` gives the newest rev per day in any state and `WinnerReadings` prefers the highest ok rev, so the job uses the first and says so; `TestIvRv20Rv20DataDayMustMatch`.
3. Owner step is wrong, entry order (advisor B3, adversarial 5 and must-fix 1): adopted. Entry 1 is the compute plus docs rule entry (Modules names it), entries 2 and 3 come after it, 3 after 2; Decision 12 deletes the owner step. The new entry is numbered 1, not 0.
4. Routing of the key (advisor B4): adopted. Decision 9 and entry 2 item 2, `TestPlanHasIvRv20Keys` and `TestIvRv20KeyIsNotAnUnderlyingKey`.
5. Age 5 calendar days (advisor, secondary): adopted for rule, job and both cells; it replaces the advisor's first definition A5 (4), which the advisor itself widened to 5 in its review.
6. Same-package tests (adversarial 6): adopted. `hv_test.go`, `rv_test.go`, `ease_test.go` and `ivp_ibkr_test.go` are in entry 2 Files with `job_test.go:264`; `ivinput_test.go` was checked and not added (it runs single keys, `:334`) and `types_test.go` uses a stub. Not adopted: dropping the Retryable wait or aligning it with `runMarket` (adversarial 3), for the reason in Doubt 4.
7. Wrong cites (adversarial 5): adopted (`combine.go:15`, `steps.go:116`, `member.go:265-270`, `:38`, `:406-421`, `types.go:16`, `cells.go:75`).
8. `git grep -c` prints nothing without a match: adopted; the checks use `git grep -n` and "prints nothing".
9. Acceptance phrased as run results, line-count diff shapes: adopted. Items name tests and their literals; the runs are under Checked by CI; line-count limits are dropped from the entries.
10. Stored code-3 reading on the page: adopted in Decision 5 (Gap `stale_input`, as `failure()` does) with `TestIvRvCellStoredCode3IsStale`. `home_market` own `data_day` parse: adopted (entry 4 item 1).
11. Retry window: adopted as a named fact (Decision 10, Today, Doubt 5). Not adopted: moving the 5-day rule into the job only and letting the page show what is stored (adversarial 3), because a stored ok figure still ages after it is stored.
12. Golden update flag: adopted (hand edit, Doubt 8).
