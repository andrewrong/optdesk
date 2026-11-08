# Valuation: company value, the buy-price band (the price line), return at a price and the five questions, ported from option-trader

Author: the optdesk designer
Owner approved: the owner's decisions below are binding (goal, source, EDINET automation); the advisor's of 2026-11-08 (port ADR 0008 and 0023 in meaning); the rest is for the design review
Revision: 6 (design verdict on revision 5 applied; revision 5: design verdict on revision 4 applied; revision 4: design verdict on revision 3 applied; revision 3: owner answers of 2026-11-09; revision 2: adversarial review adopted, watch list shared with event-sources)

Source: the owner's goal "value investing plus option selling", the loop company value, buy price band, strike, candidates, ledger, tracking; the advisor of 2026-11-08; the adversarial review of revision 1 (dispatcher scratchpad, not in the repository).
Read in option-trader at 4704569f (read only): `docs/adr/0008-valuation-and-acquisition-policy.md`, `0023-return-at-price.md`, `src/option_trader/valuation/` (`fcf_yield.py`, `reverse_dcf.py`, `price_return.py`, `five_questions.py`, `diagnosis.py`, `filing_rows.py`, `facts.py`), its `params/*.toml`, `sources/fmp/analyst_estimates.py`.
Read in optdesk on main at ea2b0da: `AGENTS.md`, `docs/design/modules.md`, `ease-v2.md`, `report-page-producer.md`, `compute-figures.md`, the `companies_page`, `core_page`, `diagnosis_page`, `price_lines_page` v1 schemas, `contracts/datasets/`, `docs/conventions/register.toml`, `internal/sources`, `internal/compute`, `internal/jobs`, `internal/store`.

Priority and the hand price (owner 2026-11-09): valuation is lower priority than options. Until this lands the owner hand-enters `buy_price` in `watch.toml` and keeps doing so; this design never reads it as a figure of its own and never overwrites it. Once he has approved a band, the page shows the band beside that hand price, two separate values in the price line cell. `buy_price` is a field of `watch.toml` that the event-sources design (PR 606) owns as `WatchEntry.BuyPrice` (with `SalePrice`), loaded and validated by its `cmd/ot` loader; this design adds no field and reads it only for display, from that type.

Goal: for each company the owner holds or watches, a stored buy-price band made only from filings, shown as the price line of the companies page, with the return a price earns and the four numeric answers of the five questions beside it; every figure registered with its hash. The last step of the loop, the largest strike an offer may have (strike minus premium at or below the band), is the filter of a later candidates design: this design stores and shows the figure it will read and does not filter offers. US first: the SEC path is complete end to end before the EDINET issues start.

## What the owner gets

- A buy-price band per company: the highest price he would accept as assignment price (value at an 8% free-cash-flow yield) and a lower edge (10%); 6% is shown as a reference outside the band. It is the price line on the page, shown beside his hand-entered `buy_price` when the row has one. The figure is valid 90 days from the day he approves it (owner 2026-11-09, as option-trader ADR 0008); he approves with one `ot` command, and after the 90 days the page says "expired" and shows no band. Before he approves, the page shows the figure labelled "awaiting approval".
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

1. Ported in meaning (advisor, 2026-11-08): `fcf_yield` v2 (5-year median of operating cash flow minus capital expenditure minus stock compensation, over current diluted shares, over the required yield; 6%, 8%, 10%, primary 8%; leverage and instability flags; share-count sanity; eligible years: 350 to 380 days, annual forms only, consecutive); reverse DCF (10 years, terminal 2%, rates 8%, 10% primary, 12%, no fade); `price_return` v1 with its four lines, gate, refusals and flags (ADR 0023 and its three notes); five questions Q1 to Q4; the diagnosis. Parameters are the option-trader files pinned by git blob id: `fcf_yield_v2.toml` 4952b59720ce, `reverse_dcf_v1.toml` e1e1f731c7b8, `price_return_v1.toml` 8d7be25e35c3, `five_questions_v1.toml` 6755537f67d5, `diagnosis_v1.toml` 643178784313, `filing_rows_v1.toml` 967d8d64ff48. They are US-GAAP and USD; Japan needs its own parameter file (concepts, JPY), written in `valuation-compute-jp`, new and registered as such, not "ported".
2. Arithmetic: Decimal everywhere, no float64. Library `github.com/shopspring/decimal` (MIT, widely used, stable, no dependencies), added to `go.mod` by the first issue that imports it (`compute-filings`). Python used precision 34 per call (`localcontext(prec=34)`); shopspring has a package-global `DivisionPrecision`, which no code here sets or reads: every division goes through one helper `div` that keeps 34 significant digits explicitly, rounding half-even as the old code. The goldens compare exact text with the Python output; if parity cannot be reached the issue stops and reports, and `math/big.Rat` (standard library) is the fallback behind the same helper (own glue behind one function, owner 2026-11-08). The interface carries decimal text, so no library type is in an interface.
3. The band. `valuation.buy_band` v1, subject kind `underlying`, subject the symbol, value = value per share at 8% (the maximum assignment price), parameters `low` (value at 10%), `ref_high` (6%), `base_fy_end`, flags. It is price-free: no function that makes it takes a price. Owner 2026-11-09: the band runs from the value at 10% (lower edge) to the value at 8% (the maximum acceptable price, primary per ADR 0008), and it is the only system price proposal (`fcf_yield` alone; `floor_dcf` is not ported). The figure is valid 90 days counted from the owner's approval day, not from the data day. The approval is a reading `valuation.band_approval` v1 of the existing dataset `readings.reading` r1: subject kind `underlying`, subject the symbol, `day` = the approval day, `value` = the 8% value text it approves (unit `money`, the figure's currency), state `ok`, origin `owner_command`. That origin is a new additive value of the `origin` enum (issue 2, contracts; the old dataset allowed `compute_figures` only, and the store has no database check on it). A second approval of the same day takes the next `rev`; the view returns the winner of the day, which is that newest approval. It is written by the owner-run command `ot approve-band --symbol S [--day D]` (issue 18, in `cmd/ot`; no Policy, no notice, no batch). The band function holds no expiry and no clock: the page builder (issue 9) reads the band and the latest approval (`WinnerReadings` of the pages view reader over the 400 days before today), and shows the band as valid while today is within 90 days of the approval `day` and the approval `value` equals the band's current 8% value; a different value (a new filing) or no approval shows "awaiting approval", and a day past the 90 shows "expired". ADR 0008:25 ("the system's value is not a price limit; the owner's explicit maximum assignment price is the only one") and 0008:14 stay in force for any Policy: this design creates no Policy and no approval flow. The band is a displayed figure; that candidates read it as the cap follows the owner's goal and is decided in the candidates design with the owner.
4. Return at a price keeps the bare strike as ADR 0023:258-260 says (the premium is already in the put's own return). The net cost (strike minus premium) belongs to the candidates filter design. The reverse DCF implied growth is solved at page build from the quote, as ADR 0008 says, and is not stored; `valuation.implied_growth` and `valuation.return_at_price` are registered figures computed at page build, never stored; `compute-fcf-band` checks that `TestRulesMatchRegister` accepts a registered figure with no stored reading and, if it does not, reports the file to change instead of editing it.
5. Unvalued is a state with a reason, not zero: SIC 6000-6799 (the SIC code comes from the SEC `submissions` document, which `companyfacts` lacks, fetched by `sec-source` and stored as one fact of the fact dataset, Decision 6; a company with no SIC fact is refused with a reason), the cyclical SIC ranges, a rejection (fewer than 5 eligible years, non-consecutive years, share-count jump, stale latest year over 457 days, non-positive base) gives `missing_input` with the reason in parameters. Cyclical and financial names get the diagnosis instead of a band.
6. Fundamentals, US (owner choice a). SEC `companyfacts` directly (User-Agent from the environment variable `SEC_USER_AGENT`, at most 5 requests a second, public CIK only). Concepts kept: the files' `NetCashProvidedByUsedInOperatingActivities`, the four capex concepts, `ShareBasedCompensation` and `AllocatedShareBasedCompensationExpense`, `WeightedAverageNumberOfDilutedSharesOutstanding`, `LongTermDebt`, `LongTermDebtNoncurrent`, the concepts `filing_rows_v1.toml` and `five_questions_v1.toml` list, and `dei:EntityCommonStockSharesOutstanding` (cover-page share check), taxonomies `us-gaap` and `dei`, and the SIC code (taxonomy `sec`, concept `StandardIndustrialClassification`, unit `sic`, from `submissions`). One dataset `obs.fundamental_fact` r1: `source` (`sec`, later `edinet`; a later source extends the enum additively in its own contracts change), `subject` (the issuer text of the watch list: `cik:<10 digits>`, later `edinet:<code>`), `taxonomy`, `concept`, `unit`, `currency`, `period_start`, `period_end`, `value` (decimal text), `form`, `doc_id`, `filed`, `retrieved_at`, `raw_sha256`; key `(source, subject, doc_id, taxonomy, concept, unit, period_start, period_end)`, append only; in the table `period_start` is stored as `period_start_key`, `NOT NULL` with `0001-01-01` for an absent start, so a repeated instant fact is a duplicate in PostgreSQL. The writer role is `ot_w_fundamental` (`OT_PG_DSN_FUNDAMENTAL`), made by `setup.Setup`; `OpenFactStoreFunc` opens the store. Every filed value is kept, so a later recast never hides the annual-form fact that makes a year eligible.
7. Facts to Filings is compute's: `BuildFilings` takes the facts as the store returns them, applies the as-of rule (latest filed on or before the cutoff; `fy`/`fp` never decide) and the eligible-year rule, and returns ten fiscal years of Decimal text figures or a refusal reason. The job and the page both call it; no formula is in pages. The store query returns all rows filed on or before the cutoff for a subject (no winner picked in SQL).
8. Japan (owner: EDINET automation). EDINET API v2 lists documents by day, not by company: the client scans days in chunks and keeps no state; the day cursor (the last day scanned) is one text line in the file `$OT_HOME/edinet.cursor`, read and written through a `Cursor` interface (`Last() (contracts.Day, error)`, `Save(contracts.Day) error`) that issue 5 declares in `internal/sources/api.go` and `cmd/ot` implements, so sources do no file or database I/O. The backfill command `ot backfill-edinet --from D` (issue 22, `cmd/ot/backfill_edinet.go`) scans from the cursor to today in chunks through the source, writes through `FactWriter` and saves the cursor, until five eligible annual reports exist for the watch list names; the daily `fundamentals` job takes `edinet:` entries in issue 23. J-GAAP and IFRS map to the same concept names through a table; a standard not recognised or competing elements give no fact and a typed reason. Every fact keeps its EDINET document id.
9. The subject list is not defined here. The event-sources design (`docs/design/event-sources.md`, Decision 1) owns the one owner-edited list `$OT_HOME/watch.toml`, type `jobs.WatchEntry{Issuer, Market, ReportDateOverride, BuyPrice, SalePrice string; Contract broker.Contract}` in `internal/jobs/api.go` (valuation ignores the override and the sale price; `BuyPrice` is the decimal text the page shows as the hand price), loaded and validated by the `cmd/ot` loader of 606 (`watch-loader-earnings-wire`), rows keyed by issuer (`cik:<10 digits>` US, `edinet:<code>` JP). Valuation reuses the file and type: the fundamentals job plans from the entries (the CIK or EDINET code is the issuer text), the band's subject is the entry's symbol. This design depends on that design's `earnings-job` (adds `WatchEntry`, needed by `fundamentals-job`) and `watch-loader-earnings-wire` (the loader, needed by `valuation-wire-us`) and adds no second list. The pages module may not import `internal/jobs` (its depends cell is `contracts store compute`), so the page builder takes its own `Subject{Issuer, Market, Symbol}` and `cmd/ot` fills it from the entries. The owner lists every company he holds in `watch.toml` too. Which are held (page block `holdings` against `watch_list`) is an optional input of the page builder, `Held func(ctx) (map[string]bool, error)` keyed by issuer, nil puts every row in the watch list block. No positions reader exists on main, so issue 18 passes nil and a later design (account data) wires it; this design waits for no account-data issue. A name without an issuer id is skipped with a typed reason, never guessed.
10. Owner 2026-11-09: the analyst line stays left out. FMP and analyst estimates are left out. The source in option-trader is FMP (ADR 0025). `ReturnAtPrice` takes an optional analyst input that the page passes as absent; lines 3 and 4 show the ADR's own state "left out: no analyst input".
11. Registration: each stored figure is a `[[figure]]` entry of `register.toml` with `source_hash` of its Go files and a `history` line naming the option-trader blob ids of Decision 1, landed with the code (as `compute-ease`): six entries, each v1: `valuation.buy_band`, `valuation.band_approval` (an owner fact with no formula), `valuation.implied_growth`, `valuation.return_at_price`, `valuation.question`, `valuation.diagnosis`. Goldens come from a generator committed under `tools/compute-golden/valuation/` (own uv project, own `uv.lock`, not run in CI); the committed JSON is what tests read, so no check needs the old system.
12. One page: `companies_page` v2 adds optional `core`, `diagnosis`, `questions` blocks to v1 (v1 stays readable). The owner text (fifth question, moat) and the override reach the builder through optional interfaces (`OwnerText`, `PriceOverride`, signatures in Interface) declared in the page package; nil gives a gap "not entered"; their stores are later designs. `core_page` and `diagnosis_page` stay unproduced.

## Modules

New directories: `internal/sources/sec`, `internal/sources/edinet`, `internal/compute/valuation`, `internal/jobs/fundamentals`, `internal/pages/companies_page`, `internal/store/fundamental`, `tools/compute-golden/valuation` (compute row). No row or depends cell changes. `docs/design/modules.md` and the generated `arch.toml` change in the entry `valuation-docs-arch` (issue 10): it adds the six `internal/...` implementation packages (`internal/store/fundamental`, `internal/sources/sec`, `internal/sources/edinet`, `internal/compute/valuation`, `internal/jobs/fundamentals`, `internal/pages/companies_page`) to the sentence that lists what `cmd/ot` may import and regenerates `arch.toml` with `go run ./scripts/arch`; `arch.toml` is derived from the table alone, so it can list a package before its directory exists. `.go-arch-lint.yml` also reads the tree and flags a package that belongs to no component, so the entry that creates a package carries the lint lines: issues 4, 5, 6, 7, 9 and 19 each list `.go-arch-lint.yml` (regenerated, never hand edited). Issues 4 to 7, 9 and 19 are ordered after issue 10, so `cmd/ot` may name the packages when issue 18 lands.

Cross-module entries licensed by this design, each Files line being exactly the licensed set: issues 4, 5, 6, 7, 9 and 19 (their module and `.go-arch-lint.yml` of repo), issue 10 (docs and repo: `docs/design/modules.md`, `arch.toml`), issue 11 (compute and repo: `go.mod`, `go.sum`), issues 13, 14, 15 and 21 (compute and docs: `docs/conventions/register.toml`, `docs/GLOSSARY.md`, `internal/compute/rules/rules.json` are the docs and compute files each lists).

Migration numbers are "the next free one at cut time" in every issue (account-data and events take earlier numbers). Shared files and their Order: `internal/store/api.go`, `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `docs/contracts/store.md`, `internal/jobs/api.go` and `docs/contracts/jobs.md` are also edited by the account-data and events designs (the events design's `earnings-store` edits the store and setup files, its `earnings-job` the jobs ones); each issue here that edits one is ordered after the merge of the other design's issue that edits it, read from main at cut time, and waits if it is not merged. `contracts/fixtures/MANIFEST.synthetic-v1.json` is edited by issues 1, 2 and 3 only, in that order.

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | fact-dataset (interface) | contracts | M | first |
| 2 | band-approval-origin (interface) | contracts | S | after 1 |
| 3 | companies-page-v2 (interface) | contracts | M | after 2 |
| 4 | fact-store (interface and implementation) | store, plus repo | M | after 1, 10, events `earnings-store` |
| 5 | sec-source (interface and implementation) | sources, plus repo | M | after 1, 10 |
| 6 | valuation-compute-interface (interface) | compute, plus repo | M | after 1, 10 |
| 7 | fundamentals-job (interface and implementation) | jobs, plus repo | M | after 4, 5, 10, events `earnings-job` |
| 8 | valuation-job-step (interface and implementation) | jobs | M | after 7, 11, 13, 15 |
| 9 | companies-page-band (interface and implementation) | pages, plus repo | M | after 3, 6, 10, 13 |
| 10 | valuation-docs-arch | docs, plus repo | S | after events `events-docs-modules` |
| 11 | compute-filings | compute, plus repo (`go.mod`, `go.sum`) | M | after 6 |
| 12 | compute-golden | compute | M | after 11 |
| 13 | compute-fcf-band | compute, plus docs | L | after 12 |
| 14 | compute-return-at-price | compute, plus docs | L | after 13 |
| 15 | compute-questions-diagnosis | compute, plus docs | L | after 14 |
| 16 | companies-page-sections | pages | L | after 9, 14, 15 |
| 17 | companies-page-web | web | M | after 3 |
| 18 | valuation-wire-us | ot | M | after 4, 5, 7, 8, 16, events `watch-loader-earnings-wire` |
| 19 | edinet-client | sources, plus repo | M | after 18, 10 |
| 20 | edinet-mapping | sources | L | after 19 |
| 21 | valuation-compute-jp | compute, plus docs | M | after 11, 15, 20 |
| 22 | valuation-wire-jp | ot | M | after 21, 23 |
| 23 | fundamentals-edinet | jobs | S | after 7, 20 |

Issues 1 to 18 are the US path; 19 to 23 start only when 18 is merged and the owner has seen a real US band. `register.toml`, `rules.json` and `docs/GLOSSARY.md` are in 13, 14, 15 and 21 only, in that order (21 adds no entry and no glossary line: a `history` line on `valuation.buy_band` and its rules entry), each adding its own entries. `internal/compute/api.go` is in 6 only; `internal/sources/api.go` in 5 only (the Japan constructor type is declared in 5 so 19 needs no interface edit); `companies_page/builder.go` in 9 then 16 (9 builds every block but the price line as a `Gap`; 16 fills the other blocks in `Build` by calling the functions it adds, so `TestBuildEqualsOkFixture` can pass); `internal/jobs/fundamentals/plan.go` and `run.go` in 7 then 23; `internal/compute/valuation/params.go` in 11 then 21; `cmd/ot/main.go` and `docs/contracts/ot.md` in 18 then 22 `cmd/ot/wire.go` in 18 then 22. `.go-arch-lint.yml` is edited by 4, 5, 6, 7, 9 and 19, each adding only its own component lines on the main read at cut time, in table order (7 after 4 and 5, 19 after 18).

## Interface

- contracts: dataset `obs.fundamental_fact` r1 and generated `contracts.FundamentalFact` (alias in `internal/contracts/api.go`); the additive origin `owner_command` of `readings.reading` r1; `page.companies_page.v2.json` with its generated TypeScript (the contracts row owns `web/src/generated/**`).
- store: `FactWriter.InsertFacts`, `FactReader.FactsFiledBy(ctx, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error)` (`subject` is the issuer text of the watch list, `cik:<10 digits>`), `FactStore`, `OpenFactStoreFunc func(ctx context.Context, dsn string) (FactStore, error)`, `DSNEnvFundamental = "OT_PG_DSN_FUNDAMENTAL"` (role `ot_w_fundamental`; the page opens the same store with `DSNEnvPages`; `FactsFiledBy` reads the view `rd.fundamental_fact_r1` on both).
- sources: `Cursor`, `FactPlan{Subjects []string; From, To contracts.Day}`, `FactSource.Fetch`, `SECConfig{BaseURL, UserAgent, Clock}`, `EDINETConfig{BaseURL, APIKey, Clock}`, `NewSECFunc`, `NewEDINETFunc`.
- compute, function types in the style of `ReadingFunc` (no five-method interface): `StandardOfFunc func(facts []contracts.FundamentalFact) Standard` (the accounting standard of a company read from its own facts: `US_GAAP` when the facts have source `sec`, else the unit of the fact with taxonomy `dei` and concept `AccountingStandard` that `edinet-mapping` writes per filing (`J_GAAP` or `IFRS`, value `1`); none gives the empty `Standard`, and `BuildFilings` then refuses with `no_standard`), `BuildFilingsFunc func(facts []contracts.FundamentalFact, asOf contracts.Day, std Standard) FilingsResult`, `BandFunc func(id string, f Filings, asOf contracts.Day) BandFigure`, `ImpliedGrowthFunc func(f Filings, spot string) ImpliedResult`, `ReturnAtPriceFunc func(f Filings, costs []string, a *Analyst, asOf contracts.Day) ReturnResult`, `QuestionsFunc func(f Filings, asOf contracts.Day) QuestionsResult`, `DiagnosisFunc func(f Filings, asOf contracts.Day) DiagnosisResult` (the result types are declared in issue 6: Q1 to Q4 figures or `missing_input` with a reason, and the cyclical or financial group figures); `ParamsFor(std Standard) (Params, bool)` in the valuation package (concepts and thresholds by standard); text decimals, no I/O, no clock.
- jobs: `NewFundamentalsFunc func(sec, edinet sources.FactSource, w store.FactWriter, entries []WatchEntry, c clock.Clock) (contracts.Job, error)` (`cik:` entries go to `sec`, `edinet:` entries to `edinet`; a nil `edinet` means no Japan source, and the job skips and names those entries; declared once by issue 7 so issue 23 edits no interface); `NewComputeFiguresValuationFunc func(ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, ValuationDeps, clock.Clock) (contracts.Job, error)` with `ValuationDeps{Facts store.FactReader; Entries []WatchEntry; StandardOf compute.StandardOfFunc; BuildFilings compute.BuildFilingsFunc; Band compute.BandFunc; Questions compute.QuestionsFunc; Diagnosis compute.DiagnosisFunc}`; the job plans one key per entry that has an issuer and writes readings whose subject is the entry's contract symbol, facts read by the issuer; issue 18 passes the entries the `watch.toml` loader returns. The existing constructors are unchanged.
- pages, in `internal/pages/companies_page/builder.go`, importing contracts, store and compute only (no `internal/jobs`): `Subject{Issuer, Market, Symbol, HandPrice string}` (`HandPrice` is `WatchEntry.BuyPrice` of 606, the decimal text of the row's `buy_price` in `watch.toml`, empty when none); `OwnerText interface { Moat(ctx, symbol string) (text string, ok bool, err error); FifthQuestion(ctx, symbol string) (text string, ok bool, err error) }`; `PriceOverride interface { Line(ctx, symbol string) (price string, ok bool, err error) }`; `Config{Zone *time.Location; Today func() contracts.Day; Subjects []Subject; Held func(ctx context.Context) (map[string]bool, error); Read store.ReadingViewReader; Facts store.FactReader; Spot func(ctx context.Context, symbol string, day contracts.Day) (string, bool, error); Offers func(ctx context.Context, symbol string, day contracts.Day) ([]string, error); StandardOf compute.StandardOfFunc; BuildFilings compute.BuildFilingsFunc; Band compute.BandFunc; ImpliedGrowth compute.ImpliedGrowthFunc; ReturnAtPrice compute.ReturnAtPriceFunc; Questions compute.QuestionsFunc; Diagnosis compute.DiagnosisFunc; Text OwnerText; Override PriceOverride}`. `Offers` returns the strikes (decimal text) of the puts shown for the symbol on the day; `Spot` returns the last quote text of the symbol on or before the day; a nil `Offers` or `Spot` gives a gap. `Read` is the view reader opened on the pages DSN (`OpenReadingViewFunc`, role `ot_ro_pages`, which reads `rd.reading_r1` only); `Facts` is opened on the same DSN. `internal/pages/api.go` unchanged.
- ot: `newCompaniesPage`, `newSEC`, `newEDINET` in `cmd/ot/wire.go`; nil when a DSN or key is not set; the environment names are `SEC_USER_AGENT` and `EDINET_API_KEY` (third-party keys carry no `OT_` prefix; the DSN lines keep it). `cmd/ot` fills `Subjects` from `jobs.WatchEntry` (issuer, market, contract symbol, `HandPrice` from `WatchEntry.BuyPrice` of 606, copied as text, never parsed or validated here), `Today` from the clock, `Offers` from the `OptionEodReader`, `Spot` from the quote snapshot reader with the instrument name reader mapping the symbol to its instrument id, `Held` nil. The `Cursor` file of Decision 8 and `ot backfill-edinet` are in `cmd/ot` (issue 22). `ot approve-band --symbol S [--day D]` in `cmd/ot/approve.go` writes the `valuation.band_approval` reading (Decision 3).

## Issues

Every test calls `t.Parallel()`, uses `t.TempDir()`, a fake clock, no sleep, no fixed port, no live service (recorded bodies, local `httptest` servers on a port the OS picks), no live database (a PostgreSQL test carries the tag `pg`). Every issue's last Acceptance item is: `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the paths of its Files line. No new file passes 600 lines. A Go test added to a file of 800 lines or more goes to a new file.

### fact-dataset (interface)

- Module: contracts. Size: M. Order: first (1). Contracts: changes `contracts/datasets`.
- Design: docs/design/valuation.md#Decisions (6)
- Files: contracts/datasets/obs.fundamental_fact.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, internal/contracts/api.go, contracts/fixtures/obs.fundamental_fact.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/fundamental_fact_test.go, docs/contracts/datasets/obs.fundamental_fact.md

- Goal: the dataset (fields of Decision 6, with `subject` and the key there) and the generated type, listed in `contracts/generate.sh` (its `--schema-root-type`, `--schema-output` and input arguments) with the alias `FundamentalFact` in `internal/contracts/api.go`; a fixture of four records (SEC instant, SEC duration, the SEC industry code, a JPY record with `source` `edinet`). No other file is edited; a further file is reported.

Acceptance:
1. `go test ./internal/contracts -race -count=1` passes with `TestFundamentalFactFixture` (four records validate; a `source` of `fmp` fails; a duration fact without `period_start` fails; a record without `subject` fails).
2. `grep -c "obs.fundamental_fact" contracts/generate.sh` is at least 3; `contracts/generate.sh` then `git diff --exit-code` leaves no change.
3. The diff names exactly the Files paths.

Checked by CI: ci-go; the fixture manifest test; `scripts/hygiene`.

### band-approval-origin (interface)

- Module: contracts. Size: S. Order: after 1 (the manifest). Contracts: changes `contracts/datasets`.
- Design: docs/design/valuation.md#Decisions (3)
- Files: contracts/datasets/readings.reading.r1.json, internal/contracts/gen/records.go, contracts/fixtures/readings.reading.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/reading_origin_test.go, docs/contracts/datasets/readings.reading.md

- Goal: the `origin` enum of `readings.reading` r1 gains `owner_command` (additive: no field and no key changes, existing records stay valid), with one fixture record of `valuation.band_approval` (`day` the approval day, `value` the approved 8% value, `unit` `money`, origin `owner_command`), so `ot approve-band` can write a record the dataset takes. The store has no database check on `origin`, so no migration is needed.

Acceptance:
1. `go test ./internal/contracts -race -count=1` passes with `TestReadingOwnerCommandOrigin` (the new fixture validates; origin `other` still fails) and the existing `schema_test.go` unchanged.
2. `grep -c "owner_command" contracts/datasets/readings.reading.r1.json internal/contracts/gen/records.go` is at least 1 in each; `contracts/generate.sh` then `git diff --exit-code` leaves no change.
3. The diff names exactly the Files paths.

Checked by CI: ci-go; the fixture manifest test.

### companies-page-v2 (interface)

- Module: contracts. Size: M. Order: after 2 (the manifest). Contracts: adds `contracts/schemas/page.companies_page.v2.json`.
- Design: docs/design/valuation.md#Decisions (12)
- Files: contracts/schemas/page.companies_page.v2.json, contracts/fixtures/page.companies_page.v2.input.json, contracts/fixtures/page.companies_page.v2.ok.json, contracts/fixtures/page.companies_page.v2.gap.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/companies_page_v2_test.go, web/src/generated/companies_page.v2.ts

- Goal: v2 = v1 plus `core` (rows as `core_page` `CoreRow`, Treasury yield, a Japan rate gap), `diagnosis` (as `diagnosis_page` groups) and `questions` (Q1 to Q4 figures or `Gap`, owner text or `Gap`), each `anyOf` the block or `Gap`; the price line cell says `band`, `owner override`, `awaiting approval` or `expired`, and carries the optional `hand_price` text beside the band. The input fixture holds what the builder reads (subjects, readings with a band and an approval, facts, a quote, offer strikes, owner text) so pages, `cmd/ot` and web tests share one offline input; the ok fixture is the page it builds.

Acceptance:
1. `go test ./internal/contracts -race -count=1` passes with `TestCompaniesPageV2Fixtures` (ok and gap validate; an unknown verdict field is refused; the input fixture parses).
2. `contracts/generate.sh` then `git diff --exit-code` leaves no change; the hash of v1 in `carryover.sha256` is unchanged.
3. The diff names exactly the Files paths.

Checked by CI: ci-go, ci-web (`tsc`, `eslint`, `vitest`, the build).

### fact-store (interface and implementation)

- Module: store, plus repo (`.go-arch-lint.yml`, the lint lines of the new component). Size: M. Order: after 1 and 10, and after the merged events `earnings-store` and account-data edits of `internal/store/api.go`, `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go` and `docs/contracts/store.md` (read at cut time).
- Design: docs/design/valuation.md#Decisions (6)
- Files: internal/store/api.go, internal/store/api_fact_test.go, internal/store/setup/setup.go, internal/store/setup/setup_test.go, migrations/NNNN_fundamental_fact.sql, internal/store/fundamental/store.go, internal/store/fundamental/store_test.go, internal/store/fundamental/store_pg_test.go, docs/contracts/store.md, .go-arch-lint.yml

- Goal: `FactWriter`, `FactReader`, `FactStore`, `OpenFactStoreFunc` and `DSNEnvFundamental` in `api.go`; the contract test in the new file from both sides with a fake; role `ot_w_fundamental` (INSERT on the base table and SELECT on the view, line `OT_PG_DSN_FUNDAMENTAL`) made by `setup.Setup` in `setup.go`, with its test; the table `obs.fundamental_fact` (append only, no update or delete grant) whose primary key is `(source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end)`, where `period_start_key` is `NOT NULL` and holds `0001-01-01` for an absent `period_start` (the view returns NULL again), so a repeated instant fact is a duplicate; a view `rd.fundamental_fact_r1` granted SELECT to `ot_ro_pages` and to `ot_w_fundamental` (neither role reads the base table); `FactsFiledBy` reads the view only, so one implementation serves the figures job opened with `DSNEnvFundamental` and the page opened with `DSNEnvPages`, and returns every row of the subject filed on or before the cutoff. `NNNN` is the next free number at cut time.

Acceptance:
1. `go test ./internal/store/... -race -count=1` passes; `git diff $(git merge-base HEAD origin/main) --stat -- internal/store/api_test.go | grep -c .` prints 0.
2. `grep -ci "update\|delete" migrations/*_fundamental_fact.sql` prints 0; `grep -c "FactWriter\|FactReader\|FactStore\|OpenFactStoreFunc\|DSNEnvFundamental" internal/store/api.go` is at least 5; `grep -c "ot_w_fundamental" internal/store/setup/setup.go` is at least 1; `grep -c "period_start_key" migrations/*_fundamental_fact.sql` is at least 2; `grep -c "GRANT SELECT ON rd.fundamental_fact_r1 TO ot_w_fundamental" migrations/*_fundamental_fact.sql` prints 1.
3. `go run ./scripts/arch --check` prints `arch: ok`.
4. The diff names exactly the Files paths.

Checked by CI: ci-go-pg (insert twice adds 0 rows for a duration and an instant fact; `FactsFiledBy` works on both DSNs through the view; neither the page role nor the writer role can read the base table, and the page role cannot write); the size rule.

### sec-source (interface and implementation)

- Module: sources, plus repo (`.go-arch-lint.yml`). Size: M. Order: after 1 and 10. Second-opinion case: new outside service (`data.sec.gov`; public CIK only; the User-Agent contact text comes from the environment).
- Design: docs/design/valuation.md#Decisions (5, 6)
- Files: internal/sources/api.go, internal/sources/api_test.go, internal/sources/sec/client.go, internal/sources/sec/mapping.go, internal/sources/sec/pacing.go, internal/sources/sec/client_test.go, internal/sources/sec/mapping_test.go, internal/sources/sec/testdata/companyfacts_sample.json, internal/sources/sec/testdata/submissions_sample.json, .go-arch-lint.yml

- Goal: `Cursor`, `FactPlan`, `FactSource`, both configs and both constructor types (additive); `sec.New` fetches `/api/xbrl/companyfacts/CIK##########.json` and maps each kept concept to a `FundamentalFact` (`subject` the issuer text, `accn` as `doc_id`, `filed`, `form`, `raw_sha256`, `retrieved_at` from the clock), and `/submissions/CIK##########.json` for the SIC code, kept as one fact (taxonomy `sec`, concept `StandardIndustrialClassification`, unit `sic`, `period_end` the retrieval day) so the bank and cyclical rule of Decision 5 has its input; pacing as the FRED package (reused if possible), both requests counted; an error never carries the User-Agent.

Acceptance:
1. `go test ./internal/sources/... -race -count=1` passes with `TestMapKeepsAccessionFiledAndForm`, `TestCoverPageSharesKept` (the `dei` concept), `TestSicFromSubmissions` (SIC 6021 kept as a fact), `TestUnlistedConceptDropped`, `TestPacingFiveAPerSecond`, `TestErrorHidesUserAgent`.
2. `git diff $(git merge-base HEAD origin/main) -- internal/sources/api.go | grep '^-' | grep -vc '^---'` prints 0.
3. `grep -c "time.Sleep\|http.Get\|data.sec.gov" internal/sources/sec/*_test.go` prints 0 in each.
4. `go run ./scripts/arch --check` prints `arch: ok`; the diff names exactly the Files paths.

Checked by CI: ci-go; arch (the package imports no store).

### valuation-compute-interface (interface)

- Module: compute, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M. Order: after 1 and 10; `internal/compute/api.go` is in this issue only.
- Design: docs/design/valuation.md#Interface
- Files: internal/compute/api.go, internal/compute/api_valuation_test.go, docs/contracts/compute.md, internal/compute/valuation/doc.go, .go-arch-lint.yml

- Goal: the function types and result types of Interface with doc comments (text decimals, `Standard` with `US_GAAP`, `J_GAAP`, `IFRS`, `FilingsResult` carrying the SIC text, states, reasons of Decision 5, the four line keys `SPENDING_STAYS`, `SPENDING_EARNS`, `ANALYST_PATH`, `SPENDING_EARNS_ANALYST`, no verdict word); a fake of each, a caller test in the new file; `doc.go` as a package clause so the lint file names the component.

Acceptance:
1. `go test ./internal/compute -race -count=1` passes; `go list -deps ./internal/compute | grep -c "shopspring"` prints 0.
2. `go run ./scripts/arch --check` prints `arch: ok`; `grep -c "internal/compute/valuation" .go-arch-lint.yml` is at least 1.
3. The diff names exactly the Files paths.

Checked by CI: ci-go; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`.

### fundamentals-job (interface and implementation)

- Module: jobs, plus repo (`.go-arch-lint.yml`). Size: M. Order: after 4, 5 and 10, and after the merge of events `earnings-job` (adds `WatchEntry` to `internal/jobs/api.go`) and of the account-data edits of `internal/jobs/api.go` and `docs/contracts/jobs.md`, read at cut time.
- Design: docs/design/valuation.md#Decisions (6, 9)
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/fundamentals/job.toml, internal/jobs/fundamentals/plan.go, internal/jobs/fundamentals/run.go, internal/jobs/fundamentals/run_test.go, docs/contracts/jobs.md, .go-arch-lint.yml

- Goal: `NewFundamentalsFunc` and job `fundamentals` shaped like `fredseries`: plan keys are the `cik:` entries of the watch list; the US window after the SEC publish time; writes through `FactWriter`; an entry with no CIK is skipped with a typed reason; `edinet:` entries are skipped and named while `edinet` is nil (they are fetched by issue 23).

Acceptance:
1. `go test ./internal/jobs/... -race -count=1` passes; running twice adds 0 on a fake writer; a subject with no CIK is skipped and named in the result.
2. `grep -c "NewFundamentalsFunc" internal/jobs/api.go` is at least 1; `go run ./scripts/arch --check` prints `arch: ok`.
3. The diff names exactly the Files paths.

Checked by CI: ci-go; the manifest check of `docs/contracts/jobs.md`.

### valuation-job-step (interface and implementation)

- Module: jobs. Size: M. Order: after 7, 11, 13 and 15 (same `api.go`; the compute funcs and the facts it reads, SIC included, exist).
- Design: docs/design/valuation.md#Decisions (3, 5, 7)
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/computefigures/job.toml, internal/jobs/computefigures/valuation.go, internal/jobs/computefigures/valuation_test.go, docs/contracts/jobs.md

- Goal: `NewComputeFiguresValuationFunc` with `ValuationDeps` (Interface, jobs); with non-nil funcs the job plans one key per entry with an issuer, reads the facts of its issuer, takes the standard from `deps.StandardOf(facts)` and calls `BuildFilings` with it, and writes `valuation.buy_band`, `valuation.question`, `valuation.diagnosis` readings (`subject_kind` `underlying`, subject the entry's contract symbol); a financial or cyclical SIC from the facts gives a `missing_input` band reading with the reason; existing constructors and their plan are unchanged.

Acceptance:
1. `go test ./internal/jobs/computefigures -race -count=1` passes; with nil funcs the plan has no `valuation.` key.
2. `TestStandardFromFacts` (a fake `StandardOf` returning `J_GAAP` reaches `BuildFilings`, and the empty standard gives a `missing_input` reading with `no_standard`). A table test writes three readings for a company and one `missing_input` reading with its reason for a bank whose facts hold SIC 6021.
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### companies-page-band (interface and implementation)

- Module: pages, plus repo (`.go-arch-lint.yml`). Size: M. Order: after 3, 6, 10 and 13 (the register id `valuation.buy_band`).
- Design: docs/design/valuation.md#Decisions (3, 5, 9, 12)
- Note: until section funcs exist (issue 16) the core, diagnosis and questions blocks are `Gap`; issue 16 edits `builder.go` to fill them. The hand price is only displayed, never changed.
- Files: internal/pages/companies_page/builder.go, internal/pages/companies_page/band.go, internal/pages/companies_page/builder_test.go, internal/pages/companies_page/band_test.go, .go-arch-lint.yml

- Goal: `Subject`, `Config`, `OwnerText`, `PriceOverride` (Interface) and `Build` returning v2; rows from `Subjects` (no import of `internal/jobs`); the price line cell reads `valuation.buy_band` (band, `owner override` when given; `awaiting approval` without a `valuation.band_approval` reading from `Config.Read` (`WinnerReadings`) whose value equals the band's 8% value; `expired` when `Today` is more than 90 days after the approval `day`), `unvalued` with the reason text for a `missing_input`, the moat text from `OwnerText`; `Subject.HandPrice` shown beside the band as `hand_price`; every other block a `Gap`; the builder holds no formula.

Acceptance:
1. `go test ./internal/pages/companies_page -race -count=1` passes with `TestBandIsThePriceLine`, `TestOverrideIsLabelled`, `TestExpiredAfter90DaysFromApproval` (fake `Today`), `TestAwaitingApprovalWhenValueDiffers`, `TestBankIsUnvalued`, `TestNilReadersGiveGaps`, `TestHandPriceShownBesideBand`.
2. `grep -c "decimal\|float64\|math\.\|internal/jobs" internal/pages/companies_page/band.go internal/pages/companies_page/builder.go` prints 0 in each; `git diff $(git merge-base HEAD origin/main) -- internal/pages/api.go | grep -c .` prints 0; `go run ./scripts/arch --check` prints `arch: ok`.
3. The diff names exactly the Files paths.

Checked by CI: ci-go; the size rule.

### valuation-docs-arch

- Module: docs, plus repo (`arch.toml`, generated). Size: S. Order: after the merge of events `events-docs-modules` (the same sentence of modules.md, read at cut time); issues 4 to 7, 9 and 19 follow it.
- Design: docs/design/valuation.md#Modules
- Files: docs/design/modules.md, arch.toml

- Goal: add `internal/store/fundamental`, `internal/sources/sec`, `internal/sources/edinet`, `internal/compute/valuation`, `internal/jobs/fundamentals` and `internal/pages/companies_page` to the implementation list of `cmd/ot`, then `go run ./scripts/arch`; no row and no depends cell changes.

Acceptance:
1. `grep -o "internal/store/fundamental\|internal/sources/sec\|internal/sources/edinet\|internal/compute/valuation\|internal/jobs/fundamentals\|internal/pages/companies_page" docs/design/modules.md | sort -u | wc -l` prints 6.
2. `go run ./scripts/arch --check` prints `arch: ok`; `grep -c "sources/sec\|store/fundamental\|jobs/fundamentals\|pages/companies_page" arch.toml` is at least 4.
3. The diff names exactly the Files paths.

Checked by CI: the modules-table check of `crew issue check`; `go run ./scripts/arch && git diff --exit-code arch.toml`.

### compute-filings

- Module: compute, plus repo (`go.mod`, `go.sum`: the new dependency). Size: M. Order: after 6; `params.go` is edited again by 21, after this issue. New direct dependency: `github.com/shopspring/decimal`, MIT, named in the pull request.
- Design: docs/design/valuation.md#Decisions (2, 5, 7)
- Files: internal/compute/valuation/arith.go, internal/compute/valuation/filings.go, internal/compute/valuation/years.go, internal/compute/valuation/params.go, internal/compute/valuation/filings_test.go, internal/compute/valuation/years_test.go, go.mod, go.sum

- Goal: `div` with explicit 34 significant digits, `BuildFilings` (as-of rule, annual forms only, 350 to 380 day years, consecutive, a later 8-K recast replaces the value but not the year's eligibility; the SIC fact read into `FilingsResult`, none giving a refusal reason), refusal reasons of Decision 5. This is the port of the year selection of `filing_rows.py` for the parameters of Decision 1. Concepts and thresholds are read through `ParamsFor(std Standard)` in `params.go`, which holds the US table only (`US_GAAP`) and reports `false` for another standard; `BuildFilings` and issue 13's `BandFunc` call it and name no concept themselves, so issue 21 adds the Japan table without editing them.

Acceptance:
1. `go test ./internal/compute/valuation -race -count=1` passes with `TestAsOfLatestFiledWins`, `TestRecastKeepsYearEligible`, `TestNonConsecutiveRefused`, `TestQuarterlyFactsNeverMakeAYear`, `TestDivKeeps34Digits`, `TestSicRead`, `TestParamsForUsOnly`, `TestStandardOfFromFacts` (source `sec` gives `US_GAAP`; a `dei` `AccountingStandard` fact with unit `IFRS` gives `IFRS`; no fact gives the empty standard and `BuildFilings` refuses `no_standard`).
2. `grep -rc "DivisionPrecision\|float64" internal/compute/valuation/*.go` prints 0 for every file; `grep -c "shopspring/decimal" go.mod` is 1.
3. The diff names exactly the Files paths.

Checked by CI: ci-go (`go mod tidy` leaves no diff).

### compute-golden

- Module: compute. Size: M. Order: after 11.
- Design: docs/design/valuation.md#Decisions (11)
- Files: tools/compute-golden/valuation/pyproject.toml, tools/compute-golden/valuation/uv.lock, tools/compute-golden/valuation/gen.py, internal/compute/valuation/testdata/golden_fcf.json, internal/compute/valuation/testdata/golden_return.json, internal/compute/valuation/testdata/golden_questions.json

- Goal: a dev-only generator, its own uv project, that takes the path of an option-trader checkout at 4704569f as an argument, builds synthetic filings inputs (eight for the band and reverse DCF, ten for return at a price including each refusal and `GROWTH_LIMITED_BY_R`, six for questions and diagnosis) and writes the three JSON files with inputs and Decimal-text outputs. It is not run in CI and nothing imports it; the committed JSON is what tests read.

Acceptance:
1. `git ls-files tools/compute-golden/valuation | grep -c uv.lock` is 1; `grep -c "requires-python" tools/compute-golden/valuation/pyproject.toml` is 1; `python3 -I -m py_compile tools/compute-golden/valuation/gen.py` exits 0 (no package download, no network).
2. `jq length internal/compute/valuation/testdata/golden_fcf.json` is 8, `golden_return.json` at least 10, `golden_questions.json` 6; each file's first key `source_commit` is `4704569f`.
3. `grep -rc "option-trader" internal/compute/valuation/*.go` prints 0 (no test needs the old system).
4. The diff names exactly the Files paths.

Checked by CI: none (the tool is not run in CI).

### compute-fcf-band

- Module: compute, plus docs (`docs/conventions/register.toml`, `docs/GLOSSARY.md`; `internal/compute/rules/rules.json` is a compute path). Size: L. Order: after 12.
- Design: docs/design/valuation.md#Decisions (1, 3, 4, 5, 11)
- Files: internal/compute/valuation/fcf.go, internal/compute/valuation/flags.go, internal/compute/valuation/reverse.go, internal/compute/valuation/fcf_test.go, internal/compute/valuation/reverse_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml, docs/GLOSSARY.md

- Goal: `BandFunc` (thresholds and concepts through `ParamsFor`; value at 6%, 8%, 10%, leverage and range flags; no expiry, the approval day sets it), `ImpliedGrowthFunc` (bisection, 64 iterations over -0.5 to 1), and the entries `valuation.buy_band`, `valuation.band_approval` (an owner fact, no formula; its hash is that of its rule text) and `valuation.implied_growth` with real hashes; the check of Decision 4 on `TestRulesMatchRegister`.

Acceptance:
1. `go test ./internal/compute/valuation ./internal/compute/rules -race -count=1` passes; the golden tests compare every Decimal text exactly.
2. `grep -c "4952b59720ce\|e1e1f731c7b8" docs/conventions/register.toml` is at least 2; `grep -c 'id = "valuation\.' docs/conventions/register.toml` is 3.
3. `go doc ./internal/compute BandFunc | grep -c spot` prints 0 (the band function has no price parameter).
4. The diff names exactly the Files paths.

Checked by CI: ci-go.

### compute-return-at-price

- Module: compute, plus docs. Size: L. Order: after 13 (same register and rules files).
- Design: docs/design/valuation.md#Decisions (1, 4, 10, 11)
- Files: internal/compute/valuation/price_return.go, internal/compute/valuation/paths.go, internal/compute/valuation/gate.go, internal/compute/valuation/price_return_test.go, internal/compute/valuation/gate_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml

- Goal: `ReturnAtPriceFunc` with the four lines, low path, `R`, the spending gate, `PLANT_REVENUE_FELL`, `LATEST_LEVEL_NOT_POSITIVE`, refusals, analyst anchoring and scale check (the analyst input optional); entry `valuation.return_at_price` v1. The result has no value figure and no verdict word.

Acceptance:
1. `go test ./internal/compute/valuation -race -count=1` passes; golden cases cover each refusal, `GROWTH_LIMITED_BY_R` and a nil analyst input giving lines 3 and 4 "left out: no analyst input".
2. `grep -c "8d7be25e35c3" docs/conventions/register.toml` is at least 1; `go test ./internal/compute/rules -race -count=1` passes; `grep -c 'id = "valuation\.' docs/conventions/register.toml` is 4.
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### compute-questions-diagnosis

- Module: compute, plus docs. Size: L. Order: after 14 (same register, rules and glossary files).
- Design: docs/design/valuation.md#Decisions (1, 11, 12)
- Files: internal/compute/valuation/questions.go, internal/compute/valuation/capital.go, internal/compute/valuation/diagnosis.go, internal/compute/valuation/questions_test.go, internal/compute/valuation/diagnosis_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml, docs/GLOSSARY.md

- Goal: `QuestionsFunc` (Q1 return on tangible capital, Q2 growth cost, Q3 pricing power, Q4 cash behind profit; Q5 has no figure) and `DiagnosisFunc` (cyclical margins and inventory; financial ROE and ROA); entries `valuation.question` and `valuation.diagnosis`; a figure that cannot be formed is `missing_input`, never zero.

Acceptance:
1. `go test ./internal/compute/valuation ./internal/compute/rules -race -count=1` passes; golden values equal the six cases.
2. `grep -c 'id = "valuation\.' docs/conventions/register.toml` is 6 (`buy_band`, `band_approval`, `implied_growth`, `return_at_price`, `question`, `diagnosis`).
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### companies-page-sections

- Module: pages. Size: L. Order: after 9, 14 and 15.
- Design: docs/design/valuation.md#Decisions (4, 7, 10, 12)
- Files: internal/pages/companies_page/builder.go, internal/pages/companies_page/core.go, internal/pages/companies_page/questions.go, internal/pages/companies_page/core_test.go, internal/pages/companies_page/questions_test.go

- Goal: `Build` in `builder.go` calls the functions added here to fill the three blocks; `core` (spot from `Config.Spot` and the strike of each shown offer from `Config.Offers` as the cost, then the compute funcs on `BuildFilings` called with the standard from `Config.StandardOf(facts)`; implied growth from the quote; `DGS10` at most 7 days old, a US name only; a JP name shows a rate gap), `diagnosis` and `questions` from readings and owner text; four lines per cost, no rank, no filter, no colour word.

Acceptance:
1. `go test ./internal/pages/companies_page -race -count=1` passes with `TestStandardChosenFromFacts` (a fake `StandardOf` returning `IFRS` reaches `BuildFilings`), `TestCostIsTheBareStrike`, `TestNoAnalystInputLeavesTwoLinesOut`, `TestTreasuryStaleGivesGap`, `TestJpHasRateGap`, `TestRowsKeepCallerOrder`, `TestNilOffersGivesGap`.
2. The page built from `contracts/fixtures/page.companies_page.v2.input.json` equals `page.companies_page.v2.ok.json` byte for byte (test `TestBuildEqualsOkFixture`).
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### companies-page-web

- Module: web. Size: M. Order: after 3 (the generated types).
- Design: docs/design/valuation.md#Decisions (12)
- Files: web/src/pages/companies/CompaniesPage.tsx, web/test/companies_page_v2.test.tsx

- Goal: the page renders the three v2 blocks and the price-line label from the fixtures, computes nothing, and still renders v1.

Acceptance:
1. `pnpm vitest run web/test/companies_page_v2.test.tsx` passes (ok, gap, and a v1 document).
2. `pnpm tsc --noEmit && pnpm eslint web/src/pages/companies` prints no error.
3. The diff names exactly the Files paths.

Checked by CI: ci-web.

### valuation-wire-us

- Module: ot. Size: M. Order: after 4, 5, 7, 8 and 16, and after events `watch-loader-earnings-wire` (the `watch.toml` loader). Second-opinion case: new outside service (the SEC is wired here; the environment name `SEC_USER_AGENT` only, no value).
- Design: docs/design/valuation.md#Interface
- Note: `Held` is nil here (no positions reader on main, Decision 9). The page reads readings through the view reader on the pages DSN, not the readings writer.
- Files: cmd/ot/wire.go, cmd/ot/env.go, cmd/ot/runonce.go, cmd/ot/pagelist.go, cmd/ot/approve.go, cmd/ot/approve_test.go, cmd/ot/main.go, cmd/ot/wire_test.go, docs/contracts/ot.md

- Goal: `newSEC`, the `fundamentals` job in the registry (`NewFundamentalsFunc` with the SEC source and a nil `edinet`; fact store opened with `OpenFactStoreFunc` on `OT_PG_DSN_FUNDAMENTAL`), the valuation step passed to the figures job as `ValuationDeps` (the loader's `WatchEntry` list as `Entries`, and the one real `StandardOf` of compute for both `ValuationDeps` and the page `Config`, so issue 22 needs no edit outside `cmd/ot`), `companies_page` in the page list on the pages DSN with `Read` opened by `OpenReadingViewFunc` and `Facts` by `OpenFactStoreFunc` on that DSN, `Subjects` filled from the loader's `WatchEntry` list, `Held` nil, `Today` from the clock, `Offers` from the `OptionEodReader` and `Spot` from the quote snapshot reader; an unset value leaves the job out and the page on gaps. Also the owner-run `ot approve-band --symbol S [--day D]` (day default today in the zone), which reads the current `valuation.buy_band` for S and writes one `valuation.band_approval` reading (origin `owner_command`, `day` the approval day, value its 8% value, next `rev` of that day); it refuses a symbol with no band.

Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with a test each for unset (job left out) and set (built over a fake source).
2. `go test ./cmd/ot -race -count=1 -run TestCompaniesPageOverFixtureInputs` passes: it loads `contracts/fixtures/page.companies_page.v2.input.json` into fake stores, builds `companies_page` through `newCompaniesPage`, and the document equals `page.companies_page.v2.ok.json` (offline, no `ot` binary run).
3. `grep -rn "SEC_USER_AGENT" cmd/ot | grep -c "= *\""` prints 0 (names only).
4. `go test ./cmd/ot -race -count=1` passes with `TestApproveBandWritesApproval` and `TestApproveBandRefusesNoBand` (fake store and clock); `go run ./scripts/arch --check` prints `arch: ok`.
5. The diff names exactly the Files paths.

Checked by CI: ci-go; `go run ./scripts/arch --check`. Not verified offline: a real band for a real symbol; the owner runs `ot run-once` for one day and reads the page.

### edinet-client

- Module: sources, plus repo (`.go-arch-lint.yml`). Size: M. Order: after 18 and 10. Second-opinion case: new outside service (`api.edinet-fsa.go.jp`, API key from the environment variable `EDINET_API_KEY`).
- Design: docs/design/valuation.md#Decisions (8)
- Files: internal/sources/edinet/client.go, internal/sources/edinet/cursor.go, internal/sources/edinet/client_test.go, internal/sources/edinet/testdata/documents_list.json, .go-arch-lint.yml

- Goal: `edinet.New`: list a day's documents (annual securities reports, form code 030000 and amendments), download the XBRL CSV of a document id, return rows with document id, filing day and `raw_sha256`; the key goes in the query and never in an error or a log line; day chunks driven through the `Cursor` interface of issue 5 (a fake in the test), so a backfill scans the days after the last one saved; no file or database access here.

Acceptance:
1. `go test ./internal/sources/edinet -race -count=1` passes with `TestListKeepsAnnualReportsOnly`, `TestDownloadReturnsDocID`, `TestKeyNeverInErrorText`, `TestCursorResumes`.
2. `grep -c "time.Sleep\|http.Get\|edinet-fsa.go.jp" internal/sources/edinet/client_test.go` prints 0; `go run ./scripts/arch --check` prints `arch: ok`.
3. The diff names exactly the Files paths.

Checked by CI: ci-go. Not verified offline: the first real fetch; the recorded files follow the API's published format and the real CSV (UTF-16, tab separated) may differ, so the owner's first real day is the check.

### edinet-mapping

- Module: sources. Size: L. Order: after 19.
- Design: docs/design/valuation.md#Decisions (1, 8)
- Files: internal/sources/edinet/mapping.go, internal/sources/edinet/concepts.go, internal/sources/edinet/mapping_test.go, internal/sources/edinet/testdata/jgaap_rows.csv, internal/sources/edinet/testdata/ifrs_rows.csv

- Goal: map J-GAAP and IFRS elements to the concept names of Decision 6 (unit JPY, `subject` the `edinet:<code>` issuer text); a table lists what each standard does not report (J-GAAP: diluted shares, stock compensation). Each filing also gets one fact that names its standard (taxonomy `dei`, concept `AccountingStandard`, unit `J_GAAP` or `IFRS`, value `1`) for `StandardOf`. The Japan parameter file and its loading are issue 21, in compute.

Acceptance:
1. `go test ./internal/sources/edinet -race -count=1` passes with `TestJGaapMapsToSameConcepts`, `TestIfrsMapsToSameConcepts`, `TestJGaapMissingDilutedSharesIsNoFact`, `TestCompetingElementsRefused`, `TestStandardFactWritten` (a J-GAAP and an IFRS filing each give one `AccountingStandard` fact with the unit of its standard).
2. `wc -l internal/sources/edinet/*.go` shows every file under 600.
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### valuation-compute-jp

- Module: compute, plus docs (`docs/conventions/register.toml`). Size: M. Order: after 11 (creates `params.go`), 15 (same register and rules files) and 20 (the concept names).
- Design: docs/design/valuation.md#Decisions (1, 8, 11)
- Files: internal/compute/valuation/params_jp.json, internal/compute/valuation/params.go, internal/compute/valuation/params_jp_test.go, internal/compute/rules/rules.json, docs/conventions/register.toml

- Goal: the Japan parameter file `params_jp.json` (the concept names of Decision 6, JPY, the thresholds of the US file; new, with no option-trader blob) and `params.go` (created by issue 11) gaining the Japan table so `ParamsFor` answers `J_GAAP` and `IFRS` and `BuildFilings` and `BandFunc` handle them without an edit: a complete IFRS input gives a band, a J-GAAP input missing diluted shares or stock compensation gives `missing_input` with the reason (ADR 0008 rejects an incomplete input); a `history` line on `valuation.buy_band` names `params_jp.json` as new, its `source_hash` and the rules entry follow the file.

Acceptance:
1. `go test ./internal/compute/valuation ./internal/compute/rules -race -count=1` passes with `TestIfrsGivesBand`, `TestJgaapIncompleteIsMissingInput`, `TestParamsJpLoaded`.
2. `grep -c "params_jp" docs/conventions/register.toml` is at least 1; `grep -c 'id = "valuation\.' docs/conventions/register.toml` is still 6.
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### fundamentals-edinet

- Module: jobs. Size: S. Order: after 7 (same `plan.go` and `run.go`) and 20 (the `edinet:` facts exist).
- Design: docs/design/valuation.md#Decisions (8, 9)
- Files: internal/jobs/fundamentals/plan.go, internal/jobs/fundamentals/run.go, internal/jobs/fundamentals/edinet_test.go

- Goal: the `fundamentals` job also plans and fetches the `edinet:` entries of the watch list through the `edinet` `FactSource` (the second argument of `NewFundamentalsFunc`) (a window of days after the filing season, written through `FactWriter`); an entry with no EDINET code is skipped with a typed reason; with no EDINET source given the `edinet:` entries are skipped and named. `internal/jobs/api.go` is unchanged: `NewFundamentalsFunc` already takes both sources (issue 7), the writer and the entries; issue 22 passes the EDINET source and issue 18 passes nil.

Acceptance:
1. `go test ./internal/jobs/fundamentals -race -count=1` passes with `TestPlansEdinetEntries`, `TestEdinetSkippedWithoutSource` (a nil `edinet` argument skips and names the entries, the `cik:` entries still run), `TestRunTwiceAddsZero`.
2. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep -c .` prints 0.
3. The diff names exactly the Files paths.

Checked by CI: ci-go.

### valuation-wire-jp

- Module: ot. Size: M. Order: after 21 and 23 (20 and 19 through them); `cmd/ot/wire.go`, `cmd/ot/main.go` and `docs/contracts/ot.md` are edited after issue 18. Second-opinion case: new outside service (`EDINET_API_KEY` name only).
- Design: docs/design/valuation.md#Decisions (8, 9)
- Files: cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/main.go, cmd/ot/backfill_edinet.go, cmd/ot/backfill_edinet_test.go, docs/contracts/ot.md

- Goal: `newEDINET` passed as the `edinet` argument of `NewFundamentalsFunc` (issue 18 passed nil) and the Japan subjects (`edinet:` entries) into the `fundamentals` job and the valuation step (the accounting standard is chosen by `StandardOf` from the facts, so this issue edits nothing outside `cmd/ot`); a missing key leaves Japan out, the US unchanged. The file cursor `$OT_HOME/edinet.cursor` (the `Cursor` of Decision 8) and `ot backfill-edinet --from D` (registered in `main.go`): it scans days from the cursor to today through the source in chunks, writes through the fact store and saves the cursor after each chunk.

Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with an unset and a set case.
2. `go test ./cmd/ot -race -count=1 -run 'TestBackfillEdinet'` passes with `TestBackfillEdinetResumesFromCursor` and `TestBackfillEdinetSavesCursorPerChunk` (fake source, fake writer, cursor file in `t.TempDir()`, fake clock).
3. `grep -rn "EDINET_API_KEY" cmd/ot | grep -c "= *\""` prints 0.
4. The diff names exactly the Files paths.

Checked by CI: ci-go.

## Left out

- The largest strike figure and the filter of offers by it, net cost (strike minus premium), ledger and tracking: the later candidates design, because the owner decides there whether the band caps strikes (Decision 3). The owner text store, the override store and the price-lines page post: each is its own small store with no figure in it, so the page reads them through optional interfaces. The ADR 0008 approval flow (Policy, notice, batches): not ported, ADR 0008:25 stays and `ot approve-band` is a stored acknowledgement, not a Policy.
- `floor_dcf` v1 (owner 2026-11-09: band only). The sale price (ADR 0019): it belongs to the exit side of the loop, which no owner decision has asked for. The fundamental history and implied-growth record tables: the page recomputes from the stored facts, and a record table would only cache a figure the page does not need. Valuation drift: it needs two stored bands to compare and the owner has not asked for a drift signal.
- FMP and analyst estimates (Decision 10; owner 2026-11-09: the analyst line stays left out). A Japanese 10-year yield source. The producers of `core_page` and `diagnosis_page`: the one `companies_page` v2 carries both blocks, so a second producer would show the same figures twice.
- Banks, insurers, cyclicals: diagnosis only, no method.
- Alternatives not chosen: `math/big.Rat` first (a mainstream library is preferred by AGENTS.md, `Rat` stays the fallback behind `div`); a normalised fundamentals dataset (ADR 0008 wants the raw filed fact, and a later recast must not hide an eligible year); dropping the five questions (the owner wants them on the page); a dataset of its own for the approval (a one-value enum extension of the readings dataset is smaller and the page already reads readings).

## Owner answers (2026-11-09)

Both earlier questions are answered; the recommended options were chosen.

1. Band: lower edge the value at a 10% yield, top the value at 8% (primary per ADR 0008), 6% only a reference. The 90 days count from the owner's approval day (ADR 0008), not from the data day (Decision 3, issues 2, 9, 13, 18).
2. System price: `fcf_yield` band only; `floor_dcf` is not ported now (Left out).
3. The analyst line stays left out (Decision 10).

## Adversarial

Review file: dispatcher scratchpad `valuation-adversarial.md` (not in the repository). Verdict on revision 1: goal not met. Answers:

1. Gap, no holdings or subject list or symbol-to-CIK mapping: adopted (Decision 9: the shared `watch.toml` of event-sources owns the list; the held view is an optional page input).
2. No web rendering: adopted (issue 15). No max strike: adopted as stated in Goal and Left out (candidates design). Price as input of the band: adopted (`BandFunc` has no price; implied growth solved at page build, Decision 4).
3. JP: adopted. No claim of old EDINET code (Decisions, main today); US-first order (16 before 17); J-GAAP names ending unvalued stated; JP rate a gap with a candidate source; a Japan parameter file in 18; backfill by day cursor (17).
4. Expiry start, ADR 0008:25 and 0008:14, `floor_dcf`: adopted as plain statements (Decision 3, no Policy created) superseded by the owner's answers of 2026-11-09: the start is approval and `floor_dcf` is not ported.
5. Facts to Filings has no owner: adopted (Decision 7, issue 5; the store returns all rows filed by the cutoff).
6. Simpler cut: adopted, merging store interface and implementation, sources interface into sec-source, page interface into band, function types instead of one interface, US first. Not adopted: dropping questions and diagnosis (the owner wants them on the page), and a normalised dataset (ADR 0008 wants the raw fact).
7. Decimal: adopted (explicit 34 digits via `div`, no global, `Rat` fallback, dependency added in `compute-filings`). Not adopted: `math/big.Rat` first, since AGENTS prefers a mainstream library.
8. Alpha Vantage claim false: adopted (FMP, Decision 10). Cost strike minus premium: adopted, bare strike kept (Decision 4). Band 8% and 10%: confirmed by owner 2026-11-09.
9. Concept names and the cover-page share count: adopted (Decision 6). `rules.json` with a page-time figure: adopted as a check in issue 7. Web path in contracts: adopted (the contracts row owns `web/src/generated/**`).
10. Goldens needing the old system: adopted (issue 6, generator under `tools/compute-golden/`, own uv project, committed JSON). Test files over 600 lines: adopted by splitting files per issue. Migration number: adopted ("next free at cut time", no number in Files). Same-file overlap with account-data and events: adopted (Order paragraph in Modules). Register, rules and glossary serial edits: adopted (issues 7, 8, 9 in order).
11. Not adopted: a hand-written EDINET sample being the only offline check; it cannot be avoided, and the design says the first real day is the check.
12. Design verdict on revision 3 (head 3dc5c66): the pages row may not import jobs, so a page-local `Subject` (Decision 9, Interface); modules.md and `arch.toml` edits in issue 10 with lint lines in the package-creating issues; the fact store gets `OpenFactStoreFunc`, `DSNEnvFundamental`, the role and the setup files; the dataset gets `subject` and a non-null key encoding; SIC fetched from `submissions` (issue 5); `generate.sh` and the alias in issue 1; the manifest chain 1, 2, 3; the approval stored as a reading with a new origin (issue 2); six register entries; `params_jp.json` loaded by issue 21; notes on Config types, order cells, Left out reasons and offline acceptance applied.
13. Design verdict on revision 4 (head 4604159): issue 16 adds `builder.go` (Modules sentence); issue 23 for the EDINET job and the backfill command with the cursor in a file behind `Cursor` (Decision 8, issues 5, 22); the figures constructor type given in full with `ValuationDeps` and the entries; the page takes `Spot` instead of a quote reader; `Held` nil in issue 18; `ParamsFor` by standard in issue 11 so issue 21 only adds a table; the page reads readings through `ReadingViewReader` on the pages DSN; `QuestionsFunc` and `DiagnosisFunc` signatures; third-party env names `SEC_USER_AGENT` and `EDINET_API_KEY` without `OT_`; the owner's hand-entered `buy_price` stays beside the approved band.
