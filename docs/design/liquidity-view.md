# liquidity view: a background block on the macro page (four lines and taps), six more FRED series, the Ease score behind a tap on both pages

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 1

Source: owner 2026-11-13/14: accepted the advisor's liquidity proposal (background only, never a gate), "Q1 b" (the Ease 0-100 score moves behind a tap on both pages, no headline, data kept), "Q2 a" (net liquidity is one tap line labelled "practitioner construct, no evidence it leads prices"), S4 of `series-registry.md` ("liquidity later, background only"), "do not hard-wire the schema; a new series is a registry row". The advisor's research notes (2026-11-14, kept with the dispatcher, not in the repository) are the source of every [V], [S], [NF] tag below. Tags: [V] read on the source's own page; [S] seen only in a search snippet or a secondary page; [M] memory, not checked; [O] owner decision; [T] trial value, to be settled by a replay; [NF] looked for, not found.

Goal: the macro page (not the home page) gets a block titled as background with four one-sentence lines (reserves, money market, credit, conditions) and a few taps with numbers only. Six more FRED series are collected for it. Derived figures (change over 4 and 13 weeks, spread of two series, percentile over a printed window, one ratio, net liquidity) are computed in `compute` and nowhere else. No score, no regime, no colour, no threshold except the published zero line of the two indexes. The existing Ease score becomes a tap on the macro page and on the home Macro card: no figure, gauge or band outside a closed fold. For the owner: the macro page shows what funding and credit look like today, and says in its title that it changes no decision; nothing in the put sentence reads it (no tested rule exists, [NF]).

## Today

All paths at origin/main be0e4e4e.

- Stored series (`internal/jobs/fredseries/job.toml`, 16 ids, nine windowed with `lookback_days = 14`, daily 07:30 New York): `DGS10 DGS3MO SP500 VIXCLS VXNCLS RVXCLS VXVCLS WALCL WDTGAL RRPONTSYD DFF BAMLH0A0HYM2 BAMLC0A0CM BAA10Y SOFR WRESBAL`. So WRESBAL (week average ending Wednesday, millions USD), RRPONTSYD (billions USD), WALCL and WDTGAL (the TGA at the Wednesday level; WTREGEN, its week average, is not stored and not needed), BAMLH0A0HYM2 (the high-yield OAS, the one HY series stored), BAMLC0A0CM (IG OAS) and BAA10Y are stored. Not stored: IORB, SOFR99, NFCI, STLFSI4, TLAACBW027SBOG, WLCFLPCL [V from the advisor's read of the job manifest].
- How the 16 were added (`migrations/0043_series_registry.sql`, `series-registry.md` Decisions 1 and 6): a row in `internal/sources/fred/mapping.go` `seriesUnits` (unit, currency, `shift`, `pinVintage`; a series without a row is refused), a recorded unit text in `internal/sources/fred/testdata/series_units.json` (`TestSeriesUnitsMatchMetadata`), the id in `job.toml` `series` (and `windowed`), and a registry row seeded by a migration (`obs.series_definition`: title, category, frequency; the category check allows `rates, credit, liquidity, volatility, fx, activity`). `internal/jobs/fredseries/seed_test.go` `TestRegistrySeedCoversManifest` reads only `0043_series_registry.sql` and asserts every manifest id appears in it. `run_test.go:183` pins 16 keys and `plan_test.go` the plan hash. A series with no registry row still stores (the writer inserts a definition with defaults), but is unclassified.
- NY Fed second source (`internal/sources/nyfed`, `docs/contracts/sources.md:80`): `SOFR` only, a backup that fills gaps and never replaces a value. The new series have none.
- The series writer inserts a definition (unit, currency) for any stored series, `ON CONFLICT DO NOTHING` (`internal/store/series/writer.go:33`); title, category and frequency are read only by `registry.go` `ActiveSeries`, which no job plans from (`series-registry.md` Left out). `internal/store/series` upserts by `(series_id, observation_date)` (migration 0043); a forced past day on the `fred_series` console row repairs a series. `ot series-load --series ID --from DAY --to DAY` loads history once (`internal/jobs/serieshistory`, at most 5 years, insert-only) (`docs/contracts/ot.md:28`).
- FRED keeps only 3 years of observations of BAMLH0A0HYM2 and BAMLC0A0CM since April 2026 [V]: a percentile over a long window can only be built from our own stored history.
- Macro page: `internal/pages/macro_page` (`builder.go` 276 lines, `ease.go`, `page.go`) writes `page.macro_page.v1` (`contracts/schemas/page.macro_page.v1.json`, a carried copy listed in `carryover.sha256`, so it stays byte-identical). Its package doc says it "only formats stored values; it computes nothing". It reads series through `store.SeriesReader` (`Build(ctx, r, c, day, generatedAt)`), formats only, and reads the stored `macro.ease` reading through `compute.ReadingFunc`. `cmd/ot/wire.go` (796 lines) builds it in `newMacroPage`, with one indicator (`DGS10`). Its `ease` is a top-level object (`Ease`, `schema` line 3 of the v1 file) shown by `web/src/pages/macro/MacroPage.tsx` as the card "Ease" with a 2xl figure, and `sentence` says "ease and events: no data yet".
- Home page: `page.home_page.v3` composes `page.macro_summary.v2`; the Macro card (`web/src/pages/home/MacroCard.tsx`) shows `card.say` as its description and the Ease figure and gauge as its first block. `internal/pages/home_page/overview.go:80` copies `card.say` into the Overview line of the home page. `internal/pages/home_macro/member.go` writes `say` as `sayEase` = "Ease %s, %s." filled with the band word and the week change, in fact "Ease neutral, vs last week −0.2." (`home_macro/testdata/ok.golden.json:17`; no score figure, but a week change outside any fold), or "Ease: no data yet." (`sayNoEase`). `home_macro.Build(ctx, c, day, at)` has `Config.Read` (readings) and no series reader; `home_market.Build` takes one (`cmd/ot/home_site.go:146`). The `Ease` def is at `page.home_page.v1.json:405` and in the macro page v1 file; the shape is the same in all.
- Compute: pure figures live in `internal/compute/**` behind interfaces in declarations-only packages (`blockapi`, `trackapi`, ...); `internal/compute/api.go` stands at 839 lines against its ceiling 840 in `docs/design/ratchet.json` and does not grow. The stored net liquidity of the Ease (`internal/compute/ease/netliquidity.go`: WALCL minus WDTGAL minus RRPONTSYD) is private to the Ease today; entry 1 adds one exported function to it (Decision 8) and changes nothing else there. `compute.Series` is ascending by day, one `float64` per day (`compute.Point`); `compute.FigureValue` is decimal text or a gap.
- `internal/contracts/fixtures_test.go` maps `web/fixtures/macro-page` to the v1 schema; `web/test/shell/pages.test.tsx:111` and `cmd/ot/pagereg_test.go:85` list the page names only (no version); `cmd/ot/pagelist_test.go:96` expects `schema_version` 1 and `page.macro_page.v1`; `web/src/types/macro_page.ts` supports version 1 only and is a hand-written placeholder over `macro_page.v1.types.ts`; the web tests already read `contracts/fixtures` in places (`web/test/pages/flex_import/view.test.tsx`).
- `scripts/hygiene/main.go:143-156` fails a file under a fixtures directory that the directory's manifest does not list, and `internal/contracts/page_schema_test.go:107` `TestFixtureManifest` checks its hashes: `contracts/fixtures/MANIFEST.synthetic-v1.json`. `web/scripts/generate-types.mjs:23-34` regenerates only files that already exist in `web/src/generated` plus the names given on its command line.
- `cmd/ot/page_test.go:226` compares the macro_page demo output with `testdata/macro_page_demo.golden.json` and rewrites it under the `-update` flag of that file.

## Decisions

1. Background, never a gate [O, advisor]. No source tests a funding, reserve or financial-conditions measure against equity drawdowns or volatility in a form a put seller can use [NF]; Adrian, Boyarchenko and Giannone (AER 2019) tie the NFCI to the lower tail of GDP growth, not to equity returns [V via search summary]. So: the block title says "background", it states "changes no decision", and no other page, reading, rule or sentence reads these figures. Nothing in this design touches the put sentence.
2. The block (macro page only). Title "Funding and credit background". Four lines in a fixed order, each one sentence (English, `ui-english.md`); every figure in a sentence carries its own date. Examples, not data:
   - Reserves: "Bank reserves were $2.95tn in the week to Nov 11, up $48bn over 4 weeks and down $112bn over 13 weeks; ON RRP is $0.3bn on Nov 13." (WRESBAL, RRPONTSYD)
   - Money market: "SOFR was 3.92% on Nov 12, 2.0 bp above IORB 3.90%; its 99th percentile is 3.98%." (SOFR, IORB, SOFR99). A signed number, no threshold, no colour: no official bp threshold exists [NF]; the NY Fed found SOFR well above IORB when reserves were scarce, but dealer payment delays also spike SOFR [S].
   - Credit: "High-yield spread is 3.12% on Nov 12, at the 41st percentile of our stored history since 2023-11-14 (780 days); investment grade 0.89%; Baa minus 10-year 1.62%." (BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y). No published threshold [NF]: level and percentile only; the window is printed beside the percentile.
   - Conditions: "Chicago Fed NFCI is -0.46 for the week to Nov 6; above 0 means tighter than average." (NFCI.) The zero line is the FRED series note [V]; the Chicago Fed's own page does not state it [V].
3. Taps, numbers only, one row per number with its date (a tap is a closed fold, `Accordion` as `DataTimesFold`; the page sends every string):
   - `stlfsi`: St. Louis Fed Financial Stress Index 4 (STLFSI4), "above 0 = above-average stress" (zero is the average since late 1993) [V]; STLFSI3 is discontinued, STLFSI4 is current [V].
   - `reserves_over_assets`: WRESBAL divided by TLAACBW027SBOG (all commercial banks' assets), percent. Reserves above about 12-13% of bank assets are called abundant in an NY Fed paper [S, abstract snippet]; the number carries no label and no threshold here.
   - `discount_window`: WLCFLPCL (primary credit at the Fed, Wednesday level). No threshold [NF].
   - `net_liquidity`: WALCL minus WDTGAL minus RRPONTSYD at the newest Wednesday that has all three, one row, note "Practitioner construct, no evidence it leads prices." [O "Q2 a"]. Used by practitioner pages only; no Fed or academic definition or test was found [NF], and the 2022-26 ON RRP drain coincided with equity gains [S].
4. The Ease score [O "Q1 b"]. The `ease` object, its stored reading and its data are unchanged. On the macro page and on the home Macro card the web view shows it only inside a closed fold ("Ease score"; everything the object carries, including its gap badge, ticks, neutral band, `reading_text` (the band word and the week change) and `week_change`, is inside), with no figure or gauge outside it. The macro page gets the attribute `data-ease-figure` on its figure (the home card has it today), which the tests use. The home card's `say`, which the Overview line of the home page copies (`overview.go:80`), stops quoting the Ease and becomes a dated fact of the block [advisor]: reserves with the 4-week change, in the form "Reserves $3.0tn, 4 weeks −$40bn (2026-11-11)." (level in tn with one decimal from $1tn, else bn with none; change in bn, signed with `−`; the day is the reserves day). `home_macro` gets the series through a new `Config.Series` (`store.SeriesReader`), calls the same `liquidityapi.Func` with the WRESBAL series only and formats the `Reserves` and `Reserves4w` figures of the result. A missing or stale input gives "Reserves: no data yet." and a card gap `no_data` (the gap shape of `home_macro` today), never a number. The week change and the band words stay in the Ease fold only. No schema changes for this: `say` stays a required string of `page.macro_summary.v2`, the Ease is the existing `ease` object, so `page.home_page.v3` and `page.macro_summary.v2` are untouched. The macro page `sentence` becomes "Funding and credit background from stored series; events: no data yet." (no consumer copies it).
5. Six series are added (FRED ids; units are what the FRED page says; all `pinVintage` (as the 16: after `0043` this only sets the realtime fields to the observation date, a revised value inside the lookback still replaces the stored one by the key), windowed with the existing 14-day lookback):
   - `IORB` (interest on reserve balances, percent, daily) [V, last 3.90]; `SOFR99` (SOFR 99th percentile, percent, daily) [V, last 3.98];
   - `NFCI` (Chicago Fed National Financial Conditions Index, weekly, ending Friday, published Wednesday; unit "Index", stored as `index_points`) [V]; `STLFSI4` (weekly, "Index", `index_points`) [V];
   - `TLAACBW027SBOG` (all commercial banks: total assets, weekly Wednesday level, billions USD, `shift` 9) [M: id and unit text from the owner's list, not read]; `WLCFLPCL` (primary credit, weekly Wednesday level, millions USD, `shift` 6) [M, same].
   The units of the last two are read on the FRED pages by the sources entry (acceptance 3); a difference is reported in its pull request, not guessed. No registry row is written by this design (Decision 6). No second source: the `nyfed` source answers `SOFR` only; extending it to `SOFR99` is Left out. The NFCI is revised weekly, more near month starts [V]: the page shows the value stored, the newest one fetched while the date was inside the 14-day lookback; a later revision is picked up by a forced run of that day (`fred_series` console row), as for the other series. The "above 0" reading therefore can differ from FRED's current series for an old week; for background this is accepted and named here.
6. Adding them (the mechanism of the 16 minus the registry seed, no migration): entry 3 (a `seriesUnits` row each, a recorded unit text each), entry 4 (`job.toml` `series` and `windowed`, 22 ids). The writer inserts the definition of a stored series with its unit and currency, so the series is stored and readable without a registry row; only title, category and frequency stay empty (an unclassified row is visible, not an error: `series-registry.md` Decision 1), and nothing reads them (Left out). So: no migration, no deploy hold, no migration number to race for. `seed_test.go` would fail on the six ids (it requires every manifest id in `0043`): entry 4 gives it an explicit list of the six unregistered ids it skips, named in the test. Entry 3 lands before entry 4 (a manifest id without a mapping row fails the whole run).
7. Where figures are computed. A new package `internal/compute/liquidity` (pure, no I/O, no clock) behind a declarations-only package `internal/compute/liquidityapi` (`Func`), the same shape as `techstate` behind `blockapi`. The macro page builder reads the series through `store.SeriesReader` (as it reads its indicators), calls the `Func` it is given and formats the result as text; `cmd/ot` gives it `liquidity.Compute`. This is compute-on-build, not a stored reading: `compute-figures.md` stores each figure as a reading with a register id, but the page already has this precedent (`blockapi`, `trackapi`: pages call a compute `Func` on the series or rows they read), and the package doc of `macro_page` is changed to say so ("formats stored values and the figures its `Liquidity` function returns; it computes nothing itself"). Why not reading rows: the figures are background, no rule, alert or replay reads them, so a register entry, a rule version, a `compute_figures` step and a manifest key would be four more entries for a history nobody uses; a later replay can promote one figure to a reading without changing the page. The cost is owned: every build reads 14 series (13 of them over a 400-day window, the high-yield series from the 2000 floor, about 3 years of rows today, Decision 8) and recomputes; the same stored data gives the same page, so a rerun any day is idempotent. Computed by `compute` and nowhere else: change over 4 and 13 weeks, SOFR minus IORB, percentile over a printed window, WRESBAL over bank assets, net liquidity. The page and the web view add, compare and rank nothing; rounding for display is the only arithmetic of the page.
8. The figures [T unless said]. All arithmetic is exact decimal (`math/big` over the shortest decimal text of each `float64`, as `compute/ease` does); results are decimal text.
   - Level: the newest point on or before `Day`, with its own day. Stale when older than `DailyMaxAgeDays` = 4 (the freshness rule of `macro_page.go`, `freshnessLimitDays` [V]) for a daily series, `WeeklyMaxAgeDays` = 14 for a weekly one (the NFCI is dated Friday and published the next Wednesday, so up to 12 days old); a stale or absent level is the gap `stale_input` or `no_data` of that figure, never a zero.
   - Change over N weeks (N = 4 and 13): newest value minus the value exactly `7*N` days earlier (the weekly series are dated on one weekday); no point on that day is `missing_input`. 13 weeks is the window of the stored Ease (`ease-v2.md` Decision 3 [V]); the 4-week window is common practice, no Fed rule.
   - Spread: SOFR minus IORB in basis points at the newest SOFR day on which IORB also has a point (same day, one day, printed); a different day is not used; stale (`stale_input`, no number) when `Day` minus that day exceeds `DailyMaxAgeDays`.
   - Percentile (HY only): over all stored points of the series up to `Day`, the share of days whose value is at or below the newest value, in percent, 0 decimals; the result carries the first stored day, the last and the count, and is `short_history` under `PercentileMinDays` = 250. The page prints "since <first day> (<count> days)". The window is whatever is stored: FRED serves about 3 years of BAMLH0A0HYM2 [V], so unless the daily job stored it earlier the window stays about 3 years, and the printed start and count say so; the page reads the series from the floor day 2000-01-01.
   - Ratio: WRESBAL over TLAACBW027SBOG on the same day, percent, 2 decimals (the two are in whole USD after `shift`); stale when `Day` minus that day exceeds `WeeklyMaxAgeDays`. The discount-window level has the weekly rule as every weekly level.
   - Net liquidity: not a second implementation. `compute/ease/netliquidity.go` (`weeklyNetLiquidity`, WALCL minus WDTGAL minus RRPONTSYD on one Wednesday) gets one exported function `NetLiquidity(walcl, wdtgal, rrp compute.Series, asOf contracts.Day) liquidityapi.Num` (level in whole USD with its day, by the Ease's own day rule), and the tap shows that value, so the tap and the Ease input cannot disagree by date. The result has a second data day, the RRP day (`Num.Aux`), because the Ease's rule takes the newest RRP within the 7 days up to the Wednesday: the tap row names both ("on Nov 5, RRP of Nov 3"). `liquidity.Compute` applies the age rule to it: stale (`stale_input`, no number) when `Day` minus the Wednesday exceeds `WeeklyMaxAgeDays`; it otherwise uses the value as returned. `liquidity.Compute` receives it as `Input.Net` (a `liquidityapi.NetFunc`); `cmd/ot` passes `ease.NetLiquidity`; `liquidity` does not import `ease`.
9. Contract. `page.macro_page.v2` (new file; v1 stays) is the macro page: every field of v1 unchanged, plus `liquidity_title` (string) and `liquidity` (`Liquidity` or `Gap`, a gap `no_reader` is allowed by the schema; the builder writes `liquidity` only when it has a `Func`, see Decision 13). The lines and rows carry their own gaps; the top-level `gaps` array of the page (at most 20, shared with the macro-events design) is not used by the block, and when both designs write to it, whichever lands second merges the gaps by `reason` (one entry per reason). It is a v2-line file (conventions of `page-schemas.md`: `schema_version` const 2, shared defs `Gap`, `Id`, `SourceTime`, `StaleLabel`, `Timestamp` by `$ref` to `common.v1.json`, as `page.macro_summary.v2` does). Fields in `## Interface`. The builder writes `Meta{macro_page, page.macro_page.v2}`.
10. Front end. It renders what the page sends: the four sentences, the taps as folds, the Ease as a fold. It computes nothing and formats nothing; no number is parsed.
11. Missing-input reason. `compute.Reason` has no `missing_input` and `internal/compute/api.go` (839 of 840) takes no new constant: `liquidityapi` declares `const MissingInput compute.Reason = "missing_input"`. The page maps the reasons to its gap reasons by one table: `no_input` to `no_data`, `missing_input`, `short_history`, `zero_range` and `bad_input` to `missing_input`, `stale_input` to `stale_input`.
12. History. A new series starts with 14 days. After the jobs entry merges, the window runs `ot series-load` once for `IORB SOFR99 NFCI STLFSI4 TLAACBW027SBOG WLCFLPCL WRESBAL` (and `BAMLH0A0HYM2` if its stored history is under 250 days) for the longest span FRED gives up to 5 years (owner allows `ot` commands run by the window). Until then the changes and the percentile are named gaps (`missing_input`, `short_history`), which is the design's behaviour, not an error.
13. Deploy and order. No migration: nothing holds the auto-deploy. Entry 3 merges before entry 4. The web entry (5) accepts both versions of the macro page and renders the block when present, so it can merge before any producer writes v2. The page entry (6) is additive: `macro_page.Config` and `home_macro.Config` gain the liquidity fields, and with their zero value (no `Liquidity` `Func`) the packages write exactly what they write today (`page.macro_page.v1`, schema version 1, no `liquidity` fields; the old `say`), so the v1 tests and the demo golden of `cmd/ot` stay green on entry 6 alone and no entry waits for an unmerged sibling. The composition root (entry 7) is the one step that switches: it wires the `Func`s and moves its tests and the golden to v2. Removing the v1 output from the page packages and from the web is Left out (a later step once nothing writes v1).
14. Funding-calendar dates (quarter-ends, tax dates) belong to the macro-events design and are not here.

## Modules

`docs/design/modules.md` row `compute`: the entry cell gains `internal/compute/liquidityapi` (declarations only, like `blockapi`); this design's commit makes that edit. No other row changes. Entries touch compute (`internal/compute/**`, `docs/contracts/compute.md`; entry 1), contracts (`contracts/**`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md`; entry 2), sources (entry 3), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entry 4), web (`web/**`; entry 5), pages (`internal/pages/**`, `docs/contracts/pages.md`; entry 6) and ot (`cmd/**`; entry 7). New import edges, all inside the map: `internal/pages/macro_page` and `internal/pages/home_macro` import `internal/compute/liquidityapi` (pages depends on compute); `internal/compute/liquidity` imports `internal/compute`, `internal/compute/liquidityapi` and `internal/contracts`; `cmd/ot` imports `internal/compute/liquidity` and `internal/compute/ease` (already). `internal/compute/ease` imports `internal/compute/liquidityapi` (declarations only).

## Interface

Two interface changes, entries 1 and 2 (entry 1 also adds one exported function to `internal/compute/ease`, an implementation package).

Entry 1, module compute, the new declarations-only package `internal/compute/liquidityapi` (`api.go`, `api_test.go`; `internal/compute/api.go` is not edited):

    // Params are the trial constants of the figures. liquidity.TrialParams is their one source.
    type Params struct {
        DailyMaxAgeDays   int // 4: a daily level older than this many calendar days is stale
        WeeklyMaxAgeDays  int // 14: the same for a weekly level
        PercentileMinDays int // 250: fewer stored days is short_history
    }

    // Input is everything the figures need. Day is the as-of day. Each series is ascending by day (compute.Series),
    // nil when none is stored; values are in the unit the store holds (percent as percent, money in whole USD).
    type Input struct {
        Day contracts.Day
        WRESBAL, RRPONTSYD, WALCL, WDTGAL, TLAACBW027SBOG, WLCFLPCL compute.Series
        SOFR, IORB, SOFR99                                          compute.Series
        BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y                            compute.Series
        NFCI, STLFSI4                                               compute.Series
        Net                                                         NetFunc // ease.NetLiquidity
        Params                                                      Params
    }

    // NetFunc is the net liquidity level on the newest day it can be had at or before asOf, in whole USD;
    // internal/compute/ease.NetLiquidity is its one implementation (the Ease's own).
    type NetFunc func(walcl, wdtgal, rrp compute.Series, asOf contracts.Day) Num

    // Num is one figure: Value is decimal text (empty when Reason is set), Day its own data day. Reason is empty or
    // one of compute's no_input, stale_input, short_history, bad_input, or missing_input.
    type Num struct {
        Value  string
        Day    contracts.Day
        Aux    contracts.Day // a second data day, the RRP day of net liquidity; empty otherwise
        Reason compute.Reason
    }

    // MissingInput is the reason of a figure whose second input has no point; compute.Reason has no such value
    // and internal/compute/api.go cannot grow.
    const MissingInput compute.Reason = "missing_input"

    // Change is a difference over Weeks weeks; Day is the newest day and From the day it is taken against.
    type Change struct {
        Num
        Weeks int
        From  contracts.Day
    }

    // Rank is the percentile of the newest value in the stored history: Value 0 to 100 as text, First and Day the
    // first and last stored day of the window, Days the number of days in it.
    type Rank struct {
        Num
        First contracts.Day
        Days  int
    }

    // Result holds every figure of the block and its taps. SofrIorbBP is in basis points, one decimal;
    // ReservesOverAssets in percent, two decimals; NetLiquidity in whole USD.
    type Result struct {
        Reserves, RRP                                           Num
        Reserves4w, Reserves13w                                 Change
        SOFR, IORB, SOFR99, SofrIorbBP                          Num
        HighYield, InvestmentGrade, Baa10Y                      Num
        HighYieldRank                                           Rank
        NFCI, STLFSI4                                           Num
        ReservesOverAssets, DiscountWindow, NetLiquidity        Num
    }

    // Func computes the Result; no I/O, no clock (internal/compute/liquidity.Compute).
    type Func func(Input) Result

Entry 2, module contracts, `contracts/schemas/page.macro_page.v2.json` (title `MacroPageV2`) and its generated type `web/src/generated/macro_page.v2.ts`. Top level: the fields of v1 and, new and required, `liquidity_title` (string, 1 to 200) and `liquidity`. Defs new in this file:

    liquidity     = Liquidity | Gap(common.v1)
    Liquidity     = { background_text: string (1..200),            // "Background only: nothing here changes a put decision."
                      lines: LiquidityLine[4],                      // minItems = maxItems = 4, order reserves, money_market, credit, conditions
                      taps:  LiquidityTap[0..4] }
    LiquidityLine = { key: "reserves"|"money_market"|"credit"|"conditions", label: string,
                      sentence: string (1..200) | Gap, source_text: string }   // e.g. "FRED WRESBAL, RRPONTSYD"
    LiquidityTap  = { key: "stlfsi"|"reserves_over_assets"|"discount_window"|"net_liquidity", label: string,
                      rows: LiquidityRow[1..4], note: string | null }
    LiquidityRow  = { label: string, text: string | Gap, data_day: date text | null }

`Gap` reasons used: `no_data`, `missing_input`, `stale_input`, `no_reader`. Every other field, the `Ease` def and the envelope are the v1 text.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. No migration, nothing holds the auto-deploy. Every entry has its own `Contracts:` line: entry 2 alone changes a file under `contracts/schemas` (with its generated type); all others say `Contracts: unchanged`. Order: 1, 2 and 3 first; 4 after 3; 5 after 2; 6 after 1, 2 and 5; 7 after 1 and 6. Test runs are in `## Checked by CI`.

### 1 liquidity-view-compute (interface)

Module: compute. Size: M.
Order: first.
Files: `internal/compute/liquidityapi/api.go`, `internal/compute/liquidityapi/api_test.go`, `internal/compute/liquidity/liquidity.go`, `internal/compute/liquidity/rank.go`, `internal/compute/liquidity/liquidity_test.go`, `internal/compute/liquidity/testdata/liquidity.json`, `internal/compute/ease/netliquidity.go`, `internal/compute/ease/netliquidity_test.go`, `docs/contracts/compute.md`
Contracts: unchanged
Design: docs/design/liquidity-view.md#Interface
Acceptance:
1. `liquidityapi/api.go` declares `Params`, `Input`, `Num` (with `Aux`), `Change`, `Rank`, `Result`, `NetFunc`, `Func` and the constant `MissingInput` of `compute.Reason` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of its package; `grep -c 'internal/compute/liquidityapi' docs/design/modules.md` prints 1 (the line is already on BASE); `git diff --name-only BASE -- internal/compute/api.go internal/compute/api_test.go docs/design/ratchet.json` prints nothing.
2. `liquidity.go` has `func Compute(in liquidityapi.Input) liquidityapi.Result` and `TrialParams() liquidityapi.Params` returning 4, 14 and 250; `rank.go` holds the percentile. Neither file imports `time` except to parse a day, none reads a clock, none imports a store, a job, a page or `os`; exact decimal arithmetic only (`math/big`), no `float64` operation on a value.
3. The figures are as Decision 8: level with stale limits 4 and 14 days (a stale figure is the reason `stale_input`, never a number); the age rule of Decision 8 for the spread, the ratio, the discount window and the net liquidity (its `Num.Day` is the Wednesday, `Num.Aux` the RRP day); change over `7*N` days exactly (`missing_input` when that day has no point); spread SOFR minus IORB in bp on the newest SOFR day where IORB has a point on the same day, one decimal half-even; percentile as the share of stored days at or below the newest value, 0 decimals, with `First`, `Day` and `Days`, `short_history` under `PercentileMinDays`; ratio WRESBAL over TLAACBW027SBOG on one day, two decimals; net liquidity is `Input.Net`'s value as returned (no arithmetic of its own). A missing figure has an empty `Value` and a `Reason`; no figure is ever 0 for lack of data.
4. `testdata/liquidity.json` is hand-written (invented numbers, labelled so in the file) and read by new tests in `liquidity_test.go` (all `t.Parallel()`): `TestLiquidityLevelsStale` (new: a daily level of 5 days old and a weekly one of 15 days are `stale_input`, 4 and 14 days are not); `TestLiquidityChangeExactDays` (new: 4 and 13 weeks, and a missing day gives `missing_input`); `TestLiquiditySofrIorbSameDay` (new: bp with one decimal, a sign test both ways, IORB with no point on the day gives `missing_input`); `TestLiquidityRankWindow` (new: the percentile, the first day and the count, 249 days give `short_history`, a tie counts as at or below); `TestLiquidityReservesRatio` (new); `TestLiquidityNetIsFromNetFunc` (new: the result's `NetLiquidity` is what a fake `NetFunc` returns, and a nil `NetFunc` gives the reason `no_input`); `TestLiquidityEmptyInput` (new: every figure is a gap with a reason, nothing panics); `TestLiquidityNetStale` (new: a Wednesday older than 14 days gives `stale_input` and no value; the same day is not stale at 14); `TestLiquiditySpreadStale` (new: a SOFR day older than 4 days); `TestLiquidityRatioStale` (new); `TestLiquidityDiscountStale` (new); `TestLiquidityTrialParams` (new).
5. `internal/compute/ease/netliquidity.go` gains one exported `NetLiquidity(walcl, wdtgal, rrp compute.Series, asOf contracts.Day) liquidityapi.Num` built on the existing `weeklyNetLiquidity` (the ease's own day rule, with the RRP day in `Aux`; `git diff BASE -- internal/compute/ease/netliquidity.go` removes no line of the existing functions); `netliquidity_test.go` has a new test `TestNetLiquidityLevelMatchesEasePart` (new) showing its day and value equal those the 13-week part of the Ease uses for the same series; `liquidity` imports no `internal/compute/ease`.
6. `api_test.go` has a new contract test `TestLiquidityFuncContract` (`t.Parallel()`): a fake `Func` is called with an `Input` and the caller reads every field of `Result`; a second new test checks that `liquidity.Compute` is assignable to `liquidityapi.Func`.
7. `docs/contracts/compute.md` names the package, `Func`, the five derived figures and that nothing is stored as a reading.
8. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.

### 2 liquidity-view-contract (interface)

Module: contracts. Size: M.
Order: first.
Files: `contracts/schemas/page.macro_page.v2.json`, `contracts/fixtures/page.macro_page.v2.golden.json`, `contracts/fixtures/page.macro_page.v2.gap.json`, `contracts/fixtures/page.macro_page.v2.stale.json`, `contracts/fixtures/page.macro_page.v2.bad-version.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `web/src/generated/macro_page.v2.ts`, `internal/contracts/macropage_v2_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.macro_page v2 (new; the schema and its generated type land together)
Design: docs/design/liquidity-view.md#Interface
Acceptance:
1. `page.macro_page.v2.json` has title `MacroPageV2`, `schema_version` const 2 (integer), and differs from `page.macro_page.v1.json` only in that, the schema id, the defs `Liquidity`, `LiquidityLine`, `LiquidityTap`, `LiquidityRow` of the Interface section, the properties `liquidity_title` and `liquidity` (both required) and the use of `common.v1.json` `$ref`s for `Gap`, `SourceTime`, `StaleLabel` as `page.macro_summary.v2.json` does; `page.macro_page.v1.json` and `carryover.sha256` are byte-identical to BASE (`git diff --name-only BASE -- contracts/schemas/page.macro_page.v1.json contracts/schemas/carryover.sha256` prints nothing).
2. `lines` has `minItems` and `maxItems` 4 and `key` the four-value enum; `taps` has `maxItems` 4 and `key` the four-value enum; a `LiquidityRow` has `data_day` as a date pattern or null; `additionalProperties` is false on every new def; no `ROUND_HALF_UP`, every `x-timezone` is `owner` (the v2-line conventions of `page-schemas.md`).
3. The four fixtures are written by hand (invented data, said so in the line `page-schemas.md` adds): `golden` has four sentences, four taps (`net_liquidity` with the note "Practitioner construct, no evidence it leads prices."), the Ease object and the title "Funding and credit background"; `gap` has `liquidity` as a `no_reader` gap; `stale` has the stale label; `bad-version` has `schema_version` 1 and fails only at `/schema_version` with the keyword `const`.
4. `macropage_v2_test.go` has new tests (`t.Parallel()`) in the form of `macrorates_v3_test.go`: `TestMacroPageV2Fixtures` (new: golden, gap and stale validate; bad-version fails at `/schema_version` const) and `TestMacroPageV2LiquidityShape` (new: a payload with three lines, one with five, one with a tap key `score`, and one with a line `key` of `regime` each fail at the named location).
5. `web/src/generated/macro_page.v2.ts` exists, is the output of `pnpm --dir web run generate:types -- page.macro_page.v2` (the generator only creates a file named on its command line; banner kept), exports `interface MacroPageV2`, and running that command again leaves it unchanged; `git diff --stat BASE` shows `web/package.json`, `pnpm-lock.yaml`, every v1 file and `internal/contracts/fixtures_test.go` absent.
6. `MANIFEST.synthetic-v1.json` lists the four new fixtures with their sha256 (`TestFixtureManifest` of `page_schema_test.go` and the hygiene check of `scripts/hygiene/main.go` name no unlisted fixture).
7. `docs/contracts/page-schemas.md` adds the row `page.macro_page.v2 | 2` and one line naming this design; `git diff --name-only BASE` lists only paths of the Files line.

### 3 liquidity-view-sources

Module: sources. Size: S.
Order: first.
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/mapping_test.go`, `internal/sources/fred/testdata/series_units.json`, `docs/contracts/sources.md`
Contracts: unchanged
Design: docs/design/liquidity-view.md#Issues (Decision 5)
Acceptance:
1. `mapping.go` `seriesUnits` has six new rows, each with `pinVintage: true`: `IORB` and `SOFR99` (unit `percent`), `NFCI` and `STLFSI4` (unit `indexPoints`), `TLAACBW027SBOG` (`ccyAmount`, currency `USD`, `shift: 9`) and `WLCFLPCL` (`ccyAmount`, currency `USD`, `shift: 6`); no other row changes (`git diff BASE -- internal/sources/fred/mapping.go` shows only six added table lines and, if needed, a comment line).
2. `mapping_test.go` has a new test per added row that maps a recorded observation of the series (inline in the test) to the unit, currency, scaled value and pinned realtime fields.
3. `testdata/series_units.json` gets six entries, each with a `note` saying the units text was read on the FRED series page on the day the issue was worked (`IORB` and `SOFR99` "Percent", `NFCI` and `STLFSI4` "Index", and the two bank series as the page says: expected "Billions of U.S. Dollars" and "Millions of U.S. Dollars"); the pull request description states the text read for the last two and, if it differs from the expectation, the unit row follows the page and the difference is named there.
4. `docs/contracts/sources.md` lists the six ids with their units.
5. `git diff --name-only BASE` lists only paths of the Files line.

### 4 liquidity-view-job

Module: jobs. Size: S.
Order: after 3.
Files: `internal/jobs/fredseries/job.toml`, `internal/jobs/fredseries/run_test.go`, `internal/jobs/fredseries/seed_test.go`, `docs/contracts/jobs/fred_series.md`
Contracts: unchanged
Design: docs/design/liquidity-view.md#Issues (Decisions 5, 6)
Acceptance:
1. `job.toml` `series` and `windowed` gain `IORB`, `SOFR99`, `NFCI`, `STLFSI4`, `TLAACBW027SBOG`, `WLCFLPCL`; `lookback_days`, `[window]` and `[schedule]` are unchanged.
2. `run_test.go` expects 22 keys at the line that expects 16 today (`plan_test.go` builds its own synthetic manifests and is not edited).
3. `seed_test.go` `TestRegistrySeedCoversManifest` keeps reading `0043_series_registry.sql` and skips exactly the six ids of item 1, listed in a variable named `unregistered` with a comment naming this design; a new test `TestUnregisteredAreInManifest` (new) asserts that each id of `unregistered` is in `job.toml` `series` (so the list cannot rot) and none is in `0043_series_registry.sql`.
4. `docs/contracts/jobs/fred_series.md` states 22 series and names the six, and that they have no registry row.
5. No job id is added (`fred_series` stays one job with more series), so the job-count tests of `cmd/ot` (`jobconsole_jobs_test.go`) are not edited: `git diff --name-only BASE -- cmd` prints nothing.
6. `git diff --name-only BASE` lists only paths of the Files line; `git diff --name-only BASE -- migrations` prints nothing.

### 5 liquidity-view-web

Module: web. Size: M.
Order: after 2.
Contracts: unchanged
Files: `web/src/pages/macro/MacroPage.tsx`, `web/src/pages/macro/LiquidityBlock.tsx`, `web/src/pages/home/MacroCard.tsx`, `web/src/types/macro_page.ts`, `web/test/pages/macro/MacroPage.test.tsx`, `web/test/pages/macro/LiquidityBlock.test.tsx`, `web/test/pages/home/MacroCard.test.tsx`
Design: docs/design/liquidity-view.md#Issues (Decisions 3, 4, 10, 13)
Acceptance:
1. `web/src/types/macro_page.ts` supports both versions (`SUPPORTED_VERSIONS = [1, 2]`) and `Payload` is the union of the existing v1 type and `MacroPageV2` from `@/generated/macro_page.v2`; a v1 payload still validates and renders as today (without the block).
2. `LiquidityBlock.tsx` (new) renders `liquidity_title`, `background_text`, the four `sentence` strings in order with their `label` and `source_text`, and each tap as a closed `Accordion` item whose trigger is the tap `label`, whose content is its rows (`label`, `text`, `data_day`) and its `note` when not null; a `Gap` renders its `text`. It imports no formatter, parses no number and has no colour class chosen from a value (`git grep -n "parseFloat\|Number(\|toFixed\|Intl" web/src/pages/macro/LiquidityBlock.tsx` prints nothing).
3. `MacroPage.tsx` renders the block before the events when the payload is version 2 and `liquidity` is not a gap (a gap renders its text), and renders the Ease card inside a closed `Accordion` item titled by `ease_title` for both versions, with the attribute `data-ease-figure` on the figure (it has none today); the figure, the gauge and the neutral band are inside the fold only (a new test shows the closed page has no `[data-ease-figure]` and the opened one has it).
4. `MacroCard.tsx` (home) renders the Ease block inside a closed `Accordion` item whose trigger text is "Ease score"; `card.say` stays the description (the text it is sent); a gap badge for a missing Ease is shown inside the fold too. `MacroCard.test.tsx` opens the fold before it reads `[data-ease-figure]`, ticks and the neutral band, and has a new test that the closed card contains no ease figure text.
5. `MacroPage.test.tsx` keeps reading `web/fixtures/macro-page` (version 1) and gains new tests that read `contracts/fixtures/page.macro_page.v2.golden.json`, `.gap.json`, `.stale.json` and `.bad-version.json`; `web/fixtures/macro-page` is unchanged and still validated against the v1 schema by `internal/contracts/fixtures_test.go`; `web/test/shell/pages.test.tsx` lists the name `macro_page` only and is not edited (the entry reads it and says in the pull request description if a check there calls `validate`). `LiquidityBlock.test.tsx` has new tests: the four sentences are the fixture strings, a tap opens on click, a `Gap` row shows its text, the `net_liquidity` note is shown inside its fold only.
6. `git diff --name-only BASE` lists only paths of the Files line, and `git diff --name-only BASE -- web/fixtures web/package.json pnpm-lock.yaml` prints nothing.

### 6 liquidity-view-page

Module: pages. Size: L.
Order: after 1, 2, 5.
Contracts: unchanged
Files: `internal/pages/macro_page/liquidity.go`, `internal/pages/macro_page/liquidity_test.go`, `internal/pages/macro_page/builder.go`, `internal/pages/macro_page/builder_test.go`, `internal/pages/macro_page/page.go`, `internal/pages/macro_page/page_test.go`, `internal/pages/home_macro/member.go`, `internal/pages/home_macro/member_test.go`, `internal/pages/home_macro/testdata/reserves.golden.json`, `docs/contracts/pages.md`
Design: docs/design/liquidity-view.md#Issues (Decisions 2, 3, 4, 7, 9, 11)
Acceptance:
1. `macro_page.Config` gains `Liquidity liquidityapi.Func`, `LiquidityParams liquidityapi.Params` and `Net liquidityapi.NetFunc` (put into `Input.Net`); With `Liquidity` nil (the zero value) `NewPage` returns `Meta{Name: "macro_page", Schema: "page.macro_page.v1"}` and `Build` output is byte-identical to BASE for the same input (`git diff --name-only BASE -- cmd contracts` prints nothing and the existing tests of the package pass unedited). With `Liquidity` set, `NewPage` returns `Meta{..., Schema: "page.macro_page.v2"}` and `Build` writes `schema_version` 2 with `liquidity_title` "Funding and credit background" and `liquidity`; no series is read for the block when `Liquidity` is nil.
2. `liquidity.go` (new, under 600 lines) reads the fourteen series of `liquidityapi.Input` through `store.SeriesReader` (the high-yield series from the floor day 2000-01-01, so the window is whatever is stored, the others from 400 days before `Day`), builds `Input`, calls `c.Liquidity` once, and turns the `Result` into the four lines and the four taps of Decision 2 and 3: English sentences with every figure's date, `Rank.First` and `Rank.Days` printed beside the percentile, the net liquidity row naming the Wednesday and the RRP day (`Aux`), no threshold, no colour word, no "tighter/looser" word except in the NFCI sentence's published zero-line clause "above 0 means tighter than average". A figure with a `Reason` makes its sentence or row the gap of the one mapping table of Decision 11 (`bad_input` is mapped there), never a number; an error of a read is an error of `Build`, never a gap. The only arithmetic in the file is rounding for display; `git grep -n "big\." internal/pages/macro_page/liquidity.go` prints nothing and the file calls no function of `liquidity`.
3. The `net_liquidity` tap has exactly one row and its note is the text "Practitioner construct, no evidence it leads prices."; `stlfsi` carries the zero-line clause "above 0 = above-average stress"; no other tap has a note.
4. `builder.go` keeps `ease` and `ease_title` as they are (the Ease data is unchanged), `sentence` and `sentenceEase` become the text of Decision 4, and the package doc of `builder.go` says the page formats stored values and the figures its `Liquidity` function returns and computes nothing itself (Decision 7); `liquidity_test.go` and `builder_test.go` have new tests with a fake `Func` and a fake reader: `TestLiquidityLinesFromResult` (new: every sentence of a full `Result`), `TestLiquidityNetLine` (new: one row naming both days, and the note), `TestLiquidityGapPerFigure` (new: one `Reason` gives one gap and the other lines stay), `TestLiquidityReasonMapping` (new: every reason of the table, `bad_input` included), `TestLiquidityNilFuncKeepsV1` (new: the JSON with a nil `Liquidity` equals the v1 golden bytes and `Meta.Schema` is v1), `TestLiquidityFuncMakesV2` (new), `TestLiquidityReadErrorFailsBuild` (new), `TestLiquidityNoScoreWords` (new: the built JSON of a full result contains none of `score`, `regime`, `signal`, `bullish`, `bearish` outside the `ease` object), `TestBuildV2SchemaVersion` (new). `page_test.go` gains a new test for the v2 `Meta`; its existing tests are not edited.
5. `home_macro.Config` gains `Series store.SeriesReader`, `Liquidity liquidityapi.Func` and `LiquidityParams liquidityapi.Params`; when `Liquidity` is set (and only then) `say` is the sentence of Decision 4 ("Reserves $3.0tn, 4 weeks −$40bn (2026-11-11)." in form) built from the `Reserves` and `Reserves4w` figures of the `Func` result over the WRESBAL series alone; with `Liquidity` set but a nil `Series`, or a figure with a `Reason`, `say` is "Reserves: no data yet." and the card has a `no_data` gap; with `Liquidity` nil `say` is today's sentence (`sayEase`, `sayNoEase`, `weekSay` stay); `ease`, its data and its fold fields are unchanged. `member_test.go` has the new tests `TestSayIsReservesFact` (new), `TestSayMissingReserves` (new: no series, a stale figure, a missing 4-week day) and `TestSayHasNoEaseWords` (new: with `Liquidity` set `say` contains neither "Ease" nor "vs last week"), `TestSayUnchangedWithoutLiquidity` (new), and the existing tests and `testdata/ok.golden.json` are not edited (a new golden `testdata/reserves.golden.json` holds the wired output; it is added to the Files line); `internal/pages/home_page` and the schemas `page.home_page.v3` and `page.macro_summary.v2` are unchanged (`git diff --name-only BASE -- internal/pages/home_page contracts` prints nothing).
6. `docs/contracts/pages.md` states `page.macro_page.v2` (the line that names v1 is changed), the `Config` fields of both pages and that `liquidity` is a gap without a `Func`.
7. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.

### 7 liquidity-view-wire

Module: ot. Size: S.
Order: after 1, 6.
Contracts: unchanged
Files: `cmd/ot/wire_macro.go`, `cmd/ot/wire.go`, `cmd/ot/home_site.go`, `cmd/ot/wire_macro_test.go`, `cmd/ot/home_site_test.go`, `cmd/ot/page_test.go`, `cmd/ot/pagelist_test.go`, `cmd/ot/testdata/macro_page_demo.golden.json`
Design: docs/design/liquidity-view.md#Issues (Decision 7)
Acceptance:
1. `newMacroPage` (called by `builtPages` of `pagelist.go` through `deps.macroPage`) moves unchanged from `wire.go` to the new `wire_macro.go` and gains `Liquidity: liquidity.Compute`, `LiquidityParams: liquidity.TrialParams()` and `Net: ease.NetLiquidity`; entries 6 and the zero value mean this is the step that makes the page v2; `wire.go` loses the function and, if nothing else uses it, the `macro_page` import, so it is shorter than 796 lines (`git diff --numstat BASE -- cmd/ot/wire.go` shows no added line other than at most one import line).
2. `home_site.go` `homeBuild` passes the series reader (`series`, as it passes it to `home_market.Build`), `liquidity.Compute` and `liquidity.TrialParams()` into `home_macro.Config`; `git diff BASE -- cmd/ot/home_site.go` shows no other line changed.
3. `wire_macro_test.go` has the new test `TestMacroPageWiresRealLiquidity` (new): the page built by `newMacroPage` reports `Meta.Schema == "page.macro_page.v2"`, and its `Build` (which takes the reader; `newMacroPage` has none) called with a fake reader holding the fourteen series returns a `liquidity` object with four lines, a reserves figure equal to the one `liquidity.Compute` gives for the same series, and a `net_liquidity` tap row equal to `ease.NetLiquidity` on them, so a nil `Func` (the gap `no_reader`) fails the test. `home_site_test.go` has a new test `TestHomeMacroSayIsReserves` (new) showing the home macro member built by `homeBuild` has the reserves `say` over the same fake series.
4. `pagelist_test.go` `TestMacroPageRealBuild` expects `schema_version` 2 and `page.macro_page.v2` where it expects 1 and v1 today; `page_test.go` and `testdata/macro_page_demo.golden.json` (the demo build of `page_test.go:226`) are the v2 output, the golden written with the `-update` flag of that test, not by hand; its `liquidity` is named gaps for the series the demo reader lacks.
5. `git diff --name-only BASE` lists only paths of the Files line; `wire_macro.go` is under 600 lines.

## Checked by CI

`go test ./internal/compute/... ./internal/contracts/... ./internal/sources/... ./internal/jobs/fredseries/... ./internal/pages/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch --check`, the hygiene check (fixture manifest), `go build ./...`, `golangci-lint`, the contracts generation check (`pnpm --dir web run generate:types -- page.macro_page.v2` leaves `web/src/generated/macro_page.v2.ts` unchanged), and the web checks (`tsc`, `eslint`, `vitest`, the build). Results only a run shows: the figures on the hand-written series of entry 1; the v2 schema accepting the fixtures and refusing the bad ones; the 22-key run; the closed and opened folds of entry 5; the home `say` golden of entry 6; the demo golden of entry 7.

## Left out

- The site's scores and regime ladder, net-liquidity impulse labels, signals, backtests, the FX-futures dollar proxy, CFTC positioning, crypto, hedge costs: nothing tests them against equity drawdowns, and the owner's rule is no score and no regime here (advisor, usd-flow site read).
- A composite or a traffic light of the four lines, a colour or a threshold on SOFR minus IORB or any spread: none is published [NF].
- M2 (monthly, four-week lag, revised; no source ties it to put timing), cross-currency basis, central bank swap lines (SWPT, about 0 outside crises), the broad dollar index DTWEXBGS, T10Y2Y (a curve, not liquidity), ANFCI and the NFCI sub-indexes, the OFR financial stress index, the New York Fed reserve-demand elasticity, repo operations (RPONTSYD): optional, the owner may ask for any of them.
- NY Fed as a second source for `SOFR99` and the other new series (`internal/sources/nyfed` stays `SOFR` only).
- Funding-calendar dates (quarter-ends, tax dates): the macro-events design.
- Any change to the home page schema, the `macro_summary` schema, the stored `macro.ease` reading, or `compute/ease`; any figure of this design on the home page or in a stored reading; any use of these figures by a rule, an alert or a sentence about puts.
- Showing a chart behind a tap (numbers only); a history view of a figure.
- Reading the ratio of reserves to GDP (needs quarterly GDP, a second series; the NY Fed staff measure) [T, not recommended in the proposal].
- A registry row for the six series (title, category, frequency, by a migration with `INSERT ... ON CONFLICT (series_id) DO UPDATE`, since the writer's default row would keep a blank title). It would only fill fields that nothing reads today; the owner's "a new series is a registry row" is met by the writer's row, and the migration can follow when a consumer reads the registry.
- Removing version 1 from the web (`SUPPORTED_VERSIONS`, the v1 fixtures `web/fixtures/macro-page`) and the v1 output and old `say` from `macro_page` and `home_macro`: a later cleanup once nothing writes v1.
- A job that plans from the registry, point-in-time vintages, a registry view (`series-registry.md` Left out).

## Doubts

None. The home `say` question is settled by the advisor (Decision 4). Points the reader can check, not owner questions: the ids and unit texts of `TLAACBW027SBOG` and `WLCFLPCL` are [M] until entry 3 reads the FRED pages; the example sentences of Decision 2 are examples, their wording is pinned by the tests of entry 6; the windows (4 and 13 weeks, 250 days, freshness 4 and 14 days) are trial values [T], changeable in `liquidity.TrialParams` alone.

## Adversarial

The attack file `liquidity-adversarial.md` is kept in the dispatcher window's scratchpad (not in the repository).
- B1 (fixture manifest missing): adopted. Entry 2 Files and acceptance 6 name `MANIFEST.synthetic-v1.json`.
- B2 (generator creates nothing without the name): adopted. Entry 2 acceptance 5 gives the command with `-- page.macro_page.v2` and states the file exists and is unchanged by a rerun; Checked by CI uses it.
- B3 (compute-on-build against compute-figures.md and the package doc): adopted. Decision 7 states compute-on-build behind an api `Func`, cites `blockapi` and `trackapi`, says why not readings rows, owns the read cost, and entry 6 changes the package doc of `macro_page`. The `modules.md` pages row is not edited: pages calls a compute function and does not compute.
- B4 (percentile window): adopted. Decision 8 and entry 6 say "whatever is stored", about 3 years, printed start and count; the 10-year claim is gone.
- Gap, home Overview: adopted in part. `say` keeps its text so the Overview line keeps its content; superseded in round 3 (below): `say` becomes a dated reserves fact. The attack's other point (aria text and data times inside the fold): the whole `ease` object is inside the fold (Decision 4); `data_times_text` is a separate fold and stays.
- Gap, `data-ease-figure`: adopted, entry 5 acceptance 3.
- Gap, second net liquidity: adopted. One implementation, `ease.NetLiquidity`, reached through `liquidityapi.NetFunc` (Decision 8, entries 1 and 7).
- Biggest risk, v1 consumers: adopted. Today and entries 5 to 7 name `pages.test.tsx:111`, `pagereg_test.go:85` (names only, unchanged), `pagelist_test.go:96`, and the v1 fixtures (kept, validated as v1, read by no web test). NFCI first vintage: corrected, not exempted: after `0043` pinning no longer keeps the first value, a revised value inside the lookback replaces it (Decision 5).
- Simpler, no migration: adopted. Entry 4 (registry) is gone and the series are numbered 1 to 7. Not adopted as asked: the seed test is not left alone, because `TestRegistrySeedCoversManifest` would fail on the six ids; entry 4 gives it a named skip list and a test that the list is in the manifest.
- Lock-in (enum keys, params in code): accepted; stated in Interface and Doubts.
- Weak claims: (a) fixed with B2; (b) the type file is now the first generated type of `macro_page`, entry 5 acceptance 1; (e) entry 7's test goes through `Build`; (f) wire.go shrinks.
- Slip: manifest (B1); golden update named as the `-update` flag of `page_test.go`; the empty plan-hash item is dropped (`plan_test.go` is not edited); Order lines follow the new numbers.

Round 3 (advisor review r1, kept with the dispatcher):
- B1 (no ot entry): adopted. Entry 7 wires `liquidity.Compute` and `ease.NetLiquidity` into `newMacroPage` and `homeBuild`, moves `TestMacroPageRealBuild` and the demo golden to v2, and has a test that fails on a nil `Func`.
- B2 (order): adopted. The web entry (5) accepts versions 1 and 2; the page entry (6) is additive (zero value = today's v1 output and `say`), so it merges alone with `cmd/ot` green; the ot entry (7) is the switching step; removing v1 from the page packages and the web is Left out. (A first wording, "merged back to back", was dropped: each entry is its own pull request.)
- B3 (ages): adopted. Decision 8 and entry 1: stale rule for net liquidity, the spread, the ratio and the discount window, the RRP day in `Num.Aux`, one new test per rule.
- B4 (home say, real sentence): adopted. Decision 4 and entry 6: `say` is the dated reserves fact; the Ease words stay in the fold; no schema change.
- B5 (Contracts line): adopted. Each entry has its own line; only entry 2 carries the v2 contract.
- Named items: the `MissingInput` constant and the reason table with `bad_input` (Decision 11, entry 1); the top-level `gaps` rule (Decision 9); no job id is added (entry 4 acceptance 5).
