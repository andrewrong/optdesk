# purity tests: one table and one scanner instead of 29 near-identical files

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-25
Plan: P1

Source: `git ls-tree -r --name-only origin/main | grep 'purity_test.go$'` gives 29 files, 1774 lines, in three modules: compute (6 files), notify (1, `internal/notify/core/morning_purity_test.go`), pages (22). Each file re-implements the same scan: parse the non-test `.go` files of its own package, fail on a banned import, fail on a float name or on a clock call. A new package copies a neighbour and changes a list; the copies have drifted (see "Today"), and a rule change has to be made in 29 places.

Result for the owner: the rule of every package is one row in one table; a new package is one new line; the 29 copies are gone and no package is checked less than today.

## Today (read from origin/main, 29 bodies diffed)

The shared shape: glob or walk the non-test `.go` files, parse, check imports against a list, then look for float names and clock calls. The bodies differ in four ways, all of which the table must carry as data, because a single hard-coded rule would weaken some package:

1. Import match. Exact path, path prefix (`bad/`), path suffix (`/bad`) or substring (`/bad`), depending on the file. Examples: optionnotice and optioncand match banned std paths by equality only; advice_page, household_page, positions_page, risk_page use `Contains(path, "/"+bad)`; home_macro, home_options, market_page use equal, suffix or `/bad/` inside.
2. Float rule. Three forms: an identifier `float32`/`float64` anywhere in the syntax tree (optioncand, optionnotice, optiontrack, home_options, option_candidates, option_notices, option_pnl, option_tracking); the text `float32`/`float64` anywhere in the source, comments included (advice, household, positions, risk, home_companies, home_household; optiontier only `float64`); a conversion call `float64(...)` or `float32(...)` (company_data, company_quarters). No float rule at all: optionfig, optionshortlist, notify/core, base_data, data_health, home_macro, home_market, market_page, market_state_page, strategy_replay, valuation_import, valuation_page.
3. Clock rule. `time.Now` or `time.Sleep` (optioncand when `time` is imported, optiontrack, option_candidates, option_notices, option_tracking); `time.Now` only (optionfig, optionnotice, home_companies, home_household, home_market); `Now`, `Since`, `Until` (base_data, company_data, data_health, home_options); the import of `time` itself is banned in optionnotice, optionshortlist, optiontier, strategy_replay, valuation_import; none elsewhere.
4. Files scanned. All non-test files of the package, except: company_data_page (only `builder.go`, `rows.go`, `strings.go`), company_quarters_page (only `format.go`, `builder.go`), option_tracking_page (all but `context.go` for the import and float scan, all files for the clock scan), option_candidates_page (the `tier*` files get a second rule) and notify/core (the whole tree `internal/notify`, every sub-directory).

Rules that are not an import or float scan, each owned by exactly one package:

- notify/core: no identifier named `Position`, `Candidate`, `Cushion`, `ExcessLiquidity`, `AccountReader`, `AccountValue`, `OptionLedger`, `NoticeResult` or `CashByDate` in `internal/notify`; imports containing `internal/pages`, `internal/jobs` or `optionnotice` are banned there.
- market_page and home_market: the text `math.Sqrt` and `math.Log` is banned.
- option_pnl_page: no selector named `Add`, `Sub`, `Mul` or `Div`; `marks.go` may use only `Sign`, `IsPositive`, `NewFromString` (selector and call names). No clock rule.
- option_candidates_page, option_notices_page, option_tracking_page: no call `slices.Sort*` (under the local import name). option_candidates_page also: the `tier*` files import no `math`, `math/big`, `sort` or path containing `decimal`, and call neither `percentText` nor `roundDigits`.
- company_data_page, company_quarters_page: no path containing `decimal` or ending `internal/compute/companydata`; company_quarters_page also no selector on `big`, `decimal`, `math`.
- valuation_page: no `strconv.ParseFloat*`; no `math`, `math/*` or `decimal` import (and the forbidden list below).
- strategy_replay_page: an allow-list rule: besides the banned `os`, `time`, `math`, `net/http`, `database/sql`, every import whose first path element contains a dot must be `github.com/andrewrong/optdesk/internal/contracts`.
- Not a purity rule: `TestFreeCashCopiedFromCalculator` in company_quarters_page's `purity_test.go` is a behaviour test of the builder; it moves to another file of its package and is kept.

Banned import lists per package (what the rows must carry, by package):

| package | banned imports |
|---|---|
| compute/optioncand | os, net/http, database/sql, internal/store, internal/notify, internal/jobs |
| compute/optionfig | os, internal/store, internal/jobs, internal/notify |
| compute/optionnotice | os, net/http, database/sql, time, internal/store, internal/notify |
| compute/optionshortlist | time, os, net, database/sql, math/rand |
| compute/optiontier | math, os, net/http, database/sql, time, internal/strategy, internal/compute/optionevidence, any path containing /bsm |
| compute/optiontrack | internal/store, internal/notify, internal/jobs, internal/pages |
| pages/advice_page, household_page, positions_page, risk_page | math/big, sort, database/sql, os, internal/notify, internal/jobs |
| pages/home_companies | the six above and internal/pages/companies_page |
| pages/home_household | math (and math/*), sort, database/sql, os, internal/notify, internal/jobs |
| pages/base_data_page | database/sql, os, net/http, github.com/jackc/pgx |
| pages/data_health_page | database/sql, os, net/http, internal/store/ops, github.com/jackc/pgx |
| pages/home_macro | internal/compute/ease, internal/pages/macro_page, database/sql, os, sort, internal/jobs, internal/notify |
| pages/home_market | internal/pages/market_page, database/sql, os, internal/notify, internal/jobs |
| pages/home_options | math/big, math, strconv, internal/pages/option_tracking_page, database/sql, os, sort, internal/jobs, internal/notify, internal/compute/optionledger |
| pages/market_page | internal/store/bar, internal/jobs, internal/compute/figures, database/sql, os |
| pages/market_state_page | internal/compute/marketstate, internal/compute/liquidity, internal/compute/figures, internal/jobs, database/sql, os |
| pages/option_candidates_page, option_tracking_page | math/big, sort, os, net/http, database/sql, internal/notify, internal/jobs |
| pages/option_notices_page | sort, os, net/http, database/sql, internal/notify, internal/jobs |
| pages/option_pnl_page | math/big, math, sort, os, net/http, database/sql |
| pages/valuation_import_page | database/sql, os, net/http, time, math, internal/clock, internal/store/valuationpg, internal/store/underlyingpg, github.com/jackc/pgx, internal/compute/valueimport, any path containing decimal |
| pages/valuation_page | database/sql, os, net/http, internal/clock, internal/store/valuationpg, internal/store/underlyingpg, github.com/jackc/pgx, internal/compute/valuelines, math, math/*, any path containing decimal |
| pages/company_data_page | math, math/*, any path containing decimal, os, database/sql, internal/compute/companydata |
| pages/company_quarters_page | math, math/*, any path containing decimal, internal/compute/companydata |

Packages whose rule differs from the common "page builder" rule (the six imports plus float text): every row of the table above except advice_page, household_page, positions_page and risk_page. The table keeps those differences as they are; it does not unify them. Two matching differences are merged on purpose and checked by the first issue: one matcher (path equals `bad`, or starts with `bad/`, or contains `/bad`) replaces the four matchers; it matches everything each old matcher matched. It can only report more than today, never less; if it reports a file of main, the first issue's pull request names the package and the match, and keeps that row on its old form instead.

## Decisions

1. The one test lives in module repo, as `scripts/purity/purity_test.go` and `scripts/purity/rules_test.go`: a test-only Go package that scans source from the repository root (`../..`), like `scripts/arch`. It is not put in compute, notify or pages, because the table covers three modules and would make every module issue edit another module's file.
2. One scanner, one matcher, one table. `TestPurity` loops over the table, runs each row as a subtest with `t.Parallel()`, and fails when a row names a directory that does not exist or holds no non-test file (so a rename cannot silently drop a package).
3. A row is data: directory, files (empty means all non-test `.go` files, with named exclusions), banned imports, float form (none, identifier, text, conversion), clock form (none, `Now`+`Sleep`, `Now`, `Now`+`Since`+`Until`), banned text, banned identifiers, banned selector names. The common page rule is a named default (`pageDefault`); a new page package is one line `{Dir: "internal/pages/x_page", Rule: pageDefault}`.
4. A rule that is not data (option_pnl `marks.go` sign test, option_candidates tier files, strategy_replay allow-list, notify/core whole-tree walk) is a small named function in the table file, set on its row. The row is still one line; the function is shared code, written once.
5. The 29 files are deleted, nothing is kept "for safety". Their rule must hold in the table before they are deleted, so the table issue lands first and each module issue then only deletes (and moves `TestFreeCashCopiedFromCalculator`).
6. Own code, not a library: `go/parser` and `go/ast` are the standard library; `go-arch-lint` already runs on the import graph but cannot see `time.Now`, float names or selector calls. Say so in the Design line of the pull request ("own implementation, because no library checks identifiers and calls").

## Modules

Rows used: repo (new files under `scripts/purity/`, covered by `scripts/**`), compute, notify, pages (deletions only). No change to `docs/design/modules.md`: no new module, no import edge (the test reads source text and imports only the standard library).

## Interface

none (test code only; no interface file changes)

## Issues

| issue | module | order |
|---|---|---|
| 1 purity-table-scanner | repo | first |
| 2 purity-delete-compute | compute | after 1 |
| 3 purity-delete-notify | notify | after 1 |
| 4 purity-delete-pages | pages | after 1 |

### 1 purity-table-scanner

Module: repo. Files: `scripts/purity/purity_test.go`, `scripts/purity/rules_test.go`. Order: first.

Result for the owner: one test enforces the purity rule of the 29 packages; adding a package is one line.

Acceptance:
1. `scripts/purity/purity_test.go` and `scripts/purity/rules_test.go` are new files, each under 600 lines; `purity_test.go` holds the scanner and the matcher, `rules_test.go` holds the table and the named rule functions.
2. `rules_test.go` has one row for each of the 29 packages that hold a `purity_test.go` on main (listed in the paragraph "Today" and the table above); the row of each package carries the banned imports, float form, clock form and extra rules exactly as that package's file has them on main, as listed above.
3. `TestPurity` (new) runs every row as a subtest that calls `t.Parallel()` first, and fails on a row whose directory is missing or has no non-test file.
4. A negative case is in the same package: a small table test (new) feeds the scanner one source string with a banned import, one with a float name and one with a clock call, and expects a failure for each, so the scanner is known to fail on bad input.
5. The pull request description lists every package where the merged matcher reports more than the old file did, or states "none", and states that no row bans less than its old file.
6. No file other than the two above is in the diff; no `purity_test.go` is deleted by this issue.

Checked by CI: `ci-go` runs `go test ./scripts/purity/...` and it passes on main without any change to the 29 packages.

### 2 purity-delete-compute

Module: compute. Files: `internal/compute/optioncand/purity_test.go`, `internal/compute/optionfig/purity_test.go`, `internal/compute/optionnotice/purity_test.go`, `internal/compute/optionshortlist/purity_test.go`, `internal/compute/optiontier/purity_test.go`, `internal/compute/optiontrack/purity_test.go`. Order: after 1.

Result for the owner: six copies of the scan are gone; the rule is checked by `scripts/purity`.

Acceptance:
1. The six files named in Files are deleted. Deleting them IS the task; a worker must not keep one.
2. No `purity_test.go` remains under `internal/compute`.
3. No other file is in the diff.

Checked by CI: `ci-go` runs `go test ./internal/compute/... ./scripts/purity/...`.

### 3 purity-delete-notify

Module: notify. Files: `internal/notify/core/morning_purity_test.go`. Order: after 1.

Result for the owner: the notify scan is gone; the table row of `internal/notify` checks the same names and imports.

Acceptance:
1. `internal/notify/core/morning_purity_test.go` is deleted. Deleting it IS the task.
2. `TestNoAccountTypesNoTelegramLeak` is declared nowhere under `internal/notify`.
3. No other file is in the diff.

Checked by CI: `ci-go` runs `go test ./internal/notify/... ./scripts/purity/...`.

### 4 purity-delete-pages

Module: pages. Files: the 22 `purity_test.go` files under `internal/pages/` (advice_page, base_data_page, company_data_page, company_quarters_page, data_health_page, home_companies, home_household, home_macro, home_market, home_options, household_page, market_page, market_state_page, option_candidates_page, option_notices_page, option_pnl_page, option_tracking_page, positions_page, risk_page, strategy_replay_page, valuation_import_page, valuation_page) and one new file `internal/pages/company_quarters_page/free_cash_test.go`. Order: after 1.

Result for the owner: 22 copies of the scan are gone; the one behaviour test among them is kept.

Acceptance:
1. The 22 `purity_test.go` files are deleted. Deleting them IS the task; a worker must not keep one.
2. `internal/pages/company_quarters_page/free_cash_test.go` (new) holds `TestFreeCashCopiedFromCalculator` with the body it has on main; the helpers it uses (`quarters`, `val`, `in`, `build`, `aapl`) are not changed.
3. No `purity_test.go` remains under `internal/pages`; no `TestPurity`, `TestSourceStaysPure`, `TestNoFloat*` or `TestNoClock*` function of those 22 files is declared anywhere under `internal/pages`.
4. A helper (`sources`, `nonTestFiles`, `importsOf`) that only the deleted files used is gone; one that another test file uses is kept.
5. No non-test file of `internal/pages` is in the diff.

Checked by CI: `ci-go` runs `go test ./internal/pages/... ./scripts/purity/...`.

## Left out

- Merging the 29 rules into one common rule: dropped; the lists differ on purpose (home_macro bans the ease rule, valuation_page bans the store implementations) and a common list would weaken some package.
- Other scan-shaped tests outside `purity_test.go` files (for example `internal/pages` tests that grep source for other words): not read; a later design if wanted.
- `scripts/arch` and `.go-arch-lint.yml` (the import graph): not touched; the table only covers what the graph cannot see.
- Packages that have no `purity_test.go` today: they stay unchecked; adding them is one row each, by a later issue.
