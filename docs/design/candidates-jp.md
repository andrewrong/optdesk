# candidates-jp: Japanese sell-put candidates in yen, on the existing candidates page, after the buy-price source

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19
Revision: 2

Owner ask: 2026-11-19, Japan market open: "Are there sell-put recommendations for Japan? Japan was not done right?" Standing decision: `option-candidates.md` Decision 14 (Japan next, not dropped); the owner has the JP market data subscription. Owner rule (2026-11-19, ask 5117): the buy prices of JP names come from his valuation imports, so no buy price or name list is asked of him here.
Read on origin/main (e93324c9): `AGENTS.md`, `docs/design/option-candidates.md` (Decisions 5, 14, 15), `candidates-at-open.md`, `option-open-pull.md` (Decision 21), `option-capture-jp-stocks.md`, `docs/design/modules.md`, `internal/jobs/optioncandidates/` (`job.go`, `job.toml`, `source.go`, `input.go`, `types.go`), `internal/jobs/optionopen/job_pull_jp.toml`, `internal/jobs/optioneod/job_jp.toml`, `internal/compute/optioncand/` (`put.go`, `optioncand.go`), `internal/compute/optiontier/` (`tier.go`, `badges.go`, `cash.go`), `internal/compute/api.go`, `internal/store/api.go`, `internal/store/candidate/` (`read.go`, `store.go`), `internal/pages/option_candidates_page/` (`groups.go`, `builder.go`, `format.go`, `strings.go`, `zh_texts.go`), `cmd/ot/` (`candidates.go`, `jobreg.go`, `data_health_site.go`, `jobconsole_jobs.go`), `contracts/datasets/candidates.option_candidate.r1.json`, `contracts/schemas/page.option_candidates_page.v1.json`, `watch.toml` (JP entries). Read on origin/main after the merge of pull request 3112: `docs/design/buy-price-source.md` (Decisions 2, 3, 4, 6, 9; Issues 3, 5, 6, 7, 8, 10).
Measured: the probe of 2026-11-19 (the dispatcher scratch file `optdesk-drafts/designs/candidates-jp/probe-2026-11-19.md`, runner home, not in the repository; the scheduler log of 2026-11-11 to 2026-11-19, no database and no gateway probe). Its numbers used here are written out below.

## Goal

On a Tokyo session day D the candidates page shows sell-put candidates for the JP watch names next to the US ones, from the open pull of D, about 10 minutes after the TSE open (09:10 JST, 08:10 local), in yen, with the strike times the multiplier as the cash a put needs, with the buy price the owner's valuation gives, and with every JP row marked where a rule has no JP input. Nothing the US page shows today changes, and a JP run never hides a US run or the other way round.

## Today (why there is no Japan row)

The data is there, the code is not. Four things stop a JP row, all in code:

- `job.go:133-143` `split` sends every JP entry to a skipped text; the job has no JP key and writes no row.
- The job is a New York job (`job.toml` XNYS; `input.go:17`), and `source.go` `begin` keeps only open rows with currency USD.
- The rule is a dollar rule: `put.go:11` `cashCurrency = "USD"` (a yen put against dollar funds would be `no_fx`), `cashRateGate` compares with DGS3MO, the page prints "$" (`format.go` `moneyText`), and the tier rule reads cash as USD (`optiontier/cash.go`).
- The stored row has no currency, and the page and `candidate/read.go` read only the newest run of a day and kind, so a JP run and a US run of the same date would replace each other.

Data (probe): 8 JP watch entries, 7 with options; `option_open_jp` quotes about 155 contracts a day at 09:05 JST, on 11-19 112 ok and 34 partial; four large names are about 87 percent ok, three are thin (29 of 53 partial on 11-19, one pull key failed on its share rule); `option_eod_jp` has rows since 11-11. No JP entry has a hand buy price; the valuation imports give the price (Decision 2). The data supports candidates for the four large names now and partly for the three thin ones, so no data entry comes first.

## Decisions

### 1 One job for Japan, run by run-once first

1. A new job `option_candidates_jp` in the package `internal/jobs/optioncandidates` (a second manifest `job_jp.toml` and the constructor `NewJP`; puts only), built over the JP entries: market JP, sec_type STK, an issuer, and a `Contract.Symbol` that is the code. The US job keeps its manifest, plan and tests. Reason: calendar, zone, upstream and currency differ; a branch inside one job would make every US test and the US plan hash depend on JP names (`Plan` hashes the issuer list). Rejected: one job with a market switch (`## Left out`).
2. Schedule (`job_jp.toml`): calendar `XTKS`, zone `Asia/Tokyo`, anchor `session_open`, offset `5m`, `After()` = `option_open_jp`, `run_bound` 10 minutes. It starts when `option_open_jp` ends for D (dependent run): on 11-19 the pull ended 09:08 JST. A day on which a key of the pull failed (11-19 batch-002) starts with `upstream_missing`; the rerun rule of `candidates-at-open.md` Decision 12 reruns it when the last key is ok. A scheduler restart kills a running job (32 restarts on 11-19); the dependent-run rule starts it again, as for the US job.
3. One key per JP entry with an issuer (`Plan`; the 8 names, as the US job keys every US entry). The job reads no list of another jobs package: the plan of `option_open_jp` keeps a name of `no_option` in `job_plan_jp.toml` as the underlying only, so such a name has no `option_open` row of D at all. A name with no `JPY` open row of D is named in `Skipped()` with "no option chain" and has no row; a name with open rows that are all unusable is named with "no usable open quote" (Decision 5). The calendar is `XTKS` for the schedule and for `sessionDays`: on a day that is not an XTKS session the job has no run.
4. Order of use: the first checked run is `ot run-once --job option_candidates_jp` (the window runs it, owner decision 2026-11-09 on `ot` commands), with the family registered but held in the scheduler (`startOff` in the registry, `cmd/ot/jobreg.go`); the last entry removes the hold. So the only change that can alter a scheduled day is the last one.
5. The eod fallback of the US job (the previous session's `option_eod` chain for a name with no usable open put, mark `eod_fallback`) is not used for Japan in this version: a name with no usable open put is named in `Skipped()` ("no usable open quote") and gets no row. Reason: the owner reads about the open quotes at 09:10; a day-old chain shown there is a second source and a second mark text for no new result, and the JP eod rows of D are not there before 10:30 JST. The manifest gets one key, `[source] eod_fallback` (true in `job.toml`, false in `job_jp.toml`), so the US job is unchanged. Open interest still comes from the newest `option_eod` row of the same instrument within `oi_lookback_sessions` (3) XTKS sessions, marked `oi_prior_session`, as `candidates-at-open.md` Decision 8.

### 2 Buy price: after buy-price-source

6. The JP gate entries are attached after `docs/design/buy-price-source.md` (merged, pull request 3112; this and Decision 7 rest on its merged text): JP rows carry the resolved buy price, the top of the valuation buy zone, else the hand price, else not judged (`no_buy_price`, grey tier). No new price rule is made here. The JP job is built through `f.listed(groupOptions, ...)` like the US job, so `jobs.WatchEntry.BuyPrice` and `BuySource` reach it from the `prices` step of that design (its entries 3, 5, 6 and 7).
7. Currency check, Decision 2 of buy-price-source: a zone counts only when the profile currency equals "the currency given for the company", which the job takes from `Contract.Currency`; for a JP entry that is `JPY`. A JP company the owner valued in JPY therefore meets the rule and gets its zone; a JPY entry with a profile in another currency gets the reason `currency` and falls to the hand price, then to not judged. No change to that design is needed for the job and the gate. Its strip is different: Decision 2 passes `USD` for the strip (every row was a US contract), so the strip of a JP company would read `currency`; this design's page entry passes the row currency of the group (Decision 25), after entry 8 of that design (merged text: `valueFigure` calls the injected `Resolver` with `USD`).
8. Collection follows the resolved price as for every option family (buy-price-source Decision 3): `option_open_jp` ranks a name with a price first; `option_eod_jp` reads no price (its strike band is the spot band today) and is not changed here.
9. Cross-design waits are sentences, not `Order:` lines: the entries 5 and 7 below are attached after buy-price-source entries 3, 5, 6 and 7 merge (they edit `job.go`, `source.go` and `cmd/ot` files of the same packages), entry 6 after its entry 8 (the strip, `page.go` `Config.Buy` and `valueline.go`), and entry 7 after its entry 10 (`candidatesConfig` in `evidence_site.go` sets `Buy`, the resolver; this design adds only the JP reader there). Until then no JP row has a price and every row is marked `no_buy_price`.

### 3 Source and limits

10. The put chain of a name is the `option_open` rows of D with currency `JPY` (same reader and usable rule as `candidates-at-open.md` Decision 1: status ok, or partial with a bid and an ask; DTE 21 to 45; status 7 and 4 dropped). `source.go` `begin` takes the currency from the manifest (`[source] currency`, "USD" in `job.toml`, "JPY" in `job_jp.toml`) instead of the literal; `sessionDays` takes the calendar id from the manifest (`input.go:17`).
11. Limits are trial values in the JP job's own `[params]` in `job_jp.toml`, not tuned: DTE 21 to 45 (the JP plan holds 14 to 90, so every JP expiry in the window is planned), `max_spread` 10 percent of mid, `min_bid_size` 1, `min_open_interest` 10 (the US numbers, copied, each marked trial). `Params()` (`strategy_wire.go:51`) keeps reading the US manifest; the JP params are read by `NewJP` from its own manifest, so no US caller changes.
12. The first runs show per limit how many JP rows it removed, so zero rows never reads as "no candidates in Japan". Two places: (a) the JP group of the page has a field `limit_counts`, a list of `{limit, count, text}` filled from the stored blocked rows of the run, one entry per reason `wide_spread`, `low_open_interest`, `small_bid_size`, `no_bid`, `no_quote`, with a Chinese sentence for each ("价差超限，剔除 N 行"); (b) the job logs one line per name with the counts that are not stored: contracts outside the 21-45 window, rows with an unusable quote, and the same gate counts. The page builder counts stored rows; the web code computes nothing.

### 4 Rule in yen: marks, status and tier

13. The rule gets the currency of the run through a new interface file, `internal/compute/optioncandapi/api.go` (Decision 9): `CurrencyInput` (the `compute.CandidateInput` and a `Currency` text; empty is USD) and `CurrencyRule` with `EvaluateIn(CurrencyInput, compute.CandidateParams) []compute.Candidate`, and the constructor type `NewCurrencyRuleFunc func() CurrencyRule`. `optioncand.NewCurrency` (new, `currency.go`) is the implementation and has that type, as `optioncand.New` has the type of `Evaluate`'s rule today; `NewJP` takes the `CurrencyRule` as `New` takes `compute.OptionCandidates`, and the wiring of entry 7 gets it from a new `deps` field `newCurrencyRule NewCurrencyRuleFunc` that the default of `wire.go` fills with `optioncand.NewCurrency` (next to `optioncand.New`, line 256). The stored row currency is set by the job from the run, so `compute.Candidate` is not changed. `Evaluate` and every US caller and golden file are unchanged.
14. With currency JPY (`internal/compute/optioncand`, new file `currency.go`, small edits in `put.go`):
    - Funding gate: with an account in JPY, strike times multiplier is compared with its yen funds (`fit` / `no_fit` as US). With accounts but none in JPY, the gate passes and the row carries the mark `no_jpy_funds`; the per-alias `Fits` keeps `unknown` with reason `no_fx` for a USD alias. This is a pass by mark until the window names a yen account (window decision, not an owner question). With no account at all the gate is as today.
    - Cash-rate gate: no yen short rate is stored (no new source here), so the gate passes and the row carries the mark `no_jpy_rate`. The yield is `bid / strike x 365 / dte`, a ratio, so the currency does not enter.
    - Buy-price gate, spread, bid size and open interest gates: unchanged.
15. A row that passed a gate by a mark is not equal to a row that passed by a check, and it never shows green or the top tier (standing rule: green only from conditions the owner set). Stored status stays `ok` (the rule's statuses are not changed; `no_buy_price` already works this way). The tier is decided in `optiontier`: the marks `no_jpy_rate` and `no_jpy_funds` are look-at badges (`badges.go` `storedLookAt`), so a JP row is tier `check` at best, never `fits`; with no buy price it is `grey` (not judged) as a US row. `optiontierapi.Row` gets `Currency` (the file is 104 lines); a JPY row against a USD alias has the cash cell `unknown` with the existing reason `other_currency`, not a USD comparison. Consequence, said here so nobody finds it later: while no yen cash-rate source exists the mark `no_jpy_rate` stays, so no JP row reaches `fits`; the yen rate is a separate design (`## Left out`) and is what lifts it.
16. Mark hints are Chinese texts in `zh_texts.go`, each carried in the badge text of the Marks cell (`cells.go` `marksCell`, a `ValueCell` with no hint field, as every existing mark hint is) of the page document, none in the web code:
    - `no_jpy_rate`: "日元没有现金利率，年化收益未与现金利率比较；本行最高只到“需留意”，不会显示为“合适”"
    - `no_jpy_funds`: "没有日元账户，资金检查按通过处理，不代表日元资金够用；指定日元账户后才真正检查"

### 5 Multiplier and tick

17. I looked for a read-only contract-details read and did not find one that is clearly available: the pyside read rpcs (`ListChains`, `ResolveContract`, `ListContracts`) carry the multiplier of a chain but no field of the contract proto holds a minimum tick, the live suite (`pyside/tests/live/test_live_jp_stock_options.py`) asserts the chain and does not print the multiplier, and its socket choice (main or wife) is not fixed by a command, so no probe was made (neither port 4001 nor 4002). Both values are therefore trial values here: multiplier 100 (`internal/jobs/optioneod/job_jp.toml` line 53; the collector keeps only a chain whose multiplier reads "100", `option-capture-jp-stocks.md` Decision 4, so stored JP terms are those of a 100 chain) and tick 1 yen (`job_jp.toml` line 17, "the tick table is not recorded"). The candidates rule does not read a tick (strikes come from the chain).
18. The page prints the multiplier used beside the cash-secured amount of a JP row ("¥1,790,000 = ¥17,900 x 100"), and the first run log line names the multiplier of the stored terms. The check is a step after the merge of the page entry, run by the window: read the multiplier of the 7 names from a `ListChains` read on the main gateway (port 4001 only) and write the day and values into this design's follow-up; a value other than 100 stops the schedule entry.

### 6 Store: currency column, legacy reader stays USD

19. The stored row gets a nullable `currency` column in `candidates.option_candidate` and its view `rd.option_candidate_r1` (appended, as migration 0027 appended `settings_version`); null reads as USD.
20. `Candidates(ctx, day, kind)` (`store.CandidateReader`) stays USD-only and its meaning is unchanged: it returns the newest run of the day among rows with currency null or USD (one added clause in `read.go`, the statements `columns`, `insertSQL` and `scanRow` unchanged). Its users stay USD-only and unchanged: `cmd/ot/candidates.go:179`, `cmd/ot/live_quotes.go:509`, `cmd/ot/option_page_site.go:117`, `internal/pages/option_boards/load.go:36`, `internal/pages/option_notices_page/inputs.go:74`, `internal/pages/option_tracking_page/context.go:374`, `internal/store/strategyresultpg/candidates.go` (`:51`, `:58`, `:146`, which wraps the same reader) and the page's own US groups (`groups.go:77`).
21. The per-currency read and write are new methods in a new interface package `internal/store/candidatecur` (Decision 9): `Reader.CandidatesIn(ctx, day, kind, currency)` returns the newest run of that currency; `Writer.InsertCandidatesIn(ctx, rows []Row)` with `Row` = `store.Candidate` and `Currency`. Rank is per run and runs are per currency, so two rank sequences never mix in one result. `store.Candidate` and `internal/store/api.go` are not changed.

### 7 Page

22. The page document gets one group per kind and currency: the JP group is a second group of `kind` `short_put` after the US short-put group, with the new optional fields `currency` ("JPY"; null for the US groups), `note` (the one-line Chinese note below) and `limit_counts` (Decision 12). `groups.go` `groupSpecs` stays per kind; `readKind` gets a currency (empty reads `Candidates`, "JPY" reads `CandidatesIn`), and `Config` gets one new field for the `candidatecur.Reader` (nil gives no JP group and the old page). The reader is handed to it by entry 7 (Decision 30). A group has its own `day` and `stale`.
23. The JP group looks back from the date of the build time in `Asia/Tokyo` (not the US build day): at 09:10 JST on D the US build day can still be D-1 and the JP rows of D would not be found.
24. The tier view and the short list of the JP rows are separate optional properties `short_put_tiers_jpy` and `short_put_list_jpy` built from the JP group by the same code as the US ones; `short_put_tiers` and `short_put_list` stay US. The evidence columns (margin, one-sigma move, fair value; `evcells.go`) use DGS3MO and a USD margin rule, so for JP rows they are not computed: the cells say so with a Chinese reason, never a US number.
25. Every `moneyText` user prints in the row currency: `moneyText(text, currency)` ("$1,105.75" for USD or empty, "¥8,950" for JPY, no decimals); the callers are `format.go`, `evcells.go`, `shortlist.go`, `shortlistrows.go`, `tiers.go`, `tierrows.go` and `valueline.go`. The strip resolver call passes the group currency (Decision 7).
26. The note of the JP group: "日本：东证开盘行情，金额为日元，1 张合约 = 100 股（乘数以合约资料为准）". `jpNote` ("Japanese option candidates: later design (option-candidates-jp)", `strings.go:7`) is replaced: the page-level note no longer says Japan is later. `ot candidates` (`candidates.go:43`) stops saying "no option route" for a JP name and points to the page.

30. Wiring of the page (entry 7): `cmd/ot/site.go` (`siteReaders` gets `candCur candidatecur.Reader`, set where `rd.cands` is opened, from the same store by an interface assertion, nil when the store is not one), `cmd/ot/candidates_site.go` (`buildCandidates` copies it into the `candidateReaders` it builds) and `cmd/ot/evidence_site.go` (`candidateReaders` gets `CandCur`, and `candidatesConfig` sets the page `Config` field from it). The demo profile and the named path (`ot page`) leave it nil.

### 8 Sizes, interface files, modules

27. `internal/compute/api.go` is 839 of 840, `internal/store/api.go` is 800, `cmd/ot/wire.go` is 782 (counted on e93324c9). No ceiling is raised and no file is split: new types go into new interface packages (`internal/compute/optioncandapi`, `internal/store/candidatecur`, each `api.go` declarations only with `api_test.go` testing both sides), `optiontierapi/api.go` (104 lines) takes one field, the registration is a new file (`cmd/ot/wire_optioncandidates_jp.go`, `init()` with `registerJob`, order 183), and `wire.go` gets at most 3 lines (one `deps` field and its default for the test seam, 785 at most). New source files stay under 600 lines.
28. `docs/design/modules.md`: the compute row entry cell gains `internal/compute/optioncandapi` and the store row entry cell gains `internal/store/candidatecur` (done in the design pull request); every other path matches a row already. Another design pull request also edits the compute row (buy-price-source adds `buypriceapi`): the later one is rebased before the owner question.
29. `docs/design/option-candidates.md` Decision 14 names `option-candidates-jp` and a probe still to come. The design pull request adds one sentence to Decision 14: this design (`docs/design/candidates-jp.md`) is that design, its probe is done, and it replaces the name in Decision 14, the table row and the Left out line.

## Expected day (JST; local time is one hour earlier)

| time JST (local) | what |
|---|---|
| 08:15 (07:15) | `option_open_plan_jp` (slot is open minus 45 minutes), list and resolve unchanged |
| 09:00 | TSE open |
| 09:05 | `option_open_jp`, ends about 09:08 |
| about 09:09 (08:09) | `option_candidates_jp` starts at once, about one minute |
| 09:10 (08:10) | JP group on the page, 10 minutes after the open |
| 10:30 | `option_eod_jp` (unchanged) |

## Modules

Rows used: contracts (entry 1), store (entry 2), compute (entries 3, 4), jobs (entry 5), pages (entry 6), ot (entries 7, 9), web (entry 8). `modules.md` changes as Decision 28 says. The jobs package imports no new jobs package. A migration is needed (entry 2): the merge holds the auto-deploy and the window runs migrate after it (owner decision 2026-11-09); the schedule entry is last, so no scheduled day runs over a store without the column.

## Interface

Shared artifacts, each marked (interface) in `## Issues`:
- Dataset `candidates.option_candidate.r1` gains the optional field `currency` (entry 1).
- Page schema `page.option_candidates_page.v1` gains the optional group fields `currency`, `note`, `limit_counts` and the optional properties `short_put_tiers_jpy`, `short_put_list_jpy` (entry 1); additive, `schema_version` stays 1.
- `internal/store/candidatecur/api.go` (entry 2): `Row`, `Reader`, `Writer`.
- `internal/compute/optioncandapi/api.go` (entry 3): `CurrencyInput`, `CurrencyRule`, `NewCurrencyRuleFunc`.
- `internal/compute/optiontierapi/api.go` (entry 4): `Row.Currency`.
`internal/jobs/api.go`, `internal/store/api.go` and `internal/compute/api.go` are not edited.

## Issues

All entries: `Design: docs/design/candidates-jp.md#<decision>`. Cross-design waits are in Decision 9. Every path below exists on origin/main or is marked (new).

### 1 candidates-currency-contract (interface)

Module: contracts. Size: S. Order: first. Contracts: `candidates.option_candidate r1 additive; page.option_candidates_page v1 additive`. Design: Decisions 19, 22, 24.
Files: `contracts/datasets/candidates.option_candidate.r1.json`, `contracts/fixtures/candidates.option_candidate.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `contracts/schemas/page.option_candidates_page.v1.json`, `docs/contracts/datasets/candidates.option_candidate.md`, `docs/contracts/page-schemas.md`, `internal/contracts/option_candidates_page_v1_test.go`, `internal/contracts/gen/records.go`, `web/src/generated/option_candidates_page.v1.ts`
- The dataset has an optional `currency` (three upper-case letters or null); the fixture holds one USD row without it and one JPY row with it, and the new hash of the fixture is in the manifest.
- `Group` has optional `currency`, `note` and `limit_counts` (`limit`, `count`, `text`); the top level has the optional `short_put_tiers_jpy` and `short_put_list_jpy` with the shapes of `short_put_tiers` and `short_put_list`; none is in `required`, `schema_version` is still 1.
- A document without the new fields validates and a document with them validates (tests `TestJPGroupValid` (new), `TestNoJPFieldsStillValid` (new)); an extra key of `limit_counts` fails.
- The generated Go and TypeScript are produced from the schema, never edited by hand.

### 2 candidates-currency-store (interface)

Module: store. Size: M. Order: after 1. Design: Decisions 19 to 21.
Files: `migrations/0061_candidate_currency.sql` (new; the next free number when the entry is cut), `internal/store/candidatecur/api.go` (new), `internal/store/candidatecur/api_test.go` (new), `internal/store/candidate/currency.go` (new), `internal/store/candidate/read.go`, `internal/store/candidate/currency_test.go` (new), `internal/store/candidate/store_test.go`, `internal/store/candidate/store_pg_test.go`, `docs/contracts/store/candidatecur.md` (new)
- The migration adds a nullable `currency` and rebuilds the view with `currency` appended; existing rows keep their values; `columns`, `insertSQL` and `scanRow` in `store.go` and `internal/store/api.go` are not in the diff.
- `Candidates` returns only rows with currency null or USD: a day holding a US run and a JP run returns the US run (test with a fake db); its existing tests pass unedited.
- `CandidatesIn(day, kind, "JPY")` returns the newest JPY run; a day with a US run at 22:58 and a JP run at 08:10 returns each from its own call; two JP runs return the newer one.
- A row written by `InsertCandidatesIn` with `Currency` JPY reads back with it; `InsertCandidates` writes null.
- The test with the database carries the build tag `pg`.

### 3 candidates-rule-currency (interface)

Module: compute. Size: M. Order: after 1. Design: Decisions 13, 14.
Files: `internal/compute/optioncandapi/api.go` (new), `internal/compute/optioncandapi/api_test.go` (new), `internal/compute/optioncand/currency.go` (new), `internal/compute/optioncand/put.go`, `internal/compute/optioncand/optioncand.go`, `internal/compute/optioncand/currency_test.go` (new), `internal/compute/optioncand/testdata/currency_jpy.golden.json` (new), `docs/contracts/compute/optioncandapi.md` (new)
- `internal/compute/api.go` is not in the diff (839 lines).
- A JPY put with a JPY alias of enough funds has funding `fit`, with too little `no_fit`; with only USD aliases the gate passes, the row has the mark `no_jpy_funds` and `Fits` of the USD alias is `unknown` `no_fx`; it is never `no_fit` for the currency.
- A JPY put has a cash-rate gate that passes and the mark `no_jpy_rate`; yield is `bid / strike x 365 / dte` to 6 places.
- `api.go` declares `CurrencyRule.EvaluateIn(CurrencyInput, compute.CandidateParams) []compute.Candidate` and `NewCurrencyRuleFunc func() CurrencyRule`; `optioncand.NewCurrency` has that type (a compile-time assertion in `currency_test.go`).
- `EvaluateIn` with empty currency equals `Evaluate` row by row; the existing goldens of the package do not change.

### 4 candidates-tier-currency (interface)

Module: compute. Size: S. Order: after 1. Design: Decision 15.
Files: `internal/compute/optiontierapi/api.go`, `internal/compute/optiontierapi/api_test.go`, `internal/compute/optiontier/tier.go`, `internal/compute/optiontier/badges.go`, `internal/compute/optiontier/cash.go`, `internal/compute/optiontier/jpy_test.go` (new), `internal/compute/optiontier/testdata/jpy_rows.golden.json` (new)
- `Row.Currency` exists; empty is USD and every existing test and golden is unchanged.
- A row with the mark `no_jpy_rate` or `no_jpy_funds` and every other condition met is tier `check` with that badge, never `fits`; with neither mark and the same inputs it is `fits` (the mark is the only difference).
- A JPY row with no buy price is `grey` `no_buy_price`.
- A JPY row against a USD alias has the cash cell `unknown` with `other_currency` and no USD comparison of strike times multiplier.

### 5 candidates-jp-job

Module: jobs. Size: M. Order: after 2 and 3. Design: Decisions 1, 3, 5, 10, 11, 12, 14. Cross-design: Decision 9.
Files: `internal/jobs/optioncandidates/job_jp.toml` (new), `internal/jobs/optioncandidates/jp.go` (new), `internal/jobs/optioncandidates/jp_test.go` (new), `internal/jobs/optioncandidates/source.go`, `internal/jobs/optioncandidates/input.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/convert.go`, `internal/jobs/optioncandidates/job.toml`, `internal/jobs/optioncandidates/source_test.go`, `internal/jobs/optioncandidates/job_test.go`, `internal/jobs/optioncandidates/testdata/jp_open_rows.json` (new), `docs/contracts/jobs/option_candidates_jp.md` (new)
- `NewJP(st, rule optioncandapi.CurrencyRule, entries, clk)` builds a job with id `option_candidates_jp`, calendar XTKS, After `option_open_jp`, offset 5 minutes, and one key per JP entry with an issuer (Decision 3); the US job's plan hash and keys are unchanged (existing tests pass unedited).
- On an XTKS session day `Plan` returns the 8 keys of the JP watch entries with an issuer; a name with no `JPY` open row of D at all is in `Skipped()` with "no option chain" and the run still writes the rows of the others; the job reads no list from another jobs package (no import of `optionopen`); on a day XTKS is closed the schedule gives no run.
- JPY open rows of D that are usable become rows with `ChainDay` D; a name whose open rows are not usable is in `Skipped()` with "no usable open quote" and has no row, and no `eod_fallback` mark is written by this job.
- Open rows of currency USD are never read by the JP job and JPY rows never by the US job.
- The open interest of a JP put comes from the newest eod row within 3 XTKS sessions and is marked `oi_prior_session`.
- The run evaluates through `rule.EvaluateIn` with currency JPY and writes rows through `candidatecur.Writer` with currency JPY, with its own `[params]` (21 to 45 days, spread 0.10, bid size 1, open interest 10, each marked trial in the manifest); `Params()` still reads the US manifest.
- The log has one line per name with the counts of Decision 12 (b) and the multiplier of the stored terms.
- A run on a day with a failed pull key writes the rows of the other names; no run fails for it.

### 6 candidates-jp-page

Module: pages. Size: M. Order: after 1, 2, 3 and 4. Design: Decisions 7, 12, 15, 16, 18, 22 to 26. Cross-design: Decision 9.
Files: `internal/pages/option_candidates_page/format.go`, `internal/pages/option_candidates_page/cells.go`, `internal/pages/option_candidates_page/groups.go`, `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/evcells.go`, `internal/pages/option_candidates_page/evidence.go`, `internal/pages/option_candidates_page/shortlist.go`, `internal/pages/option_candidates_page/shortlistrows.go`, `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierrows.go`, `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/jp_test.go` (new), `internal/pages/option_candidates_page/format_test.go`, `internal/pages/option_candidates_page/testdata/jp_rows.json` (new), `internal/pages/option_candidates_page/testdata/jp_rows.golden.json` (new), `docs/contracts/pages.md`
- `moneyText` takes the currency; a JPY row's strike, bid, ask, break-even and cash print as "¥8,950" with no decimals; a USD or empty-currency row prints as today and the existing goldens do not change.
- The JP rows form their own `short_put` group with `currency` JPY, its own `day` and `stale`, and the note of Decision 26; the US groups have no `currency` and no `note`.
- The JP group is read from the date of the build time in `Asia/Tokyo`: a build at 09:10 JST on D while the US build day is D-1 finds the rows of D (test with a fake clock and a fake reader).
- `short_put_tiers_jpy` and `short_put_list_jpy` hold the JP rows in yen; `short_put_tiers` and `short_put_list` hold none; a JP row with a mark of Decision 14 is never in tier `fits`; the strip of a JP company passes `JPY` to the resolver.
- The evidence cells of a JP row carry the Chinese "not computed" reason and no number; no JP row prints a US figure.
- Each mark of Decision 16 has its Chinese hint in the badge text of the Marks cell; no row prints a bare mark word; `limit_counts` lists the stored blocked reasons of the JP run with their counts and a Chinese sentence each, and a JP run whose rows are all blocked shows non-zero counts and not an empty group.
- A cash-secured amount of a JP row prints with the multiplier used.
- `jpNote` no longer says Japan is a later design; a build with no JP reader gives the page of today (existing tests unedited).
- `docs/contracts/pages.md` states the group fields.

### 7 candidates-jp-wire

Module: ot. Size: S. Order: after 5 and 6. Design: Decisions 1, 13, 21, 26, 27, 30. Cross-design: Decision 9.
Files: `cmd/ot/wire_optioncandidates_jp.go` (new), `cmd/ot/wire_optioncandidates_jp_test.go` (new), `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/site.go`, `cmd/ot/candidates_site.go`, `cmd/ot/evidence_site.go`, `cmd/ot/candidates_site_test.go`, `cmd/ot/evidence_site_test.go`, `cmd/ot/candidates.go`, `cmd/ot/candidates_test.go`, `docs/contracts/ot/option_candidates_jp.md` (new)
- The family `option_candidates_jp` is registered with order 183 (after `option_open_jp`, unique, strictly increasing) and is held in the scheduler with `startOff` and one named line; `ot run-once --job option_candidates_jp` builds it over the same stores as the US job (open store line, candidates writer through `candidatecur.Writer`, the JP watch entries, the account store).
- With the open store line missing the job is left out with one named line and the US job still runs.
- `wc -l cmd/ot/wire.go` is at most 785; `deps` gets the field `newCurrencyRule`, filled with `optioncand.NewCurrency`, and `TestNewDepsFilled` in `wire_test.go` follows the field count; the job is built with `deps.newCurrencyRule()`.
- The page gets the Japan group: `candidatesConfig` (`evidence_site.go`) sets the page `Config` JP reader from `candidateReaders.CandCur`, `buildCandidates` (`candidates_site.go`) fills it from `siteReaders.candCur` (`site.go`), and a test (`TestCandidatesConfigHasJPReader`, new, in `evidence_site_test.go`) has it non-nil with the real store and nil in the demo profile and on the named path; the resolver `Buy` that entry 10 of buy-price-source set stays.
- No existing file of `cmd/ot` outside the Files line of this entry is edited.
- `ot candidates` prints for a JP name a line that points to the page and no longer says "no option route"; its USD-only read (`candidates.go:179`) is unchanged.

### 8 candidates-jp-web

Module: web. Size: S. Order: after 1. Design: Decisions 22, 24, 26.
Files: `web/src/types/option_candidates_page.ts`, `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/src/pages/candidates/CandidateGroup.tsx`, `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/ShortList.tsx`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`, `web/test/pages/optioncandidates/fixtures/jp.ts` (new), `web/test/pages/optioncandidates/fixtures/MANIFEST.synthetic-v1.json`
- The page renders the group `note`, `limit_counts` and the JPY tier view and short list from the document, with the group `currency` as a label; it shows no text of its own and computes no sum, yield, count or conversion.
- A document with a JPY group renders under the fixture `jp.ts`; a document without one renders as today; `tsc`, `eslint`, `vitest` and the build pass.
- `web/fixtures/MANIFEST.synthetic-v1.json` holds no candidates fixture and is not edited; the fixture manifest of the page test directory is.

### 9 candidates-jp-schedule

Module: ot. Size: S. Order: after 6, 7 and 8. Design: Decisions 1, 4, 18. The entry is cut after the window has run `ot run-once --job option_candidates_jp` once and checked the page and the multiplier (a sentence, not an Order line).
Files: `cmd/ot/wire_optioncandidates_jp.go` (new in entry 7), `cmd/ot/wire_optioncandidates_jp_test.go` (new in entry 7), `cmd/ot/jobconsole_jobs.go`, `cmd/ot/data_health_site.go`, `docs/contracts/ot/option_candidates_jp.md` (new in entry 7)
- The `startOff` hold is removed: `ot scheduler` lists `option_candidates_jp` with the same stores as entry 7.
- The job console lists the job as a today-only job in `Asia/Tokyo` and "Run today" enqueues it (with `-force` for a stored day, as the US job).
- The data-health table names `candidates.option_candidate` for the JPY calendar `XTKS`.

## Owner-visible results

What the window checks on the first Tokyo day after the last entry is deployed, from the scheduler log lines after the deploy and the page:
- The page has a Japan group with rows for the four large names at about 09:10 JST, in yen, each with the buy price of the valuation zone where the owner imported a valuation, "not judged" where not, and the marks `no_jpy_rate` (and `no_jpy_funds` until a yen account is named); no row is in the top tier; no US row changes.
- The three thin names show rows or a named reason (the `limit_counts` and the log line) rather than silence.
- The entry without options is named as having no options.
- A JP run and the US run of the same date are both on the page.

## Checked by CI

`ci-go` for the entries: build, vet, the tests of `internal/jobs/optioncandidates`, `internal/compute/optioncand`, `internal/compute/optiontier`, `internal/compute/optioncandapi`, `internal/store/candidate`, `internal/store/candidatecur` (the `pg` test in GitHub CI only), `internal/pages/option_candidates_page`, `internal/contracts`, `cmd/ot`; `web` checks for entry 8. Tests use `t.Parallel()`, fakes and a fake clock; no sleep, no broker. The multiplier check of Decision 18 is a window step, not a CI item.

## Left out

- The JP covered call and held-call guard (`NewWithCalls`): not designed here.
- Index (N225) options as candidates: not a watch stock entry.
- FX conversion of dollar funds to yen: a USD alias stays `unknown`.
- A yen cash-rate source (a separate design); until it exists no JP row reaches tier `fits` (Decision 15).
- The yen margin and one-sigma evidence for JP rows (Decision 24).
- The eod fallback and the JP chain of the previous session as a quote source (Decision 5).
- JP-only limits tuned from data: the first runs show the counts (Decision 12), the numbers are changed by a later design.
- Rejected: one job with a market switch (the US plan hash and tests would depend on JP names; calendar, zone and upstream differ); a `market` column instead of `currency` (the currency is what the page needs for the amount and the funds; a second market, such as Hong Kong, adds its own change then); telling JP rows apart by the issuer prefix `edinet:`; changing the meaning of `Candidates` to "newest run per currency" (it would change eight users, Decision 20).
- Page redesign of the candidates page: only the group, the yen format, the marks and the counts.

## Adversarial

Sources: the dispatcher scratch file `optdesk-drafts/designs/candidates-jp/adversarial-raw.md` (runner home, not in the repository), the advisor's four questions (Q1 to Q4 = the four owner questions of revision 1) and his blocking points 1 to 7 (the dispatcher's rulings, in the order of the review request: 1 buy price, 2 yen rate and funds with tier, 3 limits, 4 multiplier and tick, 5 sizes, 6 store read, 7 yen format; the rulings 8 to 11 are the same review's path, Decision 14, Decision 3 and fill points).

Adversarial blocking points:
- B1 buy price source: adopted. Decision 2 names buy-price-source, the chain valuation, hand, not judged, the JPY currency check and the cross-design wait as a sentence.
- B2 yen funding gate: adopted. Decision 14 passes with `no_jpy_funds`; the yen account is a window decision.
- B3 the page is more than the table: adopted. Decisions 24 and 25 and entry 6 list every `moneyText` user and the tier view, short list and evidence; the group shape is Decision 22.
- B4 `compute/api.go` at 839 of 840: adopted. Decision 13 and 27 put the currency in a new interface package; the file is not edited.
- Biggest unnamed risk (shared reader, rank across currencies, one day per group): adopted. Decision 20 keeps `Candidates` USD-only, Decision 21 has a per-currency read, Decision 23 gives the JP group its own day.
- Simpler way, run-once first and scheduler last: adopted (Decision 4, entries 7 and 9). Dropping the eod fallback: adopted (Decision 5); the chain length is unchanged, one data path is gone. Splitting the design in two at the contract/job line: not adopted, the interface entries are already their own entries.
- Lock-in, `currency` versus `market`: not adopted; currency models the money, a second market adds its own change. JP-only params: adopted (Decision 11, own `[params]`).
- Claim most likely false, entry 7 web paths and the docs directories: adopted (entry 8 uses the real paths; docs paths are the existing ones).
- Existing goldens untouched: adopted as acceptance (entries 2, 3, 4, 6 state the unedited tests and goldens).
- Slip causes: migration number: adopted ("the next free number when the entry is cut"); view column order: adopted (Decision 19, `columns` unchanged); entries 4 and 5 parallel with page first: partly (the web entry runs after 1 only, the page after 1 to 4); `wire.go` 782: adopted (Decision 27, 785 at most); `testdata` directories are not files: adopted (named files).
- Non-blocking, thin names and the entry without options handled well: kept. Scheduler restarts and `run_bound`: noted in Decision 2.

Advisor questions (the four owner questions of revision 1):
- Q1 buy prices: answered by the owner rule of 2026-11-19 (valuation imports); the list is deleted (Decision 6).
- Q2 yen cash rate: adopted, pass with `no_jpy_rate`, no new source (Decision 14).
- Q3 window and limits: adopted as trial values in the JP params with counts of removed rows (Decisions 11, 12).
- Q4 yen account: adopted as a window decision, pass with `no_jpy_funds` until named (Decision 14).

Advisor blocking points:
- 1 buy price after buy-price-source: adopted (Decisions 6 to 9).
- 2 yen cash rate and funding pass with marks, status and tier, Chinese hints in the document: adopted (Decisions 14 to 16, entries 3, 4, 6).
- 3 limits as trial values, per-limit counts: adopted (Decisions 11, 12; entries 5, 6).
- 4 multiplier and tick: no read-only probe clearly available; both stated as trial values, the multiplier printed beside the amount, a check step after the merge (Decisions 17, 18).
- 5 sizes: adopted, counts verified (Decision 27).
- 6 store read: adopted, `Candidates` USD-only and its eight users named (Decisions 20, 21).
- 7 yen format: adopted, all users in the page entry (Decision 25, entry 6).
- Review points 8 to 11: entry 8 paths and the manifest (adopted; `web/fixtures` has no candidates file, so the test-directory manifest is named instead), Decision 14 amendment (adopted, Decision 29), calendar and Plan acceptance (adopted, entry 5), the `## Adversarial` section (this section).
