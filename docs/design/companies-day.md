# companies day: every "Last" price and every pepb price on the Companies page names its own day, in its own text

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: owner's data-correctness sweep 2026-11-17. Cites checked at origin/main 53df3ef3 (spot-from-bars design PR 2746 is merged). Entry names follow the files, not numbers.

## 你会看到什么

- /companies 页头写 "Watching 2 companies"（日期只在 stale.text 和 header.data_day 里出现一次，#2787）。页底状态行 stale.text 用同一种格式："Data of 2026-11-17"（以前是 "Data of 11-17"）。页头的日期是页面日，不是数据日；数据日在每个价格里。
- 每一行的 "Last" 单元格直接写出价格和它自己的日期，例如 "$128.40 · close 2026-11-14"（带币种符号、千分位、两位小数，#2787）。"close" 的意思是日线收盘价（bars_refresh 存下的 `trades` 日线），不是券商标记价。页面下方的备注里有一行："Last = daily close of the day shown; /positions shows the broker mark"，所以 /positions 的价格可以不同，差别有据可查。
- 日期是那根日线的交易日：周末或停牌时可以比页头早几天（最多 14 天，查找窗口不变）。
- PE/PB 区块里的 "price at percentile" 价格也写出日期，例如 "PE at the 20th percentile of 300 days since 2025-08-20: price 103.70 as of 2026-11-05 (history, not a buy price)"。
- 没有日线的名字，或日线没有交易日的名字：仍写 "no quote"，没有日期，不猜。隐含增长图例用同一条规则，表格和图例对同一行给出同一个答案。
- 网页不解析、不算任何东西：它原样打印后端给的文字。

## Goal

The Companies page names the day of each spot price it shows. A price never reaches the page without its day, and the page says that "Last" is the daily close, not the broker mark. No schema change, no contracts entry, no web entry: the day is part of the cell text that Go builds.

## Today

- `spotOf` in `cmd/ot/companies_inputs.go:147` returns `func(ctx, symbol, day) (string, bool, error)`: the `Close` of the newest `trades` bar in the 14 days up to the page day (`PriceBars`, `rows[len(rows)-1].Close`). The bar has `SessionDay` and the function drops it.
- `companies_page.Config.Spot` (`internal/pages/companies_page/builder.go:58`) has the same string signature. `Build` (`builder.go:204-210`) writes the string as the text of the third cell of every row ("Last"). `coreRowsOf` (`core.go:242`) reads it a second time, for `ImpliedGrowth` only, and checks only `spot != ""`.
- The header line is `fmt.Sprintf("%s · watching %d companies", string(today)[5:], ...)` (`builder.go:244`); `stale.text` is `"Data of " + string(today)[5:]` (`builder.go:192`): two "Data of" strings, one without it.
- The web prints the Last cell as text: `rowOf` in `web/src/pages/companies/CompaniesPage.tsx:56-66` copies `cell.text`; it prints `header.line` as given (line 189-190) and the `notes` array in a notes card (line 259-264, `CardNotes`). It parses and formats nothing of these, so no web change is needed. `Build` fills `notes` with `[]string{}` (`builder.go:182`) and nothing else writes it.
- The pepb block's `at_percentile` rows are printed by `PepbBlock.tsx` as `row.text` (the `Text` of `pepbapi.AtPercentile`, copied by `pepbCell` at `internal/pages/companies_page/pepb.go:152-155`). That text is written in `internal/compute/pepb/series.go:115` and ends "(history, not a buy price)"; it has the window start but no day of the price. The price is the ratio at that rank times today's basis over today's shares (`series.go:111-113`); today is the last bar of the window, `days[len(days)-1]`.
- /positions uses the broker mark and /companies the daily close; that is by design and stays.

## Decisions

1. Day in the text. The "Last" cell text is `<price> · close <YYYY-MM-DD>`, for example "128.40 · close 2026-11-14", the day being the `SessionDay` of the bar that gave the price. The cell has no new field; `Cell` of the schema is unchanged, so `contracts/**` and `web/**` are not touched. The web prints `cell.text` as it is (checked in `## Today`).
2. One gloss line. `Build` writes one note, `Last = daily close of the day shown; /positions shows the broker mark`, as the only item of `notes` (the web prints `notes` already). The text is a constant of the page package.
3. One "Data of" format: `YYYY-MM-DD` for both. `header.line` is `Data of <YYYY-MM-DD> · watching <N> companies` and `stale.text` is `Data of <YYYY-MM-DD>`, the day being the page day in full (`header.data_day`). Reason: the year disappears from "MM-DD" across New Year, and one format cannot be read two ways; the page day is the day of the run, the day of a price is in its cell.
4. Spot returns its day. `companies_page` declares `type Spot struct { Price string; Day contracts.Day }` and `Config.Spot` becomes `func(ctx context.Context, symbol string, day contracts.Day) (Spot, bool, error)`. `spotOf` fills `Price` with the `Close` and `Day` with the `SessionDay` of the newest `trades` bar; look-back, basis and the gap rules (no instrument, no bar, shared symbol) do not change. A later "source" field is added to the struct, which breaks no caller.
5. One Spot rule. `Config` gets one method, `spotAt(ctx, symbol, day) (Spot, bool, error)`, that calls `cfg.Spot` and answers ok false for a nil `Spot`, a not-ok answer, an empty `Price` or an empty `Day`. `Build` and `coreRowsOf` both call it, so a row that shows "no quote" in the table has no price in the implied growth legend, and the reverse.
6. The pepb price carries its day the same way. The text of `AtPercentile` written in `series.go` becomes `<KIND> at the <ordinal> percentile of <N> days since <from>: price <P> as of <last day> (history, not a buy price)`, the last day being the last bar of the window (the day of the ratio). `pepbCell` copies the text, so the page and the web need no change.
7. Order against other designs. `docs/design/spot-from-bars.md` edits `internal/compute/blockapi`, `internal/compute/pagefig/holdings*.go`, `internal/pages/positions_page/*` and `cmd/ot/positions_site*.go`; none is in a Files line below.

## Modules

- pages (`internal/pages/**`) and ot (`cmd/**`): one judged two-module entry, because `Config.Spot` is declared in pages and assigned by `spotOf` in ot; a signature change in one breaks the build of the other (as entry 2 of `docs/design/spot-from-bars.md`). It holds the header, the Last text, the note, the shared Spot rule and the docs line.
- compute (`internal/compute/**`): one entry, the text of the pepb price.
- contracts and web: not touched. The example fixtures `contracts/fixtures/page.companies_page.v2.*.json` are not edited: none of the tests compared with them reads the header, the Last cell, `stale.text` or the pepb price text (they compare `core`, `diagnosis` and `questions`, or validate against the schema), and every one still validates.

No row of `docs/design/modules.md` changes; no new dependency between modules.

## Interface

Go (module pages, `internal/pages/companies_page/spot.go`, a new file, not an interface file of `internal/pages/api.go`): `type Spot struct { Price string; Day contracts.Day }` and `Config.Spot func(ctx context.Context, symbol string, day contracts.Day) (Spot, bool, error)`. Producer in ot: `spotOf` in `cmd/ot/companies_inputs.go` returns `func(context.Context, string, contracts.Day) (companies_page.Spot, bool, error)`. No schema, no `pepbapi` change (the `Text` comment of `pepbapi.AtPercentile` stays true).

## Issues

### companies-last-day-page-wire

Module: pages and ot (one judged two-module entry, Decision 4).
Size: M.
Order: independent of companies-pepb-price-day-text.
Contracts: unchanged
Design: docs/design/companies-day.md#Decisions (Decisions 1 to 5)
Plan: P1
Files: `internal/pages/companies_page/spot.go`, `internal/pages/companies_page/builder.go`, `internal/pages/companies_page/core.go`, `internal/pages/companies_page/builder_test.go`, `internal/pages/companies_page/core_test.go`, `internal/pages/companies_page/pepb_test.go`, `cmd/ot/companies_inputs.go`, `cmd/ot/companies_inputs_test.go`, `cmd/ot/companies_pepb_test.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`, `docs/contracts/ot/pages/companies_page.md`
Acceptance:
1. `spot.go` is a new file that declares `type Spot struct { Price string; Day contracts.Day }` with a doc comment and the method `spotAt` of Decision 5; `Config.Spot` in `builder.go` has the type `func(ctx context.Context, symbol string, day contracts.Day) (Spot, bool, error)`; `git grep -n "(string, bool, error)" -- internal/pages/companies_page` finds no `Spot` line.
2. `Build` writes the text of the "Last" cell of a priced row as `<Price> · close <Day>` and the "no quote" cell as before; the holdings rows and the watch rows are built by the same code and read the same way. `Build` calls `spotAt`, not `cfg.Spot` directly; `cell` in `builder.go` has no new field.
3. `coreRowsOf` in `core.go` calls `spotAt` and passes `Spot.Price` to `ImpliedGrowth` only when `spotAt` says ok, so an empty `Day` gives no legend line, as it gives "no quote" in the table; `core.go` prints no price.
4. `Build` writes `header.line` as `Data of <YYYY-MM-DD> · watching <N> companies` and `stale.text` as `Data of <YYYY-MM-DD>`, both from the page day in full; `git grep -n "\[5:\]" -- internal/pages/companies_page/builder.go` finds no line.
5. `Build` writes `notes` as exactly one item, the constant `Last = daily close of the day shown; /positions shows the broker mark`, of at most 200 characters; no other file of the package writes `notes`.
6. New tests in `builder_test.go`: `TestBuildLastCellCarriesDay` (new: a watch row and a holdings row each have the text "128.40 · close 2026-11-04" for a fake spot whose day differs from the page day), `TestBuildNoQuoteWithoutDay` (new: a spot with a price and an empty `Day` gives the text "no quote"), `TestBuildDataOfFormat` (new: `header.line` is "Data of 2026-11-10 · watching 2 companies" and `stale.text` is "Data of 2026-11-10"), `TestBuildNotesHasLastGloss` (new: `notes` is the one gloss line). New test in `core_test.go`: `TestLegendFollowsSpotRule` (new: a fake spot with a price and an empty `Day` gives the "no quote" cell in the table and no legend line for that row; the same spot with a `Day` gives a legend line).
7. The fakes of `core_test.go` (`coreConfig` and `fixtureConfig`) return `Spot` with a non-empty `Day`; `fixtureConfig` reads it from the `session_day` of the quote (the `quotes` item struct of the test input gains `SessionDay`); `contracts/fixtures/page.companies_page.v2.input.json` is read, not edited.
8. `spotOf` in `cmd/ot/companies_inputs.go` returns `func(context.Context, string, contracts.Day) (companies_page.Spot, bool, error)` with `Price` = the `Close` and `Day` = the `SessionDay` of the newest `trades` bar in the 14 days up to the day; look-back, basis and the gap results (no instrument, no bar, shared symbol) are as before; `cmd/ot/wire.go` is not edited.
9. In `companies_inputs_test.go` the four tests that call `cfg.Spot` (`TestSymbolResolvedViaCatalog`, `TestUnknownSymbolGivesGap`, `TestSymbolResolvedViaStockReader`, `TestSharedSymbolStaysAGap`) read `Spot.Price`; the fallback bar of `fxFakeBars` gets a `SessionDay`; the new test `TestSpotCarriesBarDay` (new) has two bars on different days and asserts that `Day` is the session day of the later one, not the page day.
10. `pepb_test.go`, `companies_pepb_test.go`, `pagereg_test.go` and `pagelist_test.go` are read: each either does not name `Config.Spot` or `Spot` at all (checked by `git grep -n "Spot" -- <file>`) or is edited to the new type; no other file of `cmd/ot` names `companies_page.Config.Spot` (`git grep -n "cfg.Spot\|\.Spot = " -- cmd` lists only `wire.go` and `companies_inputs_test.go`).
11. `docs/contracts/ot/pages/companies_page.md` says that the "Last" cell text is the newest stored daily close with its day ("<price> · close <day>"), that it is not the broker mark, that `header.line` and `stale.text` use `Data of <YYYY-MM-DD>`, and gives the one note line.
12. `git diff --name-only BASE` lists no file outside the Files line, no file under `contracts/` and no file under `web/`.
13. The result matches docs/design/companies-day.md#Decisions.

### companies-pepb-price-day-text

Module: compute. Size: S.
Order: independent of companies-last-day-page-wire.
Contracts: unchanged
Design: docs/design/companies-day.md#Decisions (Decision 6)
Plan: P1
Files: `internal/compute/pepb/series.go`, `internal/compute/pepb/build_test.go`
Acceptance:
1. The `Text` of each `AtPercentile` written in `series.go` is `<KIND> at the <ordinal> percentile of <N> days since <from>: price <P> as of <last day> (history, not a buy price)`, the last day being the day of the last bar of the window, formatted as `WindowFrom` is; `Ratio` and `Price` are as before.
2. `build_test.go` pins the four texts with the day: for the 300 bars from 2023-03-01 the PE texts end "price 60.00 as of 2023-12-25 (history, not a buy price)" and "price 150.00 as of 2023-12-25 (history, not a buy price)", the PB texts likewise; the new test `TestAtPercentileTextNamesLastDay` (new) pins that the day in the text is the day of the last bar and not the window start.
3. `internal/compute/pepbapi/api.go` is not edited: its `Text` comment ("names the window, ends '(history, not a buy price)'") is still true; no file of the pages module is edited; no file of the pages module is edited, because `pepbCell` copies the text and the fake texts of `internal/pages/companies_page/pepb_test.go` are inputs of the page builder, not outputs of this function.
4. `git diff --name-only BASE` lists no file outside the Files line.
5. The result matches docs/design/companies-day.md#Decisions.

## Checked by CI

- Go: `go build ./...`, `go vet ./...` and `go test ./internal/pages/companies_page ./internal/compute/pepb ./cmd/ot ./internal/contracts` (the tests named above, in parallel, no sleeps, fake readers, `t.TempDir()`); the example fixtures of `contracts/fixtures/page.companies_page.v2.*.json` still validate against the schema (`internal/contracts`), `TestBuildEqualsOkFixture` and the `core`, `diagnosis` and `questions` comparison of `companies_inputs_test.go` still pass without edit.
- Web: `tsc`, `eslint`, `vitest` and the build, with `web/` unchanged; the web tests of `web/test/pages/companies` pass without edit.
- The tests of today other than those named pass without edit; the grep claims in the acceptance items give the results they state.
- `crew issue check` of each entry against the head of the design PR.

## Adversarial

Adversarial file: `state/optdesk-drafts/designs/companies-day/adversarial-companies-day-2026-11-17.md`.

1. Gap against the goal: adopted. The pepb price now carries its day (Decision 6, entry companies-pepb-price-day-text); the Last cell says "close" and the page carries the gloss that /positions shows the broker mark (Decisions 1 and 2); the two "Data of" strings now have one format, `YYYY-MM-DD` (Decision 3). The header stays the page day on purpose: a weekend page says the run day in the header and the data day in each price.
2. Gap-vs-core split: adopted. `coreRowsOf` and `Build` call the same `spotAt` (Decision 5); an empty `Day` is "no quote" in both; the fakes of `core_test.go` set `Day` (acceptance 3, 6, 7 of companies-last-day-page-wire).
3. A simpler way: adopted. The day is in the cell text, built in Go; no schema, no generated type, no web entry, no contracts entry (Decision 1). The web renders `cell.text`, `header.line` and `notes` as given, which is checked in `## Today`. The separate header-first entry is not needed: the header is one line of the same file as the Last text.
4. Lock-in: adopted, and mostly gone: there is no `Cell.day` and no in-place schema edit. The `Spot` struct stays; a later source field is an added struct field, not a signature break, so this part of the point is not adopted as a risk. The header wording is pinned in the new tests of one entry and in the docs line, no longer in web tests.
5. Claim most likely false: adopted. The fixtures are no longer claimed "unchanged and validating the new format": they are not edited, and the design says why none of the tests reads them for the changed texts (`## Modules`). The cites are refreshed to origin/main 53df3ef3 and PR 2746 is treated as merged. The schema-description and generated-type questions are gone with the schema change. The v1 fixture mapping question does not arise, because no web payload is built with a new field.
6. Likeliest slip, the pages+ot entry: adopted. Files now name `pepb_test.go`, `companies_pepb_test.go`, `pagereg_test.go`, `pagelist_test.go` and `core_test.go` with the check that each reads the new type; `contracts/fixtures/page.companies_page.v2.input.json` is named as read, not edited. The chain is no longer serial (no schema entry). The run-based claims moved to `## Checked by CI`. The web test base question is gone with the web entry.

## Left out

- /positions price source and its day: `docs/design/spot-from-bars.md` (moves /positions to the bars; not redone here). After both land, the broker mark on /positions still wins where it exists, so a difference between the pages stays possible and is explained by the word "close", the day and the note.
- A day on the implied growth legend and on `core.snapshot_text` ("Quotes of MM-DD", the option quotes): the legend prints no spot price and the snapshot line is its own day; left as they are. Step that brings them: none planned.
- A "stale" flag when a spot day is older than the page day by more than some days: not asked; the day is shown and the owner reads it.
- Showing a mark price next to the close on /companies: not asked.
- A separate day field, a new schema version, or a styled small line under the price: not needed, the text carries the day; it becomes an entry of its own if the owner asks for a separate column.
- The example fixtures `contracts/fixtures/page.companies_page.v2.*.json` keep the old header, `stale.text` and pepb texts (they are examples that validate; none is compared with Build output for these fields).
