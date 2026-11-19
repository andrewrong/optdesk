# buy price source: the put gate takes the valuation buy-zone top as the buy price of a company, else the hand price, else the company is not judged, and the candidates page says which

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19

Source: owner decision 2026-11-18 16:05Z (word "a", plan P5) and owner ask 5016 (2026-11-18): "writing both the valuation and the buy price by hand is too much upkeep". The order is fixed by the owner: (1) the buy-zone top of the valuation the owner imports on `/valuation-import` (`docs/design/valuation-import.md`, landed), (2) else the hand-entered price, (3) else "not judged"; no fallback to an FCF band or any computed price. This design is plan step 13 that `docs/design/candidates-valuation-line.md` (Left out, first item) and `docs/design/candidates-page-v2.md` (Decision 14, "a later design ... adds codes to `buySourceOf` ... with no schema change") name. It amends three earlier statements, named in Decision 12.

Read on origin/main at 0099a562: `AGENTS.md`, `docs/design/modules.md`, `docs/design/ratchet.json`, `docs/design/candidates-valuation-line.md`, `docs/design/candidates-tiers.md`, `docs/design/candidates-page-v2.md`, `docs/design/valuation-import.md`, `docs/design/price-lines.md`, `cmd/ot/{followfilter,listedjob,runonce,wire_underlying,candidates,evidence_site,valline_site,valuation_site}.go`, `internal/jobs/api.go`, `internal/jobs/optioncandidates/{job,calls,source,convert}.go`, `internal/jobs/optioneod/{watch,select,plan,manifest}.go`, `internal/jobs/optionopen/list_run.go`, `internal/runner/reconcile/{planday,reconcile}.go`, `internal/compute/optioncand/{put,optioncand}.go`, `internal/compute/valuelines/calc.go`, `internal/store/valuation/api.go`, `internal/store/valuationpg/store.go`, `internal/pages/option_candidates_page/{tiers,tierstrings,valueline,shortlistrows,shortlisttexts,cells,strings,page,zh_texts}.go`, `internal/pages/option_boards/{gates,option_cards}.go`, `internal/pages/price_lines_page/builder.go`, `internal/pages/valuation_page/table.go`, `internal/pages/candidate_text/format.go`, `contracts/schemas/page.option_candidates_page.v1.json`, `migrations/0052_valuation.sql`.

## What you see

The owner imports a valuation once on `/valuation-import` and types no buy price for that company. From the next run of the option jobs (the jobs read the valuation when they are built for each run), the company is judged at the valuation buy-zone top. An imported valuation applies to the next run, not to rows already stored for the day. The numbers below are invented (AAA has a valuation with buy-zone top 91.2000 and a hand price 90.00; BBB has only a hand price 41.00; CCC has neither).

`/options/candidates`, the company strip of the tier view:

    AAA   Spot $96.50 (2026-11-17)   Buy price $91.20   [Valuation buy zone]   ...
    BBB   Spot $41.20 (2026-11-14)   Buy price $41.00   [Hand price]            ...
    CCC   Spot $20.10 (2026-11-14)   Buy price not judged   [Not judged]        ...

The price is shown to two decimals; the gate and the collection use the exact stored decimal, and the hint says so. Each tag is a badge with a Chinese gloss (valuation: the buy price is the top of the buy zone the valuation computes, and the hand price, if any, is not used; hand: the price you typed; not judged: no valuation and no hand price, so the put gate does not judge this company). The existing second line, "Buy price used by the gate", stops saying "(hand-typed)" and shows the same price and the same tag. The line "Valuation buy line B_hi" beside it shows today's zone top; when the last run used an older review the two figures differ and both are visible.

`/options/candidates`, the short put list: the Buy price column of AAA rows shows 91.20 with the badge "Valuation buy zone"; BBB rows show 41.00 with "Hand price" (as today); CCC rows show the gap text and the link to the price lines, as today. The Marks column of a row does not list the source marks (the badge says it).

`/plan/price-lines` keeps its two lists as today; only its sentence and its intro are corrected, because "The put gate reads it" is no longer true for a name that has a valuation (entry 12). Showing the used price and the source on that page is a later design (Left out).

Option collection (`option_eod`, `option_open`, the strike band of a name) reads the same price, so the strikes collected for AAA are the band around 91.2000, the strikes the gate judges (Decision 3).

Labels and figures are English; every hint and gloss is Chinese, held as string constants in `zh_texts.go` (`TestZhTextsDoor`).

## Goal

One resolved buy price per company with its source, read by every option consumer: the option jobs (gate, covered calls, collection, ranking), the stored rows and the candidates page. The resolution is a pure function with three outcomes and no other source, and there is one copy of the zone rule: the strip's B_hi line calls the same resolver as the jobs. A page never recomputes which source a stored row used.

## Today

- The gate reads `BuyPrice` of a `jobs.WatchEntry` (`internal/jobs/api.go:23-25`, comment: "The owner's word wins over any automatic value"). Entries are built from the stored list row by `watchEntry` (`cmd/ot/followfilter.go:58`, `BuyPrice: r.BuyPrice`, `Issuer: r.Issuer`) inside `listedJob.entries` (`cmd/ot/listedjob.go:78-98`), which runs at every Plan and once at start (`newListedJob`, `listedjob.go:61-72`), so a job is rebuilt for each run.
- `listedJob` serves every family: the option families (`groupOptions`: `option_eod`, `option_eod_jp`, `option_open`, the option list and IV history, `wire_optioneod.go`, `wire_optionopen.go`, `wire_optionlist.go`, `wire_ivhistory.go`) and the families that never read a buy price: bars (`wire.go:511`, `wire_reconbars.go:68`), fundamentals and earnings (`wire_earnings.go:42`, `wire_fundamentals.go:65`, `wire_computefigures.go:40`) and dividends (`wire_dividend.go:31`). `jobFactory.listed(group, build)` (`wire_underlying.go:155`) already gets the group.
- The job passes `entry.BuyPrice` into the rule (`internal/jobs/optioncandidates/job.go:220`, `calls.go:166`) and counts usable puts by it (`source.go`); `optioneod` builds its strike band from `buy[symbol]` (`internal/jobs/optioneod/watch.go:103-107`, `select.go:50`: `rangeStrikes` around the buy price, or the spot band only when there is none) and `optionopen` ranks a name with a price first (`internal/jobs/optionopen/list_run.go:250-254`).
- `optioneod`'s plan hash covers the day and every setting, including `|buy<price>` per underlying (`internal/jobs/optioneod/manifest.go:385`, `plan.go:33`). The runner, on a hash that differs from the stored one, only replaces the expected key set (`internal/runner/reconcile/planday.go:30-35`, `reconcile.go:253-259`, `store/ops/ledger.go` `PutExpected`: delete and insert); the keys are the symbols and do not change, and a key already present stays present, so only a missing key is collected (`dueJobsOf`).
- The put gate with no usable price passes and marks the row `no_buy_price` (owner 2026-11-12, `internal/compute/optioncand/put.go:16-27`, `optioncand.go:93-94`); a price above the break-even blocks with `above_buy_price`. The tier view calls the first case "not judged" (`tierstrings.go:98`). This design does not change either rule.
- The stored row carries `buy_price` and the free text `marks`; the job already adds marks of its own after the rule (`internal/jobs/optioncandidates/source.go:26,46-52`, `eod_fallback`, `oi_prior_session`, called at `job.go:225` and `calls.go:173`) and the tier code ignores marks it does not know (`internal/compute/optiontier/badges.go:60-95`). The store type and the dataset have no source field; `internal/store/api.go` is 800 lines, so one more line there is over the size rule.
- Two readers print a mark they do not know as it is stored: the Marks cell of the candidates page (`cells.go:119-125`, `markHints`, test `TestMarksCellUnknownMarkStillAsStored`) and `option_boards` (`gates.go:79-92` `markText`, `option_cards.go:143-148`, which also changes the reason to "Passed, with marks: ..."). The command line text `candidate_text/format.go` prints the stored marks by design ("the stored columns in the stored order").
- The candidates page takes the strip price from the stored row (`tiers.go:223-227`, text `not set` when empty) and its source slot is `buySourceOf` (`shortlistrows.go:31-37`: `hand` when a price is stored, else none) with `buySourceTexts` (`shortlisttexts.go:65-70`, one code). The strip's second line has the label `Buy price used by the gate (hand-typed)` (`valueline.go:24`) and `gateHintZh`.
- B_hi of the valuation line is computed inside the page (`valueline.go:86-124`, `valueFigure`: key `Candidate.Issuer` equal to `Profile.Symbol` ignoring case, profile currency must be `USD`, `valuelinesapi.Calculator.Lines(...).BuyHigh`, shown through `moneyText` at 2 decimals, `format.go:90`). The candidates job keeps only US stock entries (`job.go:130-142`: a JP entry or an entry with no issuer gets no row), so every candidate row is a USD contract; `optioneod_jp` does not read `BuyPrice`.
- `valuelines.Calc.Lines` writes `BuyHigh` as the exact decimal text of a product of decimals (`calc.go:117`); a hand price has at most 4 places (`cmd/ot/watch.go:128`); the dataset pattern for `buy_price` is `^-?[0-9]+(\.[0-9]+)?$` (`candidates.option_candidate.r1.json`), so a long decimal is valid.
- `price_lines_page` says "The put gate reads it" (`builder.go:22`) and, in its intro, "clearing it removes the check for that name" (`builder.go:23`); both are tested by text (`builder_test.go:166-167`).
- Pages may import store and compute and not jobs; jobs may import store and compute; compute imports nothing else (`docs/design/modules.md`). Pages import compute `*api` packages only and receive implementations through `Config`. `internal/compute/api.go` is at 839 of its ceiling 840 and is not edited here.
- `ot_runner`, the role of the jobs' list reader (`openJobList`, `wire_underlying.go:104-118`), has no grant on the valuation views: migration `0052_valuation.sql:165-169` grants them to `ot_ro_pages` and `ot_w_settings`. The site reads the valuation with the pages line (`OT_PG_DSN_PAGES`, `runOnceConfig.pagesDSN`, `valuation_site.go:62-78`, `deps.openValuation` of `wire.go:186`), and `runonce.go:458-494` already opens other stores with `cfg.pagesDSN` for the jobs. `valuationpg.Open` connects on the first query, so a wrong DSN fails at open and a down database fails at the first read.

## Decisions

1. **One pure resolver, three outcomes.** `buypriceapi.Resolver.Resolve(Input) Result` returns `Source` `valuation` with `Price` = `Lines.BuyHigh` exactly as the calculator writes it, else `hand` with the hand price text, else `none` with an empty price. The valuation wins even when a hand price exists (owner order). A hand price is used only when it is a positive decimal; anything else is unset. No other source exists: no FCF band, no price of any other kind (owner 2026-11-18).
2. **When the valuation counts.** A zone is used only when exactly one valuation profile has the company's key (the issuer, ignoring case), its pool is not `removed`, it has a current review, the R and margin tables exist, its currency equals the currency given for the company, and the calculator returns a positive `BuyHigh`. Otherwise `Result.ZoneReason` is one of `no_profile`, `ambiguous`, `removed`, `no_review`, `no_tables`, `currency`, `bad_input`, and the price falls to the hand price. Decided here, so no worker has to choose:
   - The key of a company is `WatchEntry.Issuer` for the jobs and `Candidate.Issuer` for the strip, the key the strip uses today (`tiers.go:217`, `valueline.go:87`); the contract symbol is not the key, so a JP name or an alias cannot give the strip and the gate two answers. An entry with an empty issuer has no zone and keeps its hand price.
   - There is no age rule for a review: a review of any date counts (as in `candidates-valuation-line.md`, Left out).
   - There is no share-class or ADR rule: a profile matches by issuer text only. The import of the valuation records "V per listed share" (`docs/design/valuation-import.md`), so the owner's V is already per share of the line the profile names; no conversion is made.
   - A backfill review and a void review need no rule: `valuation.Reader.Current` already leaves out void rows and drafts, and a backfill review is not the current one (`internal/store/valuation/api.go`); the resolver sees only `Entry.Current`.
   - The currency is that of the company's contract: the job passes `Contract.Currency`, the strip passes `USD` because every candidate row is a US stock contract (`job.go:130-142`). This changes the rule "USD only" of `candidates-valuation-line.md` Decision 4 to "the profile currency equals the contract currency" (reason `currency`; the Chinese sentence `valueReasonCurrencyZh` says "not USD" and is reworded in entry 8). Today no consumer reads a non-USD price (the candidates job drops JP entries, `optioneod_jp` does not read `BuyPrice`), so the change shows only in `option_open`, which ranks any entry with a price first: a JP name with a JPY valuation now ranks like a JP name with a hand price.
3. **Collection follows the resolved price (owner question 1, option "a", decided by the dispatcher).** The jobs of the option families get one price from `WatchEntry.BuyPrice`: the gate, the covered calls, the strike band of `option_eod` and the priority of `option_open`. The reason: under the other choice a valued company with no hand price is collected on the spot band only (`watch.go:103-107`, `select.go:50`) and ranks second in `option_open` (`list_run.go:250-254`), so the gate would judge strikes that were never collected. What the runner does when the price changes (a new review, a new R version or a new import moves `|buy<price>` of `manifest.go:385` and so the plan hash): on a day already planned the stored hash differs, `PutExpected` replaces the expected rows with the same keys, keys already present are not collected again, and a missing key is collected with the new band; so a change applies from the next collection of a key. A past day keeps the strikes it stored (rows are never rewritten) and its band is not recomputed. The hash of a day planned later in the same run carries the new price. `SalePrice` stays the hand price.
4. **The jobs read the resolved price from the entry, for the option families only.** `jobs.WatchEntry` gets `BuySource` (`valuation`, `hand` or empty) and its `BuyPrice` comment is corrected. `listedJob.entries` calls one new step, `prices`, only when the group is `groupOptions`; the bars, fundamentals and dividend families never run it and never read the valuation, so a valuation outage cannot stop them. The step is `internal/jobs/buyprice.Apply`: one `valuation.Reader.Current` per Plan, then one `Resolve` per entry with its issuer, its contract currency and its hand `BuyPrice`.
5. **Failure rules, one for each moment.**
   - Service start never refuses and never reads the valuation: the read of `newListedJob` is made with the hand prices (the first call of the step, like the first call of the held reader in `listed`, `wire_underlying.go:158-165`), and one Info line `buyprice: start builds with hand prices, each Plan reads the valuation` is logged. The inner job built at start only answers the schedule; every Plan rebuilds it.
   - A `Current` error during a Plan of an option job is a Retryable `listedError` ("read valuation failed", like the list read of the same function): the run plans nothing and is retried, because judging with a hand price when a valuation exists would be silent. A read error never reaches a bars, fundamentals or dividend Plan.
   - With no pages line (`OT_PG_DSN_PAGES` empty) or no opener, no reader is opened: the step runs with no zones, so every entry gets its hand price and the source `hand`, and one Info line `buyprice: no valuation reader, hand prices only` is logged at start. A DSN that fails to open fails the wiring with a static text that holds no DSN (as the list open does).
6. **The source is stored as a mark on the row, and the job is its only writer.** The job appends `buy_from_valuation` for `BuySource` `valuation` and `buy_from_hand` for `hand` (constants of `buypriceapi`) to the marks of every row of an entry with a price, after the rule, in the way `addMarks` does; an entry with a price and an empty `BuySource` (built without the step) is marked `buy_from_hand` too, since its price is the list's hand price; nothing is added when the entry has no price. So from the first run of this design every row with a price carries exactly one source mark, with or without a reader. No store field, dataset revision or migration is added: `candidates.option_candidate` r1 already has `marks`, and `store.Candidate` is unchanged (rejected: a `buy_source` column, see Left out). The marks are forever: a row stored before this design has a price and neither mark, and a page reads that as `hand`.
7. **Pages read the source from the row; the two mark words are not shown as marks.** `buySourceOf` returns `valuation` for the mark `buy_from_valuation`, `hand` for `buy_from_hand` or a stored price with neither mark, and none for no price. The strip takes the source of the first row of the company that has a price (rows of one company come from one run). The page does not recompute which price the run used, so after a new import the strip shows the price of the last run until the next run; the B_hi line beside it shows the current valuation, so a difference is visible. The Marks cell of the candidates page and the marks of `option_boards` leave the two source words out (the badge or no tag says it); every other mark is shown as before and an unknown mark still shows as stored. `candidate_text` prints the stored marks as they are, by its own rule, and is not changed.
8. **The strip and the short list.** `Company` gets the optional property `buy_source` (`Badge` or null; additive, no version change, as `short_put_tiers`, `valuation` and `short_put_list` were added). Codes: `valuation` (label "Valuation buy zone"), `hand` (label "Hand price"), `none` (label "Not judged"); each gloss is a Chinese constant. `buy_price_note` and the gate text for no price say `not judged` (English constant `notJudged`, replacing `notSet` on the strip). The label `gateLabel` becomes "Buy price used by the gate". The short list's `buySourceTexts` gets the code `valuation`; `buyOf` is unchanged and the link to the price lines stays only when the source is none.
9. **One copy of the zone rule, one owner of the rounding.** `valueFigure` stops testing the reasons itself: it builds a `buypriceapi.Input` (issuer, `USD`, empty hand price, the zones and tables copied from `valuation.State`) and calls the injected `Resolver`; the figure is `moneyText(Result.Price)` and the reason is `Result.ZoneReason`. The page keeps one test of its own, the strip-level `ambiguous` (two strips with the same ticker), applied only when the resolver's reason is not `no_profile`, so the order of the reasons on the strip is as today. The page `Config` gets `Buy buypriceapi.Resolver`; a nil `Buy` gives no valuation block, as a nil `ValueLines` did. The one rounding is the display rounding of the page (`moneyText`, 2 decimals, as today): the resolver, the stored `buy_price`, the strike band, the gate and `above_buy_price` use the exact decimal text, and the Chinese hint of the strip says the shown price is rounded to the cent and the gate uses the exact value. The copy of `valuation.State` into zones and tables exists twice, in `internal/jobs/buyprice` and in `internal/pages/option_candidates_page` (pages cannot import jobs, compute cannot import store); each is a field copy of about 30 lines with its own test.
10. **No change to the rule.** The put gate, `no_buy_price`, `above_buy_price`, the tier verdicts, the evidence and the figures are as today and read the same `buy_price` column; only the price they get changes for a company with a valuation.
11. **Own implementation, because** the resolution is the owner's three-step rule over the existing calculator; no library resolves a valuation price. It sits behind the interface `buypriceapi` so a source can be replaced there (owner 2026-11-08).
12. **What this design amends.** (a) `docs/design/candidates-valuation-line.md` ("shown only, gate unchanged", Decisions 1, 4, 6, 7 and 11): the line B_hi is now the gate's price when a valuation exists, its figure comes from the resolver and its currency rule is the contract's; (b) `docs/design/candidates-tiers.md`, Left out first item (line 442, "the company strip shows the hand-typed buy price only, and the tiers use it"): the strip shows the resolved price with its source; (c) item 3a of the plan `state/optdesk-drafts/designs/sellput-strategy/plan-2026-11-17.md` (the section 买入价与估值的关系, question 3, the owner's answer "a: side by side, the gate unchanged"): replaced by the owner's word of 2026-11-18, "valuation first". The design pull request edits (a) and (b) with one sentence each; (c) lives outside the repository and is corrected by the window.

## Modules

`docs/design/modules.md` changes by one word in the compute row: the entry cell gains `internal/compute/buypriceapi` (done in the design pull request). The new packages `internal/compute/buyprice` and `internal/jobs/buyprice` need no row: the compute path `internal/compute/**` and the jobs path `internal/jobs/**` already match them, and the new files of `cmd/ot`, `docs/contracts/compute/`, `docs/contracts/jobs/` and `docs/contracts/ot/` match their rows. Paths and the first row whose glob matches:

- compute: `internal/compute/buypriceapi/**`, `internal/compute/buyprice/**`, `docs/contracts/compute/buyprice.md` (entries 1, 2).
- jobs: `internal/jobs/api.go` (entry 3), `internal/jobs/buyprice/**`, `internal/jobs/optioncandidates/**`, `docs/contracts/jobs/**` (entries 5, 6).
- contracts: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/**`, `internal/contracts/*_test.go`, `docs/contracts/page-schemas.md` (entry 4).
- pages: `internal/pages/option_candidates_page/**`, `internal/pages/option_boards/**`, `internal/pages/price_lines_page/**`, `docs/contracts/pages.md` (entries 8, 9, 12).
- ot: `cmd/**`, `docs/contracts/ot/**` (entries 7, 10).
- web: `web/src/pages/**`, `web/src/types/**`, `web/test/**` (entry 11).

Imports against the `depends` column: jobs imports `store/valuation` (store) and `compute/buypriceapi`; pages imports `store/valuation` and `buypriceapi` (both pages, `option_candidates_page` and `option_boards`); `buyprice` imports `valuelinesapi` (compute); `cmd/ot` imports `buyprice`, `valuelines` and `jobs/buyprice`; web imports generated types only. No store, dataset or migration change: nothing holds the auto-deploy.

## Interface

Owning module of the shared artifacts: compute (entry 1) for the resolver and the mark words, jobs (entry 3) for `WatchEntry`, contracts (entry 4) for the page schema.

(interface) Entry 1, `internal/compute/buypriceapi/api.go`, declarations only:

    // Sources of a resolved price and the marks a stored row carries (the writer is the job, the reader the page).
    const ( SourceValuation = "valuation"; SourceHand = "hand"; SourceNone = "none" )
    const ( MarkValuation = "buy_from_valuation"; MarkHand = "buy_from_hand" )
    // Reasons a valuation gave no price (Result.ZoneReason), in the order of Decision 2.
    const ( ReasonNoProfile = "no_profile"; ReasonAmbiguous = "ambiguous"; ReasonRemoved = "removed"; ReasonNoReview = "no_review";
            ReasonNoTables = "no_tables"; ReasonCurrency = "currency"; ReasonBadInput = "bad_input" )
    // Zone is one valuation profile with its current review, as plain text copied from the store.
    type Zone struct { Symbol, Currency, Pool, Market, V, P, ReviewDay string; Reviewed bool }
    // Tables are the R values and the margin rows of the newest versions; Present is false when either is missing.
    type Tables struct { Present bool; R valuelinesapi.RValues; Margin []valuelinesapi.MarginRow }
    // Input is one company: its key (the issuer) and contract currency, its hand price text ("" unset) and the valuation.
    type Input struct { Symbol, Currency, Hand string; Zones []Zone; Tables Tables }
    // Result: Price is "" for SourceNone and is exact decimal text; ReviewDay is set for SourceValuation; ZoneReason is "" when the zone was used.
    type Result struct { Price, Source, ReviewDay, ZoneReason string }
    // Resolver is pure: no I/O, no clock, no state. Resolve never fails; a bad input is a reason.
    type Resolver interface { Resolve(in Input) Result }

(interface) Entry 3, `internal/jobs/api.go`: `WatchEntry` gains `BuySource string` after `BuyPrice` ("valuation", "hand" or empty), and the comment of `BuyPrice` says it is the resolved price (docs/design/buy-price-source.md).

(interface) Entry 4, contracts: `page.option_candidates_page.v1`: `Company` gains the optional property `buy_source` (`Badge` or null, not in `required`); `ValueLine` is unchanged. `schema_version` stays 1. The Chinese badge gloss is `Badge.gloss`; the hints of the strip are the existing `ValueLine.hint` and `ValueLine.gate_hint`, so every hint the page prints has a field.

The `Config` field `Buy buypriceapi.Resolver` of `option_candidates_page` (entry 8) is read by entry 10, which is ordered after 8; a page `Config` is not a module interface file.

## Issues

Build order: entries 1, 3 and 4 are the interface entries and land first, in parallel; every later entry needs only landed interfaces, and entry 10 needs entry 8. Entries are not merged: entries 1 and 2 are split because jobs and pages wait on the interface alone; entries 3, 5 and 6 are split because the job marks (6) need only the entry field (3); the ot entries are split because 7 waits on the jobs and 10 on the page. Between the merges of 8 and 10 the strip shows no valuation block (a nil `Buy`, as with no reader); entry 10 follows at once. Companies in every test and fixture are AAA, BBB, CCC with invented numbers (no valuation number of the owner appears in the repository). No migration. A test names a Chinese constant and never repeats its text. `BASE` is main when the entry is cut.

### 1 buyprice-compute-if (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/buypriceapi/api.go`, `internal/compute/buypriceapi/api_test.go`, `docs/contracts/compute/buyprice.md`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Interface
Plan: P5
Acceptance:
1. `api.go` declares the three `Source*` constants, the two `Mark*` constants, the seven `Reason*` constants, `Zone`, `Tables`, `Input`, `Result` and `Resolver` of the Interface section, with no function body and no logic (all names are new).
2. `api_test.go` has `TestResolverContractFake` (new: a fake `Resolver` returns each of the three sources and a caller that reads `Result` compiles against it) and `TestMarkWordsDistinct` (new: the two mark words differ from each other and from `no_buy_price`).
3. `docs/contracts/compute/buyprice.md` (new) states the order valuation, hand, none, that no other source exists, that the price is exact decimal text and never rounded, and the reasons.
4. `git diff --name-only BASE` lists only the three files; `internal/compute/api.go` is not in it.
5. The result matches docs/design/buy-price-source.md#Interface.

### 2 buyprice-compute

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/buyprice/resolve.go`, `internal/compute/buyprice/resolve_test.go`, `internal/compute/buyprice/helpers_test.go`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `resolve.go` (new) declares `New(calc valuelinesapi.Calculator) buypriceapi.Resolver`; it imports no store, jobs or pages package, `database/sql` or `os`, and no `time`; it adds, rounds and compares no price (the price is the calculator's text).
2. A company with a usable zone and a hand price gets `valuation` and the calculator's `BuyHigh` text unchanged (a fake calculator that answers a text the resolver cannot produce shows it unchanged) with the review day (valuation wins over hand).
3. A company with no zone and a hand price `41.00` gets `hand` and `41.00`; with neither it gets `none`, an empty price and the reason of the zone.
4. Each reason is tested: no profile with the key (`no_profile`), two profiles with one key ignoring case (`ambiguous`), pool `removed`, no current review, `Tables.Present` false, a profile currency other than the input currency, and a calculator error or a non-positive `BuyHigh` (`bad_input`); in each case a hand price is returned as `hand`.
5. A hand price that is empty, zero, negative or not a number is unset (`none`).
6. The match ignores letter case (`aaa` finds `AAA`), and an input whose key is empty has no zone (`no_profile`).
7. `resolve_test.go` names `TestResolveValuationWinsOverHand` (new), `TestResolveHandWhenNoZone` (new), `TestResolveNone` (new), `TestResolveReasons` (new), `TestResolveBadHandIsUnset` (new), `TestResolveMatchIgnoresCase` (new), `TestResolveEmptyKeyNoZone` (new), `TestResolveCurrencyIsTheInputs` (new: a JPY profile with input currency JPY gets the zone, with USD gets `currency`) and `TestResolveIsPure` (new: `resolve.go` imports none of the packages of item 1), and runs with `t.Parallel()`.
8. The result matches docs/design/buy-price-source.md#Decisions.

### 3 buyprice-jobs-if (interface)

Module: jobs. Size: S.
Order: first.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Interface
Plan: P5
Acceptance:
1. `WatchEntry` has the field `BuySource string` after `BuyPrice` (new name) and the comment of `BuyPrice` no longer says "The owner's word wins over any automatic value" and says the price is the resolved one (valuation, else hand).
2. Every other field keeps its name, type and order (`git diff BASE -- internal/jobs/api.go` has no removed line except the edited comment); `api_test.go` gains `TestWatchEntryBuySourceZero` (new: the zero value is empty).
3. `git diff --name-only BASE` lists only the two files; `internal/jobs/api_users_test.go` still passes (the type is used by more than one package already).
4. The result matches docs/design/buy-price-source.md#Interface.

### 4 buyprice-schema (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.option_candidates_page v1 additive
Design: docs/design/buy-price-source.md#Interface
Plan: P5
Acceptance:
1. `Company` of the candidates schema has the optional property `buy_source` (`Badge` or null) and `required` of `Company` is unchanged; `schema_version` is still the const 1; `git diff BASE -- contracts/schemas` has no removed line except the closing punctuation of an edited object; no `"type": "number"` line is added.
2. The generated TS file contains `buy_source` (new name) and is in `git diff --name-only BASE` together with the schema, never edited by hand.
3. `option_candidates_page_v1_test.go` gains `TestBuySourceValid` (new: a company with a Badge validates), `TestBuySourceNullValid` (new), `TestNoBuySourceStillValid` (new: a company without the key validates, the additive rule) and `TestBuySourceUnknownField` (new: an extra key of the badge fails).
4. `docs/contracts/page-schemas.md` gains one sentence (new) naming the optional property as additive.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. The result matches docs/design/buy-price-source.md#Interface.

### 5 buyprice-adapter

Module: jobs. Size: M.
Order: after 1 and 3.
Files: `internal/jobs/buyprice/apply.go`, `internal/jobs/buyprice/apply_test.go`, `docs/contracts/jobs/buyprice.md`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `apply.go` (new) declares `Apply(ctx, rd valuation.Reader, r buypriceapi.Resolver, entries []jobs.WatchEntry) ([]jobs.WatchEntry, error)`: one `rd.Current` call, the state copied field by field into `[]buypriceapi.Zone` and `buypriceapi.Tables`, and for each entry one `Resolve` with `Issuer`, `Contract.Currency` and its hand `BuyPrice`; the result sets `BuyPrice` and `BuySource` and no other field; the input slice is not changed.
2. An entry resolved to `valuation` has `BuySource` `valuation` and the resolver's price; to `hand`, `hand` and its hand price; to `none`, an empty price and an empty source (a fake resolver shows the price is the resolver's text).
3. A `Current` error is returned (wrapped) and no entry is returned. A nil reader resolves with no zones and no read, so a hand price gives `hand` and no price gives `none`; a nil resolver returns the entries unchanged with `BuySource` `hand` where a price exists.
4. The valuation is read once per call, not once per entry (a counting fake).
5. `apply_test.go` names `TestApplyValuationWins` (new), `TestApplyHandAndNone` (new), `TestApplyReadErrorReturned` (new), `TestApplyNilReaderKeepsHand` (new), `TestApplyOneRead` (new), `TestApplyDoesNotChangeInput` (new), `TestApplyKeyIsIssuer` (new: a contract symbol that differs from the issuer does not change the match) and `TestApplyCopiesState` (new: the zone list holds the fields of each profile and review) and the package imports no other jobs package.
6. `docs/contracts/jobs/buyprice.md` (new) states the one read per Plan, the key, the currency and the nil-reader rule.
7. The result matches docs/design/buy-price-source.md#Decisions.

### 6 buyprice-job-marks

Module: jobs. Size: M.
Order: after 3.
Files: `internal/jobs/optioncandidates/buysource.go`, `internal/jobs/optioncandidates/buysource_test.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/calls.go`, `internal/jobs/optioncandidates/job_test.go`, `docs/contracts/jobs/option_candidates.md`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `buysource.go` (new) declares one function that appends `buypriceapi.MarkValuation` for `BuySource` `valuation`, `buypriceapi.MarkHand` for `hand` or for an empty source with a price, and nothing for an entry with no price, to the marks of a rule row; `job.go` and `calls.go` call it on the rows of an entry, short puts and covered calls, next to `addMarks` and before `toStoreRow`; `convert.go`, `types.go` and `internal/store/api.go` are not in `git diff --name-only BASE`.
2. A run over an entry with `BuySource` `valuation` stores rows whose `marks` contain `buy_from_valuation` and whose `buy_price` is the entry price; `hand` gives `buy_from_hand`; an entry with a price and no source gives `buy_from_hand`; an entry with no price stores `buy_price` null and neither mark.
3. The rule is unchanged: for the same entry price a row's gates, status and rank equal those of a run built without the new call (the existing tests pass unedited except where they build entries).
4. The run id of two runs over the same entries is the same (the marks are deterministic).
5. `buysource_test.go` names `TestBuySourceMarkValuation` (new), `TestBuySourceMarkHand` (new), `TestBuySourceEmptySourceIsHand` (new), `TestBuySourceNoMarkWithoutPrice` (new), `TestBuySourceOnCoveredCalls` (new) and `TestBuySourceKeepsOtherMarks` (new: `eod_fallback` stays).
6. `docs/contracts/jobs/option_candidates.md` lists the two mark words, says the job is their only writer and that a row stored before this design has a price and neither mark.
7. The result matches docs/design/buy-price-source.md#Decisions.

### 7 buyprice-ot-jobs

Module: ot. Size: M.
Order: after 2, 3 and 5.
Files: `cmd/ot/buyprice_wire.go`, `cmd/ot/buyprice_wire_test.go`, `cmd/ot/runonce.go`, `cmd/ot/listedjob.go`, `cmd/ot/listedjob_test.go`, `cmd/ot/wire_underlying.go`, `cmd/ot/wire_underlying_test.go`, `docs/contracts/ot/buyprice.md`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `jobFactory` of `runonce.go` gains the fields `valuation valuation.Reader` and `buy buypriceapi.Resolver`; `buyprice_wire.go` (new) declares `openJobValuation(a *app, cfg runOnceConfig, f *jobFactory, extra *closerSet) error`, which opens the valuation reader with `cfg.pagesDSN` through `a.deps.openValuation` (the pages line, because `ot_runner` has no grant on the valuation views), gives its closer to `extra`, sets `f.buy` to `buyprice.New(valuelines.New())`, and is called by `openJobList` of `wire_underlying.go`; no new DSN line, role or `deps` field is added and `wire.go` is not in `git diff --name-only BASE`. An open error holds no DSN.
2. `listedJob` gets a `prices` step (nil for every group but `groupOptions`; `newListedJob` and `startListed` take it as one parameter); `listedJob.entries` returns the entries after `jobsbuyprice.Apply` only when the step is set, so the entries of bars, fundamentals and dividends are unchanged and a read error cannot reach them (a fake valuation reader that fails, a `groupBars` listed job that still plans).
3. A resolved entry reaches the inner job of an option family with the valuation price and `BuySource` `valuation` (a fake valuation reader and the existing fake list reader).
4. A `Current` error during a Plan of an option family makes the Plan fail with a Retryable `listedError` whose text names the valuation read and holds no DSN; the existing list-read error is unchanged.
5. At start (the first call of the step, from `newListedJob`) no valuation read is made, the entries carry hand prices and one Info line `buyprice: start builds with hand prices, each Plan reads the valuation` is logged; the start never fails for the valuation (a failing fake reader is not called at start).
6. With no pages line or no opener the step runs with no zones: entries keep their hand prices with `BuySource` `hand`, the job is built, and one Info line `buyprice: no valuation reader, hand prices only` is logged at start (never per run).
7. `option_eod` and `option_open` get the same price: a test builds both families over one list and checks one name's `BuyPrice` and `BuySource` in the entries each inner job received.
8. `listedjob_test.go` and `wire_underlying_test.go` are edited only where the new parameter or reader needs a fake; `TestBuyPriceWiringHasNoLogic` (new: `buyprice_wire.go` declares no loop over prices).
9. `docs/contracts/ot/buyprice.md` (new) names the valuation read of the option jobs, the pages line, the three moments of Decision 5 and that no other source exists.
10. The result matches docs/design/buy-price-source.md#Decisions.

### 8 buyprice-page-candidates

Module: pages. Size: L.
Order: after 1 and 4.
Files: `internal/pages/option_candidates_page/buysource.go`, `internal/pages/option_candidates_page/zones.go`, `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/shortlistrows.go`, `internal/pages/option_candidates_page/shortlisttexts.go`, `internal/pages/option_candidates_page/cells.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/buysource_test.go`, `internal/pages/option_candidates_page/zones_test.go`, `internal/pages/option_candidates_page/page_test.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/tiersfakes_test.go`, `internal/pages/option_candidates_page/valueline_test.go`, `internal/pages/option_candidates_page/valuefakes_test.go`, `internal/pages/option_candidates_page/shortlist_test.go`, `internal/pages/option_candidates_page/shortlistfakes_test.go`, `internal/pages/option_candidates_page/markhints_test.go`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_ok.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_gaps.golden.json`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `buysource.go` (new) declares the one function that gives the source code of a stored row: `valuation` for the mark `buy_from_valuation`, `hand` for `buy_from_hand` or a stored price with neither mark, none for no price; `buySourceOf` of `shortlistrows.go` calls it; `builder.go`, `groups.go` and `docs/design/ratchet.json` are not in `git diff --name-only BASE`; every new non-test file is under 600 lines.
2. `Config` of `page.go` gains `Buy buypriceapi.Resolver`; `valueFigure` calls it with the issuer, `USD`, an empty hand price and the zones and tables that `zones.go` (new) copies field by field from `valuation.State`; the figure is `moneyText(Result.Price)` and the reason is `Result.ZoneReason`; the page tests no reason other than the strip-level `ambiguous`, applied only when the resolver's reason is not `no_profile`; a nil `Buy` gives no valuation block. `TestStripFigureEqualsResolver` (new: for one state the figure of the strip is `moneyText` of the price the resolver returns for the same issuer and zones, and the reasons of the existing `valueline_test.go` cases are unchanged).
3. The strip of a company whose first priced row has `buy_from_valuation` shows `buy_source` with code `valuation`; `buy_from_hand` or an old row with a price shows `hand`; a company with no price shows `none` and its `buy_price` is null.
4. For a company with no price `buy_price_note` and the gate text are `not judged` (constant `notJudged`); `notSet` is no longer used by the strip. `gateLabel` is "Buy price used by the gate" with no "(hand-typed)".
5. The Chinese constants of `zh_texts.go` that the change makes false are reworded, each with its golden: `valueHintZh` (line 88: B_hi is now the gate's price when a valuation exists, "shown only" goes), `gateHintZh` (89: the price is the one of the last run, valuation top else hand price, shown to the cent while the gate uses the exact value), `zhGreyGloss_no_buy_price` (50: no valuation buy zone and no hand price), `buyHintZh` (105: import a valuation or type a price), `explainFitsHead` (6), `explainCheck` (14), `noneFits` (110) (each says "has a buy price" in place of "price set") and `valueReasonCurrencyZh` (95: the valuation currency differs from the contract currency). `zhColHintBuy` (130) and `zhBuySourceHand` (142) stay true and are not edited; the new glosses `zhBuySourceValuation` and `zhBuySourceNone` are Chinese constants (`zhBuySourceHand` is reused), each 1 to 200 characters, no number inside, and no other non-test file of the package holds a Chinese character.
6. The short list: a row whose stored marks hold `buy_from_valuation` has `buy_source.code` `valuation`, label "Valuation buy zone" and the Chinese gloss; hand rows are as before; a row with no price keeps `buy_link` and no badge.
7. `marksCell` of `cells.go` leaves `buypriceapi.MarkValuation` and `buypriceapi.MarkHand` out of the Badges (a row whose only marks are the two shows the dash) and still shows every other mark with its hint and an unknown mark as stored (`TestMarksCellUnknownMarkStillAsStored` unchanged and passing; `TestMarksCellHidesSourceMarks` (new) in `markhints_test.go`).
8. Tiers, rows, counts, gates and the order of companies are equal for a build over the same rows with and without the source marks (`TestBuySourceDoesNotChangeTiers`, new).
9. `buysource_test.go` names `TestBuySourceFromMarks` (new), `TestBuySourceOldRowIsHand` (new), `TestBuySourceNoPriceNone` (new), `TestStripBuySourceBadge` (new), `TestStripNotJudgedText` (new), `TestShortRowValuationBadge` (new), `TestBuySourceTextsFromZhConsts` (new: glosses equal their constants and are 1 to 200 characters) and `TestBuySourceValidatesSchema` (new: the goldens validate against `page.option_candidates_page.v1.json`); `zones_test.go` names `TestZonesCopiesState` (new).
10. The five goldens change only by the new key `buy_source`, the strip texts above, the reworded hints of item 5 and the added badge of a valuation row; `docs/contracts/pages.md` gains one paragraph (new) naming the marks, the order, the resolver call and that the page reads the source from the row.
11. `git diff --name-only BASE` lists only files under `internal/pages/option_candidates_page` and `docs/contracts/pages.md`.
12. The result matches docs/design/buy-price-source.md#Decisions.

### 9 buyprice-page-boards

Module: pages. Size: S.
Order: after 1.
Files: `internal/pages/option_boards/gates.go`, `internal/pages/option_boards/gates_test.go`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `markBadges` of `gates.go` leaves `buypriceapi.MarkValuation` and `buypriceapi.MarkHand` out; `buildCard` (`option_cards.go`, not edited) then adds no tag for them and keeps the reason "Passed" when they are the only marks; every other mark and an unknown mark are printed as before.
2. `gates_test.go` names `TestMarkBadgesHideBuySource` (new: marks `["buy_from_valuation","itm"]` give the one badge of `itm`) and `TestMarkBadgesUnknownStillRaw` (new: `some_future_mark` is returned as stored).
3. `git diff --name-only BASE` lists only the two files; the goldens of the package are unchanged.
4. The result matches docs/design/buy-price-source.md#Decisions.

### 10 buyprice-ot-page

Module: ot. Size: S.
Order: after 8.
Files: `cmd/ot/valline_site.go`, `cmd/ot/valline_site_test.go`, `cmd/ot/evidence_site.go`, `docs/contracts/ot/pages/option_candidates_page.md`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `valline_site.go` declares `newBuyPriceResolver() buypriceapi.Resolver` returning `buyprice.New(valuelines.New())`; `candidatesConfig` of `evidence_site.go` sets `Buy: newBuyPriceResolver()` (`ValueLines: newValueLines()` stays as it is; removing the field is Left out).
2. `valline_site_test.go` names `TestBuyPriceResolverIsReal` (new: the resolver of the wiring returns the calculator's `BuyHigh` text unchanged for a state with a zone) and `TestCandidatesConfigHasBuy` (new: `candidatesConfig` has a non-nil `Buy`, with and without readers).
3. `docs/contracts/ot/pages/option_candidates_page.md` names the resolver the route passes and that the page and the jobs use one resolver.
4. `git diff --name-only BASE` lists no file outside the four; `wire.go` and `valuation_site.go` are not in it.
5. The result matches docs/design/buy-price-source.md#Decisions.

### 11 buyprice-web-candidates

Module: web. Size: S.
Order: after 4.
Files: `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/types/option_candidates_page.ts`, `web/test/pages/optioncandidates/CompanyStrip.test.tsx`, `web/test/pages/optioncandidates/fixtures/tiers.ts`, `web/test/pages/optioncandidates/fixtures/valueline.ts`, `web/test/pages/optioncandidates/fixtures/MANIFEST.synthetic-v1.json`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
按 frontend-stack：公司条用已有的 shadcn Badge 显示 buy_source，gloss 放在 Tooltip；不做运算，不用图表。
Acceptance:
1. `CompanyStrip.tsx` renders `buy_source` as a badge with its label and its gloss in a tooltip when the key is present and not null, and renders as today when it is absent or null.
2. The component renders every text from the payload and contains no arithmetic, `toFixed`, `Intl.NumberFormat`, sort or filter (`git grep -n "toFixed\|NumberFormat\|\.sort(\|\.reduce(\|\.filter(" -- web/src/pages/candidates/CompanyStrip.tsx` finds no line).
3. `CompanyStrip.test.tsx` names `TestStripShowsValuationSource` (new: AAA shows `$91.20` and the label "Valuation buy zone"), `TestStripShowsHandSource` (new), `TestStripShowsNotJudged` (new: CCC shows `not judged` and the label "Not judged") and `TestStripNoBuySourceUnchanged` (new: a company without the key renders the same text as before).
4. The fixtures use invented numbers and are listed in `MANIFEST.synthetic-v1.json`; `web/src/generated` is not in `git diff --name-only BASE`; `package.json`, `pnpm-lock.yaml` and `frontend-whitelist.json` are not in it.
5. The result matches docs/design/buy-price-source.md#Decisions.

### 12 buyprice-page-pricelines-text

Module: pages. Size: S.
Order: first.
Files: `internal/pages/price_lines_page/builder.go`, `internal/pages/price_lines_page/builder_test.go`
Contracts: unchanged
Design: docs/design/buy-price-source.md#Decisions
Plan: P5
Acceptance:
1. `sentenceText` of `builder.go:22` no longer says "The put gate reads it" without condition: it says the price is the hand buy price of each name and that the put gate reads it only for a name that has no valuation buy zone (English); `introText` (`builder.go:23`) says that clearing a hand price removes the check only for a name with no valuation (English). No Chinese text is added, no schema field is added and the page output differs from `BASE` only in these two strings.
2. `builder_test.go` is edited at the two lines that name the old texts (`builder_test.go:166-167`) and names `TestSentenceNamesValuationException` (new).
3. `git diff --name-only BASE` lists only the two files.
4. The result matches docs/design/buy-price-source.md#Decisions.

## Checked by CI

- `go build ./...`, `go vet ./...`, `go test -race` of `./internal/compute/...`, `./internal/jobs/...`, `./internal/pages/...`, `./internal/contracts`, `./cmd/ot`; every new test calls `t.Parallel()`, uses `t.TempDir()` for any file and does not sleep or read the clock; `TestZhTextsDoor` and `TestGoTreesAreEnglish` pass.
- go-arch-lint (the import edges of Modules, including that no compute package imports store and that `jobs/buyprice` imports no other jobs package) and the size check against `docs/design/ratchet.json`.
- `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, the web build; the frontend dependency check.
- The `test` check reads `Contracts:` of the pull request body: entry 4 `Contracts: page.option_candidates_page v1 additive`, the others `Contracts: unchanged`.
- `crew issue check` of each entry against the head of the design pull request.
- After the merge, by the window and not an issue: a screenshot of the strip against the samples above, and one run of `option_candidates` after an import, showing the rows of a company with a valuation carry the mark `buy_from_valuation`, its `buy_price` equals the stored exact zone top, and the strip B_hi line equals the strip price to the cent.

## Adversarial

The adversarial answer is the dispatcher scratch file `optdesk-drafts/designs/buy-price-source/adversarial-raw.md` (runner home, not in the repository). One line each:

- Adversarial 1 (gap, blocking: wiring not buildable, key mismatch, same-day import): adopted. Entry 7 lists `runonce.go` and names where the pages-line opener goes (entry 7 item 1); Decision 2 fixes the key as the issuer for the jobs and the strip; the same-day rule stays as the design says it (What you see).
- Adversarial 2 (outage stops unrelated jobs, start-time read, raw marks): adopted. Decision 4 runs the step for `groupOptions` only; Decision 5 gives start, Plan and no-reader their rules; Decision 7 and entries 8 and 9 hide the two marks.
- Adversarial 3 (simpler: 7 entries; drop the price lines page work; page calls the resolver): partly adopted. The price lines page work is cut to the sentence fix (entry 12) and the rest is Left out; the page calls the resolver (Decision 9) so the zone rule has one copy; the marks route stays because the strip must show the price the run used. Merging 1 with 2, 3 with 5 and 7 with 10 is not adopted: other modules wait on the interface entries alone and entry 6 needs only entry 3 (Issues, intro).
- Adversarial 4 (lock-in: mark words forever, no-reader disagreement between entries 5 and 7, `BuyPrice` meaning): adopted. Decision 6 states the permanent rule for an old row and makes the job the one writer; a missing reader gives `hand` in entries 5 and 7 alike; `cmd/ot/watch.go:128-155` compares hand values only (Left out).
- Adversarial 5 (claims checked: rounding, wiring, key, currency): adopted. Decision 9 names the one rounding; Decision 2 states the USD-only to contract-currency change; the rest is as in points 1 and 2; the `[stock]` hedge of the old Left out is deleted because the JP path goes through `listedJob` (`wire_optioneod.go:146`).
- Adversarial 6 (slip in parallel: entries 5, 6, 7 disagree on the mark; shared `pages.md`; interface label of entry 9): adopted. One mark rule (Decision 6); only entry 8 edits `docs/contracts/pages.md`; the price lines `Input` entry is gone with the cut.
- Advisor Q1 (collection follows the resolved price): adopted as Decision 3, with the plan-hash behaviour.
- Advisor Q2 (resolve only for the option jobs, start never refuses): adopted as Decisions 4 and 5.
- Advisor Q3 (cut the price lines page work, keep the one false sentence): adopted as entry 12 and Left out.
- Advisor Q4 (one copy of the zone rule, rounding, USD change): adopted as Decisions 2 and 9.
- Advisor blocking 1 (ot wiring Files, opener): adopted, entry 7.
- Advisor blocking 2 (no age, share class, backfill and void rules as Decisions): adopted, Decision 2.
- Advisor blocking 3 (texts made false): adopted, entry 8 item 5; `valuation_page/table.go:47` and its web label stay true (Left out).
- Advisor blocking 4 (raw marks): adopted, entries 8 and 9.
- Advisor blocking 5 (amend the earlier designs): adopted, Decision 12 and the two sentences of the pull request.
- Advisor blocking 6 (hand-mark rule): adopted, Decision 6.

## Left out

- A `buy_source` column or dataset field: rejected. It needs a store type edit (`internal/store/api.go` is at 800 lines, one more line is over the size rule and the ceiling is owner-only), a migration (which holds the auto-deploy and the owner's migrate step) and a dataset revision, for a fact the existing `marks` already carries by the precedent of `eod_fallback`. Taken later if marks are ever filtered by a reader that rejects unknown words.
- Recomputing the source on the page from today's valuation and hand price: rejected. The row's price is the price the run judged; a recomputed source could disagree with the stored price and the tiers. The B_hi line beside the strip shows today's valuation.
- The price lines page showing the used price, the source, a list of valued names without a hand price and an order hint (a schema change and a route change): a later design. Until then the page lists a valued name with no hand price under `unset` as today, and its corrected sentence (entry 12) says the gate reads the hand price only when there is no valuation buy zone.
- Any fallback to the SEC FCF band, the companies page price band or another computed price: not done (owner 2026-11-18); the `companies_page` price line and its `HandPrice` are not touched.
- A conversion for a valuation in another currency, an age rule, a share-class rule and a profile match by instrument id instead of the issuer: not done (Decision 2).
- The sale price: it stays the hand price; a hand sale price below the valuation buy price is not checked (the check of `cmd/ot/watch.go:128-155` is on the two hand values only).
- Removing `Config.ValueLines` of `option_candidates_page` (nothing reads it after entry 8; `cmd/ot` still sets it): a later cleanup with the next change of that file.
- `candidate_text/format.go` (the command line text of the rows) prints the stored marks as they are, including the two source words: its stated rule is "the stored columns in the stored order".
- `internal/pages/valuation_page/table.go:47` and its web label "Hand-typed buy price": the table prints the hand price of `settings.underlying` beside the buy zone and computes nothing from the two, which stays true; `zhColHintBuy` and `zhBuySourceHand` stay true for the same reason.
- A mark or a number for the difference between the valuation price and the hand price, and a button to clear a hand price that a valuation overrides: not asked.
- The underlyings page: its `buy_price` field stays the hand price field.

## Questions for the owner

None. Collection follows the resolved price (Decision 3) by the dispatcher's ruling of 2026-11-19; the owner can still say "b" (collection on the hand price only), which removes Decision 3 and the `option_eod` and `option_open` sentences of entry 7.
