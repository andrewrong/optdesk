# option candidates figures: the short-put table of the option candidates page shows ITM probability and excess over cash, and a readable earnings cell

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: the owner sells puts; the valuation and the buy price set the strike, and these figures tell him whether a put is worth selling. `docs/conventions/register.toml` registers `option.candidates.itm_probability`, `option.candidates.excess_yield`, `option.candidates.iv_context` and `option.candidates.earnings_date` (carried over from option-trader, `source_hash` pending, `implemented_by` names Python functions of option-trader) and no Go code implements the first three; the page `internal/pages/option_candidates_page` shows rank, strike, bid, yield, break-even, delta, IV, marks, fit and earnings only.

Goal: in the short-put table the owner sees, per ok row, the chance that the put expires in the money, the yield over cash, and the next earnings date or a named reason for none. A figure with no input is a gap that names the input, never 0%. The page and the front end compute nothing; `internal/compute` does the maths.

Existing designs: `option-candidates.md` (the job and its stored row; its Left out names the ITM probability, volatility spread and IV rank as waiting for the bars design) and `option-candidates-page.md` (the page; its Decision on stored text only is amended by Decision 1 here). `event-sources.md` (the earnings store, Decision 7) is reused unchanged.

## Today

All paths at origin/main.

- The job `internal/jobs/optioncandidates` stores one row per contract in `candidates.option_candidate` r1 (`store.Candidate`, `internal/store/api.go:561`): `Spot`, `Strike`, `DTE`, `AnnualizedYield` (6 places), `Delta`, `Iv` (the contract IV as the IBKR fraction, `input.go:186`), `NextEarnings`, `EarningsState` and more. It stores no ITM probability and no excess yield. The cash rate is read by the job (`input.go:299`, series `DGS3MO`, newest point of 14 days, percent) and used by the gate `cash_rate`, but not stored on the row.
- The rule `internal/compute/optioncand` is exact rationals and its `purity_test.go` bans `float32` and `float64`; the `CandidateInput.Rate` is percent text. So the normal distribution cannot live there. Other compute packages (`figures`, `gonumstats`) use `math`.
- `internal/compute/api.go` has 839 lines and a ceiling of 840 in `docs/design/ratchet.json`: it is not edited here. The `figures` package (which uses `math`) implements the `Figures` interface that is declared in that file, and `scripts/arch` lets a page import only declarations-only packages, so the two functions cannot simply join `figures`: they would need an interface in `api.go`. New compute interfaces go in a package of their own (as `blockapi`, `valuationapi`, listed in `docs/contracts/compute.md`).
- The page: `Build` (`builder.go:48`) receives a `store.SeriesReader` and ignores it (`_`); the served page passes none: `cmd/ot/candidates_site.go:47` calls `Build(ctx, nil, day, ...)` although the site holds one as `rd.series` (`cmd/ot/site.go:112`, opened at `site.go:227`; `ot page` already passes a real one, `cmd/ot/page.go:31`); `Config` (`page.go:14`) has `Candidates` and `Names`. Columns are `colDef`s in `cells.go:196-222` and `groupSpecs` (`groups.go:27`) lists them per group. The Delta and IV cells copy stored text; a column is `colDef.cell`, a `func(store.Candidate) any` of the row alone (`cells.go`); `purity_test.go` bans `math/big`, `sort`, `os`, `jobs`, `notify` and the identifiers `float32` and `float64`, but not `math`. A cell is a `valueCell` (text, optional numeric value, badges) or a `gap` whose reason is one of the schema list (`no_data`, `missing_input`, `stale_input` among them), so `contracts/schemas/page.option_candidates_page.v1.json` already carries everything below (cells are an open map by column id).
- Earnings: the job reads the issuer's rows through `store.EarningsDateReader` (`input.go:202`; the reader of `event-sources.md`, the same store `internal/pages/home_companies/earnings.go:28` reads through `Config.Earnings`) and stores `NextEarnings` (the `report_date_override` of the watch entry wins) and `EarningsState` (`listed`, `absent`, `unsupported`, `error`, `none`). The page cell `earningsCell` (`cells.go:89`) shows the stored date as `YYYY-MM-DD` with the state as a badge, and a dash when there is no date, whatever the state. `absent` is never stored on the row (no rows give `none`, `input.go:214`); `none` with a `report_date_override` carries the override date. `strings.go` already holds the sentences `no_earnings`, `earnings_unsupported` and `earnings_error`.
- IV history: the job `iv_history` (`internal/jobs/ivhistory`) stores IBKR's daily IV bars (`market.iv_ibkr`). `compute/figures/ivp.go` computes the percentile from an IV30 series, but the own IV30 writer step named in the register (`OwnIv30Step`) is not in Go (`git grep OwnIv30` finds nothing outside `home_companies`). The register's `iv_context` also needs HV30 and the stored IV30 and IVP readings of the stock.

## Decisions

1. ITM probability and excess yield are computed when the page is built, in `internal/compute`, from the stored row and the cash rate, and are not stored. This contradicts `register.toml`: `option.candidates.itm_probability` says its input is the `itm_probability` of the stored scan row and excludes "a recomputation from spot, strike or volatility"; `option.candidates.excess_yield` says the page "never recomputes it from the rates" and excludes "a recomputation from the annual yield and a rate". The exclusion is about the page and the browser; here the maths is in compute, on the server, and the page copies its text. The reason: storing needs dataset r2, a migration, the store, the job and the page (five modules). The register edit (`inputs`, `excludes`, `implemented_by` of both ids) is the owner's file and goes in the same owner step as the merge of the last issue; this design is not merged without the owner's word on the deviation. Cost: the figure is not frozen as scanned and moves if the rate series is corrected. For confirmation.
2. ITM probability is the risk-neutral N(-d2) of the register: d2 = (ln(S/K) + (r - sigma^2/2) T) / (sigma sqrt(T)), with S the stored spot, K the stored strike, sigma the stored contract IV (fraction), r the cash rate (percent / 100, continuous, no dividend) and T = DTE / 365 (the year length of `optioncand/put.go`). N is the standard normal distribution function, `0.5 * erfc(-x / sqrt(2))`. It is not delta (delta is not an assignment probability, GLOSSARY) and the page never calls it that. One rounding rule, from the exact value and never from a rounded one: the float result is taken exactly (`big.Rat.SetFloat64`) and rounded half up; the text is the percent to one place, the sort key is the value in hundredths of a percent as an integer. A tie is decided by the exact binary value of the float, which is the only tie a float has. Synthetic (computed to 6 places): S 100, K 95, IV 0.30, rate 4.5, DTE 30 gives 0.275460, text 27.5%, key 2755; S 100, K 100, IV 0.30, rate 0, DTE 30 gives 0.517151, text 51.7%, key 5172. The text and the key can differ by a rounding step (27.5% against 2755); each is the exact value rounded once. No 4-place value is stored or shown.
3. Excess yield is the register's: the stored annual yield minus rate/100, exact decimal, rounded half up to 6 places (the register's rule; the stored yield has 6 places and a rate has at most 2, so the rounding rarely acts); the text says `above` (excess >= 0, so zero reads above 0.0%) or `below`, then the size as a percent to one place, half up of that 6-place value, for example `above 4.5%` and `below 1.0%`; the sort key is the signed value in hundredths of a percent. The words are English in `strings.go`, like every string of the page; the English guard of `cmd/ot/english_test.go` has an empty `allowedCJK` and is not touched. Synthetic: yield 0.09, rate 4 gives 0.050000, `above 5.0%`, key 500; yield 0.03, rate 4 gives -0.010000, `below 1.0%`, key -100. Cheap, so it is in the cut. US puts only: the page has no JP rows (`option-candidates-page.md` Left out).
4. Gaps, one reason each, never 0: spot, strike or IV missing, not a number, or not above zero, or DTE below 1: `missing_input` naming the input (`ITM probability: IV not stored`); no rate point: `missing_input` (`no cash rate`); newest rate older than 7 days before the session day: `stale_input` (`cash rate is 9 days old`). 7 is the `max_rate_age_days` of `job.toml`, held by the page as a constant named for that key (Doubt 2). The age is measured differently: the job measures it from the run's clock day to the rate day when it gates the row, the page from the stored session day to the rate day; a row the job passed can therefore show a figure from a rate the job would have called stale only if the series changed in between, and the page never shows a gate result of its own. Excess yield with no stored annual yield is `missing_input`.
5. Where: a new compute package `optionfig` implements an interface in a new package `optionfigapi` (its `api.go`, declarations only; listed in `docs/contracts/compute.md` like `valuationapi`). `internal/compute/api.go` is not touched, and the existing `figures` package is not used for the reason in Today. The page takes it as `Config.Figures`; a nil one gives the gap `no_reader` in the two new columns; `cmd/ot/wire.go` sets it. The page reads the rate once per group from the `store.SeriesReader` it receives, with the job's rule (newest point of the 14 days up to the session day), formats the point's `float64` with `strconv` (the page's `purity_test.go` bans the identifier `float64`, so the read is one expression with no declared float type; `jobs.FloatToDecimal` is not importable) and passes the text and day; the maths and the age check are in compute. The page imports no `math` (it is allowed, it is just not needed).
   Per-row data: a figure column cannot be a plain `colDef.cell(store.Candidate)`. `buildGroup` computes the figures of the main rows once (one `Put` per ok put row, in row order) into a slice and builds the two columns for that group as closures over it, indexed by the row's position in the table; the static columns are unchanged. `groupSpecs` lists the two columns by id and `buildGroup` binds them.
   The served page: `cmd/ot/candidates_site.go` passes `rd.series` to `Build` instead of nil, so the real site shows figures and not `no cash rate` gaps.
6. Columns: the short-put table gets `itm` (label `ITM probability`, with a hint that says it is a model estimate and not an assignment probability) after `iv`, and `excess` (label `Vs cash`) after `yield`. The covered-call table gets neither (a call's ITM is another figure, Left out). Only ok rows of the main table are computed; the other table has no figure columns.
7. Earnings date: no new reader and no second read. The page shows what the job stored from the event-sources reader, and the cell changes in these ways: the text is `mm-dd` (the register shows `mm-dd`; the date stays the sort value in full); a state without a date is a gap and not a dash: `none` (and `absent`, never stored) gives `no_data` with the sentence `no_earnings`, `unsupported` gives `no_data` with `earnings_unsupported`, `error` gives `missing_input` with `earnings_error`, `listed` without a date gives `no_data` with `no_earnings` (the register: a known state with no date stays the gap), and any state outside these five without a date gives `unreadable`. The state badge stays; a date from the `report_date_override` with state `none` shows the date and the badge `none` as today (the job's meaning: the owner set it). The register's KNOWN, VENDOR and ESTIMATED window kinds (ADR 0016) have no counterpart in the Go store: only listed dates are shown; an estimated window is Left out.
8. IV context is not in the cut: it needs the own IV30 series and 252 valid sessions, and the writer of the own IV30 does not exist in Go. Named in Left out as a gap, not shown as zero or hidden.

## Modules

| module | paths of this design | role |
| --- | --- | --- |
| compute | `internal/compute/optionfigapi/**`, `internal/compute/optionfig/**`, `docs/contracts/compute.md` | interface; the two figures |
| pages | `internal/pages/option_candidates_page/**` | two columns, the earnings cell, the rate read, `Config.Figures` |
| ot | `cmd/ot/wire.go`, `cmd/ot/candidates_site.go`, `cmd/ot/candidates_site_test.go` | sets `Config.Figures`; passes the series reader |

Each entry owns the paths of one module; no cross-module entry is licensed by this cut. `docs/design/modules.md` has one optional change in the pull request of this design: the entry cell of the compute row adds `internal/compute/optionfigapi` (owner-only; `scripts/arch` discovers declarations-only packages from the tree, so the check does not need it). The `store` and `jobs` modules are not touched: the stored row and the dataset stay as they are (`Contracts: unchanged`).

## Interface

`internal/compute/optionfigapi/api.go` declares:

- `PutFigureInput`: `ChainDay` (the session day), `DTE` (int), `Spot`, `Strike`, `IV`, `AnnualYield` (decimal text, as stored; a missing one is the empty string), `Rate` (percent text, empty when none), `RateDay` (empty when none), `MaxRateAgeDays` (int).
- `PutFigures`: for the ITM figure `ItmPercent` (`27.5%`), `ItmKey` (`2755`), `ItmGap` (empty, or `missing_input` or `stale_input`) and `ItmWhy` (the gap sentence); for the excess figure `ExcessPercent` (`4.5%`, unsigned), `ExcessAbove` (bool), `ExcessKey` (signed integer text), `ExcessGap` and `ExcessWhy`. When a gap is set the value fields are empty.
- `OptionFigures`: `Put(PutFigureInput) PutFigures`, and `NewOptionFiguresFunc`.

The words `above` and `below` and the column sentences are the page's; compute returns the bool and English gap sentences. `docs/contracts/compute.md` lists the package as it lists `valuationapi`.

## Issues

### 1. optionfigapi: the interface (interface)

Order: 1
Design: this file, Interface.
Files: internal/compute/optionfigapi/api.go, internal/compute/optionfigapi/api_test.go, docs/contracts/compute.md

New files: the first two. The test checks the contract from both sides (a fake that implements `OptionFigures` and a caller that fills every input and reads every output field). `docs/contracts/compute.md` gets a version line and a bullet for the package, as `valuationapi` has.

Acceptance:
1. `internal/compute/optionfigapi/api.go` holds declarations only (types, one interface, one func type, no function body) and exports the names of the Interface section.
2. `docs/contracts/compute.md` names `internal/compute/optionfigapi` and its test; `internal/compute/api.go` is unchanged; no other file changes.
3. `api_test.go` has a fake type that implements `OptionFigures` and a function that takes an `OptionFigures`, sets every field of `PutFigureInput` and reads every field of `PutFigures`.

### 2. optionfig: the two figures

Order: 2, after 1
Files: internal/compute/optionfig/figures.go, internal/compute/optionfig/figures_test.go, internal/compute/optionfig/purity_test.go

New files: all three. The package has no I/O, no clock and no import of `store`, `jobs`, `notify` or `os`; `purity_test.go` checks the imports of the non-test files and that `time.Now` does not occur.

Acceptance:
1. `figures.go` has a type implementing `optionfigapi.OptionFigures` and a `New` of type `optionfigapi.NewOptionFiguresFunc`, and no file of the package is over 600 lines.
2. For S 100, K 95, IV 0.30, rate 4.5 (rate day 2 days old, limit 7), DTE 30, yield 0.09 the test table asserts `ItmPercent` 27.5%, `ItmKey` 2755, `ExcessPercent` 4.5%, `ExcessAbove` true, `ExcessKey` 450.
3. For S 100, K 100, IV 0.30, rate 0, DTE 30 the table asserts 51.7% and 5172; for yield 0.03 and rate 4 `ExcessPercent` 1.0%, `ExcessAbove` false, `ExcessKey` -100; for yield 0.04 and rate 4 `ExcessPercent` 0.0%, `ExcessAbove` true, `ExcessKey` 0.
4. The ITM result is rounded once from the exact value: `figures.go` converts the float with `big.Rat.SetFloat64` and rounds that, and does not round to 4 places on the way (`git grep` of the package finds no 4-place rounding).
5. Each of these inputs gives the gap named, empty value fields and a non-empty sentence that names the input, in the test table: empty IV, IV "0", strike "abc", spot "0", DTE 0 give `missing_input` for ITM; empty rate gives `missing_input` for both figures; rate 9 days old with limit 7 gives `stale_input` for both; empty annual yield gives `missing_input` for the excess and leaves ITM as computed.
6. The table asserts the value fields are the empty string when a gap is set (never `0%`).
7. The file comment of `figures.go` gives the formula of Decision 2 and names `option.candidates.itm_probability` and `option.candidates.excess_yield` with version 1.

### 3. option_candidates_page: columns, earnings cell, rate read

Order: 3, after 1
Files: internal/pages/option_candidates_page/page.go, internal/pages/option_candidates_page/builder.go, internal/pages/option_candidates_page/groups.go, internal/pages/option_candidates_page/cells.go, internal/pages/option_candidates_page/strings.go, internal/pages/option_candidates_page/figcells.go, internal/pages/option_candidates_page/figcells_test.go, internal/pages/option_candidates_page/builder_test.go, internal/pages/option_candidates_page/groups_test.go, internal/pages/option_candidates_page/cells_test.go, internal/pages/option_candidates_page/page_test.go, internal/pages/option_candidates_page/reasons_test.go

New files: `figcells.go` (the two figure columns, the rate read, the 7-day constant) and `figcells_test.go`. The other tests use identifiers whose use changes (`git grep` in the package: `groupSpecs` in `builder_test.go`, `groups_test.go`, `reasons_test.go`; `buildGroup` and `buildTable` in `reasons_test.go` and `groups_test.go`; `Config{` in `page_test.go` and `builder_test.go`; the earnings text in `cells_test.go`). `testdata/*.json` are schema inputs without these columns and stay unchanged. The files are far below 600 lines after the change.

Acceptance:
1. `Config` has a field `Figures` of type `optionfigapi.OptionFigures`; the package imports `internal/compute/optionfigapi` and not `internal/compute/optionfig`; `purity_test.go` is unchanged and still holds: no `math/big`, no `float64` identifier (the rate point is formatted with `strconv` in one expression).
2. The short-put entry of `groupSpecs` lists the columns `excess` directly after `yield` and `itm` directly after `iv`; the covered-call and held-call-guard entries list neither; the other table has none.
3. `buildGroup` calls `Figures.Put` once per ok put row of the main table and the two columns read that result by the row's position, as Decision 5 says; the `colDef.cell(store.Candidate)` signature of the static columns is unchanged.
4. With `Figures` nil, the two cells of an ok put row are gaps with reason `no_reader`; with a fake `Figures` returning ITM key 2755, percent 27.5%, excess above 4.5% key 450, the `itm` cell has text `27.5%` and value 2755 and the `excess` cell has text `above 4.5%` and value 450; a gap result becomes a gap cell with the figure's reason and sentence. The word `below` is the text for `ExcessAbove` false.
5. The page reads the series `DGS3MO` from the `store.SeriesReader` argument once per group, window 14 days up to the group's session day, takes the newest point with a value, and passes its text and day to `Put` with `MaxRateAgeDays` 7; a nil series or no point passes the empty rate. A reader error is an error of `Build`, as the candidate reader's is.
6. `itm` has a column hint in `strings.go` that says it is an estimate of the model; no cell or hint calls it an assignment probability. No source file of the package holds a CJK character or a `\u` escape for a figure word.
7. `earningsCell` shows `mm-dd` as text and the full day as the sort value for a stored date and keeps the state badge; for no date it returns the gaps of Decision 7 (`none`, `absent`, `listed`: `no_data` with `no_earnings`; `unsupported`: `no_data`, `earnings_unsupported`; `error`: `missing_input`, `earnings_error`; any other state: `unreadable`); the dash is no longer produced for the earnings column.
8. `contracts/schemas/page.option_candidates_page.v1.json` is unchanged (`git diff origin/main --stat` lists no file under `contracts/`) and `Build` still validates against it; no file outside the Files line changes.
9. The comment of `Build` no longer says "Nothing is computed; every number is a stored text" and says the two figures come from `Config.Figures` and the rest is stored text.

### 4. ot: wire the figures and the series reader

Order: 4, after 2 and 3
Files: cmd/ot/wire.go, cmd/ot/candidates_site.go, cmd/ot/candidates_site_test.go

`wire.go` has 793 lines: the change adds two lines (the import and one field in the `Config` literal of `newOptionCandidatesPage`), staying under 800; the `Figures` field has to be set there, the only place that names the page's `Config`, so the edit of `wire.go` cannot be dropped. `newOptionCandidatesPage` keeps its signature, so the callers in `cmd/ot/option_page_site_test.go` (found by `git grep newOptionCandidatesPage`) need no change. `candidates_site.go` (121 lines) changes the `Build` call of `buildCandidates` (line 47) and its comment.

Acceptance:
1. The `option_candidates_page.Config` literal in `newOptionCandidatesPage` sets `Figures: optionfig.New()`, and `cmd/ot/wire.go` imports `internal/compute/optionfig`.
2. `buildCandidates` passes `rd.series` as the second argument of `Build`; the string `Build(ctx, nil,` no longer occurs in `cmd/ot/candidates_site.go`.
3. `candidates_site_test.go` has a test whose `siteReaders` carries a fake series reader and asserts that the reader was asked for `DGS3MO` when the page was built through `buildCandidates`.
4. `cmd/ot/wire.go` is under 800 lines at the head and the diff of the issue touches no other file.

## Checked by CI

- The Go tests of `internal/compute/optionfigapi`, `internal/compute/optionfig`, `internal/pages/option_candidates_page` and `cmd/ot` (values of Decisions 2 and 3, the gap cases, the column lists, the English guard, the purity tests, the series read).
- `golangci-lint` and `scripts/arch`: the page imports `optionfigapi` and not `optionfig`; the only importer of `optionfig` outside compute is `cmd/ot`.
- The size ratchet (`docs/design/ratchet.json`): no ceiling changes.
- The `test` check reads `Contracts: unchanged` in the pull request bodies.

## Left out

- IV context (`option.candidates.iv_context`): the own IV30 writer (`OwnIv30Step`) does not exist in Go, HV30 and IVP are read through the readings store, and IVP needs 252 valid sessions of own IV30 which the repository cannot say it has (`iv_history` stores IBKR bars, not the own IV30). Until a design adds the own IV30 step and the 252 sessions exist, the figure is a named gap: the short-put table has no `iv_context` column. Not shown as 0 and not replaced by IBKR's IVP (the register excludes it).
- ITM probability and excess yield for covered calls and for JP puts (no JP rows on this page; the JP excess uses the higher of two rates).
- Storing the two figures on the scan row (dataset r2, migration, store, job): a later design if the owner wants the frozen value.
- The register's earnings window kinds (VENDOR mark, ESTIMATED window, qualified flag): not in the Go store; only listed dates and the stored override are shown.
- Dividends in the ITM probability, a volatility smile, and any use of delta as a probability.
- The cards of `option_page` and the `full_scan` table named in the register `pages` list: other pages, not built here.

## Doubts

1. Computing at build time (Decision 1) departs from "stored with the scan" of the register. The alternative is a five-module chain; confirm the cheaper cut.
2. The 7-day rate age is a constant in the page equal to `max_rate_age_days` in `job.toml`; two places can drift. An alternative is a field of `Config` set by `ot` from the job's parameters, which adds a line to `wire.go` (795 lines, still under 800).
3. The register edit of Decision 1 is the owner's, in the owner step of the merge; the earnings `inputs` also name the old scan row's window fields.
4. The ITM uses the contract IV of the single stored quote (no skew, no dividend) and calendar days over 365; the old option-trader function `evaluate_put` was not readable here, so the day count is taken from `optioncand/put.go`; the register's 4-place stored value is dropped (Decision 2), the one-place text is from the exact value. Advisor to check against option-trader.
5. The optional `modules.md` entry-cell edit (Modules) is owner-only and not needed by the arch check.

## Adversarial

Answers to the author's adversarial notes (scratchpad) and to the advisor's review r1, one round.

- Served page passes a nil series reader (adversarial 1, blocking): adopted. Decision 5 and issue 4 add `cmd/ot/candidates_site.go` and `cmd/ot/candidates_site_test.go`, passing `rd.series`, with a result (acceptance 2 and 3). `wire.go` stays: the `Config` literal is only there.
- Earnings `listed` without a date: adopted as the register says (`no_data`, Decision 7); an unknown state without a date: adopted as `unreadable`. The override date with state `none` showing a `none` badge: not changed, named (it is the job's own meaning; changing the badge is outside this cut).
- Double rounding (adversarial 2): adopted. One rule from the exact value; the 4-place value is dropped; recomputed literals 0.275460 gives 27.5% and key 2755, 0.517151 gives 51.7% and 5172. The advisor's 0.2755 is right only as the 4-place step, which is no longer used. Tie rule stated (exact binary value of the float).
- English words (review 1): adopted: `above` and `below`, no escape clause.
- Register conflict (review 2): adopted: Decision 1 states the contradiction plainly with the excluded phrases; the register edit is the owner's, in the same owner step as the merge, not after.
- Use the existing `figures` package instead of new packages: not adopted. `figures` is an implementation package; `scripts/arch` allows a page to import only declarations-only packages of compute, and the only interface home for it, `internal/compute/api.go`, is at 839 lines with a ceiling of 840. A new interface package is the smallest cut that obeys both; `docs/contracts/compute.md` is added to issue 1.
- Facts corrected: `wire.go` is 793 lines; the page `purity_test.go` bans the identifier `float64` and not `math`, so the rate is formatted with `strconv`; the import rule is `scripts/arch`, not depguard.
- Job-time against page-time rate age: named in Decision 4.
- Per-row data of the figure columns: Decision 5 and issue 3 acceptance 3.
- Not verified here: that the `pages` runner passes a non-nil reader to `ot page` and the site in every profile (the demo profile passes `demoReader`, `site.go:199`); the day count against option-trader.
