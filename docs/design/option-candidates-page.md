# Option candidates page: `ot serve` shows the stored short puts, covered calls and guard rows at `/options/candidates`

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Source: owner 2026-11-08/09: options trading comes first and is done completely; a put qualifies when strike minus premium is at most the buy price; gates plus annualized-yield order, no composite score; the margin cushion and the assignment cash never go to Telegram; the covered-call floor is `sale_price`, else the average cost; DTE 21 to 45, spread at most 10 percent, open interest at least 10; the yield gate is above `DGS3MO`; JP option candidates come later. `docs/design/option-candidates.md` (revision 8, merged) names this design as the next one ("a page for the candidates: builder missing; next design `option-candidates-page`"). Front end rule (AGENTS.md): the front end renders data and computes nothing.
Read on origin/main at da631f4: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-candidates.md`, `docs/design/option-pages-v22.md`, `docs/design/market-page-producer.md`, `docs/design/option-page-serve.md`, `docs/design/alignment-v25.md` (row "option candidates page builder", plan 11 W11), `internal/store/api.go` (`Candidate`, `CandidateReader`), `migrations/0018_candidates.sql`, `docs/contracts/datasets/candidates.option_candidate.md`, `internal/pages/api.go`, `internal/pages/market_page/page.go`, `cmd/ot/site.go`, `cmd/ot/market_site.go`, `contracts/schemas/page.market_page.v1.json`, `page.option_page.v1.json`, `page.full_scan.v1.json`, `web/src/shell/{pages,TopNav}.tsx`, `web/src/components/data-table/data-table.tsx`, `web/src/pages/market/MarketPage.tsx`, `web/test/pages/market/fixtures/`, `web/scripts/generate-types.mjs`, `docs/contracts/page-schemas.md`.

Goal: after the job of `option-candidates.md` has stored a run, the owner opens `/options/candidates` of `ot serve` and sees, in the stored order, the short puts he may sell (company, expiry, DTE, strike, estimated premium, annualized yield, break-even, delta, IV, marks, which account the cash fits, the next results date), and below them, folded, every blocked or not-cleared row with the reason of each gate it failed; the same for covered calls and for the guard of short calls already open, as soon as such rows are stored. The page computes nothing: every number is a stored column, printed.

## What main holds today

- The data exists. `candidates.option_candidate` r1 and the view `rd.option_candidate_r1` (`migrations/0018_candidates.sql`, `GRANT SELECT ... TO ot_ro_pages`), `store.Candidate`, `store.CandidateReader.Candidates(ctx, day, kind)` (newest run of that day, ranked rows first, never nil) in `internal/store/api.go`, and `candidate.Open(ctx, dsn)` in `internal/store/candidate/store.go` (merged, #719).
- Nothing writes the table yet: `internal/compute/optioncand` and `internal/jobs/optioncandidates` do not exist (`option-candidates.md` issues 2 to 7 are open). Until a run is stored the page shows a named gap, never an empty table.
- No page schema for candidates. `page.option_page.v1` and `page.stock_option_page.v1` are carried files whose cards are free text lines (`contract_line`, `iv_line`) and `page.full_scan.v1` has three fixed sections; none carries a gate result or a per-account fit. `option-pages-v22.md` left their builders out (each its own design) and changes none of those schemas.
- The way a page is built is fixed by `market_page`: a schema file in `contracts/schemas`, a generated type in `web/src/generated`, `web/src/types/<name>.ts` (`placeholderValidate`), a view `web/src/pages/<dir>/<Name>.tsx` registered by `export const registration` and found by the glob of `web/src/shell/pages.tsx`, a package `internal/pages/<name>` with `Config` and `NewPage(Config) pages.Page`, and a route in `cmd/ot/site.go` (`siteMarketRoute`, `buildMarket`) reached by its address and not in the bar of eight (`web/test/shell/TopNav.test.tsx` pins eight links).
- `web/src/components/data-table/data-table.tsx` (tanstack table, shadcn Table) takes `columns` and cells `{text, value?, badge?, note?}` or a gap cell, shows a sort button on every header and sorts on `value` only. `web/src/components/ui` holds `accordion`, `alert`, `badge`, `card`, `tabs`, `tooltip`, `table`; `web/src/charts/Chart.tsx` wraps recharts. `lucide-react`, `@tanstack/react-table` and `recharts` are in `web/package.json`; this design adds no dependency.
- `cmd/ot/site.go` `siteReaders` has seven readers, built positionally in `openSiteReaders`; the demo profile builds demo readers and opens no store. `ot serve` listens on `127.0.0.1:8080` by default.
- `docs/design/modules.md` line 47 lists the packages `cmd/ot` may import; `internal/store/candidate` is there, `internal/pages/option_candidates_page` is not (this pull request adds it and regenerates `arch.toml`, the precedent of `option-candidates.md`).

## Decisions

1. A new page, not a fill of the old ones. Schema `page.option_candidates_page.v1` (title `OptionCandidatesPage`), written for this repository and not carried, with typed rows, per-gate reasons and per-account fits. The builders of `option_page`, `stock_option_page` and `full_scan` stay in Left out; no carried schema is touched (a field there would need a v2).
2. Route `/options/candidates` of `ot serve`, reached by address like `/market`; not an item of the bar (the bar test pins eight links; whether to add a ninth is asked of the owner, Left out). The builder is `internal/pages/option_candidates_page` (`Config`, `NewPage`, `Build`), the same shape as `market_page`. `ot page --name option_candidates_page` builds the same bytes for the command line.
3. Source and day. The builder reads only `store.CandidateReader`, once per kind (`short_put`, `covered_call`, `held_call_guard`), through the pages role (`OT_PG_DSN_PAGES`, the view). For each kind separately it reads the build day and, when that kind has no row there, the earlier calendar days back to 7 days (`lookbackDays = 7`); the first day with rows is the `day` of that group, and the page `chain_day` is the newest of the group days. A group day before the build day sets the `stale` label. A kind is always one run (the reader's newest run of its day); the job writes all kinds in one run, so two groups of one day come from the same run. No row in 7 days gives the gap `no_run`. No calendar is read: this is day subtraction on the date text, not a session count. A group never mixes two runs.
4. Order is the stored order. `Candidates` returns ranked rows first (rank ascending: yield descending, DTE, strike, instrument id, `option-candidates.md` Decision 8). The builder keeps that order and sorts nothing; it splits rows by `status`: `ok` rows go to the main table, `blocked` and `unknown` rows to the second table (in the stored order, `blocked` and `unknown` mixed as stored), with their reasons. No composite score, no filter, no extra ranking. A browser sort comes only from the shared data-table header on cells that carry a `value`; the builder gives `value` for the numeric columns (rank, DTE, strike, bid, yield, break-even, delta, IV) as the stored decimal text written as a JSON number literal (`json.Number`, never a float), so the default is the stored order and a click is a view of the owner's own. Whether that browser sort stays is asked of the owner (Left out).
5. Columns of the main table, from stored columns only. `short_put`: rank, company (`issuer`), expiry, DTE, strike, bid (EOD estimate), annualized yield, break-even, delta, IV, marks, account fit (one cell with a badge per alias key present in the stored `fits` object, so the columns never change between runs), next results date (`next_earnings`, with `earnings_state` as a badge). `covered_call`: the same without fit, with account alias, extrinsic, next ex-date and amount; the strike floor is not a stored column and is not shown (a row under the floor is blocked with the reason `strike_floor`). `held_call_guard`: company, alias, expiry, strike, extrinsic, next ex-date and amount and the stored status as a badge (ok, blocked, unknown) with the `guard` reason. The second table has company, contract (right, strike, expiry), alias, status badge and the reasons list. The column sets are the advisor's reading of `option-candidates.md` ("what the owner gets"); the owner has decided none beyond that list, so they are asked of the owner and cheap to change (a column is data of the page, Left out).
6. Text is formatted from stored text, never computed. Yield `0.123456` is shown "12.3456%" by moving the decimal point two places in the text, with no rounding (the stored yield is a plain decimal text of 6 places, `option-candidates.md` Decision 4; a text that is not `^-?[0-9]+\.[0-9]+$` becomes a gap cell "unreadable", never a guess); money and prices are shown as stored; DTE is the stored integer; dates are stored dates. The builder never adds, subtracts, compares or ranks numbers: it holds no `float64`, no `math/big`, no `sort`. Option terms stay English (sell put, covered call, IV, Delta), other shell text is Chinese (`option-pages-v22.md` Decision 6).
7. Reasons are shown, never dropped. Each stored gate result `{gate, status, reason}` that is not `pass` becomes one `Reason {gate, status, code, text}`: `text` is the Chinese sentence for a known code from a table in the builder (`events/block/earnings_before_expiry`, `cash_rate/block/yield_below_cash`, `chain_fresh/unknown/stale_chain`, and so on: the codes of Decisions 7, 11 and 12 of `option-candidates.md`, the source of truth until its issue 2 documents them in `docs/contracts/compute.md`; a test lists those codes and requires a sentence for each), and a code with no sentence is shown as the code itself. A `gates`, `marks` or `fits` text that is not valid JSON gives the row a gap cell "unreadable" and the row stays. `unknown` is styled differently from `blocked` and is never called a pass. Marks (`itm`, `earnings_unconfirmed`, `earnings_beyond_horizon`, `no_dividend`) are badges with a short Chinese hint.
8. The premium is an estimate and the page says so: the column label is "Bid (EOD 估)" with the hint "收盘 bid,下一交易日才卖,不是成交价", and the schema field `estimate_note` is required.
9. Privacy. Fits, aliases, shares-derived gates and guard rows come from account data. The page is served by `ot serve` on the local machine as the other account pages are; the builder and `cmd/ot` code of this design import no `notify` and no Telegram sender (grep-checked), the margin cushion and the assignment cash have no row anywhere here (ADR 0010 stands).
10. Empty and partial states, each named. No reader (nil, or a missing `OT_PG_DSN_PAGES`) is the gap `no_reader`; a kind with no rows on `chain_day` is that group's gap `no_rows`; JP entries are a fixed note "日本期权候选:后续设计 (option-candidates-jp)" so that the missing market is never silent; a store error fails the build (the route answers 500 with its fixed text, as `/market` does).
11. Frontend stack (ADR 0034): shadcn/ui radix-nova components from `components/ui`; `@tanstack/react-table` through the shadcn data-table pattern (`components/data-table/data-table.tsx`) for every table; lucide icons; recharts only if a chart is added (none here: no chart is designed). No new dependency, so the root `frontend-whitelist.json` and `web/deps-allowlist.txt` do not change.

## Modules

No row of `docs/design/modules.md` is added or changed. This pull request edits line 47 of that file (one name, `internal/pages/option_candidates_page`, in the sentence that lists the implementation packages `cmd/ot` may import) and regenerates `arch.toml` with `go run ./scripts/arch` (the check `arch.toml agrees with modules.md` of `checks.yml` fails otherwise; precedent `option-candidates.md`). `.go-arch-lint.yml` is tree-derived and appears with the first Go file of the package (issue 2). Ownership by the first matching row: `contracts/**`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md` belong to contracts; `internal/pages/**` and `docs/contracts/pages.md` to pages; `web/**` to web; `cmd/**` and `docs/contracts/ot.md` to ot; `.go-arch-lint.yml` to repo (licensed pair in issue 2).

| # | issue | module | order |
|---|---|---|---|
| 1 | option-candidates-page-schema (interface) | contracts | first |
| 2 | option-candidates-page-interface (interface) | pages, plus repo (licensed pair) | after 1 |
| 3 | option-candidates-page-builder | pages | after 1, 2 |
| 4 | option-candidates-page-view | web | after 1 |
| 5 | option-candidates-page-route | ot | after 2, 3 |

Nothing waits for `option-candidates.md` issues 2 to 12: the reader and the table are on main, and the page shows `no_run` until the job has stored a run. Entries 3 and 5 share no file with each other; 2 and 3 share `internal/pages/option_candidates_page/page.go` and name each other.

## Interface

Marked: schema owned by contracts, frozen by issue 1; the Go function owned by pages, frozen by issue 2. Both are read by issues 3 to 5.

- Schema `contracts/schemas/page.option_candidates_page.v1.json`, `schema` const `option_candidates_page`, `schema_version` const 1, `additionalProperties` false, under 600 lines, `$ref`s to `common.v2.json` (`Gap`, `SourceTime`, `StaleLabel`, `RunStatus`, `Timestamp`) as `page.market_page.v1.json` does. Envelope as `market_page`: `run_id, build_id, scope, generated_at, data_times, run_status, stale (StaleLabel or null), title, notes (strings), gaps (Gap list)`. Page fields, all required: `chain_day` (date text or null), `computed_at` (text in the owner zone, or null), `rule_version` (string or null), `estimate_note` (string 1..200), `groups` (1 to 3 items in the order `short_put`, `covered_call`, `held_call_guard`).
  - `Group`: `kind` (enum `short_put`, `covered_call`, `held_call_guard`), `title`, `day` (date text or null: the day of the rows of this group), `counts {ok, blocked, unknown}` (integers, the stored row counts), `main` (`Table` or null), `other` (`Table` or null), `gap` (`Gap` or null); `gap` is non-null exactly when `main` and `other` are both null (schema `oneOf`).
  - `Table`: `columns` (`Column {id, label, align (left|right), hint (string or null)}`), `rows` (`Row`).
  - `Row`: `id` (`^cand-[A-Za-z0-9_-]{1,64}$`), `cells` (object from column id to `Cell`), `reasons` (list of `Reason`, only on `other` rows, else empty).
  - `Cell`: `{text, value (a JSON number or null), badges (list of strings), tone (up|down|flat|none)}` or a `Gap` cell `{kind: gap, reason, text}`.
  - `Reason`: `{gate (^[a-z_]{1,32}$), status (block|unknown), code (^[a-z_]{1,48}$), text}`; the gate and code are patterns, not enums, so a new gate of `option-candidates.md` needs no schema change.
- Generated type `web/src/generated/option_candidates_page.v1.ts` (title `OptionCandidatesPage`), committed with the schema.
- Go, package `internal/pages/option_candidates_page` (pages):
  `type Config struct { RunID, BuildID, Scope string; Zone *time.Location; Candidates store.CandidateReader }` (nil `Zone` is an error of `Build`; nil `Candidates` gives the gap `no_reader`);
  `func NewPage(c Config) pages.Page` with `Meta{Name: "option_candidates_page", Schema: "page.option_candidates_page.v1"}`;
  `func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error)` (the `pages.Builder` shape; `r` is unused; `at` is `generated_at` in UTC, the builder reads no clock).
- ot: `deps.optionCandidatesPage func(zone *time.Location, runID, buildID, scope string, cands store.CandidateReader) pages.Page`; route constant `siteCandidatesRoute = "options/candidates"`.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, and touches no live broker or database. Acceptance items are decided by reading, grep or git; the commands run by CI are on the `Checked by CI` line of each entry.

### 1 option-candidates-page-schema (interface)

Module: contracts. Size: M.
Files: `contracts/schemas/page.option_candidates_page.v1.json` (new), `web/src/generated/option_candidates_page.v1.ts` (new), `internal/contracts/option_candidates_page_v1_test.go` (new), `docs/contracts/page-schemas.md`
Design: docs/design/option-candidates-page.md#Interface
Order: first.
Goal: the schema of `## Interface` and its generated type, with a Go test of the contract in the style of `internal/contracts/market_page_v1_test.go` (a minimal valid page passes; a wrong `schema_version`, an unknown field, a group with both `main` and `gap`, and a `Reason` with an uppercase gate fail).
Acceptance:
1. `test -f contracts/schemas/page.option_candidates_page.v1.json`; `grep -c '"const": "option_candidates_page"' contracts/schemas/page.option_candidates_page.v1.json` prints 1; `grep -c '"additionalProperties": false'` of the file is at least 6.
2. `grep -c "export interface OptionCandidatesPage" web/src/generated/option_candidates_page.v1.ts` prints 1 and its line 1 is the generated banner; `grep -c "option_candidates_page" docs/contracts/page-schemas.md` is at least 1.
3. `grep -c "^func Test" internal/contracts/option_candidates_page_v1_test.go` is at least 4 and `grep -c "t.Parallel()" internal/contracts/option_candidates_page_v1_test.go` is at least that count.
4. `wc -l < contracts/schemas/page.option_candidates_page.v1.json` prints less than 600; `git diff $(git merge-base HEAD origin/main) -- contracts/schemas/carryover.sha256 | grep -c .` prints 0; `grep -c '"Reason"' contracts/schemas/page.option_candidates_page.v1.json` is at least 1 and the file holds no `enum` for `gate` or `code` (`grep -c '"\^\[a-z_\]{1,32}\$"'` prints 1).
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
6. The result matches docs/design/option-candidates-page.md#Interface, judged from the code and the design file.
Checked by CI: `go test ./internal/contracts -race -count=1`; `pnpm --dir web run generate:types` leaves `web/src/generated` unchanged; the pull request body carries `Contracts: page.option_candidates_page.v1 (new)`.
Frontend stack lines: none (no UI code in this entry).

### 2 option-candidates-page-interface (interface)

Module: pages, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/pages/option_candidates_page/page.go` (new), `internal/pages/option_candidates_page/page_test.go` (new), `.go-arch-lint.yml`
Design: docs/design/option-candidates-page.md#Interface
Order: after 1; no other dependency. If another open pull request has regenerated `.go-arch-lint.yml`, it merges first and the regeneration is run again.
Goal: `page.go` holds `Config`, `NewPage` and a `Build` whose body answers the error `option_candidates_page: not built`; `.go-arch-lint.yml` is regenerated by `go run ./scripts/arch` (the edit is the regeneration only). The contract test has two sides: `NewPage(Config{...})` has the stated `Meta`; a caller fills every field of `Config` with a fake `store.CandidateReader` and calls `Build`.
Acceptance:
1. `grep -c 'type Config struct\|^func NewPage\|^func Build' internal/pages/option_candidates_page/page.go` prints 3; `wc -l < internal/pages/option_candidates_page/page.go` prints less than 60; the file imports only `context`, `errors`, `time`, `contracts`, `store` and `pages` of this module (`grep -c 'internal/notify\|internal/jobs' internal/pages/option_candidates_page/page.go` prints 0).
2. `grep -c "^func TestNewPageMeta\|^func TestConfigContract" internal/pages/option_candidates_page/page_test.go` prints 2; `grep -c "time.Sleep\|time.Now()" internal/pages/option_candidates_page/page_test.go` prints 0.
3. `grep -c "pages/option_candidates_page" .go-arch-lint.yml` is at least 2; `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum arch.toml | grep -c .` prints 0 (`arch.toml` was regenerated by the design pull request); `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
4. The result matches docs/design/option-candidates-page.md#Interface, judged from the code and the design file.
Checked by CI: `go test ./internal/pages/option_candidates_page -race -count=1`, `go run ./scripts/arch --check` (prints `arch: ok`), `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go.
Frontend stack lines: none (no UI code in this entry).

### 3 option-candidates-page-builder

Module: pages. Size: L.
Files: `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/page_test.go`, `internal/pages/option_candidates_page/builder.go` (new), `internal/pages/option_candidates_page/groups.go` (new), `internal/pages/option_candidates_page/cells.go` (new), `internal/pages/option_candidates_page/reasons.go` (new), `internal/pages/option_candidates_page/strings.go` (new), `internal/pages/option_candidates_page/format.go` (new), `internal/pages/option_candidates_page/builder_test.go` (new), `internal/pages/option_candidates_page/groups_test.go` (new), `internal/pages/option_candidates_page/cells_test.go` (new), `internal/pages/option_candidates_page/reasons_test.go` (new), `internal/pages/option_candidates_page/format_test.go` (new), `internal/pages/option_candidates_page/purity_test.go` (new), `internal/pages/option_candidates_page/testdata/` (new), `docs/contracts/pages.md`
Design: docs/design/option-candidates-page.md#Decisions (Decisions 3 to 10)
Order: after 1, 2; it edits `page.go` and `page_test.go` of 2 (the stub body and its test).
Goal: `Build` reads each kind through `Config.Candidates`, finds `chain_day` by Decision 3, splits `ok` from `blocked` and `unknown` rows in the stored order, writes the columns of Decision 5, the reasons of Decision 7 and the gaps of Decision 10, and validates against the schema of issue 1.
Acceptance:
1. `grep -rhE "^func (TestOrderKept|TestOkAndOtherSplit|TestChainDayLookback|TestStaleWhenOlderDay|TestNoRunIsGap|TestNilReaderIsGap|TestKindWithoutRowsIsGap|TestYieldTextShift|TestValueIsNumberLiteral|TestReasonsFromGates|TestUnknownCodeShownAsCode|TestBadGatesJSONKeepsRow|TestFitsPerAlias|TestMarksAreBadges|TestEstimateNote|TestJPNote|TestOutputValidatesAgainstSchema|TestNoFloatNoSortNoNotify|TestNoClockNoIO)\(" internal/pages/option_candidates_page | wc -l` prints 19 (new).
2. `TestYieldTextShift`: "0.123456" gives "12.3456%", "0.050000" gives "5.0000%" (the point moves two places, the digits are kept), and "abc" is an error text cell and not a panic. `TestChainDayLookback`: put rows only on day D-3 and call rows only on D-5 give group days D-3 and D-5, `chain_day` D-3 and a stale label; rows only on D-8 give the gap `no_run`. A yield text "1e-2" gives a gap cell. `TestOrderKept`: input ranks 1, 2, 3 with ids shuffled come out in input order (the builder never reorders).
3. `TestNoFloatNoSortNoNotify` parses the package files with `go/parser` and scans the non-test `.go` files of the package (test files may read `testdata/`) and fails on any identifier `float32` or `float64`, any import of `math/big`, `sort`, `slices` with a `Sort` call, `os`, `net/http`, `database/sql`, `internal/notify` or `internal/jobs`; `TestNoClockNoIO` fails on a call of `time.Now` or `time.Sleep`.
4. `grep -c "t.Parallel()"` of each new test file is at least its count of `^func Test`; `TestOutputValidatesAgainstSchema` validates the output of three testdata inputs (ok rows, only blocked rows, no rows) against `contracts/schemas/page.option_candidates_page.v1.json`.
5. `grep -c "option_candidates_page" docs/contracts/pages.md` is at least 1; each new file is under 600 lines; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line (`testdata/` counts as one).
6. The result matches docs/design/option-candidates-page.md#Decisions, judged from the code and the design file.
Checked by CI: `go test ./internal/pages/option_candidates_page -race -count=1`, ci-go, golangci-lint, go-arch-lint.
Frontend stack lines: none (no UI code in this entry).

### 4 option-candidates-page-view

Module: web. Size: L.
Files: `web/src/types/option_candidates_page.ts` (new), `web/src/pages/candidates/OptionCandidatesPage.tsx` (new), `web/src/pages/candidates/CandidateGroup.tsx` (new), `web/src/pages/candidates/ReasonList.tsx` (new), `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx` (new), `web/test/pages/optioncandidates/fixtures/ok.ts` (new), `web/test/pages/optioncandidates/fixtures/blocked.ts` (new), `web/test/pages/optioncandidates/fixtures/gap.ts` (new), `web/test/pages/optioncandidates/fixtures/stale.ts` (new), `web/test/pages/optioncandidates/fixtures/bad-version.json` (new), `web/test/pages/optioncandidates/fixtures/MANIFEST.synthetic-v1.json` (new)
Design: docs/design/option-candidates-page.md#Decisions (Decisions 4 to 11)
Order: after 1; it reads the generated type of 1 and needs no Go code.
Goal: the view of `OptionCandidatesPage`: header card with `chain_day`, `computed_at`, `rule_version`, the `estimate_note` and the stale alert; one tab per group present (卖 put, 备兑 call, 已持 call 守卫) with the counts; in a tab the main table, below it an accordion "未通过 / 未确认 (n)" holding the second table whose rows show their reasons; a group with a gap shows the cause once and no empty table; the JP note and `notes` at the foot. Every text is a payload field; nothing is compared, summed, sorted or formatted here.
Acceptance:
1. `web/src/types/option_candidates_page.ts` imports `OptionCandidatesPage` from `@/generated/option_candidates_page.v1`, `grep -c "SUPPORTED_VERSIONS" web/src/types/option_candidates_page.ts` is at least 1, and `grep -c "registration = { schema: 'option_candidates_page'" web/src/pages/candidates/OptionCandidatesPage.tsx` prints 1.
2. `grep -rc "Math\.\|\.sort(\|\.reduce(\|parseFloat\|Number(\|toFixed" web/src/pages/candidates/OptionCandidatesPage.tsx web/src/pages/candidates/CandidateGroup.tsx web/src/pages/candidates/ReasonList.tsx` prints 0 for each file; `git diff $(git merge-base HEAD origin/main) -- web/no-compute-allow.txt web/package.json web/deps-allowlist.txt frontend-whitelist.json pnpm-lock.yaml | grep -c .` prints 0.
3. `grep -c "components/data-table/data-table" web/src/pages/candidates/CandidateGroup.tsx` is at least 1 and `grep -c "<table\|<Table" web/src/pages/candidates/*.tsx` prints 0 for the three new files (every table goes through the data-table); `grep -c "components/ui/tabs\|components/ui/accordion\|components/ui/badge\|components/ui/alert\|components/ui/card" web/src/pages/candidates/OptionCandidatesPage.tsx web/src/pages/candidates/CandidateGroup.tsx` is at least 4 in total; `git diff $(git merge-base HEAD origin/main) -- web/src/components web/src/shell web/src/pages/candidates/FullScanPage.tsx | grep -c .` prints 0.
4. `grep -rhE "^\s*(it|test)\('" web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx | wc -l` is at least 8, covering by title: the ok page shows the main table in payload order; the blocked table shows each reason text and an `unknown` reason differently from a `block`; a group with a gap shows its text and no table; the stale label shows; the estimate note shows; a bad version fails validation; the registration is found once (`pnpm --dir web run test -- web/test/shell` stays green).
5. `ok.ts` is typed `satisfies OptionCandidatesPage` and imports the type by the relative path `../../../../src/generated/option_candidates_page.v1`; the `files` list of `MANIFEST.synthetic-v1.json` there names the five fixtures and its `marker` is `synthetic-v1`.
6. Each new file is under 600 lines; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
7. The result matches docs/design/option-candidates-page.md#Decisions, judged from the code and the design file.
Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck`, `lint`, `test`, `build` pass (`tsc`, `eslint`, `vitest`, the build), and `scripts/check-frontend-deps.mjs`.
Frontend stack lines:
- 按 frontend-stack：用 shadcn tabs (`components/ui/tabs.tsx`) 做 the one-tab-per-kind switch of the groups。
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the main table and the blocked-and-unknown table of each group。
- 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 the folded "未通过 / 未确认" block。
- 按 frontend-stack：用 shadcn card, badge and alert (`components/ui`) 做 the header, the marks, the status and gate badges, the stale label and the gap causes。
- 按 frontend-stack：用 lucide-react 做 the icons of the stale, gap and unknown states。
- 按 frontend-stack：recharts is not used: no chart is in this page。

### 5 option-candidates-page-route

Module: ot. Size: M.
Files: `cmd/ot/candidates_site.go` (new), `cmd/ot/candidates_site_test.go` (new), `cmd/ot/site.go`, `cmd/ot/site_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/demo.go`, `cmd/ot/pagelist.go`, `cmd/ot/pagelist_test.go`, `docs/contracts/ot.md`
Design: docs/design/option-candidates-page.md#Decisions (Decisions 2, 3, 9, 10)
Order: after 2, 3; wiring only.
Goal: `siteHandler` serves `options/candidates` (`siteCandidatesRoute`) with the day of `--day` or today in the zone and `ot page --name option_candidates_page` builds the same page; `openSiteReaders` opens the candidate store once with the one pages line `OT_PG_DSN_PAGES` through `candidate.Open` and adds it to `siteReaders`; the demo profile serves a fixed demo reader (a few ok and blocked rows of each kind) and opens no store; a failed build answers 500 with the fixed text and the bar is unchanged.
Acceptance:
1. `grep -rhE "^func (TestCandidatesRouteServesPage|TestCandidatesRouteOnlyGet|TestCandidatesDemoNeedsNoStore|TestCandidatesUsesPagesDSN|TestCandidatesBuildErrorIs500|TestPageCommandBuildsCandidates|TestBarStillEightLinks)\(" cmd/ot | wc -l` prints 7 (new).
2. `grep -c "OT_PG_DSN_CANDIDATE" cmd/ot/candidates_site.go` prints 0 (the page reads through the pages role); `grep -c "internal/notify\|telegram" cmd/ot/candidates_site.go` prints 0; `grep -c "sort\.\|big\.\|float64" cmd/ot/candidates_site.go` prints 0; `grep -c "options/candidates" docs/contracts/ot.md` is at least 1.
3. `git diff $(git merge-base HEAD origin/main) -- web | grep -c .` prints 0; each new file is under 600 lines; `wc -l cmd/ot/wire.go cmd/ot/site.go | grep -v total | awk '$1>=800' | wc -l` prints 0 (252 and 285 on main); `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
4. The result matches docs/design/option-candidates-page.md#Decisions, judged from the code and the design file.
Checked by CI: `go test ./cmd/ot -race -count=1`, golangci-lint, `go run ./scripts/arch --check`. The first live look is the owner's: after `ot migrate`, `ot setup pg` and a run of the `option_candidates` job of `option-candidates.md`, open `http://127.0.0.1:8080/options/candidates`.
Frontend stack lines: none (no UI code in this entry).

## Left out

- Asked of the owner (not decided, the first cut shows an advisor default, each is a data change of the page, not a schema change):
  1. Columns of the three tables (Decision 5): which of delta, IV, break-even, fit, next results date, extrinsic you want on the main table, and whether the second table should also show the numbers of a blocked row.
  2. Sorting and filters: the page keeps the stored order (yield descending). The shared data-table puts a sort button on every header; keep it, or have no browser sort? No filter by company or account is built; do you want one (a filter is a view of stored rows, not a rule)?
  3. Whether the page gets a ninth item in the bar (it would change the bar test of eight and `unified-site.md` Decision 3) or stays reached by address, with a link from the option data page.
  4. Whether a day with no run (weekend, before the 16:45 close) shows the last day with rows marked stale (Decision 3, up to 7 days back) or an empty page.
- The page shows rows only after `option-candidates.md` issues 7 and 12 (the job and its wiring) have stored a run; this design does not gate on them and its acceptance is checked on fixtures; until then the live page says `no_run`. The first live look is the owner's.
- JP option candidates: left out (owner: later); the page carries the fixed note and the design `option-candidates-jp` adds its rows and a market column.
- The builders of `option_page`, `stock_option_page` and `full_scan` (`alignment-v25.md`, plan 11 W11 rows "option candidates page builder" and "full scan builder"): not covered here; this page is the candidates view, the card pages wait for their own designs and for HV30 and IVP producers.
- Notifications: no Telegram or other push of candidates; the margin cushion and assignment cash never go to Telegram and have no row here (the designs `account-figures` and `option-notices` of `option-candidates.md`).
- Ledger, wheel, tracking, roll and close advice, P and L (`option-ledger`, `option-tracking`); valuation and the automatic buy price (parked, PR 607; the hand-typed `buy_price` stands); the funding rule by open puts and concentration caps, FX conversion, ITM probability, IV rank (`option-candidates.md` Left out).
- A chart (premium by strike, yield by DTE): not designed, no owner request; recharts stays in the stack for it.
- History of earlier runs, a comparison of two days, a coverage check of the chain: not built; the page shows the newest run of one day.
- Editing of `buy_price` or `sale_price` on the page: the page is read-only; the owner types prices in `watch.toml`.

## Adversarial

The full answer of the fresh review agent is in the dispatcher scratchpad, file `design/option-candidates-page-adversarial.md` (not in the repository). Answers:

1. Gap. Adopted: the dependency on the job is named in Left out, and the fits columns (a badge per stored alias key) and the call and guard columns (no invented verdict or floor column; the stored status and reason) are fixed in Decision 5. Not adopted: a link to the page (the bar is the owner's question 3); columns stay asked of the owner.
2. Risk. Adopted: the reason table's source is `option-candidates.md` Decisions 7, 11 and 12 (compute.md has none of the codes, verified by grep), with a test that every listed code has a sentence; the lookback is per kind with a `day` per group and the one-run-per-kind statement replaces "never mixes two runs". Not adopted: running the builder on rows of the real rule (the rule does not exist yet; fixtures follow the dataset document).
3. Simpler way. Not adopted: `full_scan.v1` is carried with three fixed sections and no reasons; a v2 of it is a larger change. Puts-only first cut not adopted: the builder reads all three kinds for the price of one reader call each and shows `no_rows` for kinds without data. Lookback kept (the owner asks about it, question 4).
4. Lock-in. Accepted and named: v1 is a contract; JP will be a new group kind and may need a v2 (a new enum value); gate and code are patterns, so only kind is closed. The reason table lives in the page builder on purpose (compute is not importable by wording; pages may not compute). `siteReaders` grows by one field; issue 5 updates every constructor.
5. False claims. Adopted: the whitelist path (root `frontend-whitelist.json`), the compute.md codes, and the text-shift rule (the stored yield has a fixed plain form; other forms become a gap cell, tested; `pages.md` line 31 allows JSON numbers from exact decimal text, which `json.Number` is).
6. Slip. Adopted: the purity test scans non-test files only (stated); the order is stated serial 1, 2, 3 with 4 parallel after 1 and 5 after 2 and 3. Not adopted: splitting issue 3 (19 tests are small; the 600-line cap is per file and the files are split by concern). The `.go-arch-lint.yml` rebase risk is the one named in issue 2 (rerun the regeneration).
