# Symbol: one translation layer between the IBKR contract of watch.toml and the spelling of each vendor

Author: the optdesk designer (draft)
Owner approved: owner requirement 2026-11-10 (one company of watch.toml is recognised in every vendor's answer; one translation layer, not per-job patches); the pull request approval is pending
Date: 2026-11-10
Revision: 4 (answers the design verdict at 1323abe: the nil-translator path of issue 4 keeps `sameSymbol`, so no matching is lost between issues 4 and 6; the modules.md test-kit sentence covers both kits; answers adversarial.md: api.go declares a Translator interface and the one implementation lives in internal/symbol/table, wired by ot as sources are; FromVendor, the ibkr, flex and jquants vendors and the Exchange/Currency fields are cut and issue ot-jp-symbols is dropped; broken acceptance greps fixed; listed() behaviour on a translation error stated; section Adversarial added)

Source: owner 2026-11-10; the advisor's recommendation of the same day. Read on main at fe4d0cd: `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-candidates.md` (shape), `docs/adr/0004-markets-calendars-fx.md` (line 177), `internal/jobs/earningsrefresh/run.go`, `internal/jobs/optioneod/map.go` and `run.go`, `internal/sources/alphavantage/`, `internal/sources/jquants/`, `internal/sources/flex/testdata/report_sections.xml`, `cmd/ot/runonce.go`, and the `symbol` lines of `watch.toml` (read only).


## Goal

Owner requirement (2026-11-10): one company of `watch.toml` is recognised in every vendor's answer, by one translation layer and not by per-job patches (today Berkshire `BRK B` never matches Alpha Vantage's listing).

Advisor's frame (same day): the identity of a company is not its spelling. The IBKR contract symbol of `watch.toml` is the one canonical spelling optdesk keeps; each vendor's spelling is derived from it by one rule in one package and compared through that package, never by string patching in a job. A company is written once (`BRK B`); every place that talks to a vendor asks the translation layer. No job spells a share class by itself. The layer is a small interface with one own implementation behind it, so a library or a resolver can replace the implementation (owner, 2026-11-08).

## What main holds today

- `internal/jobs/earningsrefresh/run.go` `sameSymbol` (line 151) upper-cases and turns `-` and a space into `.` on both sides; it is called once, by `listed`, with the vendor symbol and `e.Contract.Symbol`. It is a per-job patch of one separator rule; it stays as the nil-translator path (issue 4) and `listed` uses the translator when `ot` supplies one (issue 6).
- J-Quants needs nothing: `internal/sources/jquants/parse.go` `fourDigits` already cuts the five-digit code the API prints to the four-character watch code before the job sees it, and `client.go` adds the `0` on the way out. `jpSymbols` in `cmd/ot/runonce.go` is the identity. There is no JP issue (Left out).
- `internal/sources/alphavantage` reads the whole calendar CSV and ignores `cfg.Symbols` (client.go line 46): no per-symbol query to translate. Only the comparison in `listed` needs the Alpha Vantage spelling.
- `internal/jobs/optioneod/map.go` `osiSymbol` pads `c.Symbol` to 6 characters, so `BRK B` becomes `BRK B ` (a space inside the root). ADR 0004 line 177 measured the option trading class of `BRK B` as `BRKB`: "trading class equal to the stock symbol with its spaces removed". `chainOf` in `optioneod/run.go` (line 322) compares `c.TradingClass == u.Symbol`, so `BRK B` finds no chain unless the manifest sets `trading_class`. The same missing rule, in a second place. `optioneod.New` is the constructor `cmd/ot` wires today (`runonce.go` line 292); `NewWithWatch` and the held variant are not wired in `cmd/ot/wire.go` on main.
- `watch.toml` is not in the repo (it is on the owner's host; `cmd/ot/watch.go` only parses it). The share-class symbols known from it and from ADR 0004 are `BRK B`; `BF B` (Brown-Forman) follows the same rule. Tests therefore use named symbols and no count of the watch list.
- pyside contract symbols are IBKR by construction; pages print the canonical form. Neither changes.

## Decisions

1. New module `symbol`, root package `internal/symbol`, standard library only, no store, no network, no clock. Layout, mirroring `internal/sources` (root `api.go` with interface types and a `NewXFunc` type, implementation in a sub-package, wired by `cmd/ot`, which modules.md allows to import implementation packages):
   - `internal/symbol/api.go`: declarations only: `Vendor`, one constant, `Translator`, `NewTranslatorFunc`.
   - `internal/symbol/table/table.go`: the implementation, `func New() symbol.Translator` (the value of `NewTranslatorFunc`).
   - `internal/symbol/contracttest/contracttest.go`: the contract suite run against any `Translator`, used by `api_test.go` with a fake and by `table/table_test.go` with the real one.
   `jobs` imports only the root `internal/symbol` and receives a `symbol.Translator` from its constructor arguments; `ot` calls `table.New()`.
2. Own glue behind an interface (owner 2026-11-08). A library was looked for and none fits: the rule is one separator replace and one space removal, and no mainstream Go package spells vendor tickers (own implementation, because the rule is a few lines and any package would carry a vendor list we do not use).
3. The canonical form is the symbol string of the IBKR contract as written in `watch.toml` (`BRK B`). There is no `Contract` struct: callers pass `e.Contract.Symbol`.
4. Input normalisation of both functions: trim, upper-case, and read a run of spaces, dots and dashes between two alphanumeric groups as one share-class separator. So the canonical `BRK B` and the vendor forms `BRK.B` and `BRK-B` all name the same company; this makes the unverified Alpha Vantage printing form irrelevant, with no `FromVendor`.
5. `ToVendor(symbol, v)`: for `AlphaVantage` returns the normalised symbol with the share-class separator `-` (`BRK B` gives `BRK-B`, `AAPL` and `278A` are unchanged). It returns an error for an empty symbol and for an unknown vendor. Adding a vendor later is one constant and one case.
6. `OSIRoot(symbol)`: the normalised symbol with every separator removed (`BRK B` gives `BRKB`, `BF B` gives `BFB`, `SPY` gives `SPY`). It has no error: the length check (at most 6) and the padding to 21 characters stay in `osiSymbol`, which already refuses a root over 6.
7. Earnings `listed` (issue 4) compares `ToVendor(e.Contract.Symbol, AlphaVantage)` with `ToVendor(l.Symbol, AlphaVantage)` by `==`, both sides through the same function, for both markets (the rule only touches separators, and JP codes have none). When `ToVendor` returns an error: for the entry's symbol, the row gets status `absent` and the reason is logged once through the logger the job already holds (slog warning, symbol and error text, no other data); for a listing's symbol, that listing is skipped (a vendor row with an empty symbol is not an entry's listing). No error is returned and no row has a status other than the existing ones.
8. Unverified: the form Alpha Vantage prints for a share class (all three accepted); the gateway listing the `BRKB` chain on `SMART` (ADR 0004's measurement, not rechecked offline); the Flex form of a share class (no Flex consumer is built, so it is not an open question of this design).


## Modules

`docs/contracts/symbol.md` and `docs/design/modules.md` change as follows. `docs/design/modules.md` changes in this pull request (not in an issue), as `internal/compute/valuationapi` did (valuation.md, Revision 11): no issue edits it. `arch.toml` and `.go-arch-lint.yml` are untracked since #894, so no Files line and no acceptance item names them. A new `api.go` package belongs to the first matching row and adding one is an interface change with a Design line: the interface issue (issue 1) carries it.

Columns of the table: `| module | paths | entry | tests | depends |`.

New row, after `gaphistory` and before `store` (no earlier row matches `internal/symbol/**`; first match wins). The depends cell is empty because the module imports the standard library only (as `repo`, whose `profile` has an empty import list), and `contracttest` and `table` import the root of their own module:

```
| symbol | internal/symbol/** docs/contracts/symbol.md | internal/symbol/api.go internal/symbol/contracttest | internal/symbol/api_test.go internal/symbol/contracttest |  |
```

Current rows on main (fe4d0cd), quoted in full:

```
| jobs | internal/jobs/** docs/contracts/jobs.md | internal/jobs/api.go | internal/jobs/api_test.go | contracts clock sources broker store compute |
| ot | cmd/** docs/contracts/ot.md docs/contracts/ot/** tools/ot-flex-export/** tools/ot-history-export/** |  | cmd/ot/main_test.go | contracts clock sources broker store runner jobs notify pages repo compute gaphistory |
```

Changed rows, exact new text (only the depends cell changes: `symbol` is appended):

```
| jobs | internal/jobs/** docs/contracts/jobs.md | internal/jobs/api.go | internal/jobs/api_test.go | contracts clock sources broker store compute symbol |
| ot | cmd/** docs/contracts/ot.md docs/contracts/ot/** tools/ot-flex-export/** tools/ot-history-export/** |  | cmd/ot/main_test.go | contracts clock sources broker store runner jobs notify pages repo compute gaphistory symbol |
```

`jobs` needs `symbol` for the `symbol.Translator` of its constructors; `ot` needs it to call `table.New()` (an implementation package of a row in its depends cell, as for `sources`). `sources` does not depend on `symbol`. Two more edits of the same file: the `Layers` sentence "data = contracts, clock, broker, sources, gaphistory, store, compute" gains `symbol` after `gaphistory`, and the table `## Contracts of the modules` gets one row after `gaphistory` (six columns `module | layer | inputs | outputs | forbidden | contract document`): `| symbol | data | a stock symbol of the IBKR contract | the spelling of a vendor, the option root | opening anything, any network, any state | docs/contracts/symbol.md |`.

## Interface

```go
// api.go: declarations only.

// Package symbol translates the stock symbol of the IBKR contract of watch.toml (the one spelling kept)
// into the spelling of a vendor. It reads nothing and keeps nothing. The implementation is
// internal/symbol/table.
package symbol

// Vendor names a spelling.
type Vendor string

// AlphaVantage is the spelling of the Alpha Vantage earnings calendar.
const AlphaVantage Vendor = "alphavantage"

// Translator is the translation layer. Both methods trim, upper-case and read a run of spaces, dots and
// dashes between two alphanumeric groups as one share-class separator, so the canonical form and the vendor
// forms of one company give one answer.
type Translator interface {
	// ToVendor returns the spelling of symbol that v uses ("BRK B" gives "BRK-B" for AlphaVantage). It fails
	// on an empty symbol and on an unknown vendor.
	ToVendor(symbol string, v Vendor) (string, error)
	// OSIRoot returns the option root and trading class of symbol: the separators removed ("BRK B" gives
	// "BRKB"). Its length is not checked and it is not padded.
	OSIRoot(symbol string) string
}

// NewTranslatorFunc is the constructor of the implementation (internal/symbol/table.New).
type NewTranslatorFunc func() Translator
```

Contract suite `internal/symbol/contracttest`: `func Run(t *testing.T, newT symbol.NewTranslatorFunc)`, table-driven, parallel subtests. Cases: `ToVendor` of `BRK B`, `BRK-B`, `BRK.B`, `brk b` and ` BRK B ` gives `BRK-B`; `BF B` gives `BF-B`; plain `AAPL` and `278A` are unchanged; an empty symbol and an unknown vendor give an error. `OSIRoot` of `BRK B` gives `BRKB`, `BF B` gives `BFB`, `SPY` gives `SPY`, ` brk  b ` gives `BRKB`. No count of the watch list; `watch.toml` is not read.

Both sides of the contract: the consumer side is a test of a function that uses a `Translator` only through the interface, with a fake (`api_test.go`); the implementation side is the same suite on `table.New()` (`table/table_test.go`).

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, and touches no live broker or database. Acceptance items are results of git and grep on the branch; every test run is listed under Checked by CI. The string `$BASE` below is `$(git merge-base HEAD origin/main)`.

| # | issue | module | order |
|---|---|---|---|
| 1 | symbol-interface (interface) | symbol | first |
| 2 | symbol-table | symbol | after 1 |
| 3 | jobs-symbol-ctor-types (interface) | jobs | after 1 |
| 4 | earnings-symbol | jobs | after 2 and 3 |
| 5 | option-eod-osi-root | jobs | after 2 and 3 |
| 6 | ot-wire-symbol | ot | after 4 and 5 |

Issues 4 and 5 touch different packages and may land in either order. No issue edits `docs/design/modules.md`, `arch.toml` or `.go-arch-lint.yml`.

### 1. symbol-interface (interface)

Module: symbol. Size: S.
Files: `internal/symbol/api.go`, `internal/symbol/api_test.go`, `internal/symbol/contracttest/contracttest.go`, `docs/contracts/symbol.md`
Design: docs/design/symbol.md#Interface
Goal: the declarations of `## Interface`, the contract suite, a fake `Translator` in `api_test.go` run through the suite and through a consumer function, and `docs/contracts/symbol.md` fixing the promises (canonical form, normalisation, errors, OSI root unpadded and unchecked, no I/O).
Acceptance:
1. `grep -c "^func " internal/symbol/api.go` prints 0, and `grep -cE "^type (Vendor|Translator|NewTranslatorFunc) " internal/symbol/api.go` prints 3.
2. `grep -cE "^import|net/http|database/sql|time\.Now|os\." internal/symbol/api.go` prints 0, and `grep -rh "github.com/andrewrong/optdesk/" internal/symbol/contracttest | grep -v 'internal/symbol"' | wc -l` prints 0.
3. `grep -hE "^func (TestTranslatorFake|TestConsumerThroughInterface)\(" internal/symbol/api_test.go | wc -l` prints 2, and `grep -l "time.Sleep" internal/symbol/api_test.go internal/symbol/contracttest/contracttest.go | wc -l` prints 0.
4. `grep -cF -e "BRK B" -e "BF B" -e AAPL internal/symbol/contracttest/contracttest.go` prints at least 3.
5. `git diff --name-only $BASE | grep -vxF -e internal/symbol/api.go -e internal/symbol/api_test.go -e internal/symbol/contracttest/contracttest.go -e docs/contracts/symbol.md | wc -l` prints 0.
Checked by CI: `go test ./internal/symbol/... -race -count=1` (`TestTranslatorFake`, `TestConsumerThroughInterface`), ci-go with `go run ./scripts/arch` and go-arch-lint (the derived component `symbol`).

### 2. symbol-table

Module: symbol. Size: S.
Files: `internal/symbol/table/table.go`, `internal/symbol/table/table_test.go`
Design: docs/design/symbol.md#Decisions (Decisions 4 to 6)
Goal: `func New() symbol.Translator` with the rules of Decisions 4 to 6; no I/O, no clock, no global table.
Acceptance:
1. `grep -c "^func New() symbol.Translator" internal/symbol/table/table.go` prints 1.
2. `grep -h "github.com/andrewrong/optdesk/" internal/symbol/table/table.go | grep -v 'internal/symbol"' | wc -l` prints 0, and `grep -cE "net/http|database/sql|time\.Now|os\.Open|os\.ReadFile" internal/symbol/table/table.go` prints 0.
3. `grep -c "contracttest.Run" internal/symbol/table/table_test.go` prints at least 1.
4. `grep -hE "^func (TestTableContract|TestToVendorAlphaVantage|TestOSIRoot)\(" internal/symbol/table/table_test.go | wc -l` prints 3, and `grep -cF -e BRK.B -e BRK-B -e BFB -e BRKB internal/symbol/table/table_test.go` prints at least 4.
5. `git diff --name-only $BASE | grep -vxF -e internal/symbol/table/table.go -e internal/symbol/table/table_test.go | wc -l` prints 0.
Checked by CI: `go test ./internal/symbol/... -race -count=1`, go-arch-lint.

### 3. jobs-symbol-ctor-types (interface)

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`
Design: docs/design/symbol.md#Decisions (Decision 1)
Goal: additive constructor types that carry a `symbol.Translator` as the last argument: `NewEarningsRefreshSymbolsFunc` (the arguments of `NewEarningsRefreshFunc`, then the translator) and `NewOptionEodSymbolsFunc` (the arguments of `NewOptionEodFunc`, then the translator). Existing types are unchanged, so `cmd/ot` still builds. The doc comments say that a nil translator means no translation: the job behaves as it does on main today, including its own matching of share-class separators.
Acceptance:
1. `grep -cE "^type (NewEarningsRefreshSymbolsFunc|NewOptionEodSymbolsFunc) " internal/jobs/api.go` prints 2, and `grep -c '"github.com/andrewrong/optdesk/internal/symbol"' internal/jobs/api.go` prints 1; `grep -c 'internal/symbol/' internal/jobs/api.go` prints 0.
2. `git diff $BASE -- internal/jobs/api.go | grep -c '^-[^-]'` prints 0 (additions only).
3. `grep -c "TestSymbolCtorTypes" internal/jobs/api_test.go` prints at least 1.
4. `git diff --name-only $BASE | grep -vxF -e internal/jobs/api.go -e internal/jobs/api_test.go | wc -l` prints 0.
Checked by CI: `go test ./internal/jobs -race -count=1`, ci-go, go-arch-lint (the edge `jobs` to `symbol`).

### 4. earnings-symbol

Module: jobs. Size: S.
Files: `internal/jobs/earningsrefresh/run.go`, `internal/jobs/earningsrefresh/run_test.go`
Design: docs/design/symbol.md#Decisions (Decision 7)
Goal: `earningsrefresh.NewWithSymbols` (a `jobs.NewEarningsRefreshSymbolsFunc`) stores the translator; `listed` compares `ToVendor(entry symbol, AlphaVantage)` with `ToVendor(listing symbol, AlphaVantage)` as Decision 7 says (entry error: row `absent` and one logged warning; listing error: listing skipped); `sameSymbol` stays unchanged. `New` keeps its signature and delegates to `NewWithSymbols` with a nil translator; with a nil translator `listed` calls `sameSymbol` exactly as on main (`-`, `.` and a space are one separator, case ignored), so `cmd/ot` matches no less between issues 4 and 6 than it does today, and issue 6 switches the wiring. `sameSymbol` is left in place for a later cleanup once no caller passes nil (not in this design).
Acceptance:
1. `git diff $BASE -- internal/jobs/earningsrefresh/run.go | grep -c '^-func sameSymbol'` prints 0 (the function is kept); `grep -c "ToVendor" internal/jobs/earningsrefresh/run.go` is at least 2; `grep -c "^func NewWithSymbols" internal/jobs/earningsrefresh/run.go` prints 1.
2. `grep -hE "^func (TestListedShareClassDash|TestListedShareClassDot|TestListedEntryError|TestListedListingError|TestListedNilTranslator)\(" internal/jobs/earningsrefresh/run_test.go | wc -l` prints 5; the first has the entry `BRK B` and the listing `BRK-B`, the second the listing `BRK.B`, the fifth builds the job with a nil translator and requires the entry `BRK B` to match the listing `BRK.B` (the old rule), the third an entry symbol that is empty and requires status `absent`, the fourth a listing with an empty symbol and requires the other listing to be used.
3. The tests use a translator fake in the test file (the real one is not imported): `grep -c "internal/symbol/table" internal/jobs/earningsrefresh/run.go internal/jobs/earningsrefresh/run_test.go | grep -vc ':0$'` prints 0.
4. `git diff --name-only $BASE | grep -vxF -e internal/jobs/earningsrefresh/run.go -e internal/jobs/earningsrefresh/run_test.go | wc -l` prints 0.
Checked by CI: `go test ./internal/jobs/earningsrefresh -race -count=1` with the five new tests, `go test ./internal/jobs/...`, go-arch-lint; `run.go` stays under its ceiling in `docs/design/ratchet.json` when listed.

### 5. option-eod-osi-root

Module: jobs. Size: S.
Files: `internal/jobs/optioneod/map.go`, `internal/jobs/optioneod/map_test.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/run_test.go`
Design: docs/design/symbol.md#Decisions (Decision 6; ADR 0004 line 177)
Goal: `optioneod.NewWithSymbols` (a `jobs.NewOptionEodSymbolsFunc`) stores the translator. `osiSymbol` pads `OSIRoot(c.Symbol)` to 6 (a root over 6 is still refused), so `BRK B` expiry 20261120 strike 500 gives `BRKB  261120C00500000`; `chainOf` takes the trading class `OSIRoot(u.Symbol)` when the manifest sets none, both for `class` and for the comparison. A symbol without a space is unchanged. `New` keeps its signature and its old behaviour: it is the path without a translator and is replaced by `NewWithSymbols` in issue 6.
Acceptance:
1. `grep -c "OSIRoot" internal/jobs/optioneod/map.go internal/jobs/optioneod/run.go | grep -vc ':0$'` prints 2; `grep -c "class = u.Symbol\|c.TradingClass == u.Symbol" internal/jobs/optioneod/run.go` prints 0 (main has both, lines 316 and 322 of `run.go`).
2. `grep -hE "^func (TestOSISymbolShareClass|TestChainOfShareClass)\(" internal/jobs/optioneod/map_test.go internal/jobs/optioneod/run_test.go | wc -l` prints 2; the first requires the 21 characters `BRKB  ` + `261120` + `C` + `00500000`.
3. The `TEST` and `SPY` cases of `map_test.go` are kept: `git diff $BASE -- internal/jobs/optioneod/map_test.go | grep -E '^-' | grep -F -e TEST -e SPY | wc -l` prints 0.
4. `git diff --name-only $BASE | grep -vxF -e internal/jobs/optioneod/map.go -e internal/jobs/optioneod/map_test.go -e internal/jobs/optioneod/run.go -e internal/jobs/optioneod/run_test.go | wc -l` prints 0.
Checked by CI: `go test ./internal/jobs/optioneod -race -count=1` with the two new tests, `go test ./internal/jobs/...`, go-arch-lint. `run.go` stays under its ceiling in `docs/design/ratchet.json` when listed. The gateway listing the `BRKB` chain is ADR 0004's measurement and is not rechecked offline.

### 6. ot-wire-symbol

Module: ot. Size: M.
Files: `cmd/ot/wire.go`, `cmd/ot/runonce.go`, `cmd/ot/dryrun.go`, `cmd/ot/runonce_test.go`, `cmd/ot/dryrun_test.go`, `cmd/ot/wire_test.go`, `cmd/ot/preflight_test.go`, `cmd/ot/scheduler_test.go`, `cmd/ot/jp_wire_test.go`, `cmd/ot/runonce_option_test.go`
Design: docs/design/symbol.md#Decisions (Decision 1)
Goal: the dependency table of `cmd/ot` builds the translator with `table.New()` and the two jobs with `earningsrefresh.NewWithSymbols` and `optioneod.NewWithSymbols`: the fields `newEarningsRefresh` and `newOptionEod` get the types of issue 3 and the call sites at `runonce.go` lines 224 and 292 pass the translator. `earningsrefresh.New` and `optioneod.New` stay. `dryrun.go` builds option_eod through `d.newOptionEod` (`dryRunBuild`, line 205), so it passes the translator too, and every test file of `cmd/ot` that assigns `newOptionEod` or `newEarningsRefresh` a function of the old signature (`dryrun_test.go` lines 77 and 137, `wire_test.go`, `preflight_test.go`, `scheduler_test.go`, `jp_wire_test.go`, `runonce_option_test.go`, `runonce_test.go`) takes the translator as the last argument, because the field types change; they are in Files for that reason and for nothing else.
Acceptance:
1. `grep -c "table.New()" cmd/ot/wire.go` prints at least 1; `grep -c "NewWithSymbols" cmd/ot/wire.go` prints 2.
2. `grep -c "symbol/table" cmd/ot/wire.go` prints 1, and `grep -rl "internal/symbol/table" cmd internal/jobs | wc -l` prints 1 (only `cmd/ot` imports the implementation).
3. `grep -hE "^func (TestRunOnceEarningsShareClass)\(" cmd/ot/runonce_test.go | wc -l` prints 1 and the test has the entry `BRK B` and the listing `BRK.B`.
4. `git diff --name-only $BASE | grep -vxF -e cmd/ot/wire.go -e cmd/ot/runonce.go -e cmd/ot/dryrun.go -e cmd/ot/runonce_test.go -e cmd/ot/dryrun_test.go -e cmd/ot/wire_test.go -e cmd/ot/preflight_test.go -e cmd/ot/scheduler_test.go -e cmd/ot/jp_wire_test.go -e cmd/ot/runonce_option_test.go | wc -l` prints 0.
Checked by CI: `go test ./cmd/ot -race -count=1`, ci-go, go-arch-lint; `cmd/ot/runonce.go`, `dryrun.go` and `wire.go` stay under their ceilings in `docs/design/ratchet.json` when listed.


## Left out

- Vendors `ibkr`, `flex`, `osiroot` as a vendor, and `FromVendor`, `Contract`, `Exchange`, `Currency`: no consumer on main (no Flex code reads a symbol; the OSI root is its own method). Adding a vendor later is one constant and one case in `table.go`.
- J-Quants and the old issue `ot-jp-symbols`: `fourDigits` in `internal/sources/jquants/parse.go` already cuts the five-digit code to four before the job sees it, `client.go` adds the `0`, and the watch code is already the four-character code. A translation there would test an input no job receives.
- An alias store table (a `master.symbol_alias`). It needs a migration, a role and a writer for a rule that is one replace. Revisit if a vendor spells a company in a way no rule explains.
- Yahoo and Stooq forms: no Yahoo source is on main; the Stooq source was dropped on 2026-11-09 (`docs/design/alignment-v25.md`).
- CIK to ticker resolution through the SEC `company_tickers.json`: it identifies a company by `issuer` instead of a spelling, but Alpha Vantage's calendar prints no CIK and the module would no longer be pure. Adopt it if a vendor appears that prints a CIK and no ticker.
- Suffix forms for foreign exchanges (`7203.T`): no vendor on main wants them.
- Changing the canonical form, the pyside contract symbols, or the symbol the pages print.
- Consumers with no mismatch today: `internal/compute/optionnotice`, `internal/jobs/optioncandidates`, `internal/pages/option_notices_page` compare IBKR symbols with IBKR symbols; not touched.
- `NewWithWatch`, `NewWithWatchWindow`, `NewWithHeld` of option_eod: not wired in `cmd/ot` on main; they take the translator in the issue that wires them.

## Adversarial

Answered against the dispatcher window's scratchpad file `design-symbol/adversarial.md` (kept on the machine, not in the repository).

1. Gap against the goal (J-Quants identity, vendors without a consumer, listed() on error): adopted. The JP issue and the 40630 test are gone; `ibkr`, `flex`, `FromVendor`, `Exchange` and `Currency` are cut; Decision 7 states the error behaviour and issue 4 tests it. The BRK B option chain stays untested live; it is ADR 0004's measurement (Decision 8).
2. Root package with logic: adopted. The root `internal/symbol` is `api.go` only; `Translator` is an interface, the implementation is `internal/symbol/table` wired by `ot` (the `sources` pattern), and the contract suite runs against a fake and the real one.
3. Simpler way (a `Key` function): partly adopted. The two-method reduction is adopted (`ToVendor` and `OSIRoot`, no `FromVendor`, no `Contract`); the normalisation of Decision 4 plays the role of `Key`. The `Vendor` argument is kept because the owner asked for a translation layer and a second vendor is one case.
4. Lock-in: partly adopted. The error signature stays for `ToVendor` (unknown vendor, empty symbol); the cells of jobs and ot gain `symbol` (needed for the interface); the lenient input is deliberate and now written in the contract.
5. Claims checked: the false items are fixed. Issue 1 no longer greps the word `optdesk` (it greps `api.go` for imports and the contracttest imports); issue 4 of the old list is dropped; no count of the watch list (the tests name `BRK B`, `BF B`, `AAPL`, and `watch.toml` is not in the repo); the `grep -vxFf` of a whole test file is replaced by a diff of removed lines.
6. Likeliest slip: addressed by the fixes of point 5.
