# macro page v2: the macro page leads with the T-bill rate, the high-yield spread, the events inside the option window and the index IV, then shows the rest of the decided list with Chinese hints

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20

Source: owner UI doc row 10 (ask optdesk-5479). The list is the one decided by the option-trader advisor (`state/optdesk-drafts/designs/macro-indicators-option-trader-2026-11-20.md`): only DGS3MO (the cash rate) and the Japan policy rate feed a rule (the Japan policy target stays hand-typed, per `docs/GLOSSARY.md`, , and is Left out of this page: nothing stores it in optdesk); display only are DFF, DGS2, DGS10, T10Y2Y, T10Y3M, T10YIE, the high-yield spread, NFCI, net liquidity (optional), the dollar index, USD/JPY, the Japan 10-year yield, and the event calendar (CPI, jobs, GDP, PCE, FOMC, BoJ, with a flag on an event inside the DTE window). The page leads with the T-bill, the high-yield spread, the events and the index IV. All series come from FRED. `docs/design/alignment-v25.md` was not found under `state/`; the alignment checked is the owner's drops of 2026-11-09 (no scanner, no scoring, no Stooq), none of which this design touches.

Goal: on `/macro` the first thing the owner reads is four rows: the cash rate, the credit spread, the next big event with the days to it and whether it falls inside the option window, and the three index volatilities. Under them the other decided series sit in three short groups. Each term has a Chinese hover hint. The producer computes every figure, change and flag; the web only prints.

Existing designs: `docs/design/liquidity-view.md` (the four funding lines and four taps), `docs/design/macro-events.md` (the event list, 60 days), `docs/design/macro-page-wiring.md`. This design edits none of them; it adds to the page and changes its order. `docs/design/market-state-ranges.md` gives the market-state page its ranges and leaves the macro page out; this design does the same (Decision 9).

## Today

Facts read on origin/main b6b01e2c.

- The page is `macro_page` (schema `contracts/schemas/page.macro_page.v2.json`, producer `internal/pages/macro_page`, web `web/src/pages/macro/MacroPage.tsx`, route `/macro` in `cmd/ot/site.go:481`). Its cards, top to bottom (`MacroPage.tsx:57-120`): the Ease gauge (folded), the funding and credit block (`LiquidityBlock`), "This week and next" events, the Indicators table, Notes.
- The Indicators table holds one row: `DGS10` "10-year Treasury yield" (`cmd/ot/macro_events_page.go:40`). Its row type has `name`, `latest`, `status`, `label`, `detail_href`, `source_text` and no hint, change or day; `latest` is a `PercentFigure` (unit const `percent`) or a Gap.
- Events: `events.go` reads the stored `macroevent` list for 60 days, at most 20, with CPI, jobs, PCE, GDP, FOMC and BoJ flagged high in `when_text` (`events.go` conventions). There is no days-to figure and no flag for the option window.
- The funding block already prints WRESBAL, RRPONTSYD, SOFR, IORB, SOFR99, the credit spreads (BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y), NFCI and, in taps, STLFSI4 and net liquidity (`liquidity.go:176-190`). It is a background block; nothing leads with the high-yield spread.
- Stored today (`internal/jobs/fredseries/job.toml`): DGS10, DGS3MO, SP500, VIXCLS, VXNCLS, RVXCLS, VXVCLS, WALCL, WDTGAL, RRPONTSYD, DFF, BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y, SOFR, WRESBAL, IORB, SOFR99, NFCI, STLFSI4, TLAACBW027SBOG, WLCFLPCL. Not stored: DGS2, T10Y2Y, T10Y3M, T10YIE, DTWEXBGS, DEXJPUS and IRLTLT01JPM156N. The Japan policy target is not a FRED series: `docs/GLOSSARY.md:343` says it is typed by hand from the BoJ statements (in option-trader's refdata); optdesk has no store for it. `internal/sources/fred/mapping.go` already has unit rows for DGS2 (line 58) and DEXJPUS (line 84, unit `jpy_per_usd`, recorded text in `testdata/series_units.json`) and none for T10Y2Y, T10Y3M, T10YIE, DTWEXBGS, IRLTLT01JPM156N (a series absent there is refused); `obs.series_definition` has rows for the 22 stored series and none for the others (`migrations/0043_series_registry.sql`).
- The index volatilities VIXCLS, VXNCLS, RVXCLS are stored and already shown on the home Market card (Index IV column); the macro page does not show them.
- No Chinese text exists on this page. Other pages keep theirs in `zh_texts.go` (for example `internal/pages/home_market/zh_texts.go`).
- The option window: `max_dte` of the rule registry, default 45, bounds 14 to 180 (`internal/compute/ruleparam/registry.go:27`). The owner's stored value is read through `ownerset.Reader.Params` and `RuleParams.Resolve` (`internal/pages/option_candidates_page/tierfacts.go:88-99` does it for another key).

## Decisions

1. **Order of the page.** Top to bottom: the lead block (Decision 2), the events (Decision 3), the Indicators in three groups (Decision 5), the Ease gauge (folded, unchanged), the funding and credit block (unchanged), Notes. Nothing is dropped; the Ease gauge and the funding block move down because they are background and the owner's rule 2026-11-17 asks for the decision rows first. The old one-row Indicators table is replaced by the three groups (its DGS10 row is a row of the rates group).
2. **The lead block.** One new optional payload field `lead`: an object of `title` ("What a put seller reads first"), `rows` (always four, in this order) and `note`. A row is `key`, `label`, `text` (the figure as the producer prints it), `data_day` (a Day or null), `change_text` (the change, or null), `hint` (Chinese) and `state` `ok` or `gap` (a gap row carries the reason in `text`). The four rows: `tbill` (DGS3MO, percent), `hy_spread` (BAMLH0A0HYM2, percent), `next_event` (Decision 3) and `index_iv` (VIXCLS, VXNCLS, RVXCLS in one line "VIX 17.2 · VXN 21.0 · RVX 20.1", each stored close with its own day when the days differ).
3. **Events and the option window.** Each event gets the optional `days_to` (whole days from the page day) and `in_window` (true when `days_to` is zero or more and at most the window days). The lead row `next_event` is the nearest event of the high kinds (CPI, jobs, PCE, GDP, FOMC decision, BoJ decision) and reads for example "FOMC decision, 2026-12-09, in 19 days, inside the 45-day option window", or "..., after the 45-day option window". With no high event stored it is a gap row (reason `no_data`) with the text "no big event stored in the next 60 days", never the word "none". The window is the owner's stored `max_dte` (read with `ownerset.Reader.Params` and resolved by `RuleParams.Resolve`, as the rules read it), and the default 45 only when none is stored. The producer gets it as a function `Window` (returns the days and whether the owner stored it) and the page says which: the lead row text ends "(your max_dte setting)" or "(default, none set)". A failed read of the setting gives the default and the word "default", never a blank.
4. **New series, all FRED.** `fred_series` gains DGS2, T10Y2Y, T10Y3M, T10YIE, DTWEXBGS, DEXJPUS (daily, windowed) and IRLTLT01JPM156N (the Japan 10-year yield, monthly, OECD via FRED, not windowed: a full fetch, as DGS10). No Japan policy-rate series is fetched: the target is hand-typed (Decision 11). `mapping.go` gets a unit row for the five that lack one (percent for T10Y2Y, T10Y3M, T10YIE and IRLTLT01JPM156N, index points for DTWEXBGS); DGS2 and DEXJPUS are already mapped and are reused, and `obs.series_definition` gets a row for each of the seven by one migration, and the seed test `TestRegistrySeedCoversManifest` is made to read that migration too. The migration (one file, named in entry 3) is run after the merge by the dispatcher window (`ot setup pg`, migrate, `ot setup pg`), not by the owner; the producer shows a gap `no_data` for a series without a row until then.
5. **Three groups.** The Indicators card has three groups, each row keeps today's `name`, `latest`, `status`, `label`, `source_text` and `detail_href` and adds `group`, `change_text`, `data_day` and `hint`: Rates and curve (DFF, DGS3MO, DGS2, DGS10, T10Y2Y, T10Y3M, T10YIE), Credit and conditions (BAMLH0A0HYM2, NFCI; net liquidity stays in its tap), Dollar and Japan (DTWEXBGS, DEXJPUS, IRLTLT01JPM156N). The list lives in one table in the producer (`indicators.go`). The change is the latest value minus the value five observations earlier for a daily series and minus the previous point for a monthly one, printed with its unit and the word "five sessions" or "previous month"; a missing earlier point gives no change text, not zero. A row older than its freshness limit (4 days daily, 45 days monthly) keeps its figure and has `status` `late`, as today's `row` does.
6. **Latest figure type.** `Indicator.latest` accepts, besides `PercentFigure` and Gap, a new `LevelFigure` (`value`, `text`, `unit` enum `index_points`, `jpy_per_usd`, `percent`), because DTWEXBGS and DEXJPUS are not percents. `PercentFigure` is unchanged.
7. **Optional fields, no version bump.** `lead`, the event `days_to` and `in_window`, and the indicator `group`, `change_text`, `hint`, `data_day` are all optional on `page.macro_page.v2` (the precedent is `market_state_page.v2` in `docs/design/market-state-ranges.md` Decision 7). A payload without them still validates; the web prints a block only when its field is present.
8. **Chinese hints, in one Go file.** Every hint is a constant of `internal/pages/macro_page/zh_texts.go` (the one door of the English guard); the web prints `hint` in the existing `tooltip` next to the term and as `aria-label`, and invents no text. The hints:
   - tbill: 三个月美国国债收益率，就是无风险现金利率；卖期权的收入要拿它来比，它高时现金本身就有回报。
   - hy_spread: 高收益债信用利差（ICE BofA，FRED BAMLH0A0HYM2）：越大说明市场越怕违约，常在股市下跌前后走阔；这里只提示，不挡单。
   - next_event: 期权窗口内的重大事件（CPI、非农、PCE、GDP、联储议息、日本央行议息）：合约跨过这一天，要把事件风险算进去。
   - index_iv: 指数隐含波动率（VIX、VXN、RVX，FRED收盘值，不是实时报价）：越高权利金越贵，卖方收入高，风险也高。
   - DFF: 联邦基金有效利率，美联储政策利率的实际水平，短期资金的价格。
   - DGS2: 两年期美国国债收益率，反映市场对未来一两年政策利率的预期。
   - DGS10: 十年期美国国债收益率，长期利率的基准，也影响估值。
   - T10Y2Y: 十年减两年国债利差；为负是倒挂，常被当作衰退预警，只作参考。
   - T10Y3M: 十年减三个月国债利差；同样是曲线形状，倒挂更受联储关注。
   - T10YIE: 十年期盈亏平衡通胀率，市场隐含的未来十年通胀预期。
   - NFCI: 芝加哥联储金融状况指数；0是平均，越高金融环境越紧。
   - DTWEXBGS: 美联储广义美元指数；美元走强常对应全球流动性收紧。
   - DEXJPUS: 美元兑日元；日元急升常伴随套利交易平仓，波动放大。
   - IRLTLT01JPM156N: 日本十年期国债收益率（OECD经FRED，月度）。
   - 事件列和窗口提示用同一套说明：“窗口内”指到期日不超过期权最长到期天数（默认45天）。
9. **No range, no colour, no rule.** The page prints figures, changes, days and flags; no normal, caution or alarm word, no tone, no threshold (ranges for the macro page are Left out; `market-state-ranges` covers the market-state rows). No rule reads any row of this page; the line "display only; no rule reads this" is the lead `note`. DGS3MO and the Japan policy rate remain the only rule inputs, wired by their own designs, not here; the Japan policy target is not on this page (Left out).
10. **Failure and reasons.** Only values the schemas already allow are written: a series with no stored point gives a row gap with the reason `no_data`; a stored point older than its freshness limit keeps its figure and has the indicator `status` `late` (an existing value); a gap on the lead is a row with `state` `gap`. A failed read of a series, of the events or of the owner setting is, as in the builder today, an error of `Build` for the series and events, and the default window for the setting (Decision 3). No new `Gap` reason and no new `status` value is introduced.

## Modules

Seven modules named; six entries (the producer and the wiring are one entry because the wiring cannot be written before the producer's new fields exist and the interface files hold none of them). contracts (`contracts/schemas/page.macro_page.v2.json`, `web/src/generated/macro_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/**`), sources (`internal/sources/fred/**`), store (`migrations/**`), jobs (`internal/jobs/fredseries/**`), pages (`internal/pages/macro_page/**`), ot (`cmd/ot/**`) and web (`web/src/pages/macro/**`, `web/test/pages/macro/**`). No row of `docs/design/modules.md` changes. Entry 1 spans contracts and web (the read of `tone` in `web/src/pages/macro/MacroPage.tsx`, which the regenerated type changes); entry 5 spans pages and ot, all named here.

## Interface

One shared item changes: `page.macro_page.v2` gains the optional fields of Decision 7 and the `LevelFigure` of Decision 6, with its generated type. `Gap` reasons and `status` values are not touched (Decision 10). No Go declaration of an interface file changes: the new `macro_page.Config` fields are inside the implementation package and are used only by `cmd/ot`, which entry 5 changes together with them. Entry 1 is `Contracts: changed`; every other entry is `Contracts: unchanged`.

## Issues

### 1 macro-page-schema (interface)

Module: contracts and web. Size: S.
Order: none.
Files: `contracts/schemas/page.macro_page.v2.json`, `web/src/generated/macro_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/macro_page_lead_test.go`, `web/src/pages/macro/MacroPage.tsx` (only the read of `tone`, which `LevelFigure` lacks, so `tsc` stays green with the regenerated type)
Acceptance: the fields of Decisions 2, 3, 5, 6 and 7 are optional; the generated type has them; a new test `TestMacroPageLeadOptional` validates an instance without and with them.

### 2 macro-fred-units

Module: sources. Size: S.
Order: none.
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/mapping_test.go`, `internal/sources/fred/testdata/series_units.json`
Acceptance: `T10Y2Y`, `T10Y3M`, `T10YIE`, `DTWEXBGS` and `IRLTLT01JPM156N` get a unit row and a recorded text (`Percent`, `Percent`, `Percent`, `Index Jan 2006=100`, `Percent`); `DGS2` and `DEXJPUS` keep their rows; `TestMacroSeriesHaveUnitRows` shows all seven map.

### 3 macro-series-registry

Module: store. Size: S.
Order: none.
Files: `migrations/0065_macro_series.sql`
Acceptance: one migration inserts the seven `obs.series_definition` rows, in the file named here (0065 is the next free number on origin/main of 2026-11-20; if it is taken the implementer uses the next free one and says so in the pull request description).

### 4 macro-fred-manifest

Module: jobs. Size: S.
Order: after 2 and 3 (a series without a unit row is refused, and the seed test reads the migration of entry 3).
Files: `internal/jobs/fredseries/job.toml`, `internal/jobs/fredseries/plan_test.go`, `internal/jobs/fredseries/run_test.go`, `internal/jobs/fredseries/seed_test.go`
Acceptance: the manifest lists the seven series, six windowed; the two tests list them; `seed_test.go` reads `migrations/*_macro_series.sql` besides `0043_series_registry.sql`, so `TestRegistrySeedCoversManifest` and `TestUnregisteredAreInManifest` stay green with `unregistered` unchanged.

### 5 macro-page-producer

Module: pages and ot. Size: L.
Order: after 1; no file shared with entry 6.
Files: `internal/pages/macro_page/lead.go`, `internal/pages/macro_page/lead_test.go`, `internal/pages/macro_page/indicators.go`, `internal/pages/macro_page/indicators_test.go`, `internal/pages/macro_page/zh_texts.go`, `internal/pages/macro_page/events.go`, `internal/pages/macro_page/events_test.go`, `internal/pages/macro_page/builder.go`, `internal/pages/macro_page/builder_test.go`, `cmd/ot/pagelist.go`, `cmd/ot/macro_events_page.go`, `cmd/ot/wire_macro_test.go`
Acceptance: the lead, the three groups, the day counts and the window flag are built by the producer, and `cmd/ot` supplies the groups and the `Window` function (stored `max_dte`, else 45), as Decisions 2, 3, 5, 8 and 10; tests `TestLeadRowsKeepOrder`, `TestLeadNextEventWindow`, `TestLeadGapIsNotZero`, `TestLeadIndexIvThreeSeries`, `TestGroupsKeepOrder`, `TestChangeFiveSessions`, `TestMonthlyLateAfter45Days`, `TestMissingEarlierPointNoChange`, `TestEventWindowFlag`, `TestWindowSourceWords`, `TestEmptyConfigKeepsPayload`, `TestHintsOnlyInZhTexts`, `TestMacroPageConfigGroups`.

### 6 macro-page-web

Module: web. Size: M.
Order: after 1; no file shared with entry 5.
Files: `web/src/pages/macro/MacroPage.tsx`, `web/src/pages/macro/LeadBlock.tsx`, `web/test/pages/macro/MacroPage.test.tsx`, `web/test/pages/macro/LeadBlock.test.tsx`
Acceptance: order, lead block, grouped table, window flag, window source and hints printed as Decisions 1, 2, 3, 5 and 8, no computation. The macro fixtures under `web/fixtures/macro-page/` are not edited: `web/test/shell/sample.test.tsx` does not compare them (checked: no macro-page line), and the new tests build their payloads in the test from `contracts/fixtures/page.macro_page.v2.golden.json`.

## Checked by CI

- `ci-go` for entries 1 to 5 and `ci-web` for entries 1 and 6 (`tsc`, `eslint`, `vitest`, build).

## After merge (checked by the dispatcher, not by review)

- The window runs the migration of entry 3 (`ot setup pg`, migrate, `ot setup pg`); the next `fred_series` run stores the seven series.
- The window sets `max_dte` on the settings page to a value other than 45 and checks that the lead row says "(your max_dte setting)" and the flag follows it.
- The window renders `/macro` and compares the four lead rows, the group rows, the day count of the next event and each hint with the stored figures and with the FRED pages, with a screenshot (owner rule of 2026-11-14), and reads the index IV row against the home Market card.

## Left out

- A store and entry form for the hand-typed Japan policy target, and the Japan policy rate as a rule input: its own design. The target is typed from the BoJ statements (`docs/GLOSSARY.md:343`) because FRED has no series for the target (the OECD call-money series is a market average, not the target), so this page has no row for it; it joins the Dollar and Japan group once a store of the typed value has its own design.
- Rejected for the page itself: a `page.macro_page.v3` instead of optional fields (the web would need two readers for one page); a stored reading for the change and day figures instead of computing them at page build (no figure of the register exists for them and the series reader already gives the points); a new `Gap` reason or `status` value (Decision 10).
- DGS3MO as a rule input: already decided elsewhere; here it is only displayed.
- Net liquidity as a lead row: it stays in the funding block tap (optional per the decided list).
- Ranges, colours, states and percentile cuts for the macro rows (see Decision 9).
- SOFR, MOVE, N225 implied volatility (IBKR, not FRED) and other series not on the decided list.
- Changes to `liquidity-view.md`, `macro-events.md` and the Ease gauge itself.

## Adversarial

- Gap against the goal: the Japan policy target is not shown at all, because optdesk has no store for it; stated in Left out. Adopted: named, not faked. The index IV row has VIX, VXN, RVX but not the Nikkei IV (IBKR only).
- Biggest unnamed risk: the FRED monthly OECD series IRLTLT01JPM156N lags by one to two months and may have been discontinued by OECD; the page would show a late or empty row. Mitigation: `status` `late` after 45 days and a gap; the migration and manifest can drop it by a one-line edit. Adopted.
- Simpler way: only add the lead block and leave the series list alone. Not adopted: the decided list needs the new series, and the lead is only four rows, so the larger part (fetch) is the point of the owner's row.
- Lock-in: optional fields on v2 mean the web must keep tolerating their absence; accepted, as for `market_state_page.v2`. The group table in `indicators.go` is data in code; a move to a file is possible later.
- Claim most likely false: "DEXJPUS is not stored today". `internal/jobs/fxrefresh/secondary.go:15` names DEXJPUS as the FRED fill series of the exchange-rate job, which may already write the same observations under another dataset; the migration and manifest may duplicate that. A second false claim candidate: the owner's `max_dte` is read by pages the same way the candidates page reads its delta key; the implementer confirms the reader is available at the macro page's wiring. The implementer checks `obs.series_point` and fxrefresh first and reports; entries 3 and 4 would then both leave DEXJPUS out (the migration rows and the manifest ids together), so `TestRegistrySeedCoversManifest` stays green with `unregistered` unchanged.
- Likeliest slip: the producer entry is large (nine files) and `builder.go` is 313 lines, near but under the limits; `lead.go` and `indicators.go` keep it so. A second slip is the Chinese text leaking outside `zh_texts.go`.
