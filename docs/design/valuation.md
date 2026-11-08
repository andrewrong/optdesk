# Valuation: company value, the buy-price band (the price line), return at a price and the five questions, ported from option-trader

Author: the optdesk designer
Owner approved: the owner's decisions below are binding (goal, source, EDINET automation); the advisor's of 2026-11-08 (port ADR 0008 and 0023 in meaning); the rest is for the design review
Revision: 2 (adversarial review adopted; watch list shared with event-sources)

Source: the owner's goal "value investing plus option selling", the loop company value, buy price band, strike, candidates, ledger, tracking; the advisor of 2026-11-08; the adversarial review of revision 1 (dispatcher scratchpad, not in the repository).
Read in option-trader at 4704569f (read only): `docs/adr/0008-valuation-and-acquisition-policy.md`, `0023-return-at-price.md`, `src/option_trader/valuation/` (`fcf_yield.py`, `reverse_dcf.py`, `price_return.py`, `five_questions.py`, `diagnosis.py`, `filing_rows.py`, `facts.py`), its `params/*.toml`, `sources/fmp/analyst_estimates.py`.
Read in optdesk on main at beaf0cf: `AGENTS.md`, `docs/design/modules.md`, `ease-v2.md`, `report-page-producer.md`, `compute-figures.md`, the `companies_page`, `core_page`, `diagnosis_page`, `price_lines_page` v1 schemas, `contracts/datasets/`, `docs/conventions/register.toml`, `internal/sources`, `internal/compute`, `internal/jobs`, `internal/store`.

Goal: for each company the owner holds or watches, a stored buy-price band made only from filings, shown as the price line of the companies page, with the return a price earns and the four numeric answers of the five questions beside it; every figure registered with its hash. The last step of the loop, the largest strike an offer may have (strike minus premium at or below the band), is the filter of a later candidates design: this design stores and shows the figure it will read and does not filter offers. US first: the SEC path is complete end to end before the EDINET issues start.

## What the owner gets

- A buy-price band per company: the highest price he would accept as assignment price (value at an 8% free-cash-flow yield) and a lower edge (10%); 6% is shown as a reference outside the band. It is the price line on the page. After 90 days without a new figure the page says "expired" and shows no band.
- The return a price earns per year in four named cases beside the US ten-year Treasury yield (ADR 0023, at the strike as it stands), and the growth today's price needs at 8%, 10% and 12% (reverse DCF).
- One companies page with Core (the returns), Diagnosis (context for cyclical and financial names) and the five questions: four answered by numbers from filings, the fifth and the moat as his own text.
- US companies from SEC filings; no hand-entered figures. A bank, insurer, cyclical or loss-making company shows "unvalued" with the reason, never a band.
- Japan comes after the US path works: EDINET filings, automated. Many Japanese names will end "unvalued": J-GAAP filers mostly report no diluted share count and no stock-compensation line, and the rule rejects a company with an incomplete input (ADR 0008). IFRS filers report more and are expected to value. Which names value is known only after the first run; the page says why for each. The Japan card has no rate to compare with until a Japanese 10-year yield source is added (candidate: the FRED monthly series `IRLTLT01JPM156N`, not verified); until then it shows a gap, not the US rate.
- A hand-drawn line is only his override, labelled as such (its store is a later design). Nothing is a verdict and nothing places an order.

## What main holds today

- Page schemas exist (`companies_page` v1 with holdings, watch list, `unvalued`, `moat`; `core_page`, `diagnosis_page`, `price_lines_page`) and web pages for them; no Go code produces any. `companies_page` v1 has no five-questions, core or diagnosis block.
- `internal/sources` has FRED only. `contracts/datasets` has no fundamentals dataset. `readings.reading` r1 stores a figure as decimal text (at most 12 places), subject kinds `underlying` and `market`, origin `compute_figures` only, with numeric and tag parameters.
- `internal/compute` has `Figures`, `Ease` and `rules.json` matched to `register.toml` (53 entries, `source_hash`) by `TestRulesMatchRegister`. `internal/store/api_test.go` is 800 lines, `api.go` 490: a store interface test goes in a new file.
- option-trader has no EDINET code (only an `edinet_code` column in its master table); ADR 0008 says Japanese facts are entered by hand and automation comes later. The EDINET issues are a new build with nothing to hash against.

## Decisions

1. Ported in meaning (advisor, 2026-11-08): `fcf_yield` v2 (5-year median of operating cash flow minus capital expenditure minus stock compensation, over current diluted shares, over the required yield; 6%, 8%, 10%, primary 8%; leverage and instability flags; share-count sanity; eligible years: 350 to 380 days, annual forms only, consecutive); reverse DCF (10 years, terminal 2%, rates 8%, 10% primary, 12%, no fade); `price_return` v1 with its four lines, gate, refusals and flags (ADR 0023 and its three notes); five questions Q1 to Q4; the diagnosis. Parameters are the option-trader files pinned by git blob id: `fcf_yield_v2.toml` 4952b59720ce, `reverse_dcf_v1.toml` e1e1f731c7b8, `price_return_v1.toml` 8d7be25e35c3, `five_questions_v1.toml` 6755537f67d5, `diagnosis_v1.toml` 643178784313, `filing_rows_v1.toml` 967d8d64ff48. They are US-GAAP and USD; Japan needs its own parameter file (concepts, JPY), written in `edinet-mapping`, new and registered as such, not "ported".
2. Arithmetic: Decimal everywhere, no float64. Library `github.com/shopspring/decimal` (MIT, widely used, stable, no dependencies), added to `go.mod` by the first issue that imports it (`compute-filings`). Python used precision 34 per call (`localcontext(prec=34)`); shopspring has a package-global `DivisionPrecision`, which no code here sets or reads: every division goes through one helper `div` that keeps 34 significant digits explicitly, rounding half-even as the old code. The goldens compare exact text with the Python output; if parity cannot be reached the issue stops and reports, and `math/big.Rat` (standard library) is the fallback behind the same helper (own glue behind one function, owner 2026-11-08). The interface carries decimal text, so no library type is in an interface.
3. The band. `valuation.buy_band` v1, subject kind `underlying`, subject the symbol, value = value per share at 8% (the maximum assignment price), parameters `low` (value at 10%), `ref_high` (6%), `expires_day`, `base_fy_end`, flags. It is price-free: no function that makes it takes a price. Expiry follows ADR 0008's 90 days, whose start there is approval; no approval exists here, so the start is the session day of the reading, and a reading is rewritten each day the facts are current (owner question 1). ADR 0008:25 ("the system's value is not a price limit; the owner's explicit maximum assignment price is the only one") and 0008:14 stay in force for any Policy: this design creates no Policy and no approval. The band is a displayed figure; that candidates read it as the cap follows the owner's goal and is decided in the candidates design with the owner.
4. Return at a price keeps the bare strike as ADR 0023:258-260 says (the premium is already in the put's own return). The net cost (strike minus premium) belongs to the candidates filter design. The reverse DCF implied growth is solved at page build from the quote, as ADR 0008 says, and is not stored; `valuation.implied_growth` and `valuation.return_at_price` are registered figures computed at page build, never stored; `compute-fcf-band` checks that `TestRulesMatchRegister` accepts a registered figure with no stored reading and, if it does not, reports the file to change instead of editing it.
5. Unvalued is a state with a reason, not zero: SIC 6000-6799, the cyclical SIC ranges, a rejection (fewer than 5 eligible years, non-consecutive years, share-count jump, stale latest year over 457 days, non-positive base) gives `missing_input` with the reason in parameters. Cyclical and financial names get the diagnosis instead of a band.
6. Fundamentals, US (owner choice a). SEC `companyfacts` directly (User-Agent from the environment, at most 5 requests a second, public CIK only). Concepts kept: the files' `NetCashProvidedByUsedInOperatingActivities`, the four capex concepts, `ShareBasedCompensation` and `AllocatedShareBasedCompensationExpense`, `WeightedAverageNumberOfDilutedSharesOutstanding`, `LongTermDebt`, `LongTermDebtNoncurrent`, the concepts `filing_rows_v1.toml` and `five_questions_v1.toml` list, and `dei:EntityCommonStockSharesOutstanding` (cover-page share check), taxonomies `us-gaap` and `dei`. One dataset `obs.fundamental_fact` r1: `source` (`sec`, later `edinet`; a later source extends the enum additively in its own contracts change), `taxonomy`, `concept`, `unit`, `currency`, `period_start`, `period_end`, `value` (decimal text), `form`, `doc_id`, `filed`, `retrieved_at`, `raw_sha256`; key `(source, doc_id, concept, period_start, period_end)`, append only. Every filed value is kept, so a later recast never hides the annual-form fact that makes a year eligible.
7. Facts to Filings is compute's: `BuildFilings` takes the facts as the store returns them, applies the as-of rule (latest filed on or before the cutoff; `fy`/`fp` never decide) and the eligible-year rule, and returns ten fiscal years of Decimal text figures or a refusal reason. The job and the page both call it; no formula is in pages. The store query returns all rows filed on or before the cutoff for a subject (no winner picked in SQL).
8. Japan (owner: EDINET automation). EDINET API v2 lists documents by day, not by company: the client keeps a stored day cursor per run and a backfill command scans the days needed for five eligible annual reports (the job's backfill path, as `ot backfill`). J-GAAP and IFRS map to the same concept names through a table; a standard not recognised or competing elements give no fact and a typed reason. Every fact keeps its EDINET document id.
9. The subject list is not defined here. The event-sources design (`docs/design/event-sources.md`, Decision 1) owns the one owner-edited list `$OT_HOME/watch.toml`, type `jobs.WatchEntry{Issuer, Market string; Contract broker.Contract}` in `internal/jobs/api.go`, loaded by a loader in `cmd/ot`, rows keyed by issuer (`cik:<10 digits>` US, `edinet:<code>` JP). Valuation reuses the file and type: the fundamentals job plans from the entries (the CIK or EDINET code is the issuer text), the band's subject is the entry's symbol. This design depends on that design's `earnings-job` (adds `WatchEntry`) and `watch-loader-earnings-wire` (the loader) and adds no second list. The owner lists every company he holds in `watch.toml` too. Which are held (page block `holdings` against `watch_list`) is an optional input of the page builder, `Held func(ctx) (map[string]bool, error)` keyed by issuer, wired by `cmd/ot` from the positions reader; nil puts every row in the watch list block. A name without an issuer id is skipped with a typed reason, never guessed.
10. FMP and analyst estimates are left out. The source in option-trader is FMP (ADR 0025). `ReturnAtPrice` takes an optional analyst input that the page passes as absent; lines 3 and 4 show the ADR's own state "left out: no analyst input".
11. Registration: each stored figure is a `[[figure]]` entry of `register.toml` with `source_hash` of its Go files and a `history` line naming the option-trader blob ids of Decision 1, landed with the code (as `compute-ease`): `valuation.buy_band`, `valuation.implied_growth`, `valuation.return_at_price`, `valuation.question`, `valuation.diagnosis`; each is v1. Goldens come from a generator committed under `tools/compute-golden/valuation/` (own uv project, own `uv.lock`, not run in CI); the committed JSON is what tests read, so no check needs the old system.
12. One page: `companies_page` v2 adds optional `core`, `diagnosis`, `questions` blocks to v1 (v1 stays readable). The owner text (fifth question, moat) and the override reach the builder through optional interfaces (`OwnerText`, `PriceOverride`) declared in the page package; nil gives a gap "not entered"; their stores are later designs. `core_page` and `diagnosis_page` stay unproduced.

## Modules

New directories: `internal/sources/sec`, `internal/sources/edinet`, `internal/compute/valuation`, `internal/jobs/fundamentals`, `internal/pages/companies_page`, `internal/store/fundamental`, `tools/compute-golden/valuation` (compute row). This pull request edits `docs/design/modules.md` by adding the five `internal/...` implementation packages other than the store one to the sentence that lists what `cmd/ot` may import, and regenerates `arch.toml` with `go run ./scripts/arch`; no row or depends cell changes. Migration numbers are "the next free one at cut time" in every issue (account-data and events take earlier numbers). Shared files and their Order: `internal/store/api.go`, `docs/contracts/store.md`, `internal/jobs/api.go` and `docs/contracts/jobs.md` are also edited by the account-data and events designs; each issue here that edits one is ordered after the merge of the other design's issue that edits it, read from main at cut time, and waits if it is not merged.

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | fact-dataset (interface) | contracts | M | first |
| 2 | fact-store (interface and implementation) | store | M | after 1 |
| 3 | sec-source (interface and implementation) | sources | M | after 1 |
| 4 | valuation-compute-interface (interface) | compute, plus repo | M | after 1 |
| 5 | compute-filings | compute | M | after 4 |
| 6 | compute-golden | compute | M | after 5 |
| 7 | compute-fcf-band | compute, plus docs | L | after 6 |
| 8 | compute-return-at-price | compute, plus docs | L | after 7 |
| 9 | compute-questions-diagnosis | compute, plus docs | L | after 8 |
| 10 | fundamentals-job (interface and implementation) | jobs | M | after 2, 3, 3 |
| 11 | valuation-job-step (interface and implementation) | jobs | M | after 5, 7, 9, 10 |
| 12 | companies-page-v2 (interface) | contracts | M | first |
| 13 | companies-page-band (interface and implementation) | pages | M | after 12, 7, event-sources `earnings-job` |
| 14 | companies-page-sections | pages | L | after 13, 8, 9 |
| 15 | companies-page-web | web | M | after 12 |
| 16 | valuation-wire-us | ot | M | after 3, 3, 11, 14 |
| 17 | edinet-client | sources | M | after 16 |
| 18 | edinet-mapping | sources | L | after 17 |
| 19 | valuation-wire-jp | ot | S | after 18 |

Issues 1 to 16 are the US path; 17 to 19 start only when 16 is merged and the owner has seen a real US band. `register.toml`, `rules.json` and `docs/GLOSSARY.md` are in 7, 8, 9 only, in that order, each adding its own entries. `internal/compute/api.go` is in 4 only; `internal/sources/api.go` in 3 only (the Japan constructor type is declared in 3 so 17 needs no interface edit); `companies_page/builder.go` in 13 only; `cmd/ot/wire.go` in 16 then 19. Issues 12 and 1 touch disjoint generated files.

## Interface

- contracts: dataset `obs.fundamental_fact` r1 and generated `contracts.FundamentalFact`; `page.companies_page.v2.json` with its generated TypeScript (the contracts row owns `web/src/generated/**`).
- store: `FactWriter.InsertFacts`, `FactReader.FactsFiledBy(ctx, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error)`, `FactStore`.
- sources: `FactPlan{Subjects []string; From, To contracts.Day}`, `FactSource.Fetch`, `SECConfig{BaseURL, UserAgent, Clock}`, `EDINETConfig{BaseURL, APIKey, Clock}`, `NewSECFunc`, `NewEDINETFunc`.
- compute, function types in the style of `ReadingFunc` (no five-method interface): `BuildFilingsFunc func(facts []contracts.FundamentalFact, asOf contracts.Day, std Standard) FilingsResult`, `BandFunc func(id string, f Filings, asOf contracts.Day) BandFigure`, `ImpliedGrowthFunc func(f Filings, spot string) ImpliedResult`, `ReturnAtPriceFunc func(f Filings, costs []string, a *Analyst, asOf contracts.Day) ReturnResult`, `QuestionsFunc`, `DiagnosisFunc`; text decimals, no I/O, no clock.
- jobs: `NewFundamentalsFunc func(sources.FactSource, store.FactWriter, []WatchEntry, clock.Clock) (contracts.Job, error)`; a second figures constructor type adding the compute funcs and `store.FactReader`, the existing constructors unchanged.
- pages: `companies_page.Config{Zone, Subjects []jobs.WatchEntry, Held, Read, Facts, BuildFilings, Band, ImpliedGrowth, ReturnAtPrice, Questions, Diagnosis, Quotes, Text, Override}`, in `internal/pages/companies_page/builder.go`; `internal/pages/api.go` unchanged.
- ot: `newCompaniesPage`, `newSEC`, `newEDINET` in `cmd/ot/wire.go`; nil when a DSN or key is not set.

## Issues

Every test calls `t.Parallel()`, uses `t.TempDir()`, a fake clock, no sleep, no fixed port, no live service (recorded bodies, local `httptest` servers on a port the OS picks), no live database (a PostgreSQL test carries the tag `pg`). Every issue's last Acceptance item is: `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the paths of its Files line. No new file passes 600 lines.

### fact-dataset (interface)

- Module: contracts. Size: M. Order: first (1). Contracts: changes `contracts/datasets`.
- Design: docs/design/valuation.md#Decisions (6)
- Files: contracts/datasets/obs.fundamental_fact.r1.json, internal/contracts/gen/records.go, contracts/fixtures/obs.fundamental_fact.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/fundamental_fact_test.go, docs/contracts/datasets/obs.fundamental_fact.md

- Goal: the dataset and generated type; a fixture of three records (SEC instant, SEC duration, a JPY record with `source` `edinet`). The generated list is what `contracts/generate.sh` rewrites; a file beyond it is reported, not edited.

Acceptance:
1. `go test ./internal/contracts -race -count=1` passes with `TestFundamentalFactFixture` (three records validate; a `source` of `fmp` fails; a duration fact without `period_start` fails).
2. `contracts/generate.sh` then `git diff --exit-code` leaves no change.
3. The diff names exactly the Files paths.

Checked by CI: ci-go; the fixture manifest test; `scripts/hygiene`.

### fact-store (interface and implementation)

- Module: store. Size: M. Order: after 1 and after the merged account-data and events edits of `internal/store/api.go` and `docs/contracts/store.md` (read at cut time).
- Design: docs/design/valuation.md#Interface
- Files: internal/store/api.go, internal/store/api_fact_test.go, migrations/NNNN_fundamental_fact.sql, internal/store/fundamental/store.go, internal/store/fundamental/store_test.go, docs/contracts/store.md

- Goal: `FactWriter`, `FactReader`, `FactStore`; the contract test in the new file from both sides with a fake; the table `obs.fundamental_fact` (append only, no update or delete grant), a view granted to `ot_ro_pages`, the writer granted the base table; `FactsFiledBy` returns every row filed on or before the cutoff. `NNNN` is the next free number at cut time.

Acceptance:
1. `go test ./internal/store/... -race -count=1` passes; `git diff $(git merge-base HEAD origin/main) --stat -- internal/store/api_test.go | grep -c .` prints 0.
2. `grep -ci "update\|delete" migrations/*_fundamental_fact.sql` prints 0; `grep -c "FactWriter\|FactReader\|FactStore" internal/store/api.go` is at least 3.
3. `go run ./scripts/arch --check` prints `arch: ok`.

Checked by CI: ci-go-pg (insert twice adds 0 rows; the page role cannot read the base table nor write); the size rule.

### sec-source (interface and implementation)

- Module: sources. Size: M. Order: after 1. Second-opinion case: new outside service (`data.sec.gov`; public CIK only; the User-Agent contact text comes from the environment).
- Design: docs/design/valuation.md#Decisions (6)
- Files: internal/sources/api.go, internal/sources/api_test.go, internal/sources/sec/client.go, internal/sources/sec/mapping.go, internal/sources/sec/pacing.go, internal/sources/sec/client_test.go, internal/sources/sec/mapping_test.go, internal/sources/sec/testdata/companyfacts_sample.json

- Goal: `FactPlan`, `FactSource`, both configs and both constructor types (additive); `sec.New` fetches `/api/xbrl/companyfacts/CIK##########.json` and maps each kept concept to a `FundamentalFact` (`accn` as `doc_id`, `filed`, `form`, `raw_sha256`, `retrieved_at` from the clock); pacing as the FRED package (reused if possible); an error never carries the User-Agent.

Acceptance:
1. `go test ./internal/sources/... -race -count=1` passes with `TestMapKeepsAccessionFiledAndForm`, `TestCoverPageSharesKept` (the `dei` concept), `TestUnlistedConceptDropped`, `TestPacingFiveAPerSecond`, `TestErrorHidesUserAgent`.
2. `git diff $(git merge-base HEAD origin/main) -- internal/sources/api.go | grep '^-' | grep -vc '^---'` prints 0.
3. `grep -c "time.Sleep\|http.Get\|data.sec.gov" internal/sources/sec/*_test.go` prints 0 in each.

Checked by CI: ci-go; arch (the package imports no store).

### valuation-compute-interface (interface)

- Module: compute, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M. Order: after 1; `internal/compute/api.go` is in this issue only.
- Design: docs/design/valuation.md#Interface
- Files: internal/compute/api.go, internal/compute/api_valuation_test.go, docs/contracts/compute.md, internal/compute/valuation/doc.go, .go-arch-lint.yml

- Goal: the function types and result types of Interface with doc comments (text decimals, states, reasons of Decision 5, the four line keys `SPENDING_STAYS`, `SPENDING_EARNS`, `ANALYST_PATH`, `SPENDING_EARNS_ANALYST`, no verdict word); a fake of each, a caller test in the new file; `doc.go` as a package clause so the lint file names the component.

Acceptance:
1. `go test ./internal/compute -race -count=1` passes; `go list -deps ./internal/compute | grep -c "shopspring"` prints 0.
2. `go run ./scripts/arch --check` prints `arch: ok`; `grep -c "internal/compute/valuation" .go-arch-lint.yml` is at least 1.

Checked by CI: ci-go; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`.

### compute-filings

- Module: compute. Size: M. Order: after 4. New direct dependency: `github.com/shopspring/decimal`, MIT, named in the pull request.
- Design: docs/design/valuation.md#Decisions (2, 7)
- Files: internal/compute/valuation/arith.go, internal/compute/valuation/filings.go, internal/compute/valuation/years.go, internal/compute/valuation/filings_test.go, internal/compute/valuation/years_test.go, go.mod, go.sum

- Goal: `div` with explicit 34 significant digits, `BuildFilings` (as-of rule, annual forms only, 350 to 380 day years, consecutive, a later 8-K recast replaces the value but not the year's eligibility), refusal reasons of Decision 5. This is the port of the year selection of `filing_rows.py` for the parameters of Decision 1.

Acceptance:
1. `go test ./internal/compute/valuation -race -count=1` passes with `TestAsOfLatestFiledWins`, `TestRecastKeepsYearEligible`, `TestNonConsecutiveRefused`, `TestQuarterlyFactsNeverMakeAYear`, `TestDivKeeps34Digits`.
2. `grep -rc "DivisionPrecision\|float64" internal/compute/valuation/*.go` prints 0 for every file; `grep -c "shopspring/decimal" go.mod` is 1.

Checked by CI: ci-go (`go mod tidy` leaves no diff).

### compute-golden

- Module: compute. Size: M. Order: after 5.
- Design: docs/design/valuation.md#Decisions (11)
- Files: tools/compute-golden/valuation/pyproject.toml, tools/compute-golden/valuation/uv.lock, tools/compute-golden/valuation/gen.py, internal/compute/valuation/testdata/golden_fcf.json, internal/compute/valuation/testdata/golden_return.json, internal/compute/valuation/testdata/golden_questions.json

- Goal: a dev-only generator, its own uv project, that takes the path of an option-trader checkout at 4704569f as an argument, builds synthetic filings inputs (eight for the band and reverse DCF, ten for return at a price including each refusal and `GROWTH_LIMITED_BY_R`, six for questions and diagnosis) and writes the three JSON files with inputs and Decimal-text outputs. It is not run in CI and nothing imports it.

Acceptance:
1. `cd tools/compute-golden/valuation && uv sync --locked && uv run ruff check .` passes; `git ls-files tools/compute-golden/valuation | grep -c uv.lock` is 1.
2. `jq length internal/compute/valuation/testdata/golden_fcf.json` is 8, `golden_return.json` at least 10, `golden_questions.json` 6; each file's first key `source_commit` is `4704569f`.
3. `grep -rc "option-trader" internal/compute/valuation/*.go` prints 0 (no test needs the old system).

Checked by CI: none (the tool is not run in CI).

### compute-fcf-band

- Module: compute, plus docs (`docs/conventions/register.toml`, `docs/GLOSSARY.md`, `internal/compute/rules/rules.json`). Size: L. Order: after 6.
- Design: docs/design/valuation.md#Decisions (1, 3, 4, 5, 11)
- Files: internal/compute/valuation/fcf.go, internal/compute/valuation/flags.go, internal/compute/valuation/reverse.go, internal/compute/valuation/fcf_test.go, internal/compute/valuation/reverse_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml, docs/GLOSSARY.md

- Goal: `BandFunc` (value at 6%, 8%, 10%, leverage and range flags, 90-day expiry from the reading's day), `ImpliedGrowthFunc` (bisection, 64 iterations over -0.5 to 1), and the entries `valuation.buy_band` and `valuation.implied_growth` with real hashes; the check of Decision 4 on `TestRulesMatchRegister`.

Acceptance:
1. `go test ./internal/compute/valuation ./internal/compute/rules -race -count=1` passes; the golden tests compare every Decimal text exactly.
2. `grep -c "4952b59720ce\|e1e1f731c7b8" docs/conventions/register.toml` is at least 2.
3. A test shows the band function has no price parameter (`go doc` signature of `BandFunc` contains no `spot`; `go doc ./internal/compute BandFunc | grep -c spot` prints 0).

Checked by CI: ci-go.

### compute-return-at-price

- Module: compute, plus docs. Size: L. Order: after 7 (same register and rules files).
- Design: docs/design/valuation.md#Decisions (1, 4, 10, 11)
- Files: internal/compute/valuation/price_return.go, internal/compute/valuation/paths.go, internal/compute/valuation/gate.go, internal/compute/valuation/price_return_test.go, internal/compute/valuation/gate_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml

- Goal: `ReturnAtPriceFunc` with the four lines, low path, `R`, the spending gate, `PLANT_REVENUE_FELL`, `LATEST_LEVEL_NOT_POSITIVE`, refusals, analyst anchoring and scale check (the analyst input optional); entry `valuation.return_at_price` v1. The result has no value figure and no verdict word.

Acceptance:
1. `go test ./internal/compute/valuation -race -count=1` passes; golden cases cover each refusal, `GROWTH_LIMITED_BY_R` and a nil analyst input giving lines 3 and 4 "left out: no analyst input".
2. `grep -c "8d7be25e35c3" docs/conventions/register.toml` is at least 1; `go test ./internal/compute/rules -race -count=1` passes.

Checked by CI: ci-go.

### compute-questions-diagnosis

- Module: compute, plus docs. Size: L. Order: after 8.
- Design: docs/design/valuation.md#Decisions (1, 11, 12)
- Files: internal/compute/valuation/questions.go, internal/compute/valuation/capital.go, internal/compute/valuation/diagnosis.go, internal/compute/valuation/questions_test.go, internal/compute/valuation/diagnosis_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml, docs/GLOSSARY.md

- Goal: `QuestionsFunc` (Q1 return on tangible capital, Q2 growth cost, Q3 pricing power, Q4 cash behind profit; Q5 has no figure) and `DiagnosisFunc` (cyclical margins and inventory; financial ROE and ROA); entries `valuation.question` and `valuation.diagnosis`; a figure that cannot be formed is `missing_input`, never zero.

Acceptance:
1. `go test ./internal/compute/valuation ./internal/compute/rules -race -count=1` passes; golden values equal the six cases.
2. `grep -c 'id = "valuation\.' docs/conventions/register.toml` is 5.

Checked by CI: ci-go.

### fundamentals-job (interface and implementation)

- Module: jobs. Size: M. Order: after 2, 3 and after the merge of event-sources `earnings-job` (adds `WatchEntry` to `internal/jobs/api.go`) and of the account-data edits of `internal/jobs/api.go` and `docs/contracts/jobs.md`, read at cut time.
- Design: docs/design/valuation.md#Decisions (6, 9)
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/fundamentals/job.toml, internal/jobs/fundamentals/plan.go, internal/jobs/fundamentals/run.go, internal/jobs/fundamentals/run_test.go, docs/contracts/jobs.md

- Goal: `NewFundamentalsFunc` and job `fundamentals` shaped like `fredseries`: plan keys are the `cik:` entries of the watch list; the US window after the SEC publish time; writes through `FactWriter`; an entry with no CIK is skipped with a typed reason.

Acceptance:
1. `go test ./internal/jobs/... -race -count=1` passes; running twice adds 0 on a fake writer; a subject with no CIK is skipped and named in the result.
2. `grep -c "NewFundamentalsFunc" internal/jobs/api.go` is at least 1.

Checked by CI: ci-go; the manifest check of `docs/contracts/jobs.md`.

### valuation-job-step (interface and implementation)

- Module: jobs. Size: M. Order: after 5, 7, 9, 10 (same `api.go`).
- Design: docs/design/valuation.md#Decisions (3, 7)
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/computefigures/job.toml, internal/jobs/computefigures/valuation.go, internal/jobs/computefigures/valuation_test.go, docs/contracts/jobs.md

- Goal: a second figures constructor type, as `NewWithEase` did; with non-nil funcs the job reads facts for each subject, calls `BuildFilings`, and writes `valuation.buy_band`, `valuation.question`, `valuation.diagnosis` readings (`subject_kind` `underlying`); existing constructors and their plan are unchanged.

Acceptance:
1. `go test ./internal/jobs/computefigures -race -count=1` passes; with nil funcs the plan has no `valuation.` key.
2. A table test writes three readings for a company and one `missing_input` reading with its reason for a bank (SIC 6021).

Checked by CI: ci-go.

### companies-page-v2 (interface)

- Module: contracts. Size: M. Order: first (12). Contracts: adds `contracts/schemas/page.companies_page.v2.json`.
- Design: docs/design/valuation.md#Decisions (12)
- Files: contracts/schemas/page.companies_page.v2.json, contracts/fixtures/page.companies_page.v2.ok.json, contracts/fixtures/page.companies_page.v2.gap.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/companies_page_v2_test.go, web/src/generated/companies_page.v2.ts

- Goal: v2 = v1 plus `core` (rows as `core_page` `CoreRow`, Treasury yield, a Japan rate gap), `diagnosis` (as `diagnosis_page` groups) and `questions` (Q1 to Q4 figures or `Gap`, owner text or `Gap`), each `anyOf` the block or `Gap`; the price line cell says `band`, `owner override` or `expired`.

Acceptance:
1. `go test ./internal/contracts -race -count=1` passes with `TestCompaniesPageV2Fixtures` (ok and gap validate; an unknown verdict field is refused).
2. `contracts/generate.sh` then `git diff --exit-code` leaves no change; the hash of v1 in `carryover.sha256` is unchanged.

Checked by CI: ci-go, ci-web (`tsc`, `eslint`, `vitest`, the build).

### companies-page-band (interface and implementation)

- Module: pages. Size: M. Order: after 12 and 7, and after event-sources `earnings-job` (the `WatchEntry` type).
- Design: docs/design/valuation.md#Decisions (3, 5, 12)
- Files: internal/pages/companies_page/builder.go, internal/pages/companies_page/band.go, internal/pages/companies_page/builder_test.go, internal/pages/companies_page/band_test.go

- Goal: `Config`, `OwnerText`, `PriceOverride` and `Build` returning v2; rows from `Subjects`; the price line cell reads `valuation.buy_band` (band, `owner override` when given, `expired` after 90 days), `unvalued` with the reason text for a `missing_input`, the moat text from `OwnerText`; every other block a `Gap`; the builder holds no formula.

Acceptance:
1. `go test ./internal/pages/companies_page -race -count=1` passes with `TestBandIsThePriceLine`, `TestOverrideIsLabelled`, `TestExpiredAfter90Days` (fake clock), `TestBankIsUnvalued`, `TestNilReadersGiveGaps`.
2. `grep -c "decimal\|float64\|math\." internal/pages/companies_page/band.go` prints 0; `git diff $(git merge-base HEAD origin/main) -- internal/pages/api.go | grep -c .` prints 0.

Checked by CI: ci-go; the size rule.

### companies-page-sections

- Module: pages. Size: L. Order: after 13, 8, 9.
- Design: docs/design/valuation.md#Decisions (4, 7, 10, 12)
- Files: internal/pages/companies_page/core.go, internal/pages/companies_page/questions.go, internal/pages/companies_page/core_test.go, internal/pages/companies_page/questions_test.go

- Goal: `core` (spot and the strike of each shown offer as the cost, from the compute funcs on `BuildFilings`; implied growth from the quote; `DGS10` at most 7 days old, a US name only; a JP name shows a rate gap), `diagnosis` and `questions` from readings and owner text; four lines per cost, no rank, no filter, no colour word.

Acceptance:
1. `go test ./internal/pages/companies_page -race -count=1` passes with `TestCostIsTheBareStrike`, `TestNoAnalystInputLeavesTwoLinesOut`, `TestTreasuryStaleGivesGap`, `TestJpHasRateGap`, `TestRowsKeepCallerOrder`.
2. The built page of the ok fixture inputs equals `page.companies_page.v2.ok.json` byte for byte.

Checked by CI: ci-go.

### companies-page-web

- Module: web. Size: M. Order: after 12 (the generated types).
- Design: docs/design/valuation.md#Decisions (12)
- Files: web/src/pages/companies/CompaniesPage.tsx, web/test/companies_page_v2.test.tsx

- Goal: the page renders the three v2 blocks and the price-line label from the fixtures, computes nothing, and still renders v1.

Acceptance:
1. `pnpm vitest run web/test/companies_page_v2.test.tsx` passes (ok, gap, and a v1 document).
2. `pnpm tsc --noEmit && pnpm eslint web/src/pages/companies` prints no error.

Checked by CI: ci-web.

### valuation-wire-us

- Module: ot. Size: M. Order: after 3, 10, 11, 14 and after event-sources `watch-loader-earnings-wire` (the `watch.toml` loader). Second-opinion case: new outside service (the SEC is wired here; the environment name `OT_SEC_USER_AGENT` only, no value).
- Design: docs/design/valuation.md#Interface
- Files: cmd/ot/wire.go, cmd/ot/env.go, cmd/ot/runonce.go, cmd/ot/pagelist.go, cmd/ot/wire_test.go, docs/contracts/ot.md

- Goal: `newSEC`, the `fundamentals` job in the registry, the valuation step passed to the figures job, `companies_page` in the page list on the pages DSN, `Subjects` from the loader and `Held` from the positions reader; an unset value leaves the job out and the page on gaps.

Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with a test each for unset (job left out) and set (built over a fake source).
2. `ot page --name companies_page` over the fixtures prints a document that validates (offline).
3. `grep -rn "OT_SEC_USER_AGENT" cmd/ot | grep -c "= *\""` prints 0 (names only).

Checked by CI: ci-go; `go run ./scripts/arch --check`. Not verified offline: a real band for a real symbol; the owner runs `ot run-once` for one day and reads the page.

### edinet-client

- Module: sources. Size: M. Order: after 16. Second-opinion case: new outside service (`api.edinet-fsa.go.jp`, API key from the environment).
- Design: docs/design/valuation.md#Decisions (8)
- Files: internal/sources/edinet/client.go, internal/sources/edinet/cursor.go, internal/sources/edinet/client_test.go, internal/sources/edinet/testdata/documents_list.json

- Goal: `edinet.New`: list a day's documents (annual securities reports, form code 030000 and amendments), download the XBRL CSV of a document id, return rows with document id, filing day and `raw_sha256`; the key goes in the query and never in an error or a log line; a day cursor so a backfill scans the days needed.

Acceptance:
1. `go test ./internal/sources/edinet -race -count=1` passes with `TestListKeepsAnnualReportsOnly`, `TestDownloadReturnsDocID`, `TestKeyNeverInErrorText`, `TestCursorResumes`.
2. `grep -c "time.Sleep\|http.Get\|edinet-fsa.go.jp" internal/sources/edinet/client_test.go` prints 0.

Checked by CI: ci-go. Not verified offline: the first real fetch; the recorded files follow the API's published format and the real CSV (UTF-16, tab separated) may differ, so the owner's first real day is the check.

### edinet-mapping

- Module: sources. Size: L. Order: after 17.
- Design: docs/design/valuation.md#Decisions (1, 8)
- Files: internal/sources/edinet/mapping.go, internal/sources/edinet/concepts.go, internal/sources/edinet/mapping_test.go, internal/sources/edinet/testdata/jgaap_rows.csv, internal/sources/edinet/testdata/ifrs_rows.csv, internal/compute/valuation/params_jp.json

- Goal: map J-GAAP and IFRS elements to the concept names of Decision 6 (unit JPY); a table lists what each standard does not report (J-GAAP: diluted shares, stock compensation); the Japan parameter file `params_jp.json` is new, registered as new at wiring. Note: this issue edits one compute path for a data file only; it is the exact cross-module Files set of this entry. Its registration line is made by issue 19.

Acceptance:
1. `go test ./internal/sources/edinet -race -count=1` passes with `TestJGaapMapsToSameConcepts`, `TestIfrsMapsToSameConcepts`, `TestJGaapMissingDilutedSharesIsNoFact`, `TestCompetingElementsRefused`.
2. `wc -l internal/sources/edinet/*.go` shows every file under 600.

Checked by CI: ci-go.

### valuation-wire-jp

- Module: ot. Size: S. Order: after 18. Second-opinion case: new outside service (`OT_EDINET_KEY` name only).
- Design: docs/design/valuation.md#Decisions (8, 9)
- Files: cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md

- Goal: `newEDINET` and the Japan subjects into the `fundamentals` job; a missing key leaves Japan out, the US unchanged.

Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with an unset and a set case.
2. `grep -rn "OT_EDINET_KEY" cmd/ot | grep -c "= *\""` prints 0.

Checked by CI: ci-go.

## Left out

- The largest strike figure and the filter of offers by it, net cost (strike minus premium), ledger and tracking: the later candidates design. The owner text store, the override store and the price-lines page post. The ADR 0008 approval flow (Policy, notice, batches): not ported.
- `floor_dcf` v1, the sale price (ADR 0019), the fundamental history and implied-growth record tables, valuation drift.
- FMP and analyst estimates (Decision 10). A Japanese 10-year yield source. Producers of `core_page` and `diagnosis_page`.
- Banks, insurers, cyclicals: diagnosis only, no method.

## Open questions for the owner

1. The band's lower edge: is it the value at a 10% yield (with 8% the top and 6% only a reference)? And do the 90 days count from the day the figure is made (this design) or from a day you confirm it, as ADR 0008 counted from approval?
2. The system's price: is the band the 5-year-median free cash flow over 8% only (`fcf_yield`), or should the safer `floor_dcf` price of ADR 0008 also be ported and used?

## Adversarial

Review file: dispatcher scratchpad `valuation-adversarial.md` (not in the repository). Verdict on revision 1: goal not met. Answers:

1. Gap, no holdings or subject list or symbol-to-CIK mapping: adopted (Decision 9: the shared `watch.toml` of event-sources owns the list; the held view is an optional page input).
2. No web rendering: adopted (issue 15). No max strike: adopted as stated in Goal and Left out (candidates design). Price as input of the band: adopted (`BandFunc` has no price; implied growth solved at page build, Decision 4).
3. JP: adopted. No claim of old EDINET code (Decisions, main today); US-first order (16 before 17); J-GAAP names ending unvalued stated; JP rate a gap with a candidate source; a Japan parameter file in 18; backfill by day cursor (17).
4. Expiry start, ADR 0008:25 and 0008:14, `floor_dcf`: adopted as plain statements (Decision 3, no Policy created) and owner questions 1 and 2; not adopted: moving the start silently.
5. Facts to Filings has no owner: adopted (Decision 7, issue 5; the store returns all rows filed by the cutoff).
6. Simpler cut: adopted, merging store interface and implementation, sources interface into sec-source, page interface into band, function types instead of one interface, US first. Not adopted: dropping questions and diagnosis (the owner wants them on the page), and a normalised dataset (ADR 0008 wants the raw fact).
7. Decimal: adopted (explicit 34 digits via `div`, no global, `Rat` fallback, dependency added in `compute-filings`). Not adopted: `math/big.Rat` first, since AGENTS prefers a mainstream library.
8. Alpha Vantage claim false: adopted (FMP, Decision 10). Cost strike minus premium: adopted, bare strike kept (Decision 4). Band 8% and 10%: kept as owner question 1.
9. Concept names and the cover-page share count: adopted (Decision 6). `rules.json` with a page-time figure: adopted as a check in issue 7. Web path in contracts: adopted (the contracts row owns `web/src/generated/**`).
10. Goldens needing the old system: adopted (issue 6, generator under `tools/compute-golden/`, own uv project, committed JSON). Test files over 600 lines: adopted by splitting files per issue. Migration number: adopted ("next free at cut time", no number in Files). Same-file overlap with account-data and events: adopted (Order paragraph in Modules). Register, rules and glossary serial edits: adopted (issues 7, 8, 9 in order).
11. Not adopted: a hand-written EDINET sample being the only offline check; it cannot be avoided, and the design says the first real day is the check.
