# Option page builders: `ot serve` shows the stored short puts as the option page (board 5) and as the one-stock page (board 18)

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: owner 2026-11-08/10: options trading comes first, fast and complete; the web renders data and computes nothing; own glue code sits behind an interface. `docs/design/option-pages-v22.md` `## Left out` ("the Go builders of `option_page`, `stock_option_page`, `full_scan` and their `ot` routes: each its own design"); `docs/design/alignment-v25.md` rows "option candidates page builder (option_page, stock_option_page)" (plan 11 W11, "missing") and "Dropped by the owner 2026-11-09" (line 230: FX 15 s stream, 15:10 premarket capture, scanner, Stooq coarse layer, composite scoring).
Read on origin/main at b43bec5: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-pages-v22.md`, `docs/design/option-candidates-page.md`, `docs/design/option-candidates.md`, `docs/design/ot-wire-split.md` (revision 1), `contracts/schemas/page.option_page.v1.json`, `page.stock_option_page.v1.json`, `page.full_scan.v1.json`, `internal/store/api.go`, `internal/contracts/gen/records.go`, `internal/pages/option_candidates_page/*.go`, `internal/pages/option_data_page/page.go`, `cmd/ot/site.go`, `cmd/ot/sample.go`, `cmd/ot/pagelist.go`, `cmd/ot/candidates_site.go`, `web/src/pages/options/OptionPage.tsx`, `web/src/pages/stockoption/view.tsx`, `web/fixtures/option-page/ok.json`, `web/test/pages/stockoption/fixtures/ok.ts`.

Goal: after the candidates job has stored a run, the owner opens `/options` of `ot serve` and sees board 5 with real rows: one card for the best ok short put of each company (contract line, annualized yield, IV line, liquidity, size), the full candidate table below it, and the strike by expiry chain of the first card; and opens `/options/stock/<symbol>` and sees board 18 for one company: top bar, strike by expiry matrix with the chosen contract, one card per ok contract, a price map per expiry and the table of all its contracts with Θ, Γ and Vega. Every number is a stored column printed as text. What nobody stores yet (HV30, IVP, probability of finishing in the money, the excess over cash, strike %, spread %) is shown as a gap ("缺"), never computed in the builder or the page.

## What main holds today

- Schemas and views exist and are carried: `contracts/schemas/page.option_page.v1.json` and `page.stock_option_page.v1.json`; types `web/src/types/option_page.v1.types.ts` (no `option_page.v1.ts` in `web/src/generated`) and `web/src/generated/stock_option_page.v1.ts`; views registered by `export const registration` at `web/src/pages/options/OptionPage.tsx:152` and `web/src/pages/stockoption/view.tsx:47` (checked). This design changes no schema and no view: `Contracts: unchanged`.
- `option_page.v1` needs `markets` (1 or 2 panes: `cards`, `candidates` table of at most 200 rows, `all_link` whose `target` is an in-page anchor id, `chain` or null), `stale`, `notes`, `gaps`; a card has `verdict` (key `worth`, `marginal`, `pending`, `skip`), free text lines and at most 10 tags; a `Gap` reason is one of an enum that holds `no_data`, `no_reader`, `missing_quote`, `stale`, `not_built`. `stock_option_page.v1` is almost all strings: `top`, `matrix` (strikes at most 100, expiries 30, cells 2000), `cards` (a ladder of exactly 3 pairs), `maps` (marks of kind `spot`, `breakeven`, `strike`; at most 2 bands), `held`, `contracts` (exactly 11 columns), `notice`, `shared_gap`, `footer`.
- The data exists. `store.Candidate` (`internal/store/api.go:561`: kind, issuer, expiry, DTE, strike, multiplier, bid, ask, spot, break-even, annualized yield, delta, IV, status, gates, marks, next earnings, rank, rank in underlying) and `store.CandidateReader.Candidates(ctx, day, kind)` (`internal/store/api.go:607`, newest run of the day, ranked rows first). `store.OptionEodReader.OptionEod(ctx, underlyingID, from, to)` (`internal/store/api.go:38`) returns `contracts.OptionEod` with volume, open interest, gamma, theta per calendar day and vega per vol point (`internal/contracts/gen/records.go:2079` ff). `store.InstrumentNameReader` (`internal/store/api.go:142`) gives a company name.
- The jobs that fill them exist: `internal/jobs/optioncandidates`, `internal/compute/optioncand`, `internal/jobs/optioneod`. Nothing computes HV30, IVP, delta from a model of our own or the probability of finishing in the money: compute stage 2 has no design (`alignment-v25.md` rows "candidates", "structures").
- No builder or route for either page. `cmd/ot/sample.go:13` serves `options` as a sample route of `option_page`, pinned by `cmd/ot/sample_test.go:65`; `/options/candidates` is the other page of the same rows (`docs/design/option-candidates-page.md`, merged, `internal/pages/option_candidates_page`). `siteReaders` already holds `cands`, `eod`, `names` (`cmd/ot/site.go:54-62`), so no new reader is opened.
- `cmd/ot/pagereg.go` does not exist yet: `docs/design/ot-wire-split.md` (revision 1) entry `2 ot-page-registry` creates `registerPage` and `pageFamily` (its Decision 8, line 27), with one exact `route` and no key. `modules.md` derives that a sub-package of pages may import only the root `pages` (no sibling package) and that `cmd/ot` may import every implementation package of pages, so a new package needs no row.
- `full_scan.v1` is not the IBKR scanner: its sections are `short_put`, `covered_call` and `put_credit_spread` tables of candidates. It is left out below for another reason.

## Decisions

1. One package, `internal/pages/option_boards`, holds both builders. They share the candidate lookback, the gate JSON, the decimal text and the envelope, and `modules.md` does not let one pages sub-package import another. Own implementation, because no library reads our gate text or shifts a decimal point in text; the small helpers of `option_candidates_page` (`shiftPoint`, `parseGates`, `daysBack`) are written again here and a later design may move both copies to one place (Left out).
2. Sources. Only `store.CandidateReader` (kind `short_put`), `store.OptionEodReader` (one call per page, for the underlyings shown, on the data day) and, for the one-stock top bar, `store.InstrumentNameReader` (nil gives a null name). Nil `Candidates` gives the page gap `no_reader`; a nil `Eod` gives gap cells for volume, open interest and the Greeks. The builders read no clock and no file, hold no `math/big` and no float parsing, and import no `notify` (grep-checked); the margin cushion and the assignment cash have no row (ADR 0010).
3. Day and lookback, the rule of `option-candidates-page.md` Decision 3 for one kind: the builder reads the build day and, when it has no row, the earlier calendar days back to 7 (day subtraction on the date text, no calendar); the first day with rows is the data day. A data day before the build day sets `stale` (text "数据日 YYYY-MM-DD,早于今日", `stale_after` the build day) and a `stale` gap. No row in 7 days gives the gap `no_data` ("没有候选行,候选任务尚未存入") and an empty page that is still valid.
4. `option_page`, one pane, US (`currency` USD). The Japanese pane is a fixed note "日本期权:后续设计 (option-candidates-jp)" as on the candidates page; it is never silent. Cards: the ok rows whose `RankInUnderlying` is 1, in the stored order, at most 6 (`maxCards`). Verdict from the stored status only: `ok` is `worth` ("值得看", tone good), `unknown` is `pending` ("数据不全", warn), `blocked` is `skip` ("不够格", muted). `marginal` is never written: it needs a judgement or a score and the owner dropped composite scoring (2026-11-09).
5. Card text, all printed from stored columns. `contract_line` "sell put 62 · 11-13 周五 · 22 天 · Δ −0.15" (strike as stored; month-day and weekday of the stored expiry; DTE as stored; delta as stored with "−" for the sign, "Δ 缺" when null). `iv_line` "IV 38% · HV30 缺 · IVP 缺" (IV by moving the decimal point two places in the stored text, no rounding; HV30 and IVP are always 缺). `annualised` is `PercentAmount` from the stored yield by the same shift, unit `percent`. `liquidity_line` "成交 V · 持仓 OI" from `OptionEod` of the row's instrument on the data day, "缺" for a missing value. `size_line` "1 张 · 行权价 62 · 乘数 100" (printed, not multiplied; `size_usd_approx` null). `premium_line` "权利金 1.20 (收盘 bid 估) · 盈亏平衡 61.38 · 到期在价内概率 缺 · 比现金高 缺". Tags: "E MM-DD" for a listed date, "E 无" for `absent`, "E 缺" otherwise, and one badge text per stored mark ("财报日期未确认", "已过财报,60 天内无财报"). The rich-premium tag has no producer and is not written. `reason` "各关卡通过" plus the mark texts. `headline` "N 个值得看。", N the count of stored ok rows. `all_link` is "全部 M 个机会", target the id of the candidate table's section.
6. Candidate table, the 12 columns of board 5 in the fixture order: symbol, verdict, annualised, strike_pct, dte, delta, bid_ask, spread_pct, volume, open_interest, ivp, earnings. All stored rows of the kind in the stored order (ok rows first, as the reader gives them), at most 200 (a note says how many were cut). `strike_pct`, `spread_pct` and `ivp` are gap cells (`no_data`, text "缺"): the first two are arithmetic on stored numbers and no stored column holds them (open question 1). The E cell is "E MM-DD 在期内" when the gate `events` of the row is `block`, else the tag text of Decision 5. A sort key `value` is an integer made by moving the decimal point in text (DTE as stored, delta times 100), and null when the text is not exact.
7. Chain card of the first card's company: rows are its stored contracts sorted by strike ascending (decimal text compare, no float), columns its expiries ascending; each cell is the stored bid (`UsdAmount` text), a pair with no stored row is a gap cell `no_data`; summary "已选:SYM MM/DD 行权价 $K Put". No row for the company gives `chain` null (the view shows "没有链数据").
8. `stock_option_page` for one company. The key is the `Issuer` text of a stored row (`^[A-Za-z0-9_-]{1,64}$`); the route is `/options/stock/<key>` and a bare `/options/stock` uses the issuer of the first ok row of the data day, else of the first row. A key with no row gives `no_card` ("没有该标的的候选行"), an empty matrix with `empty` text, `shared_gap` text and `run_status` `partial`.
   - `top`: `symbol` the key; `name` from the name reader or null; `spot` "现价 X" (stored spot of the first row); `day_change`, `day_tone`, `quote`, `refresh_expiry`, `refresh_strike` null (no stored source); `iv` "IV 38% · HV30 缺 · IVP 缺" (IV of the first ranked row, "IV 缺" when null); `earnings` "财报 MM-DD" or "财报 无" or "财报 缺" from the stored date and state; `trend` "趋势 缺"; `policy` the fixed text "策略 sell put,买价线内才卖".
   - `matrix`: expiries ascending, strikes ascending (decimal text compare), one cell per stored row, key `c<instrument id>`. `yield_text` "年化 12.3456%", `premium` "权利金 1.20" (stored bid), `delta` "Δ −0.15", `volume` "成交量 N" or "成交量 缺". `shade` `worth` for status ok, else `none` (`marginal` never written, Decision 4); `star` when `RankInUnderlying` is 1; `chosen` for the one ok row with the lowest stored `Rank`; `thin` when the gate `quote` is not `pass`; `earnings` when the gate `events` is `block`; `out` is always false because no delta band is defined anywhere (Left out). Cells and strikes over the schema limits are cut in the stored order and `notice` says so.
   - `cards`: one per ok row, at most 10, in the stored order (`in_matrix` true, `chosen` as in the matrix): title "put 62 11-13", `pass_tag` true, `yield_line` "年化 12.35% · 22 天", a ladder of three pairs (买价 bid, 中间价 缺, 卖价 ask), `advice` "建议 卖出 1 张 put (收盘 bid 估,次日才卖)", `amounts` ["行权价 62 · 乘数 100", "盈亏平衡 61.38"], `risk` "风险 股价低于 62 会被行权", `liquidity` "流动性 成交量 N · 持仓 M".
   - `maps`: one per expiry (at most 30): marks `spot` (stored spot), `breakeven` and `strike` (stored texts of the ok rows of that expiry), `bands` empty and `description` "波动区间 缺 (compute 尚无)".
   - `contracts`: the 11 columns "到期, 行权价, 买价, 卖价, Δ, IV, Θ/天, Γ, Vega, 成交量, 年化"; one row per stored row of the company in the stored order (at most 200); Θ, Γ and Vega from `OptionEod` as the shortest decimal text, "缺" when null. `held` is an empty list and `notice` points to `/options/tracking`; `footer` "由存档收盘报价与候选表生成".
9. Shared envelope. `run_status` is `ok`, or `partial` when any gap exists; `data_times` has one source "candidates" with the run's `ComputedAt` in the owner zone; `scope` is `options` or `stock-options`; `run_id`, `build_id`, zone and `generated_at` (UTC, given by the caller) as in `option_candidates_page`. A store error fails the build (the route answers 500 with its fixed text).
10. Routes. `/options` registers the family `option_page` (name `option_page`, order 40: `ot-wire-split` reserves 10, 20 and 30) and replaces the sample route of that path. `/options/stock/<key>` registers one family with a key; it has no name, so `ot page --name` does not know it (it needs a key). Both are registered through `registerPage` in their own files of `cmd/ot` with their own page documents under `docs/contracts/ot/pages/`; `ot.md`, `wire.go`, `runonce.go` and `pagelist.go` are not edited.
11. Frontend stack (ADR 0034): the views exist and are not touched, so no UI line is added here. A payload that does not match its schema is a bug of the builder: each builder test validates its output against the schema file.

## Modules

None changed. The new package `internal/pages/option_boards` belongs to the row pages (`internal/pages/**`, `docs/contracts/pages.md`); `cmd/**` and `docs/contracts/ot/**` to ot. `cmd/ot` imports the package by the rule of the row ot (every discovered implementation package of its depends rows), `arch.toml` is generated, and `docs/design/modules.md` is not edited.

## Interface

Owner: pages, frozen by entry 1, read by entries 5 and 6 (ot) and by entries 2 to 4. Package `internal/pages/option_boards`:

- `type OptionConfig struct { RunID, BuildID, Scope string; Zone *time.Location; Candidates store.CandidateReader; Eod store.OptionEodReader }` and `type StockConfig struct { RunID, BuildID, Scope string; Zone *time.Location; Candidates store.CandidateReader; Eod store.OptionEodReader; Names store.InstrumentNameReader; Key string }`. A nil `Zone` is an error of `Build`.
- `func NewOptionPage(c OptionConfig) pages.Page` with `Meta{Name: "option_page", Schema: "page.option_page.v1"}` and `func NewStockPage(c StockConfig) pages.Page` with `Meta{Name: "stock_option_page", Schema: "page.stock_option_page.v1"}`.
- `func BuildOption(ctx context.Context, c OptionConfig, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error)` and `func BuildStock(ctx context.Context, c StockConfig, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error)` (the `pages.Builder` shape; `r` is unused; `at` is `generated_at` in UTC).
- Entry 1 gives both `Build` functions a body that answers the error "option_boards: not built"; entries 3 and 4 replace them. Everything else of the package is unexported.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port and touches no live broker or database. Acceptance items are decided by reading, grep or git and each prints one number; the commands CI runs are under `Checked by CI`. `BASE` is `git merge-base HEAD origin/main`.

Entries 5 and 6 also wait for entry `2 ot-page-registry` of `docs/design/ot-wire-split.md` at its revision 1 (it creates `cmd/ot/pagereg.go`, `registerPage` and `siteOpeners`, which both use). That order is across designs, so no Order line states it: the dispatcher queues 5 and 6 only after that entry has merged, and re-reads their lines if its revision renames a field of `pageFamily`.

### 1 option-boards-interface (interface)

Module: pages. Size: S.
Files: `internal/pages/option_boards/page.go`, `internal/pages/option_boards/option_build.go`, `internal/pages/option_boards/stock_build.go`, `internal/pages/option_boards/page_test.go`, `docs/contracts/pages.md`
Design: docs/design/option-page-builders.md#Interface
Order: first.
Goal: the two configs, the two constructors and the two `Build` functions of `## Interface` with stub bodies; `pages.md` gets a paragraph for the package. The contract test has two sides: a caller fills every field of both configs with fakes, `NewOptionPage` and `NewStockPage` carry the stated `Meta`, and `Build` answers the stub error.
Acceptance:
1. `grep -c "^func NewOptionPage\|^func NewStockPage\|^func BuildOption\|^func BuildStock" internal/pages/option_boards/page.go internal/pages/option_boards/option_build.go internal/pages/option_boards/stock_build.go | awk -F: '{s+=$2} END {print s}'` prints 4.
2. `grep -c 'Name: "option_page"\|Name: "stock_option_page"' internal/pages/option_boards/page.go` prints 2.
3. `grep -c "option_boards" docs/contracts/pages.md` prints at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards' ':!docs/contracts/pages.md' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (new `TestPageMeta`, `TestBuildNotBuilt`); `go run ./scripts/arch` leaves nothing to commit; `Contracts: unchanged`.

### 2 option-boards-core

Module: pages. Size: M.
Files: `internal/pages/option_boards/decimal.go`, `internal/pages/option_boards/gates.go`, `internal/pages/option_boards/load.go`, `internal/pages/option_boards/eodtext.go`, `internal/pages/option_boards/envelope.go`, `internal/pages/option_boards/strings.go`, `internal/pages/option_boards/decimal_test.go`, `internal/pages/option_boards/gates_test.go`, `internal/pages/option_boards/load_test.go`
Design: docs/design/option-page-builders.md#Decisions
Order: after 1.
Goal: the shared parts of Decisions 2, 3 and 9, unexported: `decimal.go` (percent shift of a stored decimal text, signed text with "−", exact scaled integer, decimal text compare, month-day and weekday of a date text), `gates.go` (the stored gate JSON, one gate's status, the marks and their Chinese texts), `load.go` (the 7-day lookback of the short puts with the data day and the stale flag; the one `OptionEod` call keyed by instrument id), `eodtext.go` (volume, open interest and Greeks as text, "缺" for null), `envelope.go` (`SourceTime`, `StaleLabel`, `Gap` and the run status), `strings.go` (the shell texts).
Acceptance:
1. `grep -c "^func " internal/pages/option_boards/decimal.go` prints at least 5, and the shift of `"0.123456"` to `"12.3456%"`, the refusal of `"abc"` and of `"1e-3"`, and the compare of `"9.5"` and `"10"` are cases of `decimal_test.go` (named by the test names of the CI line).
2. `cat internal/pages/option_boards/*.go | grep -c "ParseFloat\|math/big\|time.Now(\|os.Getenv\|internal/notify"` prints 0.
3. `grep -c "lookbackDays = 7" internal/pages/option_boards/load.go` prints 1.
4. `wc -l internal/pages/option_boards/*.go | grep -v total | sort -n | tail -1 | awk '{print $1}'` prints a number below 600.
5. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (new `TestShiftPercent`, `TestCompareDecimal`, `TestParseGates`, `TestLookbackFindsOlderDay`, `TestLookbackStaleFlag`, `TestNoRowsInSevenDays`, `TestEodTextGap`); `Contracts: unchanged`.

### 3 option-page-builder

Module: pages. Size: L.
Files: `internal/pages/option_boards/option_build.go`, `internal/pages/option_boards/option_cards.go`, `internal/pages/option_boards/option_table.go`, `internal/pages/option_boards/option_chain.go`, `internal/pages/option_boards/option_page_test.go`, `internal/pages/option_boards/option_schema_test.go`, `internal/pages/option_boards/testdata/option_page_ok.golden.json`, `internal/pages/option_boards/testdata/option_page_no_rows.golden.json`
Design: docs/design/option-page-builders.md#Decisions
Order: after 2.
Goal: Decisions 4 to 7 and 9 for `option_page`: `BuildOption` with the cards, the 12-column candidate table, the chain card and the notes and gaps. The golden files pin the output of two fixed inputs (a day with ok, blocked and unknown rows of two companies and a stale day; no row in 7 days).
Acceptance:
1. `grep -c "HV30 缺" internal/pages/option_boards/option_cards.go` prints at least 1 and `grep -c "maxCards = 6" internal/pages/option_boards/option_cards.go` prints 1.
2. `grep -c '"marginal"' internal/pages/option_boards/option_cards.go internal/pages/option_boards/option_table.go | awk -F: '{s+=$2} END {print s}'` prints 0.
3. `grep -c "Δ −0.15\|sell put" internal/pages/option_boards/testdata/option_page_ok.golden.json` prints at least 1, and `grep -c "no_data" internal/pages/option_boards/testdata/option_page_no_rows.golden.json` prints at least 1.
4. `cat internal/pages/option_boards/option_build.go internal/pages/option_boards/option_cards.go internal/pages/option_boards/option_table.go internal/pages/option_boards/option_chain.go | grep -c "float64\|ParseFloat"` prints 0 (the Eod text goes through `eodtext.go`, entry 2).
5. `wc -l internal/pages/option_boards/option_*.go | grep -v total | sort -n | tail -1 | awk '{print $1}'` prints a number below 600.
6. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (new `TestOptionPageGolden`, `TestOptionPageNoRowsGolden`, `TestOptionPageValidatesSchema`, `TestCardsBestPerCompany`, `TestVerdictFromStatus`, `TestUnknownGateIsPending`, `TestChainGapCell`, `TestTableCutAt200`, `TestNoReaderGap`); `Contracts: unchanged`.

### 4 stock-option-page-builder

Module: pages. Size: L.
Files: `internal/pages/option_boards/stock_build.go`, `internal/pages/option_boards/stock_top.go`, `internal/pages/option_boards/stock_matrix.go`, `internal/pages/option_boards/stock_cards.go`, `internal/pages/option_boards/stock_contracts.go`, `internal/pages/option_boards/stock_page_test.go`, `internal/pages/option_boards/stock_schema_test.go`, `internal/pages/option_boards/testdata/stock_option_page_ok.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_no_key.golden.json`
Design: docs/design/option-page-builders.md#Decisions
Order: after 2.
Goal: Decision 8 and 9 for `stock_option_page`: `BuildStock` with the top bar, matrix, cards, maps, contracts table, notice and footer, the bare key rule and the empty page for an unknown key. Entry 3 and this entry share no file.
Acceptance:
1. `grep -c "趋势 缺\|HV30 缺" internal/pages/option_boards/stock_top.go` prints at least 1.
2. `grep -c '"Θ/天"' internal/pages/option_boards/stock_contracts.go` prints 1 and `grep -c "out: *false\|Out: *false" internal/pages/option_boards/stock_matrix.go` prints 1.
3. `grep -c "没有该标的的候选行" internal/pages/option_boards/stock_build.go internal/pages/option_boards/testdata/stock_option_page_no_key.golden.json | awk -F: '{s+=$2} END {print s}'` prints at least 2.
4. `cat internal/pages/option_boards/stock_*.go | grep -c "ParseFloat\|math/big\|internal/notify"` prints 0.
5. `wc -l internal/pages/option_boards/stock_*.go | grep -v total | sort -n | tail -1 | awk '{print $1}'` prints a number below 600.
6. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (new `TestStockGolden`, `TestStockNoKeyGolden`, `TestStockValidatesSchema`, `TestBareKeyIsFirstOkRow`, `TestMatrixOrder`, `TestMatrixFlagsFromGates`, `TestContractsElevenColumns`, `TestMatrixCutAtLimits`); `Contracts: unchanged`.

### 5 option-page-route

Module: ot. Size: M.
Files: `cmd/ot/option_page_site.go`, `cmd/ot/option_page_site_test.go`, `cmd/ot/sample.go`, `cmd/ot/sample_test.go`, `docs/contracts/ot/pages/option_page.md`
Design: docs/design/option-page-builders.md#Decisions
Order: after 3.
Goal: Decision 10 for `/options`: the file registers the family `option_page` (route `options`, name `option_page`, order 40) through `registerPage`; `build` calls `option_boards.NewOptionPage` with `rd.cands` and `rd.eod` through a package variable that a test replaces (the page adds no field to `deps`); `named` opens the candidate and option_eod stores with the pages line for `ot page --name option_page`, nil readers when the line is missing (gap `no_reader`), the demo readers with the demo profile; `sample.go` loses the entry `options` and `sample_test.go` the pair `"/options": "option_page"` of the map in `TestSampleRoutes`.
Acceptance:
1. `grep -c "registerPage(" cmd/ot/option_page_site.go` prints 1 and `grep -cE "order: +40" cmd/ot/option_page_site.go` prints 1.
2. `grep -c '"options"' cmd/ot/sample.go` prints 0 and `grep -c '"/options"' cmd/ot/sample_test.go` prints 0.
3. `grep -c "^## Version" docs/contracts/ot/pages/option_page.md` prints 1 and `grep -c "^Order: " docs/contracts/ot/pages/option_page.md` prints 1.
4. `cat cmd/ot/option_page_site.go | grep -c "notify\|telegram"` prints 0.
5. `git diff --name-only $(git merge-base HEAD origin/main) -- cmd/ot/site.go cmd/ot/pagelist.go cmd/ot/wire.go cmd/ot/runonce.go cmd/ot/pagereg.go docs/contracts/ot.md | wc -l` prints 0.
Checked by CI: ci-go; `go test ./cmd/ot/... -race -count=1` (new `TestOptionPageRoute`, `TestOptionPageNamed`, `TestOptionPageDemoProfile`, `TestOptionPageNoReaderGap`; `TestSampleRoutes` still passes with the five sample routes left); `Contracts: unchanged`.

### 6 stock-option-page-route

Module: ot. Size: M.
Files: `cmd/ot/stock_option_site.go`, `cmd/ot/stock_option_site_test.go`, `cmd/ot/pagereg.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/site.go`, `docs/contracts/ot/pages/stock_option_page.md`
Design: docs/design/option-page-builders.md#Decisions
Order: after 4.
Goal: Decision 8 and 10 for `/options/stock/<key>`. `pageFamily` gains one optional field `buildKey func(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, key string) ([]byte, error)`; when it is set, `route` is the prefix `options/stock` and both `options/stock` (key "") and `options/stock/<key>` call it; a key that does not match `^[A-Za-z0-9_-]{1,64}$` or has a further slash is a 404; a non-empty `route` clash still panics. `pagereg.go` gets the lookup and `site.go` the call of it in `extraPageBuilder`. The family has no name and no `named`. Entry 5 and this entry share no file.
Acceptance:
1. `grep -c "buildKey" cmd/ot/pagereg.go` prints at least 2 and `grep -c "registerPage(" cmd/ot/stock_option_site.go` prints 1.
2. `grep -c "^## Version" docs/contracts/ot/pages/stock_option_page.md` prints 1 and `grep -c "^Order: " docs/contracts/ot/pages/stock_option_page.md` prints 1.
3. `cat cmd/ot/stock_option_site.go | grep -c "notify\|telegram"` prints 0.
4. `grep -c 'stock_option_page' cmd/ot/pagelist.go` prints 0, so `ot page --name stock_option_page` stays an unknown page.
5. `git diff --name-only $(git merge-base HEAD origin/main) -- cmd/ot/option_page_site.go cmd/ot/sample.go cmd/ot/wire.go cmd/ot/runonce.go cmd/ot/pagelist.go docs/contracts/ot.md | wc -l` prints 0.
Checked by CI: ci-go; `go test ./cmd/ot/... -race -count=1` (new `TestStockRouteKey`, `TestStockRouteBareKey`, `TestStockRouteBadKey404`, `TestPageKeyRouteClash`, `TestStockRouteNoReaderGap`; the tests of `pagereg_test.go` of `ot-wire-split` pass unchanged); `Contracts: unchanged`.

## Left out

- HV30, IVP, the probability of finishing in the money, the excess over cash and a delta of our own: no producer and compute stage 2 has no design; shown as "缺". When a producer exists, a design changes only the text of `iv_line`, `premium_line` and the `ivp` cell.
- `strike_pct` and `spread_pct` of the candidate table: gap cells until either compute stage 2 stores them or the owner answers open question 1 with b.
- The rich-premium tag (PR1847) and the verdict `marginal`: a threshold or a score; composite scoring was dropped by the owner on 2026-11-09. The scanner, the Stooq coarse layer, the 15 s FX stream and the 15:10 premarket capture (dropped the same day) are not used by anything here.
- `full_scan`: not the scanner but the full candidate table by strategy (short put, covered call, put credit spread). It needs its own builder and route; until then its content for short puts is the table of `option_page` and `/options/candidates`. Covered-call extras were dropped by the owner (`option-pages-v22.md`).
- The Japanese pane and Japanese candidates (`option-candidates-jp`); only a note is shown.
- A link from a card to its one-stock page: `Opportunity` has no link field and the schema is carried, so it would be a v2; the one-stock page is reached by its address. The red E label on both pages (v2, as in `option-pages-v22.md`).
- On the one-stock page: `held` (the owner's open positions are on `/options/tracking`), `day_change`, `quote` time, `trend`, the sigma and preference bands of the price map, the delta band behind `out` (always false), the mid price of the ladder and `refresh_expiry`/`refresh_strike`; `ot page --name` for it (it needs a key).
- One shared copy of the decimal and gate helpers for `option_candidates_page` and `option_boards`: `modules.md` forbids a sibling import, so a later design moves both to the root `pages` package or a new row.
- Any change of `option_page.v1`, `stock_option_page.v1`, the web views, the fixtures and the bar of the shell: unchanged by this design.

## Open questions for the owner

1. Strike % and spread % in the candidate table: a) 缺 until compute stage 2 stores them (this design); b) the builder computes them from the stored strike, spot, bid and ask with exact decimal arithmetic now.
2. The one-stock page address: a) `/options/stock/<symbol>` with a key (this design, one extension of the page registry); b) a fixed address that always shows the top-ranked company, no registry change.
