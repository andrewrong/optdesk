# Compute core: the pure package of figures, behind interfaces, on mature libraries

Author: the optdesk designer
Owner approved: (pending)
Revision: 3

Source: the owner's decisions of 2026-11-07 and 2026-11-08 (order of work compute, then risk and alerts, then strategy; a mature library wherever one exists; own glue code, even thin, sits behind an interface so a library can replace it),
the scheduler design `docs/design/scheduler.md` rev 8 (pull request 335 of andrewrong/optdesk, head b348fd7, not on main yet), `docs/research/go-quant-ecosystem.md` (on main, #343), the restart plan v2.5 with its rev 2.1 text (the dispatcher's plan file; cited P:<line>),
the rule of `docs/conventions/register.toml` (its header: a refactor is a history line, a semantic change a version bump), schema v3 of the database, the adversarial review of rev 2 of the first draft ("compute design adversarial review, rev 2", cited ADV:<line>) and the compute design adversarial review, rev 1 of the four files (cited REV1:<finding>); both kept by the dispatcher, not committed.
Read on main at af7e583: `AGENTS.md` (line 40), `docs/design/modules.md`, `docs/GLOSSARY.md`, `docs/conventions/register.toml`, `docs/engineering.md`, `internal/contracts/api.go`, `internal/clock/api.go`, `internal/store/api.go`, `arch.toml`, `.go-arch-lint.yml`, `scripts/arch/main.go`, `go.mod`.
This is one of four designs that replace the first compute draft: this file (the pure package, lands first: no issue here waits for another design), `compute-fred-series.md`, `compute-figures.md` (readings store, rules table, register entries, job, wire) and `compute-calendar.md`.

Order of the four: core, figures, FRED, calendar. This file lands first because `compute-figures.md` uses its `Figures`, `Rules` and `Units`; figures lands before FRED (migration 0014 after 0013), FRED before calendar.

Goal: one pure package `internal/compute` that turns typed series into figures, built on gonum and go-talib, with every piece of our own arithmetic behind an interface and one golden contract suite that any replacement must pass. It stores nothing and reads nothing; the readings store and the jobs are `compute-figures.md`.
Not given here: a figure of an equity name from closes (no dataset holds closes of a stock yet), option pricing, Beta, and the pages.

## What main holds today

- `internal/compute/` does not exist. `arch.toml` has `[component."compute"]` (arch.toml:71-73); `docs/design/modules.md` has the row compute (paths `internal/compute/** docs/contracts/compute.md`, depends `contracts`, modules.md:85) and says "forbidden: any I/O" (modules.md:110).
- `go.mod` has go-toml/v2, clockwork, uuid and grpc, and no gonum, go-talib or decimal. `docs/engineering.md` lines 7 to 11 still name QuantLib, `arch` and bump-and-reprice (the rewrite is `compute-figures.md`, `docs-compute-rules`).
- `tools/` does not exist and no row of modules.md matches it (the compute row gains `tools/compute-golden/**` here). AGENTS.md:40 (pull request #342) already allows what the generator needs: "A dev-only generator under `tools/<name>/` is its own uv project with its own committed `uv.lock`; it is not run in CI and nothing at run time imports it". Nothing waits for the owner.
- GLOSSARY defines the arithmetic used here: HV(n) (GLOSSARY.md:65), own IV30 (GLOSSARY.md:76), own IVP and IVR, convention `ivp_v1` (GLOSSARY.md:78), the 3-year percentile of `N225_IV` (GLOSSARY.md:87), the 200-day mean and the VIX over VIX3M vote (market-weather lines).

## Decisions

1. Pure and typed. Inputs are `Series` of `{Day, Value}` or typed records of `contracts`; the output is a `Figure`. No store, no clock, no file. There is no "now": every figure method takes the as-of day (`asOf`).
2. A missing value is marked missing. `Figure.Status` is `ok` or `missing`; a missing figure has `Reason` (`no_input`, `short_history`, `stale_input`, `zero_range`, `bad_input`) and `Value` 0. No method returns NaN or Inf, a default, a previous value or a recomputed one (IV and Greeks come only from IBKR model values, plan rev 2.1; a missing one is never recomputed).
3. Seven interfaces in `internal/compute/api.go`: `Indicators` (go-talib), `Stats` (gonum), `Rolling`, `SeriesOps`, `Units` (own glue) and `Rules` and `Figures`. Own glue sits behind an interface (owner's rule): the rules table behind `Rules` is built in `compute-figures.md`; here it is a declaration and a fake.
4. Libraries. `gonum.org/v1/gonum` v0.17.0 (BSD-3-Clause) behind `Stats` (`stat.Mean`, `stat.StdDev`); `github.com/markcheno/go-talib` (MIT, no tags, pinned by pseudo-version) behind `Indicators`. A percent rank by `Below` or `AtOrBelow` is a count with the glossary's tie rule: the gonum issue states in its `Design:` line what it checked and why this stays ours. No `shopspring/decimal`: rounding is `math/big` of the standard library behind `Units`. The pull requests state `go mod graph` output; no claim about transitive modules is made here (ADV:61).
5. Parameters are fixed per figure id, not per call. `Rules.Rule(id)` gives the version, the convention, the unit, the scale and the numbers of the figure (window, minimum valid, years, trading days, `MaxAgeDays`, `Compare`); `Compare` (`Below` or `AtOrBelow`) is fixed per id by the register entry and is not a runtime parameter. The table is built from the register in `compute-figures.md`. The numbers are typed fields of `Rule` in `api.go`: a changed value is a rules change; a new kind of number (a field) and a new kind of figure (a typed `Figures` method) are interface issues, small and additive, and are not claimed otherwise (REV1:10).
6. Day semantics. `Figure.Day` is the as-of day the caller passes; `DataDay` is the day of the newest input point. A figure whose newest input is older than `MaxAgeDays` is missing with `stale_input`; the number is the rule's (4 calendar days for a figure on own IV30, GLOSSARY.md:72; 8 for a figure on FRED series, which lag up to 4 sessions, register.toml:1112, 1142: `compute-figures.md` Decision 6).
7. Reading a stored series. `SeriesOps.FromSeriesPoints` takes `[]contracts.SeriesPoint` (vintages and `no_value` rows) and keeps, per observation date, the point of the latest `realtime_start` whose status is `present` and whose value is not null; a duplicate day cannot reach a figure (ADV:26).
8. Warm-up. A series function returns a slice of the input's length with `NaN` before the first defined value; `Figures` turns an undefined entry into `missing`.
9. IVP rule, flagged for the owner: GLOSSARY.md:78 (`ivp_v1`, share of the past 252 valid sessions at or below today's, today excluded, none under 252 valid) wins over P:84 ("strictly below"); `IvPercentile` is `AtOrBelow` and the legacy "strictly below, 200 values" rule is not produced.
10. Percentile of index levels, named percentile because GLOSSARY.md:73-74 keeps "IV rank" for (x-min)/(max-min) and says never to mix them (REV1:11). `market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y` and `market.sp500_pctile_3y` are new register ids that cite the `N225_IV` rule of GLOSSARY.md:87: the share of stored daily values in the 3 calendar years before the day, the day excluded, strictly below the value of the day, missing days skipped, no value before the stored history spans 3 years (`short_history`). `Below` is fixed in each id's register entry.
11. Golden suites. One suite per interface in `internal/compute/contracttest` (non-test package; it imports `testing` like `testing/fstest`; it is a test-kit exception to the declarations-only entry of modules.md: the entry cell names it so that a replacement can run the suite, `cmd/ot` does not import it, and a package outside compute imports it from `_test.go` files only), fixtures under `contracttest/testdata/` read by `//go:embed`, so no caller depends on its working directory (ADV:42, ADV:60). The first fixture is `units.json`: the unit conversions of P:33 (theta per calendar day, vega per vol point, IV as a decimal) and the half-even rounding of every stored figure, with ties that are exact in binary at scale 6 (0.0078125 gives "0.007812", half up would give "0.007813"; 0.0234375 gives "0.023438"). Products are written `float64(x*y)+z` in `figures`, so a fused multiply-add on arm64 cannot change a stored digit against amd64 (Go spec: an explicit conversion rounds). TA-Lib parity comes later and needs the owner's sentence (Modules).

## Modules

Edits of `docs/design/modules.md` that ride in this design's pull request (the design review judges them; `scripts/arch` reads them):

| place | change |
|---|---|
| row compute (and the bullet on **entry**) | the bullet gains the test-kit exception of Decision 11; paths gain `tools/compute-golden/**`; entry becomes `internal/compute/api.go internal/compute/contracttest`; tests gain `internal/compute/contracttest`; depends stays `contracts` |
| text of `ot`, list "one constructor each" (read by `parseImpl`, scripts/arch/main.go:83) | gains `internal/compute/own`, `internal/compute/gonumstats`, `internal/compute/talibind`, `internal/compute/figures` (`internal/compute/contracttest` is not in the list: it is a test kit with no constructor and `cmd/ot` has no use for it; the other three designs add theirs; the scheduler adds `internal/runner/sched`; the edits are merged in the order core, figures, FRED, calendar) |

`.go-arch-lint.yml` is a repo path, generated by `go run ./scripts/arch`, never edited by hand: an issue that adds a package is a judged two-module entry with repo (precedent: `home-page-compose` of `docs/design/home-page.md`). Its regenerated content is read in the acceptance by a grep and decided by the `checks` run of `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, under Checked by CI (the acceptance checker refuses an item that runs `go run`). `arch.toml` does not change: the compute components come from the modules.md edits of this design. `go.mod` and `go.sum` are repo too. `.go-arch-lint.yml` excludes `_test.go` files.

No owner sentence is needed: AGENTS.md:40 is on main, so `compute-golden-generator` waits for nothing but `compute-figures-iv`. It and `compute-talib-indicators` are off the figures' path: the owner may strike both at the design review and no other issue changes (REV1:C3).

| # | issue | module | order |
|---|---|---|---|
| 1 | compute-interface (interface) | compute, plus repo (judged: `.go-arch-lint.yml`) | first |
| 2 | compute-fixtures-hand | compute, plus repo (judged: `.go-arch-lint.yml`) | after 1 (compute-interface) |
| 3 | compute-contracttest | compute | after 2 (compute-fixtures-hand) |
| 4 | compute-own-glue | compute, plus repo (judged: `.go-arch-lint.yml`) | after 1, 2, 3 (compute-interface, compute-fixtures-hand, compute-contracttest) |
| 5 | compute-gonum-stats | compute, plus repo (judged: go.mod, go.sum, `.go-arch-lint.yml`) | after 1, 2, 3, 4 (own-glue first: both edit `.go-arch-lint.yml`) |
| 6 | compute-figures-series | compute, plus repo (judged: `.go-arch-lint.yml`) | after 1, 2, 4, 5 |
| 7 | compute-figures-iv | compute | after 6 (compute-figures-series) |
| 8 | compute-golden-generator | compute | after 7 (compute-figures-iv) |
| 9 | compute-talib-indicators | compute, plus repo (judged: go.mod, go.sum, `.go-arch-lint.yml`) | after 1, 2, 4, 5, 6, 8 |

The numbers are the entries of `## Issues` in this file. Two entries that share a file name each other's number: `.go-arch-lint.yml` (1, 2, 4, 5, 6, 9), `go.mod` and `go.sum` (5, 9), `iv_pending.go` and its test (6, 7).

## Interface

Owner: compute. Frozen by `compute-interface`; read by every issue here, by `compute-figures.md` and by the page designs. Declarations only; standard library plus `contracts`. `internal/compute/api.go` (package `compute`):

- Types: `Point{Day contracts.Day; Value float64}`; `Series []Point` (ascending, one point per day); `Status` (`ok`, `missing`); `Reason` (the five of Decision 2); `Compare` (`Below`, `AtOrBelow`);
  `Rule{ID string; Version int; Convention string; Unit string; Scale int; Compare Compare; N, Window, MinValid, Years, TradingDays, MaxAgeDays int; Hash string}` (the numbers of one register entry, keyed by id and version; `Hash` is its rule hash; `Unit` is one word of the `reading_unit` enum of schema v3: `decimal`, `percent`, `vol_point`, `per_calendar_day`, `count`, `days`, `money`, `ratio`, `text_tag`; the word of each id is in the register entry, `compute-figures.md` Decision 5);
  `Figure{ID string; Version int; Subject string; Day, DataDay contracts.Day; Value float64; Status Status; Reason Reason; Observations int; StaleRate bool}`;
  `StrikeQuote{Strike, CallIV, PutIV, CallMid, PutMid float64; HasCall, HasPut bool}` and `ExpiryQuotes{MinutesToExpiry int; Strikes []StrikeQuote}` (all strikes of one expiry, IBKR model values only; an IV of 0 means no model IV, and no model IV is exactly 0; a mid of 0 with `Has` true is not allowed).
- `Indicators`: `SMA`, `EMA`, `WMA`, `RSI`, `ROC` (`values []float64, period int`), `MACD` (`values, fast, slow, signal`: macd, signal, histogram), `BBands` (`values, period, upDev, downDev, maType`: upper, middle, lower), `ATR` (`high, low, close, period`); each `([]float64, error)` of the input's length with the Decision 8 warm-up; the error is a `contracts.TypedError` (`Permanent` for a bad period or unequal lengths).
- `Stats`: `Mean`, `StdDev` (sample, ddof 1), `PercentileRank(values, x, Compare)`; an empty input is a `Permanent` error, never NaN.
- `Rolling` (window loop over `Stats`): `Mean`, `StdDev`, `Min`, `Max`, `PercentileRank(values, window, Compare)`, each `([]float64, error)` of the input's length.
- `SeriesOps`: `FromSeriesPoints([]contracts.SeriesPoint) Series` (Decision 7), `Align(a, b Series) (Series, Series)` (the days both have), `LogReturns(Series) Series` (dated by the later point), `Values(Series) []float64`.
- `Units`: `PercentToDecimal(p float64) float64`, `ThetaPerCalendarDay(perYear float64) float64` (divided by 365), `VegaPerVolPoint(perUnitVol float64) float64` (times 0.01), `RoundHalfEven(v float64, scale int) (string, error)` (the exact binary value of `v` rounded half to even at `scale` places, as plain decimal text; NaN, Inf and a negative scale are `Permanent`).
- `Rules`: `Rule(id string) (Rule, bool)`, `IDs() []string`.
- `Figures` (each returns one `Figure` with `ID`, `Version` from `Rules`, `Day = asOf`, and takes the register `id`): `HistoricalVol(id, closes Series, asOf)`, `Iv30(id, chain []ExpiryQuotes, rate Point, asOf)` (the glossary arithmetic in full: K* by smallest |C-P| over the strikes, the lower strike on a tie, the forward, the strike nearest it, the mean of call and put IV, the Cboe variance interpolation in minutes, 0 and 1 day ignored, exactly 30 days alone, no extrapolation; `StaleRate` is true when `rate.Day != asOf`), `IvPercentile(id, iv30 Series, asOf)`, `IvRank(id, iv30 Series, asOf)`, `MovingAverageGap(id, closes Series, asOf)` (newest close over the mean of the last `N` minus 1), `PercentileInYears(id, s Series, asOf)`, `Ratio(id, num, den Series, asOf)` (same day; zero denominator `bad_input`).
- Constructor function types: `NewIndicatorsFunc`, `NewStatsFunc`, `NewRollingFunc func(Stats) Rolling`, `NewSeriesOpsFunc`, `NewUnitsFunc`, `NewRulesFunc`, `NewFiguresFunc func(Stats, Rolling, SeriesOps, Rules) Figures`.
- `docs/contracts/compute.md`: responsibility, inputs, outputs, the invariants of Decisions 2, 6, 7 and 8, version history, contract tests.

## Issues

### compute-interface (interface)

- Module: compute, plus repo (judged: the regenerated `.go-arch-lint.yml`). Size: S. Depends on: none.
- Order: first. Interface change: new file `internal/compute/api.go`; the modules.md edits above are in the tree. This is the first Go file under `internal/compute`, and `scripts/arch` keeps a component only for a directory that holds Go files, so the issue adds the `compute` component to `.go-arch-lint.yml`. `arch.toml` already holds the compute components from this design's modules.md edits and does not change (a tree with `internal/compute/api.go` and `internal/compute/own/` regenerates only `.go-arch-lint.yml`), so no issue lists it.
- Design: docs/design/compute-core.md#Interface
- Files: internal/compute/api.go, internal/compute/api_test.go, docs/contracts/compute.md, .go-arch-lint.yml

- Goal: the declarations of the Interface section and the contract document; `api_test.go` tests the contract from both sides (a fake of each of the seven interfaces, a caller through the interface only).

Acceptance:

1. `grep -c "^type [A-Za-z]* interface" internal/compute/api.go` prints 7 and `grep -c "^func " internal/compute/api.go` prints 0.
2. `grep -c "time.Sleep" internal/compute/api_test.go` prints 0; `grep -c "^func Test" internal/compute/api_test.go` equals `grep -c "t.Parallel()" internal/compute/api_test.go`.
3. `go list -deps ./internal/compute | grep -c "gonum\|go-talib\|shopspring"` prints 0 and `go list -deps ./internal/compute | grep optdesk | grep -v 'internal/compute$'` lists only `internal/contracts` and its `gen` (the package itself is excluded from the list).
4. `TestMissingNeverNaN` passes: a missing `Figure` has `Value == 0` and a non-empty `Reason`.
5. `grep -c "latest realtime_start\|data_day\|short_history" docs/contracts/compute.md` is at least 3.
6. `grep -c '^  compute:' .go-arch-lint.yml` prints 2 (the component and its `deps` entry).
7. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the four paths of the Files line.

Checked by CI: `go test ./internal/compute -race -count=1`; the `checks` arch check (`go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change).

### compute-fixtures-hand

- Module: compute, plus repo (judged: the regenerated `.go-arch-lint.yml`). Size: M. Depends on: compute-interface.
- Order: after 1 (compute-interface). This is the first fixture of the core: `units.json` (P:33).
- Design: docs/design/compute-core.md#Decisions
- Files: internal/compute/contracttest/golden.go, internal/compute/contracttest/golden_test.go, internal/compute/contracttest/testdata/units.json, internal/compute/contracttest/testdata/stats.json, internal/compute/contracttest/testdata/series.json, internal/compute/contracttest/testdata/figures.json, .go-arch-lint.yml

- Goal: the loader of the embedded fixtures (`golden.go`, `//go:embed testdata/*.json`) and four hand-worked files; every case has `name`, `group`, `input`, `expected`, `tolerance` and a `working` string that shows the arithmetic. `units.json` first: `theta_per_calendar_day` (-36.5 per year gives -0.1), `vega_per_vol_point` (25 per unit of vol gives 0.25), `iv_decimal` (percent 25 gives 0.25) and `half_even` at scale 2 and 0 (0.125 gives "0.12", 0.375 "0.38", 2.5 "2", 3.5 "4", -2.5 "-2", 0.5 "0") and at scale 6 (0.0078125 "0.007812", 0.0234375 "0.023438", -0.0078125 "-0.007812"; each exact in binary). `stats.json` and `series.json`: literals small enough to check by hand; a case of 252 or 200 values is written as a ramp spec `{"ramp": {"from": 1, "step": 1, "count": 252}}` that the loader expands, with its answer worked in `working` by arithmetic (REV1:C6) (vintage choice with a `no_value` row, alignment with a gap, log returns, rolling windows, percent rank with ties at both `Compare`). `figures.json`: its `group` is the name of the `Figures` method and the `input` of each case carries a `rule` object (the numbers of one `Rule`) from which the suite builds a fake `Rules`, so no case needs the real rules table: HV of constant log returns, IVP at 251 and 252 valid sessions with ties, IVR with zero range, MA gap at 199 and 200 closes, percentile in 3 years (day excluded, gap skipped, `Below` against `AtOrBelow`), ratio, and Iv30 for two properties exact by hand (an exact 30-day expiry alone; two expiries with equal IV give that IV); one case for each `Reason`. The independent cross-check of Iv30 and the TA-Lib indicators come with `compute-golden-generator`.

Acceptance:

1. `TestFixtureShape` passes (each file parses, every case has the five keys and a `working`) and `TestUnitsFirst` passes (`units.json` is the first of the names the loader lists).
2. `grep -c '"group": "half_even"' internal/compute/contracttest/testdata/units.json` is at least 9 and `grep -c '0.0078125' internal/compute/contracttest/testdata/units.json` is at least 1; `TestRampExpands` passes (count, step, from); the groups `theta_per_calendar_day`, `vega_per_vol_point` and `iv_decimal` each occur at least once.
3. `TestGoldenCoversFigures` passes: one `ok` case for each `Figures` method and one case for each of the five reasons.
4. `grep -rc "time.Now\|os.Getenv" internal/compute/contracttest/golden.go` prints 0; each Go file is under 600 lines.
5. `grep -c 'internal/compute/contracttest' .go-arch-lint.yml` prints 1 (the regenerated file lists the package).
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: `go test ./internal/compute/contracttest -race -count=1`; the `checks` arch check (`go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change).

### compute-contracttest

- Module: compute. Size: M. Depends on: compute-fixtures-hand.
- Order: after 2 (compute-fixtures-hand).
- Design: docs/design/compute-core.md#Decisions
- Files: internal/compute/contracttest/units.go, internal/compute/contracttest/stats.go, internal/compute/contracttest/series.go, internal/compute/contracttest/figures.go, internal/compute/contracttest/contracttest_test.go

- Goal: the suites `contracttest.Units`, `Stats`, `Rolling`, `SeriesOps` and `Figures` (the `Indicators` suite comes with `compute-talib-indicators`). Signatures, each `func(t *testing.T, ...)`: `Units(t, compute.Units)`, `Stats(t, compute.Stats)`, `Rolling(t, compute.Rolling)`, `SeriesOps(t, compute.SeriesOps)`, and `Figures(t, newFigures compute.NewFiguresFunc, stats compute.Stats, rolling compute.Rolling, ops compute.SeriesOps, methods ...string)`. `Figures` takes no `Rules`: for each case it builds a fake `Rules` from the `rule` object of the case (compute-fixtures-hand) and calls `newFigures` with it. `methods` selects by the `group` of a case, the name of a `Figures` method (`HistoricalVol`, `Iv30`, ...): no name runs every case, a name that no case carries fails the suite. A caller whose methods are not all finished therefore passes the finished names and never edits `figures.go` of the suite. Each runs one parallel subtest per case of the embedded fixtures with the case's tolerance. The self-test runs each suite against a correct table-driven fake (passes) and a mutated fake (one value off by more than the tolerance; it must fail, seen through a recording substitute for `testing.TB`).

Acceptance:

1. `grep -c "t.Parallel()" internal/compute/contracttest/*.go` is at least the number of `t.Run(` calls in the same files (the greps run over the five files of the Files line and `golden.go`).
2. `TestSuiteRejectsMutant` passes with one subtest for each of the five suites (`Units`, `Stats`, `Rolling`, `SeriesOps`, `Figures`), each seen to fail on its mutant through the recording substitute; the test counts the five itself, nothing counts `-v` indentation (ADV:71).
3. `TestSuiteWarmupNaN` passes: an implementation that returns 0 for a warm-up entry fails the suite; `TestFiguresMethodSelector` passes (a named method runs only its cases; an unknown name fails).
4. `grep -c "time.Sleep\|time.Now\|os.Getenv\|1e-" internal/compute/contracttest/units.go internal/compute/contracttest/stats.go internal/compute/contracttest/series.go internal/compute/contracttest/figures.go` prints 0 (tolerances come from the fixtures); `go list -deps ./internal/compute/contracttest | grep -c "gonum\|go-talib"` prints 0.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: `go test ./internal/compute/contracttest -race -count=1`.

### compute-own-glue

- Module: compute, plus repo (judged: the regenerated `.go-arch-lint.yml`). Size: M. Depends on: compute-contracttest.
- Order: after 1, 2, 3 (compute-interface, compute-fixtures-hand, compute-contracttest); 1 and 2 edit `.go-arch-lint.yml` too.
- Design: docs/design/compute-core.md#Decisions
- Files: internal/compute/own/units.go, internal/compute/own/series.go, internal/compute/own/rolling.go, internal/compute/own/own_test.go, .go-arch-lint.yml

- Goal: our own glue behind the interfaces: `own.NewUnits` (conversions; `RoundHalfEven` through `math/big`: the exact value of the float as a `big.Rat`, scaled, rounded half to even), `own.NewSeriesOps` (Decision 7, align, log returns) and `own.NewRolling` (the window loop over the injected `Stats`). No library, no `time`.

Acceptance:

1. `grep -c "contracttest\.\(Units\|SeriesOps\|Rolling\)" internal/compute/own/own_test.go` is at least 3 (Rolling runs with a small in-test `Stats` over the standard library, so no gonum).
2. `go list -deps ./internal/compute/own | grep -c "gonum\|go-talib\|shopspring"` prints 0.
3. `TestFromSeriesPointsLatestVintage` passes (two vintages of one date give the later `realtime_start`; a `no_value` row and a null value are dropped; no duplicate day) and `TestRoundHalfEvenBadInput` passes (NaN, Inf, negative scale are `Permanent`).
4. `TestAlignGap`, `TestLogReturnsGap` pass (a return is dated by the later point); `TestRollingMatchesNaive` (200 seeded windows, mean and stdev within 1e-12) and `TestRollingWarmup` pass.
5. `grep -c 'internal/compute/own' .go-arch-lint.yml` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: `go test ./internal/compute/own -race -count=1`; the `checks` arch check (`go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change).

### compute-gonum-stats

- Module: compute, plus repo (judged: `go.mod`, `go.sum`, the regenerated `.go-arch-lint.yml`). Size: S. Depends on: compute-contracttest.
- Order: after 1, 2, 3, 4 (compute-interface, compute-fixtures-hand, compute-contracttest, compute-own-glue); 1, 2 and 4 edit `.go-arch-lint.yml` too.
- Design: docs/design/compute-core.md#Decisions
- Files: internal/compute/gonumstats/stats.go, internal/compute/gonumstats/stats_test.go, go.mod, go.sum, .go-arch-lint.yml

- Goal: `gonumstats.New` (`compute.NewStatsFunc`) with gonum v0.17.0: `stat.Mean`, `stat.StdDev`; `PercentileRank` counts values below or at or below `x` itself (ours: the tie rule is the glossary's). Bad input is a `contracts.TypedError`, never NaN.
- New direct dependency: `gonum.org/v1/gonum` v0.17.0, BSD-3-Clause; widely used, stable API, permissive. The pull request states `go mod graph | grep gonum`.

Acceptance:

1. `grep -c "contracttest.Stats" internal/compute/gonumstats/stats_test.go` prints 1.
2. `grep -c "gonum.org/v1/gonum v0.17.0" go.mod` prints 1; `go list -deps ./internal/compute | grep -c gonum` prints 0.
3. `TestPercentileRankBothWays` passes (ties; `x` outside the range) and `TestBadInput` passes (empty input, one value for the standard deviation: `Permanent`, no NaN).
4. `grep -c 'internal/compute/gonumstats' .go-arch-lint.yml` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: `go test ./internal/compute/gonumstats -race -count=1`; the `checks` arch check (`go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change).

### compute-figures-series

- Module: compute, plus repo (judged: the regenerated `.go-arch-lint.yml`). Size: M. Depends on: compute-own-glue, compute-gonum-stats.
- Order: after 1, 2, 4, 5 (compute-interface, compute-fixtures-hand, compute-own-glue, compute-gonum-stats); all four edit `.go-arch-lint.yml` too.
- Design: docs/design/compute-core.md#Interface
- Files: internal/compute/figures/figures.go, internal/compute/figures/series.go, internal/compute/figures/series_test.go, internal/compute/figures/iv_pending.go, internal/compute/figures/iv_pending_test.go, .go-arch-lint.yml

- Goal: `figures.New` (`compute.NewFiguresFunc`) with `HistoricalVol` (HV(n), GLOSSARY.md line 65: sample stdev of the last `N` log returns times the square root of `TradingDays`; `N`+1 closes else `short_history`; a zero or negative close `bad_input`), `MovingAverageGap` (through `Rolling.Mean`, not `Indicators`), `PercentileInYears` (GLOSSARY.md line 87 for the four index ids: calendar years before the day, day excluded, missing skipped, `Observations` the count, `short_history` when the history does not reach back `Years`) and `Ratio`. Every method reads its numbers from `Rules`, applies `MaxAgeDays` against `asOf` (`stale_input`), sets `DataDay`, and leaves nothing to a caller. The type is `calc` (receiver of every method). The three IV methods (`Iv30`, `IvPercentile`, `IvRank`) are stubs that return a missing `bad_input`, and live only in `iv_pending.go` with their test `TestUnfinishedAreMissing` in `iv_pending_test.go`, so that `compute-figures-iv` removes both files and never edits `figures.go` (REV1:2).

Acceptance:

1. `grep -c "contracttest.Figures" internal/compute/figures/series_test.go` prints 1 and `grep -c '"HistoricalVol", "MovingAverageGap", "PercentileInYears", "Ratio"' internal/compute/figures/series_test.go` prints 1 (the call passes the four finished methods to the `methods` argument, so the three IV stubs are not run); `TestUnfinishedAreMissing` passes.
2. `TestHistoricalVol` passes (constant log returns give the golden value; 30 closes `short_history`; a zero close `bad_input`) and `grep -c "252" internal/compute/figures/series.go` prints 0 (the factor is `Rule.TradingDays`).
3. `TestMovingAverageGap` (199 closes `short_history`, 200 ok, gap equal to `close/mean - 1` to 1e-12), `TestPercentileInYears` (day excluded, gap skipped, `Below` and `AtOrBelow` differ on a tie, two years of history for `Years` 3 is `short_history`) and `TestRatio` (zero denominator `bad_input`, stale denominator `stale_input`) pass.
4. `TestStaleInput` passes: an input older than `MaxAgeDays` against `asOf` is `missing` with `stale_input` and `DataDay` is the newest input day; `TestCompareFromRule` passes (the same series ranks differently under two rules; no method has a `Compare` parameter).
5. `go list -deps ./internal/compute/figures | grep -c "gonum\|go-talib"` prints 0 and `grep -c "time\.Now" internal/compute/figures/*.go` prints 0 (a package list never holds a symbol); `grep -c 'internal/compute/figures' .go-arch-lint.yml` prints 1; each Go file is under 600 lines.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the six paths of the Files line.

Checked by CI: `go test ./internal/compute/figures -race -count=1`; the `checks` arch check (`go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change).

### compute-figures-iv

- Module: compute. Size: M. Depends on: compute-figures-series.
- Order: after 6 (compute-figures-series); both edit `iv_pending.go` and `iv_pending_test.go`.
- Design: docs/design/compute-core.md#Decisions
- Files: internal/compute/figures/iv30.go, internal/compute/figures/ivp.go, internal/compute/figures/iv_test.go, internal/compute/figures/iv_pending.go, internal/compute/figures/iv_pending_test.go

- Goal: `Iv30` (the arithmetic of GLOSSARY.md line 76 in full, Interface section: per expiry K* is the strike with the smallest |C-P|, the lower strike on a tie, the forward `F = K* + exp(rT)(C-P)` with `T` from minutes, the strike nearest `F`, ATM IV as the mean of call and put model IV; two expiries around 30 days joined by the Cboe variance interpolation in minutes; an expiry of exactly 30 days alone; 0 and 1 day ignored; no extrapolation; a missing IV on either leg of the chosen strike makes the expiry unusable, never recomputed; `StaleRate`), `IvPercentile` (`ivp_v1`, Decision 9: at or below, 252 valid sessions, today excluded, `short_history` under `MinValid`) and `IvRank` (`zero_range`). Arithmetic only: no pricing, no solving. `iv_pending.go` and `iv_pending_test.go` are deleted in this issue (their methods now live in `iv30.go` and `ivp.go`); `figures.go` is not edited. `Iv30` is split into the helpers `strikeStar`, `forward`, `atmIV` and `interpolate` (gocyclo 20, funlen 80 in `.golangci.yml`).

Acceptance:

1. `TestIv30`, `TestIvPercentile` and `TestIvRank` pass the cases of `figures.json` (exact 30-day expiry alone, equal-IV expiries, 0 and 1 day ignored, one side only `no_input`, a missing call or put IV `no_input`, a tie in |C-P| takes the lower strike).
2. `TestIvPercentile` and `TestIvRank` pass at 251 and 252 valid sessions, today excluded, ties counted `AtOrBelow`, and a zero range; `TestIv30StaleRate` passes (`rate.Day != asOf` gives `StaleRate` true).
3. `grep -c "Scholes\|CDF\|Newton\|bisect" internal/compute/figures/iv30.go` prints 0; `go list -deps ./internal/compute/figures | grep -c "gonum\|go-talib"` prints 0.
4. `grep -c "contracttest.Figures" internal/compute/figures/iv_test.go` prints 1 (called with no method name: all seven methods, the stubs are gone) and `TestAllFiguresFinished` passes (no method returns the stub); `test ! -e internal/compute/figures/iv_pending.go && test ! -e internal/compute/figures/iv_pending_test.go` holds; `git diff $(git merge-base HEAD origin/main) -- internal/compute/figures/figures.go` is empty; `grep -c "^func " internal/compute/figures/iv30.go` is at least 5.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line; each file is under 600 lines.

Checked by CI: `go test ./internal/compute/figures -race -count=1`; `golangci-lint` (gocyclo, funlen) on the package.

### compute-golden-generator

- Module: compute. Size: M. Depends on: compute-figures-iv.
- Order: after 7 (compute-figures-iv); AGENTS.md line 40 is on main (#342), so no owner sentence is awaited and no grep gates the cut. Needs a host with the network once (the implementer runs the generator and says so in the pull request).
- Design: docs/design/compute-core.md#Decisions
- Files: tools/compute-golden/gen.py, tools/compute-golden/pyproject.toml, tools/compute-golden/uv.lock, internal/compute/contracttest/testdata/indicators.json, internal/compute/contracttest/testdata/iv30_independent.json, internal/compute/contracttest/provenance_test.go, internal/compute/figures/iv30_independent_test.go

- Goal: the dev-only generator (own `pyproject.toml` and `uv.lock`; not in CI, not in the runtime: AGENTS.md line 40) and its two files: `indicators.json` (the eight `Indicators` functions, three parameter sets each, outputs of Python TA-Lib, `null` for warm-up) and `iv30_independent.json` (Iv30 by an independent numpy implementation of the glossary arithmetic over seeded chains, so the hand-worked cases of `compute-fixtures-hand` are not the only check, ADV:69). Each file starts with a `provenance` object (generator, libraries with versions, `generated_at`, `gen_sha256`, `lock_sha256`).
- New dependencies: none in Go. Python, dev only, in its own project: `TA-Lib` (BSD) and `numpy` (BSD-3-Clause).

Acceptance:

1. `TestProvenanceHashes` passes: `gen_sha256` equals the SHA-256 of `tools/compute-golden/gen.py` and `lock_sha256` that of its `uv.lock` (paths from the module root found by walking up to `go.mod`); editing `gen.py` in a `t.TempDir()` copy makes the check fail.
2. `TestIv30MatchesIndependent` passes within each case's tolerance.
3. `TestGoldenCoversIndicators` passes for the 8 functions of `Indicators`; `TestFixtureLengths` passes (`expected` as long as `input`) and a mutated copy fails it.
4. `grep -rn "compute-golden" .github go.mod internal cmd` prints only the two test files; `test -f tools/compute-golden/uv.lock` holds; `git diff $(git merge-base HEAD origin/main) -- AGENTS.md pyside` is empty.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: `go test ./internal/compute/contracttest ./internal/compute/figures -race -count=1`; the first `talibind` parity run (`compute-talib-indicators`) is what shows the indicator fixtures are real TA-Lib output; a file written by hand cannot be told from a real one offline. `gen.py` itself is not run in CI.

### compute-talib-indicators

- Module: compute, plus repo (judged: `go.mod`, `go.sum`, the regenerated `.go-arch-lint.yml`). Size: M. Depends on: compute-golden-generator.
- Order: after 1, 2, 4, 5, 6, 8 (compute-interface, compute-fixtures-hand, compute-own-glue, compute-gonum-stats, compute-figures-series, compute-golden-generator); 5 also edits `go.mod` and `go.sum`, and 1, 2, 4, 5 and 6 the arch files.
- Design: docs/design/compute-core.md#Decisions
- Files: internal/compute/contracttest/indicators.go, internal/compute/talibind/indicators.go, internal/compute/talibind/wrap.go, internal/compute/talibind/indicators_test.go, internal/compute/talibind/testdata/divergence.json, go.mod, go.sum, .go-arch-lint.yml

- Goal: the `Indicators` suite and `talibind.New` (`compute.NewIndicatorsFunc`): the shortest adapter over go-talib (NaN for padding, `Permanent` for a bad period or unequal lengths), and `wrap.go` with our own corrected function for each one the parity test shows diverging from TA-Lib (EMA seeding is the suspect; a divergence is wrapped to parity, not accepted); `divergence.json` lists each wrapped function with `reason`, `first_index` and `max_abs_diff`. Nothing in the figures uses `Indicators`: it is ready for the pages that need an indicator.
- New direct dependency: `github.com/markcheno/go-talib`, MIT, pinned by pseudo-version (no tags; pure Go; float64 slices). The pull request states `go mod graph`; a reviewer who judges "maintained" differently needs the owner's word.

Acceptance:

1. `grep -c "contracttest.Indicators" internal/compute/talibind/indicators_test.go` prints 1 (the suite passes for all 8 functions as `New` returns them).
2. `grep -c "github.com/markcheno/go-talib v0.0.0-" go.mod` prints 1 (no `replace`); `go list -deps ./internal/compute | grep -c "go-talib"` prints 0.
3. `TestDivergenceListIsExact` passes: each entry has the three keys, and the raw library output differs from the fixture for exactly the listed functions (a stale entry fails; an unlisted divergence fails).
4. `TestWarmupIsNaN` and `TestBadPeriod` pass.
5. `grep -c 'internal/compute/talibind' .go-arch-lint.yml` prints 1; each Go file is under 600 lines; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eight paths of the Files line.

Checked by CI: `go test ./internal/compute/talibind -race -count=1` (the parity run against the TA-Lib fixtures), lint, the `checks` arch check (`go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change) and the ratchet.

## Left out

- `OptionMath` (price, IV, Greeks): plan rev 2.1 has no option pricing (P:37); py_vollib and QuantLib are not used. If it returns it is a new interface with a golden suite.
- Beta, `Correlation`, `Quantile`, `ZScore`, `NormalCDF` of `Stats`, and `PctChange` and `Drawdown` of `SeriesOps`: the first draft had them; no figure here uses them and a figure needs its register entry first (owner rule: Go does only glossary arithmetic). Beta (GLOSSARY.md:66) comes with a closes dataset and its entry.
- `Stoch`, `ADX`, `OBV`, `MFI`, `CCI` of go-talib: no consumer; added with their fixtures when one exists. Rejected: cutting the first indicator set to SMA only (ADV:43): the owner named the family and the parity test is the point; the figures do not wait for it.
- Departure from P:33, named: P:33 asks for unit goldens from the old repository's synthetic fixtures (用旧仓合成夹具). The fixtures of `compute-fixtures-hand` are hand-worked, each case with a `working` string: main holds no such fixture (only `contracts/fixtures` for datasets and pages), and a worked case can be checked by the owner without running code. The departure is flagged for the owner.
- "delayed +70" of P:33: a market-data rule of the broker and the pages, not a unit conversion of compute; the three conversions of `units.json` are the part that is arithmetic. Left out until a page design states what it converts.
- Daily closes of stocks, indices and ETFs: no dataset, no history rpc; `HistoricalVol` is tested here but `market.hv30_own` is not registered until a bars design gives it a producer (REV1:14).
- Not produced by any of the four designs (REV1:14): RV20 and IV-RV20 of N225, the N225_IV 3-year percentile itself (GLOSSARY.md:87), breadth, core page returns, diagnosis ratios and ranks. Each is a new typed `Figures` method or a new input kind, hence an interface issue of a later design.
- The rules table (`Rules` implementation), the register entries, the readings store and the jobs: `compute-figures.md`.
- DuckDB window SQL as a second `Rolling`, `cinar/indicator` (AGPL-3.0), `qframe`, `arrow-go`, `shopspring/decimal`: not needed now; the golden suite is what lets one replace the own `Rolling` later. Rejected: gonum for the percent rank (see Decision 4).
- Rejected: `Compare` or `years` as a call parameter (the register fixes them per id); one `Figures.Compute(id, inputs)` method (the typed methods keep the input kinds visible); generating `rules` from the register at build time (it is in `compute-figures.md`, with a test that compares the two).

## Adversarial

Read: the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1:<finding>), and, for the older answers, the review of rev 2 of the first draft (ADV). Adopted means the text above changed.

- ADV rev 2 (findings 11, 25-29, 42-43, 48, 50, 51, 60-61, 67-71, 77-78, 80): adopted as written in rev 1 of this file (arch files regenerated, `ExpiryQuotes` with all strikes, `FromSeriesPoints`, embedded fixtures, `go test -json` counts, wrapper route for go-talib); the adapter cut of ADV:43 stays not adopted (see REV1:C3).
- REV1:1 (AGENTS.md:40 exists; the grep never matches): adopted. Lines "What main holds" and Modules say so, the generator issue waits for nothing but `compute-figures-iv`, no grep gates it.
- REV1:2 (`figures.go` missing from `compute-figures-iv` Files): adopted by restructuring: the stubs and their test live in `iv_pending.go` and `iv_pending_test.go`, created by `compute-figures-series` and deleted by `compute-figures-iv`; no file crosses issues, `figures.go` is untouched there (acceptance 4 of the iv issue).
- REV1:10 (`Rule` is in `api.go`; adding a number is an interface change): adopted as a correction. Decision 5 says what is what: a changed value is a rules change; a new field and a new `Figures` method are small interface issues.
- REV1:11 (percentile and rank mixed): adopted. `PercentileInYears` and the ids `market.*_pctile_3y`; `IvRank` keeps the glossary's (x-min)/(max-min).
- REV1:14 (what the pages wait for; `market.hv30_own` without a producer): adopted in Left out; the id is no longer registered (`compute-figures.md`).
- REV1:15 (arch files and the modules.md list make the issues textually serial): not adopted: the generated files cannot be split. The serial order is stated (own-glue and gonum-stats both edit `.go-arch-lint.yml`; each Order line names the other entry's number, so gonum-stats is after own-glue); the design no longer calls them parallel.
- REV1:C1 (no golden of the stored text; 0.1234565 proves nothing): adopted. Binary-exact ties at scale 6 in `units.json` (0.0078125 gives "0.007812"); the per-id stored-text golden is `compute-rules` in `compute-figures.md`.
- REV1:C2 (Iv30 inputs undefined; same-implementer cross-check; arm64 fused multiply-add): inputs adopted in `compute-figures.md` (mapping issue); FMA adopted (`float64(x*y)+z`, Decision 11); the shared-misreading risk of the numpy check is not removed: it reads the glossary text like the Go code does, so the owner reads `working` of the hand cases at review.
- REV1:C3 (cut Indicators, go-talib, generator, talib issues): not adopted. The owner named TA-Lib parity and the mature-library rule; the two issues are off the figures' path and can be struck at review without touching another issue (Modules).
- REV1:C4 (lock-in: `Rule`, `StaleRate`, `Hash`, exported `contracttest`): kept, named in Decision 5; `StaleRate` is the glossary's `stale_rate` flag; the `contracttest` entry in the modules.md cell is what lets a replacement run the suite.
- REV1:C5 (false claims): the AGENTS.md claim and the grep are corrected (REV1:1); C:270 corrected (REV1:10).
- REV1:C6 (252 values not hand-checkable; lint limits; vacuous greps): adopted. Ramp specs expanded by the loader (`TestRampExpands`); `Iv30` split into four helpers and a function-count check; the `time.Now` check reads source files (`grep -c "time\.Now" figures/*.go`), the package list only holds package names; `contracttest.Rules` is gone from `compute-figures.md`.
- REV1 cross-file, goal check (own glue behind an interface for `figures.New`'s `Rolling` over `Stats` and for the rule-hash recipe): `Rolling` is already behind its interface and `figures.New` takes `Stats`, `Rolling`, `SeriesOps` and `Rules` as interfaces, so a library (DuckDB window SQL, Left out) replaces the ten lines of `own.NewRolling` without an edit in `figures`; `own.NewRolling(Stats)` is the test seam (a fake `Stats`). A second interface over a constructor argument of an interface would give a replacement nothing to replace, so none is added; the golden suite of `Rolling` is the guard. The hash recipe is not arithmetic that a library could replace: no library hashes the normalized source of listed Go functions, and it makes no figure. It reaches the code only as data, `Rule.Hash` through `Rules`, which is the interface; the recipe itself is the check of `compute-rules` (`compute-figures.md` Decision 12 and Adversarial). Not adopted as a new interface; the owner may ask for one at the design review.
