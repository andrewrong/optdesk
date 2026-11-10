# Option page names: the option pages show the ticker, not the issuer, and read the option_eod rows of the chain day

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: two defects found live on 2026-11-10 on `ot serve`: `/options/candidates` and `/options` show `cik:0000320193` where the owner expects `AAPL`; `/options` and `/options/stock/<symbol>` print "缺" for volume, open interest and the Greeks although `option_eod` holds rows. It amends `docs/design/option-page-builders.md` Decisions 2 and 8 (merged, revision 1) and `docs/design/option-candidates-page.md` (merged); the amended text of both stays as written and this file wins where they differ. Owner 2026-11-08: own glue code sits behind an interface.
Read on origin/main at 3c91d89d: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-page-builders.md`, `docs/design/web-shell-layout.md` (format), `docs/contracts/ot/pages/stock_option_page.md`, `internal/store/api.go`, `internal/pages/option_boards/*.go`, `internal/pages/option_candidates_page/{page,cells}.go`, `internal/jobs/optioncandidates/{input,convert}.go`, `cmd/ot/{wire,site,pagereg,option_page_site,stock_option_site,candidates_site}.go`.

Goal: the owner opens `/options/candidates`, `/options` and `/options/stock/AAPL` and sees the ticker wherever a company is named, `/options/stock/AAPL` shows the company (today every real key is a 404), and the volume, open interest, Θ, Γ and Vega cells hold the stored values of the last chain day instead of "缺". No schema, no view and no stored column changes.

## What main holds today

- Cause 1 (the name). The job stores the issuer text of the watch list, `cik:<10 digits>` (`internal/jobs/optioncandidates/convert.go:142`, `Issuer: ul.issuer`), and no ticker: `store.Candidate` has `Issuer` and `UnderlyingID` and no symbol column. Every display reads `Issuer`: `option_candidates_page/cells.go:195` (`company`), `option_boards/option_cards.go:116`, `option_table.go:100`, `option_chain.go:71-72`, `stock_build.go:155` and `topSymbol`. The ticker is stored one join away: the job itself finds the underlying by `bySymbol[ToUpper(symbol)]` from `store.InstrumentNameReader` (`input.go:120`), so for a stock `InstrumentName.Name` is the ticker (`internal/store/api.go:136`). `StockConfig` has `Names` and uses it only for `top.name`, which is the same ticker; `OptionConfig` and `option_candidates_page.Config` have no names reader, and `deps.optionCandidatesPage` (`cmd/ot/wire.go:173`) passes none. `siteReaders.names` exists (`cmd/ot/site.go:61`).
- Cause 1b (the key). `option-page-builders.md` Decision 8 keys the one-stock page by the `Issuer` text and the route accepts `^[A-Za-z0-9_-]{1,64}$` (`cmd/ot/pagereg.go:64`). `cik:0000320193` has a colon, so the route answers 404 before the builder runs: the one-stock page cannot be opened for any stored row. Decision 8 is wrong for real data; the tests use issuers without a colon.
- Cause 2 (the day). `option_boards/load.go` `loadEod` reads `OptionEod(underlying, day, day)` with `day` the candidate day (`SessionDay` of the stored rows). Decision 2 says "reads `from` = `to` = the data day", and the data day is the day of the candidate run. The job does not read that day: it reads the chain day, the newest day on or before the run day that holds `option_eod` rows of the underlying, up to 14 days back (`input.go:22`, `locate`, `priorChainDay`), because the capture of the run day may not exist (weekend, holiday, run before the capture). The stored rows keep the run day only, so on any run day that has no `option_eod` rows the read is empty and every eod cell is a gap. Decision 2 and the job disagree; the job is right (the bids, spot and yields are of the chain day), so the page must read the chain day.
- `/options/candidates` does not read `option_eod` at all (it prints `BidEod` and the other stored columns); only `/options` and `/options/stock` have cause 2.
- `option_boards` and `option_candidates_page` are two packages of the module pages and may not import each other (`modules.md`), so each gets its own copy of the small name helper; the copy is a known cost (`option-page-builders.md` Left out).

## Decisions

1. The display name of a company is the ticker: `InstrumentName.Name` of the row's `UnderlyingID`, read in one call per page build for the distinct underlying ids of the rows shown (at most 500 ids, the reader's limit; the pages show at most 200 rows). A nil `Names`, a missing id or an empty name falls back to the stored `Issuer` text: the page never shows an empty cell and never fails because a name is missing. A reader error fails the build (the route answers 500), as for every other store error. Own implementation, because no library maps an instrument id to the issuer text of our rows; the helper is one small file per package (`names.go`) behind the existing `store.InstrumentNameReader` interface.
2. The three pages use the display name in every place that showed the issuer: the company cell of `/options/candidates`, the card symbol, the table symbol cell and the chain card title and summary of `/options`, and the top bar symbol and the bare key of `/options/stock`. Grouping and ranking keep using `Issuer`; only the printed text changes. `option_candidates_page` and `option_boards` get a field `Names store.InstrumentNameReader` in their configs (an interface change, entry 1); `OptionConfig.Names` is new, `StockConfig.Names` stays and gains the role of the ticker source. The name column of the top bar (`top.name`) keeps the reader's `Name` as today; a company's long name has no source (Left out).
3. The key of the one-stock page is the ticker. `/options/stock/AAPL` shows the rows whose display name equals the key compared upper-case; a key equal to a stored `Issuer` text is also accepted by the builder (so a row with no name still has a page, and the earlier tests stay true), though the route pattern lets only issuers without a colon reach it. The bare `/options/stock` uses the display name of the first ok row of the data day, else of the first row (Decision 8 of `option-page-builders.md` with the name in place of the issuer). Two issuers with one ticker are one company on the page (the same rows as one key). The route, `pageKeyPattern` and `cmd/ot/pagereg.go` do not change.
4. The chain day. For each distinct `UnderlyingID` of the rows shown, `loadEod` makes one `OptionEod(underlying, dataDay-14, dataDay)` call (14 is the job's `lookbackDays`, written as its own constant with that comment), takes the greatest `SessionDay` among the rows returned as the chain day of that underlying, keeps the rows of that day, and for each instrument id keeps the row with the greatest `capture_id` (the rule of Decision 2 of `option-page-builders.md`, applied to the chain day). The signature of `loadEod` does not change. When the chain day of an underlying is before the data day, the page adds the note "量与持仓取自 YYYY-MM-DD(链日)" for the earliest such day, so the owner sees which day the cells are of; no gap is raised, the cells are filled. An underlying with no row in the window keeps gap cells. `OptionEod` is ordered by session day, so the greatest day is the last row. Bound: one call per distinct underlying of the rows shown (the size of the watch list, at most 200 rows), each of at most 15 session days of one underlying's chain; there is no row limit in the reader, so the cost is up to about 15 times the old read, which is accepted for a page built per request on a small watch list, and entry 2 has a test with a window of 15 days. The 14 is a copy of the job's `lookbackDays` (a comment names the line); a shared constant would need an import between a page and a job, which `modules.md` does not allow.
5. Known limit, named: the job and the page both pick "the newest day on or before the run day", but the job picked it when it ran and the page picks it when it is built. If `option_eod` rows of a later day (still on or before the run day) landed after the run, the page shows the newer day's volume next to the older bid. The note of Decision 4 names the day shown; storing the chain day in the candidate row would remove the limit, but it is a column and a migration of `candidates.option_candidate` and is Left out.
6. The tests use the fakes of the packages: a names fake with a known id, an unknown id and an empty name; an eod fake that holds rows on the day before the data day only. Every test calls `t.Parallel()`, uses `t.TempDir()` for any file, no clock sleep and no live store. Each builder test still validates its output against its schema file. Contracts: unchanged; the `ot` page documents are edited in the same entry as their code.

## Modules

None changed. The packages `internal/pages/option_boards` and `internal/pages/option_candidates_page` belong to the row pages (`internal/pages/**`); `cmd/**` and `docs/contracts/ot/**` to ot. `docs/design/modules.md` is not edited. Each entry below names the files of one module.

## Interface

Owner: pages, frozen by entry 1, read by entries 3, 4 and 5 (ot).

- `internal/pages/option_boards/page.go`: `OptionConfig` gains `Names store.InstrumentNameReader` (nil gives the stored issuer text as the display name); `StockConfig` keeps its fields and its comment says `Key` is the ticker or a stored issuer text and `Names` is the ticker source.
- `internal/pages/option_candidates_page/page.go`: `Config` gains `Names store.InstrumentNameReader` with the same rule.
- No function is added or removed. `deps.optionCandidatesPage` in `cmd/ot/wire.go` is a composition field, not an interface file of a package; entry 5 gives it one more argument.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port and touches no live broker or database. Acceptance items are decided by reading, grep or git and each prints one number; the commands CI runs are under `Checked by CI`. `BASE` is `git merge-base HEAD origin/main`.

### 1 option-names-interface (interface)

Module: pages. Size: S.
Files: `internal/pages/option_boards/page.go`, `internal/pages/option_boards/page_test.go`, `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/page_test.go`
Design: docs/design/option-page-names.md#Interface
Order: first.
Goal: the two `Names` fields of `## Interface` and the contract tests of both sides: a caller fills `Names` with a fake in each config and `Build` still answers as before for a nil `Names`.
Acceptance:
1. `grep -c "Names *store.InstrumentNameReader" internal/pages/option_boards/page.go` prints 2.
2. `grep -c "Names *store.InstrumentNameReader" internal/pages/option_candidates_page/page.go` prints 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards/page.go' ':!internal/pages/option_boards/page_test.go' ':!internal/pages/option_candidates_page/page.go' ':!internal/pages/option_candidates_page/page_test.go' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards ./internal/pages/option_candidates_page -race -count=1` (new `TestConfigNamesField` in each package); `go run ./scripts/arch` leaves nothing to commit; `Contracts: unchanged`.

### 2 option-boards-chain-day

Module: pages. Size: S.
Files: `internal/pages/option_boards/load.go`, `internal/pages/option_boards/load_test.go`
Design: docs/design/option-page-names.md#Decisions
Order: first.
Goal: Decision 4 in `loadEod` and a function that names the earliest chain day before the data day; the signature of `loadEod` does not change, so no other file is edited. The note text is returned by a function of `load.go` and added to the notes by entry 3 (until then it is not shown).
Acceptance:
1. `grep -cE "chainLookbackDays +[=] +14" internal/pages/option_boards/load.go` prints 1.
2. `grep -c "OptionEod(ctx, int64(c.UnderlyingID), day, day)" internal/pages/option_boards/load.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards/load.go' ':!internal/pages/option_boards/load_test.go' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (new `TestEodChainDayBeforeDataDay` (rows on the day before only fill the cells), `TestEodChainDayNewestWins` (two days in the window, the newer is read), `TestEodChainDayCaptureWins`, `TestEodNoRowInWindow` (gap cells), `TestEodOneCallPerUnderlying` still passes); `Contracts: unchanged`.

### 3 option-boards-names

Module: pages. Size: M.
Files: `internal/pages/option_boards/names.go`, `internal/pages/option_boards/names_test.go`, `internal/pages/option_boards/option_build.go`, `internal/pages/option_boards/option_cards.go`, `internal/pages/option_boards/option_table.go`, `internal/pages/option_boards/option_chain.go`, `internal/pages/option_boards/stock_build.go`, `internal/pages/option_boards/stock_top.go`, `internal/pages/option_boards/option_page_test.go`, `internal/pages/option_boards/stock_page_test.go`, `internal/pages/option_boards/testdata/option_page_ok.golden.json`, `internal/pages/option_boards/testdata/stock_option_page_ok.golden.json`
Design: docs/design/option-page-names.md#Decisions
Order: after 1, after 2.
Goal: Decisions 1 to 4 in `option_boards`: `names.go` (one `InstrumentNames` call for the distinct underlying ids, a map id to display name with the issuer fallback), the display name in the cards, table, chain card, top bar and the key match of the one-stock page, the bare key, and the chain-day note of entry 2 (in `notes` of `/options`, appended to `notice` of the one-stock page, which has no `notes`). The one-stock builder reads the names once and reuses the map for the key match, `bareKey` and `top.name` (the separate `companyName` read is removed). The golden inputs name the underlyings `AAPL` and `MSFT` and hold an eod row on the day before the data day.
Acceptance:
1. `grep -c "InstrumentNames(" internal/pages/option_boards/names.go` prints 1.
2. `cat internal/pages/option_boards/option_cards.go internal/pages/option_boards/option_table.go internal/pages/option_boards/option_chain.go internal/pages/option_boards/stock_top.go internal/pages/option_boards/stock_build.go | grep "Issuer" | grep -c "Symbol:\|Text:\|Title:\|Summary:\|return r.Issuer"` prints 0 (grouping lines such as `r.Issuer == first.Issuer` stay).
3. `grep -c "cik:" internal/pages/option_boards/testdata/option_page_ok.golden.json` prints 0 and `grep -c "AAPL" internal/pages/option_boards/testdata/option_page_ok.golden.json` prints at least 1.
4. `grep -c "AAPL" internal/pages/option_boards/testdata/stock_option_page_ok.golden.json` prints at least 1.
5. `ls internal/pages/option_boards/*.go | grep -v _test.go | xargs wc -l | grep -v total | sort -n | tail -1 | awk '{print $1}'` prints a number below 600.
6. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_boards' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_boards -race -count=1` (new `TestNamesTicker`, `TestNamesFallbackIssuer` (nil reader, missing id, empty name), `TestNamesReaderError`, `TestStockKeyTicker` (upper and lower case), `TestStockKeyIssuerStillMatches`, `TestStockBareKeyIsTicker`, `TestChainDayNoteShown`; the goldens and `TestOptionPageValidatesSchema`, `TestStockValidatesSchema` pass); `Contracts: unchanged`.

### 4 option-candidates-page-names

Module: pages. Size: S.
Files: `internal/pages/option_candidates_page/names.go`, `internal/pages/option_candidates_page/names_test.go`, `internal/pages/option_candidates_page/cells.go`, `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/builder_test.go`, `internal/pages/option_candidates_page/testdata/`
Design: docs/design/option-page-names.md#Decisions
Order: after 1.
Goal: Decision 1 and 2 in `option_candidates_page`: the company cell prints the display name; grouping by `Issuer` is unchanged. `testdata/` here is the directory of the existing goldens of this package, edited only where a company cell changes.
Acceptance:
1. `grep -c "InstrumentNames(" internal/pages/option_candidates_page/names.go` prints 1.
2. `grep -c "c.Issuer" internal/pages/option_candidates_page/cells.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) -- . ':!internal/pages/option_candidates_page' | wc -l` prints 0.
Checked by CI: ci-go; `go test ./internal/pages/option_candidates_page -race -count=1` (new `TestCompanyCellTicker`, `TestCompanyCellFallbackIssuer`, `TestNamesReaderError`; the existing golden and schema tests pass); `Contracts: unchanged`.

### 5 option-names-wire

Module: ot. Size: M.
Files: `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/candidates_site.go`, `cmd/ot/candidates_site_test.go`, `cmd/ot/option_page_site.go`, `cmd/ot/option_page_site_test.go`, `cmd/ot/stock_option_site_test.go`, `docs/contracts/ot/pages/option_candidates_page.md`, `docs/contracts/ot/pages/option_page.md`, `docs/contracts/ot/pages/stock_option_page.md`
Design: docs/design/option-page-names.md#Decisions
Order: after 3, after 4.
Goal: the names reader reaches both pages. `deps.optionCandidatesPage` takes the reader as one more argument and `newOptionCandidatesPage` sets `Config.Names`; `buildCandidates` passes `rd.names`; `optionPageOf` passes it to `OptionConfig.Names` and `buildOptionPage` passes `rd.names`; `candidatesPageNamed` and `optionPageNamed` open the instrument-name store with `a.deps.openInstrumentName` and the pages line (nil with a missing line, the demo reader with the demo profile, closed on the close function). `stockBuild` already passes `rd.names` and is not edited; its page document says the key is the ticker. The three page documents state the display name and the chain day.
Acceptance:
1. `grep -c "Names:" cmd/ot/wire.go` prints at least 1 and `grep -c "rd.names" cmd/ot/candidates_site.go cmd/ot/option_page_site.go | awk -F: '{s+=$2} END {print s}'` prints at least 2.
2. `grep -c "openInstrumentName" cmd/ot/candidates_site.go cmd/ot/option_page_site.go | awk -F: '{s+=$2} END {print s}'` prints at least 2.
3. `grep -c "ticker" docs/contracts/ot/pages/stock_option_page.md` prints at least 1 and `grep -c "chain day" docs/contracts/ot/pages/option_page.md` prints at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main) -- cmd/ot/site.go cmd/ot/pagereg.go cmd/ot/stock_option_site.go cmd/ot/sample.go docs/contracts/ot.md | wc -l` prints 0.
Checked by CI: ci-go; `go test ./cmd/ot/... -race -count=1` (new `TestCandidatesNamesReaderPassed`, `TestOptionPageNamesReaderPassed`, `TestNamedOpensNameStore` for both named pages, `TestNamedNoLineNilNames`; `TestSampleRoutes` and the `pagereg_test.go` tests pass unchanged); `Contracts: unchanged`.

## Left out

- A cheaper chain-day lookup through `store.OptionEodCatalog` (`OptionEodLatest`, as the job uses): the page configs hold no catalog reader and it has no upper bound for a rerun of a past day; revisit if the 15-day read is slow.
- A long company name ("Apple Inc."): no stored source (the instrument name of a stock is its ticker); a later design may add a name column to the watch list or a fundamentals reader.
- Storing the chain day or the ticker in `candidates.option_candidate` (a column and a migration, with the compute and job entries that fill it): it would remove the join and the limit of Decision 5, but it is a contract change; the join through `InstrumentNameReader` needs none.
- A link from a card or table row to `/options/stock/<ticker>`: the schemas carry no link field (`option-page-builders.md` Left out).
- One shared copy of the name and decimal helpers for the two pages packages: `modules.md` forbids a sibling import (`option-page-builders.md` Left out).
- Japanese candidates and their `edinet:<code>` issuers: `option-candidates-jp`; the fallback to the issuer text keeps them readable.
- Changing the route pattern or `pageFamily`: not needed, the key is the ticker.

## Adversarial

A fresh read-only reviewer attacked this file with six questions; the full answer is the author's working note `adversarial-option-names.md`.

1. Gap against the goal: adopted in part. The chain-day note of the one-stock page goes into `notice`, said in entry 3; no note on `/options/candidates` because it reads no `option_eod`; the long name and the Decision 5 race were already named and stay.
2. Biggest unnamed risk (the 15-day read): adopted. Decision 4 now states the bound and the cost; the catalog lookup is named in Left out.
3. A simpler way (store the ticker or chain day in the row): not adopted, a column and a migration are a contract change; already in Left out.
4. Lock-in (key is the ticker, union key space, merged issuers, copied 14): not adopted as a change. The key is the ticker by design, an issuer match is only a fallback, and the 14 is commented with the job's line because a page may not import a job.
5. Claim most likely false (entry 3 acceptance 2 hits the grouping line and names `stock_top.go`): adopted. The grep now counts only printing lines and covers `stock_build.go`.
6. Likeliest slip: adopted in part. Same grep fixed; the one-stock builder reads names once; the `(interface)` label stays because AGENTS.md makes the one interface file of a Go package a declarations file and `page.go` is that file of both packages; the acceptance words are English in the contract documents.
