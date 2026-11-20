# macro page v2: the macro page leads with the T-bill rate, the high-yield spread, the events inside the option window and the index IV, then shows the rest of the decided list with Chinese hints

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20

Source: owner UI doc row 10 (ask optdesk-5479). The list is the one decided by the option-trader advisor (`state/optdesk-drafts/designs/macro-indicators-option-trader-2026-11-20.md`): only DGS3MO (the cash rate) and the Japan policy rate feed a rule (the Japan policy target stays hand-typed, per `docs/GLOSSARY.md`, and is shown from that stored setting); display only are DFF, DGS2, DGS10, T10Y2Y, T10Y3M, T10YIE, the high-yield spread, NFCI, net liquidity (optional), the dollar index, USD/JPY, the Japan 10-year yield, the Japan policy target and the event calendar (CPI, jobs, GDP, PCE, FOMC, BoJ, with a flag on an event inside the DTE window). The page leads with the T-bill, the high-yield spread, the events and the index IV. All series come from FRED. `docs/design/alignment-v25.md` was not found under `state/`; the alignment checked is the owner's drops of 2026-11-09 (no scanner, no scoring, no Stooq), none of which this design touches.

Goal: on `/macro` the first thing the owner reads is four rows: the cash rate, the credit spread, the next big event with the days to it and whether it falls inside the option window, and the three index volatilities. Under them the other decided series sit in three short groups. Each term has a Chinese hover hint. The producer computes every figure, change and flag; the web only prints.

Existing designs: `docs/design/liquidity-view.md` (the four funding lines and four taps), `docs/design/macro-events.md` (the event list, 60 days), `docs/design/macro-page-wiring.md`. This design edits none of them; it adds to the page and changes its order. `docs/design/market-state-ranges.md` gives the market-state page its ranges and leaves the macro page out; this design does the same (Decision 9).

## Today

Facts read on origin/main b6b01e2c.

- The page is `macro_page` (schema `contracts/schemas/page.macro_page.v2.json`, producer `internal/pages/macro_page`, web `web/src/pages/macro/MacroPage.tsx`, route `/macro` in `cmd/ot/site.go:481`). Its cards, top to bottom (`MacroPage.tsx:57-120`): the Ease gauge (folded), the funding and credit block (`LiquidityBlock`), "This week and next" events, the Indicators table, Notes.
- The Indicators table holds one row: `DGS10` "10-year Treasury yield" (`cmd/ot/macro_events_page.go:40`). Its row type has `name`, `latest`, `status`, `label`, `detail_href`, `source_text` and no hint, change or day; `latest` is a `PercentFigure` (unit const `percent`) or a Gap.
- Events: `events.go` reads the stored `macroevent` list for 60 days, at most 20, with CPI, jobs, PCE, GDP, FOMC and BoJ flagged high in `when_text` (`events.go` conventions). There is no days-to figure and no flag for the option window.
- The funding block already prints WRESBAL, RRPONTSYD, SOFR, IORB, SOFR99, the credit spreads (BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y), NFCI and, in taps, STLFSI4 and net liquidity (`liquidity.go:176-190`). It is a background block; nothing leads with the high-yield spread.
- Stored today (`internal/jobs/fredseries/job.toml`): DGS10, DGS3MO, SP500, VIXCLS, VXNCLS, RVXCLS, VXVCLS, WALCL, WDTGAL, RRPONTSYD, DFF, BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y, SOFR, WRESBAL, IORB, SOFR99, NFCI, STLFSI4, TLAACBW027SBOG, WLCFLPCL. Not stored: DGS2, T10Y2Y, T10Y3M, T10YIE, DTWEXBGS, DEXJPUS and IRLTLT01JPM156N. The Japan policy target is not a FRED series: `docs/GLOSSARY.md:343` says it is typed by hand from the BoJ statements (in option-trader's refdata); optdesk has no store for it. `internal/sources/fred/mapping.go:56` lists DGS2 (a series absent there is refused) and not the others; `obs.series_definition` has rows for the 22 stored series and none for the others (`migrations/0043_series_registry.sql`).
- The index volatilities VIXCLS, VXNCLS, RVXCLS are stored and already shown on the home Market card (Index IV column); the macro page does not show them.
- No Chinese text exists on this page. Other pages keep theirs in `zh_texts.go` (for example `internal/pages/home_market/zh_texts.go`).
- The option window: `max_dte` of the rule registry, default 45, bounds 14 to 180 (`internal/compute/ruleparam/registry.go:27`). The owner's stored value is read through `ownerset.Reader.Params` and `RuleParams.Resolve` (`internal/pages/option_candidates_page/tierfacts.go:88-99` does it for another key).

## Decisions

1. **Order of the page.** Top to bottom: the lead block (Decision 2), the events (Decision 3), the Indicators in three groups (Decision 5), the Ease gauge (folded, unchanged), the funding and credit block (unchanged), Notes. Nothing is dropped; the Ease gauge and the funding block move down because they are background and the owner's rule 2026-11-17 asks for the decision rows first. The old one-row Indicators table is replaced by the three groups (its DGS10 row is a row of the rates group).
2. **The lead block.** One new optional payload field `lead`: an object of `title` ("What a put seller reads first"), `rows` (always four, in this order) and `note`. A row is `key`, `label`, `text` (the figure as the producer prints it), `data_day` (a Day or null), `change_text` (the change, or null), `hint` (Chinese) and `state` `ok` or `gap` (a gap row carries the reason in `text`). The four rows: `tbill` (DGS3MO, percent), `hy_spread` (BAMLH0A0HYM2, percent), `next_event` (Decision 3) and `index_iv` (VIXCLS, VXNCLS, RVXCLS in one line "VIX 17.2 · VXN 21.0 · RVX 20.1", each stored close with its own day when the days differ).
3. **Events and the option window.** Each event gets the optional `days_to` (whole days from the page day) and `in_window` (true when `0 <= days_to <= WindowDays`). The lead row `next_event` is the nearest event of the high kinds (CPI, jobs, PCE, GDP, FOMC decision, BoJ decision) and reads for example "FOMC decision, 2026-12-09, in 19 days, inside the 45-day option window", or "..., after the 45-day option window". With no high event stored it is a gap "no big event stored in the next 60 days", never the word "none". The window is the owner's stored `max_dte` (read with `ownerset.Reader.Params` and resolved by `RuleParams.Resolve`, as the rules read it), and the default 45 only when none is stored. The producer gets it as a function `Window` (returns the days and whether the owner stored it) and the page says which: the lead row text ends "(your max_dte setting)" or "(default, none set)". A reader error gives the default and the word "default", never a blank.
4. **New series, all FRED.** `fred_series` gains DGS2, T10Y2Y, T10Y3M, T10YIE, DTWEXBGS, DEXJPUS (daily, windowed) and IRLTLT01JPM156N (the Japan 10-year yield, monthly, OECD via FRED, not windowed: a full fetch, as DGS10). No Japan policy-rate series is fetched: the target is hand-typed (Decision 11). `mapping.go` gets a unit row for each (percent for the rates, the spreads and the Japan yield, index points for DTWEXBGS, and DEXJPUS in yen per dollar as FRED states it), and `obs.series_definition` gets a row for each by one migration. The migration is run after the merge by the dispatcher window (`ot setup pg`, migrate, `ot setup pg`), not by the owner; its number is the next free one at implementation time; the producer shows a gap `no_data` for a series without a row until then.
5. **Three groups.** The Indicators card has three groups, each row `name`, `latest`, `change_text`, `data_day`, `hint`: Rates and curve (DFF, DGS3MO, DGS2, DGS10, T10Y2Y, T10Y3M, T10YIE), Credit and conditions (BAMLH0A0HYM2, NFCI; net liquidity stays in its tap), Dollar and Japan (DTWEXBGS, DEXJPUS, IRLTLT01JPM156N, and the Japan policy target of Decision 11). The list lives in one table in the producer (`indicators.go`). The change is the latest value minus the value five observations earlier for a daily series and minus the previous point for a monthly one, printed with its unit and the word "five sessions" or "previous month"; a missing earlier point gives no change text, not zero. A row older than its freshness limit (4 days daily, 45 days monthly) keeps its figure and says "stale" in `status`, as today's `row` does.
6. **Latest figure type.** `Indicator.latest` accepts, besides `PercentFigure` and Gap, a new `LevelFigure` (`value`, `text`, `unit` enum `index_points`, `yen_per_dollar`, `percent`), because DTWEXBGS and DEXJPUS are not percents. `PercentFigure` is unchanged.
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
   - jp_policy: 日本央行政策利率目标（隔夜拆借利率的指导水平），由人工按央行声明录入，不是市场利率；旁边标的是录入的日期。
   - 事件列和窗口提示用同一套说明：“窗口内”指到期日不超过期权最长到期天数（默认45天）。
9. **No range, no colour, no rule.** The page prints figures, changes, days and flags; no normal, caution or alarm word, no tone, no threshold (ranges for the macro page are Left out; `market-state-ranges` covers the market-state rows). No rule reads any row of this page; the line "display only; no rule reads this" is the lead `note`. DGS3MO and the Japan policy rate remain the only rule inputs, wired by their own designs, not here.
11. **The Japan policy target row.** It is the last row of the Dollar and Japan group. The producer gets it as an optional value `JPPolicy` (the typed percent, the day it was read and the day it takes effect, or nil). With a value the row prints "around x percent" with "read on <day>" and, when announced but not yet effective, the new target beside the one in force; with nil it is the gap `not_entered` ("not entered yet"). Nothing here stores or fetches it: today `cmd/ot` passes nil, so the row is a gap until the store of the typed target has its own design (Left out). The web prints it like any row.
12. **Failure.** A series with no stored point gives a row gap `no_data`; a failed series read gives `read_failed`; a gap never turns the lead into a blank or changes `run_status` beyond what a missing indicator does today.

## Modules

Seven modules, one entry each. contracts (`contracts/schemas/page.macro_page.v2.json`, `web/src/generated/macro_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/**`), sources (`internal/sources/fred/**`), jobs (`internal/jobs/fredseries/**`), store (`migrations/**`), pages (`internal/pages/macro_page/**`), ot (`cmd/ot/**`) and web (`web/src/pages/macro/**`, `web/test/pages/macro/**`). No row of `docs/design/modules.md` changes.

## Interface

One shared item changes: `page.macro_page.v2` gains the optional fields of Decision 7 and the `LevelFigure` of Decision 6, with its generated type. No Go declaration of an interface file changes. Entry 1 is `Contracts: changed`; every other entry is `Contracts: unchanged`.

## Issues

### 1 macro-page-schema (interface)

Module: contracts. Size: S.
Order: none.
Files: `contracts/schemas/page.macro_page.v2.json`, `web/src/generated/macro_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/macro_page_lead_test.go`
Acceptance: the fields of Decisions 2, 3, 5, 6 and 7 are optional; the generated type has them; a new test validates an instance without and with them.

### 2 macro-fred-units

Module: sources. Size: S.
Order: none.
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/mapping_test.go`, `internal/sources/fred/testdata/series_units.json`
Acceptance: the seven series of Decision 4 have a unit row and a recorded FRED units text.

### 3 macro-fred-manifest

Module: jobs. Size: S.
Order: after 2 (a series without a unit row is refused).
Files: `internal/jobs/fredseries/job.toml`, `internal/jobs/fredseries/plan_test.go`, `internal/jobs/fredseries/run_test.go`
Acceptance: the manifest lists the seven series, the six daily ones windowed; the two tests list them.

### 4 macro-series-registry

Module: store. Size: S.
Order: none; the dispatcher window runs the migration after the merge.
Files: `migrations/0065_macro_series.sql`
Acceptance: one migration inserts the seven `obs.series_definition` rows.

### 5 macro-page-producer

Module: pages. Size: L.
Order: after 1; no file shared with entries 6 and 7.
Files: `internal/pages/macro_page/lead.go`, `internal/pages/macro_page/lead_test.go`, `internal/pages/macro_page/indicators.go`, `internal/pages/macro_page/indicators_test.go`, `internal/pages/macro_page/zh_texts.go`, `internal/pages/macro_page/events.go`, `internal/pages/macro_page/events_test.go`, `internal/pages/macro_page/builder.go`, `internal/pages/macro_page/builder_test.go`
Acceptance: the lead, the three groups, the Japan policy row, the day counts and the window flag are built and tested as Decisions 2, 3, 5, 8, 11 and 12.

### 6 macro-page-wiring

Module: ot. Size: S.
Order: after 5.
Files: `cmd/ot/macro_events_page.go`, `cmd/ot/wire_macro_test.go`
Acceptance: the page config lists the groups' series ids, gives the window function (stored `max_dte`, else 45) and passes no Japan policy value.

### 7 macro-page-web

Module: web. Size: M.
Order: after 1; no file shared with entry 5.
Files: `web/src/pages/macro/MacroPage.tsx`, `web/src/pages/macro/LeadBlock.tsx`, `web/test/pages/macro/MacroPage.test.tsx`, `web/test/pages/macro/LeadBlock.test.tsx`
Acceptance: order, lead block, grouped table, window flag, window source and hints printed as Decisions 1, 2, 3, 5, 8 and 11, no computation. The macro fixtures under `web/fixtures/macro-page/` are not edited: `web/test/shell/sample.test.tsx` does not compare them (checked: no macro-page line), and the new tests build their payloads in the test from `contracts/fixtures/page.macro_page.v2.golden.json`.

## Checked by CI

- `ci-go` for entries 1 to 6 and `ci-web` for entries 1 and 7 (`tsc`, `eslint`, `vitest`, build).

## After merge (checked by the dispatcher, not by review)

- The window runs the migration of entry 4 (`ot setup pg`, migrate, `ot setup pg`); the next `fred_series` run stores the seven series.
- The window sets `max_dte` on the settings page to a value other than 45 and checks that the lead row says "(your max_dte setting)" and the flag follows it.
- The window renders `/macro` and compares the four lead rows, the group rows, the day count of the next event and each hint with the stored figures and with the FRED pages, with a screenshot (owner rule of 2026-11-14), and reads the index IV row against the home Market card.

## Left out

- A store and entry form for the hand-typed Japan policy target, and the Japan policy rate as a rule input: its own design. The target is typed from the BoJ statements (`docs/GLOSSARY.md:343`) because FRED has no series for the target (the OECD call-money series is a market average, not the target), so this page only shows the stored setting with its day, and a gap until it exists.
- DGS3MO as a rule input: already decided elsewhere; here it is only displayed.
- Net liquidity as a lead row: it stays in the funding block tap (optional per the decided list).
- Ranges, colours, states and percentile cuts for the macro rows (see Decision 9).
- SOFR, MOVE, N225 implied volatility (IBKR, not FRED) and other series not on the decided list.
- Changes to `liquidity-view.md`, `macro-events.md` and the Ease gauge itself.

## Adversarial

- Gap against the goal: the Japan policy target row is a gap until a store for the typed value exists, so the owner sees the row but not the number at first; stated in Decision 11 and Left out. Adopted: shown, not faked. The index IV row has VIX, VXN, RVX but not the Nikkei IV (IBKR only).
- Biggest unnamed risk: the FRED monthly OECD series IRLTLT01JPM156N lags by one to two months and may have been discontinued by OECD; the page would show a stale or empty row. Mitigation: staleness status after 45 days and a gap; the migration and manifest can drop it by a one-line edit. Adopted.
- Simpler way: only add the lead block and leave the series list alone. Not adopted: the decided list needs the new series, and the lead is only four rows, so the larger part (fetch) is the point of the owner's row.
- Lock-in: optional fields on v2 mean the web must keep tolerating their absence; accepted, as for `market_state_page.v2`. The group table in `indicators.go` is data in code; a move to a file is possible later.
- Claim most likely false: "DEXJPUS is not stored today". `internal/jobs/fxrefresh/secondary.go:15` names DEXJPUS as the FRED fill series of the exchange-rate job, which may already write the same observations under another dataset; the migration and manifest may duplicate that. A second false claim candidate: the owner's `max_dte` is read by pages the same way the candidates page reads its delta key; the implementer confirms the reader is available at the macro page's wiring. The implementer checks `obs.series_point` and fxrefresh first and reports; entry 3 would then leave DEXJPUS out.
- Likeliest slip: the producer entry is large (nine files) and `builder.go` is 313 lines, near but under the limits; `lead.go` and `indicators.go` keep it so. A second slip is the Chinese text leaking outside `zh_texts.go`.
