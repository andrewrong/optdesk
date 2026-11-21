# macro event values: the /macro event table shows the previous value, and after a release the actual, from stored FRED series

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-21
Plan: P1

Source: owner UI doc, ask optdesk-5618: the "Previous" column of the /macro event table says "previous value: not built" on every row. Existing designs: `docs/design/macro-events.md` (the event rows), `docs/design/macro-page-v2.md` (page shape, Chinese hints), `docs/design/series-registry.md` (stored FRED points). This design edits none of them.

Goal (what the owner sees): on `/macro`, each CPI, employment (NFP), PCE, GDP, PPI, retail sales, JOLTS, H.4.1 and FOMC row shows the previous value (the last published figure, e.g. CPI "3.1% y/y") and, once the release is stored, the actual in a new "Actual" column with a Chinese gloss of the column. Consensus is not shown: no free, reliable source exists (Decision 7); the page says so in the column hint. A figure that cannot be computed shows a visible reason, never a shifted or guessed value. FOMC minutes, BoJ, quarter-end and tax rows have no figure and show a dash.

## Today

All paths at origin/main 7207de34.

- `internal/pages/macro_page/events.go` `newEventRow` sets `Previous` to a `not_built` gap; `eventBlock` reads `macroevent.Reader.Between(day, day+60)`. `Build(ctx, r store.SeriesReader, c, day, ...)` already holds a series reader (`SeriesPoints(ctx, id, from, to)`, one row per observation date, newest vintage).
- `contracts/schemas/page.macro_page.v2.json` `Event` has `previous` (string or Gap), no `actual`. `web/src/pages/macro/MacroPage.tsx` and `web/src/pages/home/MacroCard.tsx` list `previous`.
- `obs.macro_event` (migration 0048) holds only kind and date. FRED is the only source for the eight release kinds (`internal/jobs/macroevents`, release ids in `plan.go`). The stored series do not include the release series: `internal/jobs/fredseries/job.toml` has none of them; its windowed fetch looks back at most 31 days (`lookback_days` rule in `plan.go`), so a monthly point dated the 1st of a month released on the 13th is never fetched.
- Last migration on main is `0067_runner_migrate_read.sql`; the next free number is 0068.

## Modules

contracts (schema + generated type), sources (units), store (migration), jobs (fredseries), pages (macro_page), web. Six entries; the interface entry first.

## Interface

One shared item changes: `Event` in `page.macro_page.v2` gains two optional members, no version bump (an instance without them stays valid): `actual` and `previous_figure`, each a `Figure` object `{value: number, unit: string, text: string}` or a Gap. `value` and `unit` (`percent`, `thousands`, `millions`, `usd_trillions`, `percent_range` with `value` the upper bound) are the data; `text` is the display string Go formats; the web prints `text` and computes nothing. `previous` stays the string or Gap it is, equal to the `text` of `previous_figure`, so a later consensus or surprise column compares numbers with no schema break. No Go interface file changes.

## Decisions

1. No new source and no new table. Previous and actual come from FRED series that `fred_series` stores in `obs.series_point`; `obs.macro_event` is unchanged. Own implementation, because the figures are a few lines of arithmetic on stored points, no library fits.
2. Series per kind (all ten ids confirmed on the FRED pages, `macro-event-values.adversarial.md`): cpi `CPIAUCNS` (NSA, the index BLS quotes the y/y of), employment_situation `PAYEMS`, pce `PCEPI`, gdp `A191RL1Q225SBEA`, ppi `PPIFID` (NSA y/y; BLS headline m/m is SA and not shown), retail_sales `RSAFS`, jolts `JTSJOL`, h41 `WALCL` (already stored), fomc `DFEDTARU` and `DFEDTARL`.
3. Figure per kind, computed in Go (`Figure.text`, the unit always in the text): cpi, pce, ppi: y/y percent of the index, one decimal ("3.1%"); employment_situation: month change of PAYEMS ("+227K"); gdp: the stored annualised percent as is ("2.8%"); retail_sales: m/m percent ("+0.4%"); jolts: level in millions ("7.52M"); h41: level in trillions ("$6.74T"); fomc: the target range "4.00-4.25%". Kinds minutes, boj, quarter_end, us_tax: `previous` is the string "-"; `actual` and `previous_figure` are left out of the event (no figure).
4. The observation is chosen by reference date, never by position: for an event dated D the expected observation is the month before D's month (jolts two months, gdp the quarter before D's quarter, h41 the Wednesday on or before D, fomc: previous is the target on D-1, actual the target on D+1). y/y is the point at the exact date twelve months before the observation; m/m and the NFP change use the exact previous month. A point that is absent or stored with no value (FRED "." as for CPIAUCNS 2025-10-01, the shutdown hole) gives a `no_data` gap whose text names the month ("CPI Oct 2025 not published by BLS"), never a value taken from a neighbouring date. Previous is the figure one period before the expected observation; for an event after the page day previous is the newest stored figure, shown only when it is the expected period minus one, else a `stale` gap "latest stored: <date>". A missing series is a `no_data` gap per cell naming the series; a read error is an error of Build, never a gap.
5. GDP: the advance, second and third estimates share one release id. The first gdp event of a quarter's release run has previous = the prior quarter. A later estimate row (any gdp event after the first one for the same reference quarter, read from `obs.macro_event`) should show the earlier estimate of the same quarter as previous; `obs.series_point` keeps only the newest vintage, so that figure is not stored and the cell is a `no_data` gap "earlier estimate not stored (only the latest vintage is kept)". It is never the prior quarter's value. Actual of every gdp row is the stored (latest) figure of the quarter.
6. Values are the latest vintage: a previous figure is the revised one; an actual is the first print until a later release revises it, so an older row can show a number the release did not print. The column hint says this in Chinese.
7. Consensus: left out. FRED, BLS, BEA and the Fed publish none; the free sources found have no stable API or licence. The hint says "共识预期没有免费可靠来源，不显示". No "not built" column is added.
8. `fred_series` fetches the nine new series through a new manifest list `long`, from the day minus `long_lookback_days` (460 days: 15 monthly points for a y/y of the previous period, plus margin; the manifest check requires 400..800). The fetch runs on every run and writes by key (idempotent), so the first run after deploy stores the whole history in that one run; no later run is needed to fill it. `long` is in the plan hash (`plan.go`: the sorted list and the lookback are hash parts) and every `long` id is in `series`. The page never calls FRED.
9. Timing, stated as it is. The run is at 07:30 ET on XNYS days only (no weekend or holiday run) and releases are at 08:30 ET, so an actual appears at the next XNYS-day run at the earliest. For fomc, `DFEDTARU` and `DFEDTARL` are daily series that FRED posts a business day or more after the date; the D+1 point may be stored two runs after the meeting, and until then the actual is a `no_data` gap "not yet published by FRED" (not "no meeting"). The page does not promise a same-day actual.
10. The table also lists the events of the 7 days before the page day, so a missed release shows its actual. `days_to` is negative and `in_window` false for them; `lead.go` does not choose one as the next big event.
11. Units and registry: `mapping.go` gets unit rows for the nine new ids, with the existing enum (`count` is already in `obs.series_unit` and the schema, so no contracts or store file introduces a unit). Row, shift (decimal places the FRED text moves right) and the FRED units text to record in `testdata/series_units.json`: `CPIAUCNS` index_points, 0, "Index 1982-1984=100"; `PCEPI` index_points, 0, "Index 2017=100"; `PPIFID` index_points, 0, "Index Nov 2009=100"; `PAYEMS` count, 3, "Thousands of Persons"; `JTSJOL` count, 3, "Level in Thousands"; `RSAFS` ccy_amount USD, 6, "Millions of Dollars"; `A191RL1Q225SBEA` percent, 0, "Percent Change from Preceding Period"; `DFEDTARU` and `DFEDTARL` percent, 0, "Percent". None of the nine has recorded metadata in the repo yet (only CPIAUCSL has "Index 1982-1984=100"); the texts above were read on the nine FRED series pages on 2026-11-21 and match. `compareUnits` in `mapping_test.go` today accepts only "Percent", "Index" and "Index Jan 2006=100", and "Millions/Billions of U.S. Dollars", and checks nothing for `count`; entry 2 widens it: percent also accepts "Percent Change from Preceding Period"; an index text is any text beginning "Index"; ccy_amount also accepts "Millions of Dollars" with shift 6; `count` requires "Thousands of Persons" or "Level in Thousands" with shift 3 (and the reverse). Migration `0068_macro_event_series.sql` inserts the `obs.series_definition` rows, `ON CONFLICT DO NOTHING`, as 0066 does (last migration on main is 0067). `seed_test.go` of fredseries names the registry files it reads one by one (0043 and 0066), so entry 4 adds 0068 to that list.
12. Chinese text only in `zh_texts.go`: column hints and glosses (y/y 同比, m/m 环比, 非农, 目标区间); the web prints a hint as is. The home card (`MacroCard.tsx`) is not changed (Left out).

## Issues

All entries: `Plan: P1`, `Design: docs/design/macro-event-values.md#issues`. Slugs are the `Depends on:` names.

### 1 macro-ev-1-event-schema (interface)

Module: contracts. Size: S. Order: none.
Files: `contracts/schemas/page.macro_page.v2.json`, `web/src/generated/macro_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/macropage_v2_test.go`, `contracts/fixtures/page.macro_page.v2.golden.json`
Acceptance: 1. `Event` has optional `actual` and `previous_figure` of type Figure-or-Gap (Interface). 2. The generated type has both. 3. `TestMacroPageEventFigureOptional` (new) validates an event without and with them.

### 2 macro-ev-2-fred-units

Module: sources. Size: S. Order: none.
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/mapping_test.go`, `internal/sources/fred/testdata/series_units.json`
Acceptance: 1. The nine new ids have a unit row and a recorded FRED units text. 2. The rows, shifts and texts are those of Decision 11; `compareUnits` accepts them and still fails a row whose text disagrees (a test with a wrong shift for PAYEMS). 3. `TestMacroEventSeriesHaveUnitRows` (new) shows each maps to its unit, a PAYEMS value of 158000 (thousands) is stored as 158000000, an RSAFS value is stored in whole dollars, and a "." observation is stored with no value.

### 3 macro-ev-3-series-registry

Module: store. Size: S. Order: none.
Files: `migrations/0068_macro_event_series.sql`
Acceptance: 1. The file inserts the nine rows with `ON CONFLICT DO NOTHING`. 2. Its Down removes only rows with no stored point, as 0066 does. Test: `go test -tags pg` migration check in CI.

### 4 macro-ev-4-fred-long-lookback

Module: jobs. Size: M. Order: after 2, 3.
Files: `internal/jobs/fredseries/job.toml`, `internal/jobs/fredseries/seed_test.go`, `internal/jobs/fredseries/plan.go`, `internal/jobs/fredseries/run.go`, `internal/jobs/fredseries/window.go`, `internal/jobs/fredseries/plan_test.go`, `internal/jobs/fredseries/run_test.go`
Acceptance: 1. `job.toml` lists the nine ids in `series` and `long`, with `long_lookback_days = 460`. 2. A `long` series is fetched from the day minus that lookback in the one run; the 1..31 rule of `lookback_days` is unchanged. 3. The plan hash differs when `long` or its lookback differs. 4. `readRegistrySQL` in `seed_test.go` reads `0068_macro_event_series.sql` too; `TestRegistrySeedCoversManifest` and `TestUnregisteredAreInManifest` pass with the nine ids in the manifest and `unregistered` unchanged. 5. `TestLongSeriesLookback`, `TestPlanHashIncludesLong`, `TestManifestLongInSeries` (new), the first with a fake source that holds 16 monthly points and a second run that writes no row.

### 5 macro-ev-5-event-values

Module: pages. Size: M. Order: after 1, 4.
Files: `internal/pages/macro_page/eventvalues.go`, `internal/pages/macro_page/eventvalues_test.go`, `internal/pages/macro_page/events.go`, `internal/pages/macro_page/events_test.go`, `internal/pages/macro_page/lead.go`, `internal/pages/macro_page/zh_texts.go`, `internal/pages/macro_page/testdata/`
Acceptance: 1. `newEventRow` no longer writes `not_built`; Decisions 3 to 5 give `previous`, `previous_figure` and `actual`. 2. The window starts 7 days before the day; `lead.go` skips past events. 3. Every Chinese hint is in `zh_texts.go`. Tests in "Checked by CI".

### 6 macro-ev-6-web-actual-column

Module: web. Size: S. Order: after 1.
Files: `web/src/pages/macro/MacroPage.tsx`, `web/test/pages/macro/MacroPage.test.tsx`, `web/fixtures/macro-page/ok.json`
Acceptance: 1. The events table has an "Actual" column after "Previous" printing `actual.text` or the gap, with the hints. 2. A page without `actual` renders as before. 3. The web computes nothing.

## Checked by CI

- Go (`internal/pages/macro_page`), fixtures of at least 16 monthly points for each y/y kind, so the window of Decision 4 is covered: `TestEventValuesPerKind` (CPI y/y, NFP change, GDP, retail, JOLTS, H.4.1, FOMC range; previous for a future event, actual for a past one), `TestEventValuesYoYHole` (CPI with the Oct 2025 point stored empty: `no_data` naming the month, no shifted value), `TestEventValuesGdpRevision` (second estimate: gap, not the prior quarter), `TestEventValuesFomcLag` (D+1 point absent: "not yet published"), `TestEventValuesGaps`, `TestPastEventsNotLead`; `go build ./...`, `go vet ./...`, `go test ./...`.
- Web: `tsc`, `eslint`, `vitest`, build.

## Adversarial

Full answer: the dispatcher draft store (not committed) (Sources, the ten-id table, six points, Smallest fixes).

1. Gap vs goal: adopted. 460-day `long` lookback, first run fills (Decision 8); GDP revisions no longer show the prior quarter (Decision 5); first print vs revised stated (Decision 6).
2. Hole risk: adopted. Exact-date lookup and a named `no_data` (Decision 4); `TestEventValuesYoYHole`. PPIFID and RSAFS holes are covered by the same rule.
3. Simpler way (ship H.4.1 and FOMC first): not adopted. The six entries are small and ordered, and the y/y kinds are the owner's main rows; the 1..31 cap is left alone (a new key).
4. Lock-in: adopted. Numeric `Figure` beside the text (Interface).
5. False claims: adopted. "Next run fills it" removed; FOMC lag and XNYS-day runs stated (Decision 9).
6. Slip: adopted. Fixtures cover the window, `long` is in the plan hash (macro-ev-4 acceptance 3 and 4); the outcome check is the live `/macro` CPI row of the Oct 2026 release (a no_data naming Oct 2025 is the right answer).

## Left out

- Consensus (Decision 7). A paid or scraped source would need the owner's word.
- The home card `MacroCard.tsx` and `internal/pages/home_macro`: still "previous value: not built" until a follow-up reuses this figure code; the owner is told here.
- BoJ and FOMC minutes figures; the Japan policy rate is hand-typed in another design.
- First-print versus revised values and the earlier GDP estimate (FRED vintages are not stored).
