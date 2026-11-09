# Cut-over: `ot account-import` and `ot bar-import` with `--verify`, the replay by `ot backfill`, the parallel run and `ot compare-figures`, the web no-compute test, week-13 closure

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 2
Revision note: revision 2 answers the read-only attack, kept in the window's scratch (six points; answers under `## Adversarial`). Window decisions applied: no Importer or Target framework and no new module, two import commands in the pattern of `ot ledger-import` each with a read-only `--verify`; the old figures were read in the option-trader source first and the baseline is a parallel run (Decision 2); a table splits the register into figures replayable from imported inputs and live-only ones (Decision 8); imports stop at the last complete session day (Decision 4); the kind='import' column and the merge views are dropped (Decision 5); the part that needs `compute-stage2` is in Left out; the register holds 54 figures, 44 of them `pending-recompute` (the alignment index says 53).

Status: draft, not yet a pull request. Builds on main at a4a5fe5: `store.AccountWriter`, `store.AccountReader`, `store.Capture` and migration `0019_account.sql` (schema `account`, views `rd.account_*_r1`), `store.OptionLedgerWriter`, `store.OptionLedgerReader`, `jobs.ImportResult`, `jobs.ImportOptionTraderFunc` (declared; implemented by `docs/design/option-ledger.md` entry 10), `store.ReadingReader` (`LatestReadings`), `cmd/ot/backfill.go` (`ot backfill --day`), `tools/ot-flex-export/`, `docs/conventions/register.toml`, `web/no-compute-allow.txt` (empty), `github.com/pelletier/go-toml/v2` (direct in `go.mod`), and `docs/design/alignment-v25.md` Decision 3 item 9 with the rows "ot import + import --verify", "replay tool", "kind='import' merge" and "stop-the-world import, replay last N sessions, cut-over".
Not on main yet, so some entries wait, each by entry name: `account-store-impl` and `account-wire` (`docs/design/account-data.md` entries 8 and 10); `bar-datasets` and `bar-store-api` (`docs/design/market-bars.md`, PR 768, entries 1 and 3); `optionledger-wire` (`docs/design/option-ledger.md` entry 12); `compute-stage2` and `builders-wave` (follow-ons, not yet drafted) for the figures that no job computes yet (Left out). The plan text is not in the repository; the scope is read from the alignment index and the gap table of 2026-11-09.
Needs the owner (owner-only paths): none. No entry edits `contracts/proto/**`, `.github/**`, `docs/design/ratchet.json`, `services.toml` or `pyside/src/optdesk/ibkr/transport.py`. `internal/store/api.go` is 799 lines on main and no entry edits it.
Source: owner 2026-11-08/09: options trading first and complete; all account data before cut-over (Flex and daily FX included); only the main IBKR account must work; no silent drops; own glue code sits behind an interface; prefer a mainstream library; the web renders data and computes nothing. Plan v2.5 section 11, week 7 (`ot import`, `--verify`, the replay tool, note 5), week 12 (stop-the-world import, replay of the last N sessions, cut-over, check-every-figure) and week 13 (allowlist to zero, enforce mode, runbook).

## Goal

On the day the owner chooses, the old system (option-trader, a DuckDB file) is stopped for good and the account history, the daily bars and the daily rates it holds are in the new database, proven by a read-only verify; for N sessions both systems have run side by side and every figure the old system showed is set beside the figure optdesk stored, each difference explained by the owner or fixed; the web is guarded against computing; one runbook says the steps in order. This design stores and reports. It builds no page, no job that computes a figure and no figure.

## Decisions

1. What is imported, from where. A dev-only exporter `tools/ot-history-export/` (its own uv project with a committed `uv.lock`, not run in CI, imported by nothing at run time, as AGENTS.md allows; the mainstream, MIT `duckdb` package, as `tools/ot-flex-export/` does) reads a copy of option-trader's DuckDB file, read-only, and writes JSON Lines: `account.jsonl` (one line per alias and session day: that day's last snapshot with its summary values and positions), `price_bar.jsonl`, `fx_rate.jsonl`, and `figure.jsonl` (Decision 2). The Flex rows come from the existing `tools/ot-flex-export/` and `ot ledger-import` and are not touched here. The exporter refuses a table whose columns differ from the ones it was written for, and refuses a file in which a value of an alias column looks like an IBKR account id (letters then digits): only aliases leave the old system.
2. Where the old figures live (read in the option-trader source, 2026-11-09). The finding is mixed. Stored: the table `reading` (`src/option_trader/readings/store.py`, `_SCHEMA`: name, version, subject_kind, subject, day, rev, value, unit, state) has the same key as `readings.reading` of optdesk, and the scheduler writes some figures into it (`scheduler/record_levels.py`, `scheduler/flex_readings.py`: `household.monthly_premium_received`, the market levels, the macro readings). Computed at page build: the figures of the pages are made by the view-model builders (`viewmodels/options_figures.py`, `viewmodels/household_page_builder.py` and the other `viewmodels/*_page_builder.py`), which the register names in `implemented_by` (for example `option_trader.viewmodels.options_figures._series`) and which read the stored inputs, not a stored figure. So there is no stored baseline for most page figures. The baseline is what the old system shows: for the N sessions of the parallel run (Decision 7) the owner saves the page JSON of the old serve each session day to a directory, and the exporter turns it into `figure.jsonl` by reading each register entry's `pages` path in it (for example `options_summary.card.premium_series_6m.premium`), and adds the rows of the stored `reading` table for the same days. A `pages` path the exporter cannot find in a saved page is counted `path_not_found` in its printed counts and the figure is `unchecked` with that reason; it is never written as zero. `figure.jsonl` lines carry name, version, subject kind, subject, day, the value as decimal text, the places of the old display, and `from` (`reading` or `page`).
3. Two commands, no framework (window decision 3, the pattern of `option-ledger.md` Decision 7). `ot account-import DIR [--map from=to ...] [--through DAY] [--verify] [--report FILE]` reads `DIR/account.jsonl` and writes each line as one `store.Capture` through `store.AccountWriter.WriteCapture`, with the capture id `<alias>:<session_day>:import` (the id rule of `store.Capture`, run key `import`). `ot bar-import DIR [--through DAY] [--verify] [--report FILE]` reads `price_bar.jsonl` and `fx_rate.jsonl` and writes them through the writers of `internal/store/bar` (after PR 768). Both are insert-only and idempotent: a second run counts `Same`. Counts only are printed: no row, no value, no account number. The alias map is as in the ledger: the default is the same label; when at least one `--map` is given, an alias that no `--map` names refuses the whole file before any write.
4. Complete days only. `--through DAY` is the last complete session day of the old system, and the exporter and the importers refuse any line after it: the day of the export, the day the old collectors were stopped and a bar of the current session (an unfinal bar) are never imported, so the files of a later export agree with the files of an earlier one on every day they share. The new scheduler starts on a later session day, so an import capture and a live capture never share a day.
5. kind='import' (window decision 4: kept only if still needed). It is not needed. The plan note asks that imported rows can be told from live ones and merged; the capture id already ends in `:import` for an imported capture, the ledger rows already carry `source = ot_import`, bars and rates carry it after entry 5, and by Decision 4 no day holds both kinds, so there is nothing to merge. The generated `kind` column, the `rd.account_*_r2` views and the reader change of the first draft are dropped; they are named in Left out with what would bring them back.
6. `--verify` (read-only, two rehearsals). `--verify` writes nothing: it reads the same file, asks the reader whether each line is stored with the same content (the SHA-256 of the canonical line against the same hash of the stored rows rebuilt through `store.AccountReader.Positions` and `Values`, or the bar and rate readers) and reports per dataset `Missing` (no stored row of the key) and `Different` (a stored row of the key with another hash, for example a day the old system later restated, or a stale import capture from an earlier export: insert-only keeps the old row, so an import reports it as `Conflict` and a verify as `Different`). It exits 0 only when both are zero. `--report FILE` writes counts, the SHA-256 of the input file and `ok`, no row. A rehearsal is an import and a verify on a fresh scratch database each time (window default — owner may change): the scratch database is a new database from `ot setup`, named by the environment file of the profile used, so a second rehearsal on a later export has no stale capture to conflict with. Rehearsal 1 and rehearsal 2 use two exports; the final import is the same commands on the live database. Three reports with `ok` are the evidence (Decision 10).
7. The parallel run and the replay (window decision 2). Both systems run side by side for N session days (window default — owner may change: N is 10) after the first import into the live database, the old system on its own, optdesk running its scheduler. Each session day the owner saves the old serve's page JSON. The replay of past sessions needs no new tool: `ot backfill --day D` (shipped, `cmd/ot/backfill.go`) runs the jobs of a past day on a scratch profile after the imports, one command per day for the last 20 session days; what it computes is limited to the jobs that exist (Left out). Then `ot compare-figures` (entry 6) reads `figure.jsonl` and reads optdesk's readings back with `store.ReadingReader.LatestReadings`, once for each distinct `(name, subject_kind, subject)` of the old file over the range of its days (the subject list comes from the old file, because `LatestReadings` needs it).
8. Replayable and live-only figures (window default — owner may change the table). The 54 figures of the register by id prefix, with the inputs they need. Replayable from the imported account, Flex, bar and rate history: `home.options_summary` (2), `home.household` (5), `household.*` (14), `option.tracking` (1), `market.spx_close` (1) and `market.sp500_vs_200d` (2). Live-only (they need quotes, chains, IV or Greeks of the day, which no import holds; they are checked by the parallel run only): `home.options` (4), `structure.*` (10), `companies.volatility` (2), `option.candidates` (4) and `market.index_iv` (1). Made in optdesk with no old value (their `agreed_in` is not `carried over from option-trader`): `market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y`, `market.sp500_pctile_3y`, `market.iv30_own`, `market.ivp_own`, `market.ivr_own`, `market.vix_term_ratio`, `macro.ease` (9), plus the second version of `market.sp500_vs_200d`. The table is data in `docs/deploy/cut-over-figures.toml` (`[[class]]` with `prefix` and `class` = `replayable`, `live_only` or `new`), which the compare step reads; an id that no prefix names is `unclassified` and fails the compare.
9. Compare, every figure. `ot compare-figures --old FILE --register FILE --classes FILE --explained FILE` sets, for every `[[figure]]` of the register (go-toml/v2), the old rows beside optdesk's readings and gives it one status: `checked` (every row equal), `explained` (a row differs or is missing and the owner's `[[explained]]` entry has `id`, `reason` and `date`), `unexplained`, `new` (class `new`, nothing to compare), `unchecked` (no old row; the reason is printed: `path_not_found`, `no_old_value`, or `not_computed_yet` when optdesk has no reading because no job makes the figure). It exits 0 only when `unexplained` is 0 and `unclassified` is 0; `unchecked` is printed, each with its reason, for the owner to read, and the closure record (Decision 10) states their count. Equality is exact on the decimal text after both sides are rounded half-even to the old `places` with `math/big` (window default — owner may change: no tolerance). It checks values, not pixels: the look of a page against its design is the owner's comparison of screenshots (Left out).
10. Stop-the-world, runbook and closure (window default — owner may change: the restore drill). The old serve is not restarted and the collection gap between the last complete day and the first new capture is accepted (alignment index; plan line 25). `docs/deploy/cut-over.md` lists, in order: back up the new database and restore the backup into a scratch database, comparing row counts (the restore drill the alignment index defers to this design); copy the old DuckDB file (a copy, never the live file); run the exporter; rehearsal 1 and rehearsal 2 (Decision 6); stop the old collectors (owner); copy and export again with the last complete day as `--through`; the final imports and their verifies; start the new scheduler (owner); the parallel run of N sessions; replay with `ot backfill`; `ot compare-figures`; close. `docs/deploy/cut-over-record.md` is the owner's record of the closure: three verify report hashes, the dates, the exit code of the compare, its count of `unchecked` and the reason of each, and the check that `web/no-compute-allow.txt` has no line. The cut-over is closed when the owner fills the record and merges it.
11. The web test (Decision 9b of the first draft, the part this design can finish). `web/no-compute-allow.txt` is empty on main but no test reads it. A vitest reads it, fails when it holds a line, and fails when a file under `web/src/` outside `web/src/generated/` holds `toFixed`, `toLocale`, `Intl.` or `Math.`. This is the enforce mode of the web half. `web/deps-allowlist.txt` is a dependency list and stays. The register half, 44 `pending-recompute` hashes to zero and the test that refuses the word, needs `compute-stage2` and is in Left out.
12. Library or own code. JSON Lines reading, the canonical hash and the comparison by key are own code on the standard library (`encoding/json`, `bufio`, `crypto/sha256`, `math/big`): no mainstream library does an import keyed by dataset with a per-line content hash, and a diff library would not know the keys; own implementation, because of that. It sits behind three function types in `internal/jobs/api.go` (Interface), so a library can replace each. The register is read with go-toml/v2, mainstream and MIT, already in `go.mod`. Python uses `duckdb` (MIT). No new direct dependency.
13. Tests. No test touches option-trader's database, a live broker or service data: the exporter's test builds a small DuckDB file in a temporary directory with invented rows; every Go test reads fixture lines with invented aliases and values; PostgreSQL tests carry the build tag `pg`. Tests run in parallel (`t.Parallel()`, `t.TempDir()`), none sleeps, time comes from `clock.Clock`. No source file reaches 600 lines.
14. Nothing here goes to Telegram. Commands print counts and statuses to the terminal and to report files only; the margin cushion and the assignment cash appear in no report.

## Modules

No new row. One changed cell in `docs/design/modules.md` of this pull request: the paths of `ot` gain `tools/ot-history-export/**` (the exporter, in its own uv project), and the list of implementation packages `cmd/ot` may import gains `internal/jobs/cutover` (with `arch.toml` regenerated by `go run ./scripts/arch`, which the check in `checks.yml` compares). The import and compare code is the sub-package `internal/jobs/cutover` of the row `jobs`, as the ledger import is in `internal/jobs/optionledger`; jobs already depends on contracts, clock, sources, broker, store and compute, so no `depends` cell changes. The new sub-package is a go-arch-lint component `jobs/cutover`, generated from the directory, so entry 4 which creates it carries `.go-arch-lint.yml` (the licensed pair with repo); no other entry does. `docs/deploy/cut-over*` belong to docs.

`internal/jobs/cutover` imports `internal/store/bar`, a declarations-only package of the store row that PR 768 introduces. The paragraph of PR 768 on declarations-only packages, copied from `docs/design/modules.md` of `origin/design/market-bars` verbatim:

> A declarations-only package named in an entry cell (today `internal/store/bar`) holds declarations only (one `api.go` and its test, no logic) and is listed so that other modules import an interface without importing its implementation; the implementation (`internal/store/barpg`) is a separate package that is not an entry. It is imported like the root package of its row: the root component and the sub-packages of any row that has its row in the depends cell may import it (jobs and `jobs/*` import `store/bar`). This is the general import rule for declarations-only packages, and this paragraph is its one statement; the `depends` paragraph below refers to it. `scripts/arch` derives the rule from the table plus one list of its own, `declarationsOnly` (today `internal/store/bar`), because the table cannot tell a declarations-only package from the test kit: a root component receives an entry-listed package only when the package is in that list, so `internal/compute/contracttest` and `internal/contracts/genpb` never go to a root component. A sub-package component may also import a declarations-only package named in the entry cell of its own row and on that list (a row whose design adds such a package adds it to its entry cell and to the list in the same pull request). The sub-packages of a dependent row may import every entry-listed package; the generated sub-packages themselves (`contracts/gen`, `contracts/genpb`) import nothing and keep an empty import list. That includes `contracttest`, which a sub-package of jobs, notify or pages may therefore import (`_test.go` files, as above). `scripts/arch` follows neither part yet (a root component receiving a declarations-only package, a sub-package receiving the declarations-only packages of its own row); the interface entry `bar-store-api` of `docs/design/market-bars.md` carries the change (its Files line names `scripts/arch/main.go`).

The wait: the bar half (entries 2 and 7) starts only after the entry `bar-store-api` of `docs/design/market-bars.md` has merged, which creates `internal/store/bar` and teaches `scripts/arch` the rule above, and after the entry `bar-datasets` of the same design. The account half (entries 1, 3, 4, 6, 8) does not import `internal/store/bar` and does not wait for it. This design does not edit that paragraph; if PR 768 changes it, this copy is brought in line when it merges.

## Interface

- jobs, `internal/jobs/api.go`, declarations only, additions only (entry 1, the account half; entry 2, the bar half):
  - `type VerifyResult struct { Lines, Missing, Different int }`; `jobs.ImportResult` (shipped) is reused for an import.
  - `type ImportAccountFunc func(ctx context.Context, r io.Reader, w store.AccountWriter, aliasMap map[string]string, through contracts.Day) (ImportResult, error)` and `type VerifyAccountFunc func(ctx context.Context, r io.Reader, rd store.AccountReader, aliasMap map[string]string, through contracts.Day) (VerifyResult, error)` (`internal/jobs/cutover.ImportAccount`, `VerifyAccount`).
  - `type ImportBarsFunc func(ctx context.Context, bars, rates io.Reader, w jobs.BarWriter, through contracts.Day) (ImportResult, error)` and `type VerifyBarsFunc func(ctx context.Context, bars, rates io.Reader, rd jobs.BarReader, through contracts.Day) (VerifyResult, error)`, where `jobs.BarWriter` and `jobs.BarReader` are declared in the same file as `type BarWriter interface { bar.PriceBarWriter; bar.FxRateWriter }` and `type BarReader interface { bar.PriceBarReader; bar.FxRateReader }`.
  - `type FigureStatus string` with the values of Decision 9; `type FigureResult struct { ID string; Version int; Status FigureStatus; Reason string }`; `type FigureReport struct { Results []FigureResult; OK bool }`; `type CompareFiguresFunc func(ctx context.Context, old, register, classes, explained io.Reader, rd store.ReadingReader) (FigureReport, error)` (`internal/jobs/cutover.CompareFigures`).
- contracts (entry 5): `source` of `obs.price_bar` r1 and `obs.fx_rate` r1 also accepts `ot_import`.
- ot (entries 8 to 10): subcommands `account-import`, `bar-import`, `compare-figures`, each a file that registers itself with `register(...)` as `backfill` does.
- web (entry 11) and docs (entry 12): a test and text.

## Issues

| # | issue | module | order |
|---|---|---|---|
| 1 | cutover-account-api (interface) | jobs | first |
| 2 | cutover-bar-api (interface) | jobs | after 1 and `bar-store-api` |
| 3 | history-exporter | ot | first |
| 4 | account-import | jobs, plus repo (licensed pair) | after 1 |
| 5 | bar-source-import (interface) | contracts | after `bar-datasets` |
| 6 | figure-compare | jobs | after 1, 4 |
| 7 | bar-import | jobs | after 2, 4, 5 |
| 8 | account-import-wire | ot | after 4, `account-store-impl`, `account-wire` |
| 9 | bar-import-wire | ot | after 7, 8 |
| 10 | compare-wire | ot | after 6, 9 |
| 11 | enforce-web-no-compute | web | first |
| 12 | cutover-runbook | docs | after 9, 10 |

Entries 1 and 2 share `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` (2 after 1). Entries 4, 6 and 7 add files to one package and share no file (4 carries `.go-arch-lint.yml` alone). Entries 8, 9 and 10 share `docs/contracts/ot.md` and are one chain, 8, 9, 10; each registers itself in its own file.

### 1 cutover-account-api (interface)

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `docs/contracts/jobs.md`
Design: docs/design/cut-over.md#Interface
Order: first. It shares `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` with 2, which comes after it.
Goal: the account half of the Interface section: `VerifyResult`, `ImportAccountFunc`, `VerifyAccountFunc`, `FigureStatus`, `FigureResult`, `FigureReport`, `CompareFiguresFunc`, declarations only, with a test of the contract from both sides.
Acceptance:
1. `grep -cE '^type (VerifyResult|ImportAccountFunc|VerifyAccountFunc|FigureStatus|FigureResult|FigureReport|CompareFiguresFunc) ' internal/jobs/api.go` prints 7.
2. The five status values of Decision 9 are constants of `FigureStatus`: `grep -cE 'FigureStatus = "(checked|explained|unexplained|new|unchecked)"' internal/jobs/api.go` prints 5.
3. The additions hold no function body and the file imports nothing from `internal/store/bar`: `grep -c 'store/bar' internal/jobs/api.go` prints 0 in this entry.
4. `internal/jobs/api_test.go` has, for each of the three function types, a fake that implements it and a caller that uses it, and each added test calls `t.Parallel()`.
5. `docs/contracts/jobs.md` has a paragraph for each of the three function types and says that `--verify` writes nothing.
Checked by CI: `go test ./internal/jobs -race -count=1`, golangci-lint, go-arch-lint.

### 2 cutover-bar-api (interface)

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `docs/contracts/jobs.md`
Design: docs/design/cut-over.md#Interface and #Modules (the wait)
Order: after 1 (the same three files) and after the entry `bar-store-api` of `docs/design/market-bars.md`, which creates `internal/store/bar` and the arch rule that lets `internal/jobs` import it.
Goal: `BarWriter`, `BarReader`, `ImportBarsFunc`, `VerifyBarsFunc` as the Interface section gives them.
Acceptance:
1. `grep -cE '^type (BarWriter|BarReader|ImportBarsFunc|VerifyBarsFunc) ' internal/jobs/api.go` prints 4.
2. `BarWriter` embeds `bar.PriceBarWriter` and `bar.FxRateWriter`, and `BarReader` embeds `bar.PriceBarReader` and `bar.FxRateReader`: `grep -c 'bar\.PriceBarWriter\|bar\.FxRateWriter\|bar\.PriceBarReader\|bar\.FxRateReader' internal/jobs/api.go` is at least 4.
3. The additions hold no function body.
4. `internal/jobs/api_test.go` has a fake and a caller for each of the two function types, with `t.Parallel()` in each added test.
5. `docs/contracts/jobs.md` says that imported bars have basis `trades`, epoch 1 and source `ot_import`.
Checked by CI: `go test ./internal/jobs -race -count=1`, go-arch-lint, golangci-lint.

### 3 history-exporter

Module: ot. Size: L.
Files: `tools/ot-history-export/pyproject.toml`, `tools/ot-history-export/uv.lock`, `tools/ot-history-export/README.md`, `tools/ot-history-export/src/ot_history_export/__init__.py`, `tools/ot-history-export/src/ot_history_export/__main__.py`, `tools/ot-history-export/src/ot_history_export/accounts.py`, `tools/ot-history-export/src/ot_history_export/bars.py`, `tools/ot-history-export/src/ot_history_export/figures.py`, `tools/ot-history-export/tests/test_export.py`
Design: docs/design/cut-over.md#Decisions (Decisions 1, 2, 4, 12, 13)
Order: first. It shares no file with another entry.
Goal: `python -m ot_history_export COPY.duckdb PAGES_DIR OUTDIR --through DAY` writes `account.jsonl`, `price_bar.jsonl`, `fx_rate.jsonl` and `figure.jsonl`, read-only, printing counts only.
Acceptance:
1. The project is its own uv project with `duckdb` as its runtime dependency: `grep -c 'duckdb' tools/ot-history-export/pyproject.toml` is at least 1, and no file outside `tools/ot-history-export/` mentions `ot_history_export`.
2. Every module opens the file read-only: `grep -c 'read_only=True' tools/ot-history-export/src/ot_history_export/accounts.py tools/ot-history-export/src/ot_history_export/bars.py tools/ot-history-export/src/ot_history_export/figures.py` is at least 1 for each file.
3. `--through` is required, and no line with a day after it is written (a function that the test calls with an export day and a later day).
4. `accounts.py` keeps the last snapshot of each alias and day and writes the alias only; a value that looks like an IBKR account id makes it refuse the file; each exporter refuses a table whose columns differ from the expected ones, naming the table and no row.
5. `figures.py` reads the register's `pages` paths in each saved page JSON, counts the paths it cannot find as `path_not_found` in the printed counts, and writes `from` as `page` or `reading` on each line; a missing value is never written as zero.
6. `tools/ot-history-export/tests/test_export.py` builds its DuckDB file in `tmp_path` with invented rows and has no `sleep`; `tools/ot-history-export/README.md` says to use a copy of the file and has no absolute path and no account id.
Checked by CI: none (a dev-only tool is not run in CI, AGENTS.md); the owner runs `uv run --project tools/ot-history-export pytest` by hand.

### 4 account-import

Module: jobs, plus repo (licensed pair: `.go-arch-lint.yml`, the new component `jobs/cutover`). Size: M.
Files: `internal/jobs/cutover/doc.go`, `internal/jobs/cutover/jsonl.go`, `internal/jobs/cutover/alias.go`, `internal/jobs/cutover/accountimport.go`, `internal/jobs/cutover/accountverify.go`, `internal/jobs/cutover/accountimport_test.go`, `internal/jobs/cutover/accountverify_test.go`, `internal/jobs/cutover/jsonl_test.go`, `.go-arch-lint.yml`
Design: docs/design/cut-over.md#Decisions (Decisions 3, 4, 6, 12)
Order: after 1. It shares `.go-arch-lint.yml` with no other entry; 6 and 7 add files to its package.
Goal: `ImportAccount` and `VerifyAccount` with the signatures of entry 1.
Acceptance:
1. `ImportAccount` builds a `store.Capture` whose `ID` is the alias, the session day and the text `import` joined by colons and calls `WriteCapture` once per line: `grep -n '"import"' internal/jobs/cutover/accountimport.go` finds the run key, and `grep -c 'WriteCapture' internal/jobs/cutover/accountimport.go` is at least 1.
2. A line with a day after `through` refuses the whole file before the first `WriteCapture`: the refusal is a named function in `accountimport.go` (`grep -n 'afterThrough' internal/jobs/cutover/accountimport.go` finds the definition and one call above the first `WriteCapture`).
3. When the alias map is not empty, an alias it does not name refuses the whole file before any write (the function is in `alias.go`, and `accountimport.go` calls it before the first `WriteCapture`).
4. `VerifyAccount` never writes: `grep -c 'Write' internal/jobs/cutover/accountverify.go` prints 0; it calls `Positions` and `Values` of the reader and compares the SHA-256 of the canonical line (keys sorted, no space, in `jsonl.go`) with that of the rebuilt one; `Missing` counts a day with no stored capture, `Different` one whose hash differs.
5. A line that is not valid JSON is an error that names the line number and no content; no error, log line or report holds a value or an account number; no float type is in the package: `grep -rn 'float64\|float32' internal/jobs/cutover` prints nothing.
6. `.go-arch-lint.yml` has a component for `internal/jobs/cutover`: `grep -c 'internal/jobs/cutover$' .go-arch-lint.yml` is at least 1. The tests use a fake of `store.AccountStore` and call `t.Parallel()` in each test.
Checked by CI: `go test ./internal/jobs/cutover -race -count=1`, go-arch-lint, golangci-lint.

### 5 bar-source-import (interface)

Module: contracts. Size: M.
Files: `contracts/datasets/obs.price_bar.r1.json`, `contracts/datasets/obs.fx_rate.r1.json`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/obs.price_bar.md`, `docs/contracts/datasets/obs.fx_rate.md`
Design: docs/design/cut-over.md#Interface
Order: after the entry `bar-datasets` (entry 1 of `docs/design/market-bars.md`), which creates these files and shares `records.go` with this one; that entry lands first.
Goal: the `source` of both datasets also accepts `ot_import`, with the generated record and the dataset documents committed together.
Acceptance:
1. `grep -c 'ot_import' contracts/datasets/obs.price_bar.r1.json contracts/datasets/obs.fx_rate.r1.json` is at least 1 for each file, and the values `ibkr` and `ibkr_midpoint` are still listed.
2. `bash contracts/generate.sh && git diff --exit-code internal/contracts/gen` leaves no change.
3. Each of the two documents says that a row with source `ot_import` is written only by `ot bar-import`.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/contracts -race -count=1`, the generate check of `checks.yml`; the pull request body carries `Contracts: changed`.

### 6 figure-compare

Module: jobs. Size: L.
Files: `internal/jobs/cutover/compare.go`, `internal/jobs/cutover/register.go`, `internal/jobs/cutover/classes.go`, `internal/jobs/cutover/explained.go`, `internal/jobs/cutover/compare_test.go`, `internal/jobs/cutover/register_test.go`, `internal/jobs/cutover/testdata/`
Design: docs/design/cut-over.md#Decisions (Decisions 8, 9, 12)
Order: after 1, 4 (the package exists after 4). It shares no file with another entry.
Goal: `CompareFigures` with the signature of entry 1 and the statuses of Decision 9.
Acceptance:
1. `register.go` reads the register with `github.com/pelletier/go-toml/v2` and takes `id`, `version` and `agreed_in` of each `[[figure]]`: `grep -c 'go-toml/v2' internal/jobs/cutover/register.go` is 1.
2. `classes.go` reads `[[class]]` entries with `prefix` and `class` and gives an id that no prefix names the status `unclassified` in the error, which makes `OK` false; `explained.go` refuses an `[[explained]]` entry with an empty `reason`.
3. `compare.go` calls `LatestReadings` once for each distinct `(name, subject_kind, subject)` of the old file over the range of its days, and compares decimal text after rounding half-even to the old `places` with `math/big`: `grep -c 'LatestReadings' internal/jobs/cutover/compare.go` is at least 1, and `grep -n 'float' internal/jobs/cutover/compare.go` prints nothing.
4. `OK` is set in one place, true only when no result is `unexplained` and none is `unclassified`; a figure with no old row has the status `unchecked` and a `Reason` that is one of `path_not_found`, `no_old_value`, `not_computed_yet`.
5. The package never opens a file for writing: `grep -rn 'os.Create\|os.OpenFile\|WriteFile' internal/jobs/cutover/compare.go internal/jobs/cutover/register.go internal/jobs/cutover/classes.go internal/jobs/cutover/explained.go` prints nothing.
6. `testdata/` holds a register of three invented figures, a classes file, an explained file and an old file; one test covers each of the five statuses; each test calls `t.Parallel()`.
Checked by CI: `go test ./internal/jobs/cutover -race -count=1`, go-arch-lint, golangci-lint.

### 7 bar-import

Module: jobs. Size: M.
Files: `internal/jobs/cutover/barimport.go`, `internal/jobs/cutover/barverify.go`, `internal/jobs/cutover/barimport_test.go`, `internal/jobs/cutover/barverify_test.go`
Design: docs/design/cut-over.md#Decisions (Decisions 3, 4, 6)
Order: after 2, 4, 5 (the interface of entry 2, the package of entry 4, the source value of entry 5). It shares no file with another entry.
Goal: `ImportBars` and `VerifyBars` with the signatures of entry 2.
Acceptance:
1. Every row is built with `Source` `ot_import`, `Basis` `trades` and `AdjustEpoch` 1, as named constants in `barimport.go`; `ImportBars` writes through `InsertPriceBars` and `InsertFxRates` only.
2. A line after `through` refuses the file before the first insert (a named function called above the first insert call).
3. `VerifyBars` never writes: `grep -c 'Insert' internal/jobs/cutover/barverify.go` prints 0; it compares open, high, low, close, volume and the rate as decimal text.
4. No float type and no `strconv.ParseFloat` in the two files.
5. The tests use fakes of `jobs.BarWriter` and `jobs.BarReader` and call `t.Parallel()` in each test.
Checked by CI: `go test ./internal/jobs/cutover -race -count=1`, go-arch-lint, golangci-lint.

### 8 account-import-wire

Module: ot. Size: M.
Files: `cmd/ot/accountimport.go`, `cmd/ot/accountimport_test.go`, `docs/contracts/ot.md`
Design: docs/design/cut-over.md#Decisions (Decisions 3, 4, 6)
Order: after 4, and after `account-store-impl` and `account-wire` of `docs/design/account-data.md` (entries 8 and 10), whose store constructor it uses. It shares `docs/contracts/ot.md` with 9 and 10, which come after it.
Goal: `ot account-import DIR [--map from=to ...] --through DAY [--verify] [--report FILE]`.
Acceptance:
1. `grep -c 'register("account-import"' cmd/ot/accountimport.go` prints 1, and the file imports no `pgx` package.
2. `--through` is required; `--verify` calls `VerifyAccount` and the other branch calls `ImportAccount`, never both: `grep -c 'VerifyAccount' cmd/ot/accountimport.go` and `grep -c 'ImportAccount' cmd/ot/accountimport.go` print at least 1 each, in different branches.
3. The command exits non-zero when the verify result has `Missing` or `Different` above 0 or when an import has `Conflict` above 0, and prints counts only.
4. `--report FILE` writes JSON with the keys `dataset`, `file_sha256`, `counts` and `ok`; no file name is fixed in the source.
5. `docs/contracts/ot.md` has a section for `ot account-import`: its flags, `--through`, and that verify writes nothing.
Checked by CI: `go test ./cmd/ot -race -count=1`, go-arch-lint, golangci-lint.

### 9 bar-import-wire

Module: ot. Size: S.
Files: `cmd/ot/barimport.go`, `cmd/ot/barimport_test.go`, `docs/contracts/ot.md`
Design: docs/design/cut-over.md#Decisions (Decisions 3, 4, 6)
Order: after 7, 8. It shares `docs/contracts/ot.md` with 8 (before it) and 10 (after it).
Goal: `ot bar-import DIR --through DAY [--verify] [--report FILE]`.
Acceptance:
1. `grep -c 'register("bar-import"' cmd/ot/barimport.go` prints 1, and the file imports no `pgx` package and no `internal/store/barpg` other than through the constructor of the bar store that PR 768 wires.
2. The same verify and exit rules as entry 8 (`ImportBars` and `VerifyBars` in different branches; non-zero exit on `Missing`, `Different` or `Conflict`).
3. `--report FILE` writes the same four keys as entry 8.
4. `docs/contracts/ot.md` has a section for `ot bar-import`.
Checked by CI: `go test ./cmd/ot -race -count=1`, go-arch-lint, golangci-lint.

### 10 compare-wire

Module: ot. Size: M.
Files: `cmd/ot/comparefigures.go`, `cmd/ot/comparefigures_test.go`, `docs/contracts/ot.md`
Design: docs/design/cut-over.md#Decisions (Decisions 7, 9)
Order: after 6, 9. It shares `docs/contracts/ot.md` with 8 and 9, which come before it.
Goal: `ot compare-figures --old FILE --register FILE --classes FILE --explained FILE [--report FILE]`.
Acceptance:
1. `grep -c 'register("compare-figures"' cmd/ot/comparefigures.go` prints 1.
2. The command opens the reading store read-only (the reader of `store.ReadingStore` only is passed to `CompareFigures`) and exits non-zero when `FigureReport.OK` is false.
3. It prints one line for each figure whose status is not `checked`: id, version, status and reason, and no value.
4. `docs/contracts/ot.md` documents the command, its flags and its exit codes.
Checked by CI: `go test ./cmd/ot -race -count=1`, go-arch-lint, golangci-lint.

### 11 enforce-web-no-compute

Module: web. Size: S.
Files: `web/test/scripts/noCompute.test.ts`
Design: docs/design/cut-over.md#Decisions (Decision 11)
Order: first. It shares no file with another entry.
Goal: a vitest that fails when `web/no-compute-allow.txt` has a line and when a source file under `web/src/` other than a generated one holds a computing call.
Acceptance:
1. `grep -c 'no-compute-allow' web/test/scripts/noCompute.test.ts` is at least 1.
2. The test scans `web/src/` except `web/src/generated/` for `toFixed`, `toLocale`, `Intl.` and `Math.`: each of the four words occurs in the test file.
3. The test names the offending file and the pattern and no line content, and uses no network and no fixed port.
4. `web/no-compute-allow.txt` and `web/deps-allowlist.txt` are not in the diff.
Checked by CI: `pnpm --dir web run test` (vitest), `tsc`, `eslint` and the build.

### 12 cutover-runbook

Module: docs. Size: M.
Files: `docs/deploy/cut-over.md`, `docs/deploy/cut-over-record.md`, `docs/deploy/cut-over-figures.toml`
Design: docs/design/cut-over.md#Decisions (Decisions 8, 10)
Order: after 9, 10 (it names their flags). It shares no file with another entry.
Goal: the steps of Decision 10 in order, the blank closure record, and the classes file with the table of Decision 8.
Acceptance:
1. `docs/deploy/cut-over.md` lists the steps of Decision 10 as a numbered list in the order given there: `grep -c '^[0-9]*\. ' docs/deploy/cut-over.md` is at least 12.
2. Each command line uses a flag of `docs/contracts/ot.md` (`--map`, `--through`, `--verify`, `--report`, `--old`, `--register`, `--classes`, `--explained`), and the text has no absolute path, no account id and no token.
3. `docs/deploy/cut-over-figures.toml` has `[[class]]` entries whose prefixes are exactly those of Decision 8, `grep -c '^\[\[class\]\]' docs/deploy/cut-over-figures.toml` is at least 14, and no `[[explained]]` entry (the owner adds them).
4. `docs/deploy/cut-over-record.md` has one line for each item of the record in Decision 10, with an empty value.
5. The runbook says that the old serve is not restarted, that the old DuckDB copy is kept until the record is merged, and that live-only figures are checked by the parallel run only.
Checked by CI: none (text only).

## Left out

- A replay tool of our own. Why: `ot backfill --day` (shipped) already runs the jobs of a past day; what it computes is what is registered. Replay of the account, Flex and bar figures waits for the jobs that compute them (`compute-stage2`, `builders-wave`, not yet drafted): until then the compare reports those figures `unchecked` with the reason `not_computed_yet`, never as checked.
- The 44 `pending-recompute` hashes to zero and the test that refuses the word (`internal/compute/rules/rules_test.go`). Why: the rehash is the work of `compute-stage2`; a test that refuses the word before it lands fails CI. Waits for `compute-stage2`; one small compute entry then flips the test.
- The `kind` column of `account.capture`, the `rd.account_*_r2` merge views and a reader change (the first draft). Why: Decision 5, no day holds both an import and a live capture. What brings them back: a decision to import a day that has live captures, or to import on a day after the new scheduler started.
- Execution rows of the old system. Why: the old account store holds snapshots, positions, summaries, orders and prices (`account_snapshot`, `account_position`, `account_summary`, `account_order`, `account_price`) and no table of executions; trade history comes through the Flex rows.
- Earlier snapshots of a day, working orders and fallback marks of the old store. Why: the new datasets hold one capture per run and no orders; the last snapshot of a day is what a page reads. Asked as a one-word question if the owner wants the orders kept.
- Pixel comparison with the v2.2 design images. Why: the owner compares screenshots himself and explains each difference (owner 2026-11-09).
- Importing the old readings as optdesk readings. Why: the new readings come from the new rules; old values are for comparison only and `figure.jsonl` is never written to the database.
- The weekly `adjusted_last` history of the old system. Why: the old system kept no adjust epoch; the new bars job re-pulls it (`docs/design/market-bars.md`); only `trades` bars are imported, as epoch 1.
- Timescale tick tables, the Stooq layer, the 15 s FX stream, the 15:10 premarket capture, the scanner: dropped or deferred by the owner (alignment index, Left out).
- A restart of the old serve and the gap between the last complete day and the first new capture: accepted (plan line 25).
- Not chosen by this design: a new module `cutover` with an Importer and a Target for each dataset (the first draft; window decision 3).

## Adversarial

The read-only attack, kept in the window's scratch, made six points on revision 1.

1. Gap, replay feeds only imported inputs and `ReplayInput` has no subject list. Adopted. Decision 8 splits the register into replayable, live-only and new figures, and the live-only ones are checked by the parallel run alone; `CompareFigures` reads one `LatestReadings` range for each `(name, subject_kind, subject)` of the old file, so the subject list comes from the old file (entry 6 acceptance 3). The week-13 closure no longer waits on `compute-stage2` for anything this design finishes; the register-hash part is in Left out with its dependency.
2. Risk, no stored baseline. Adopted, and checked first in the option-trader source (Decision 2): some figures are stored in `reading`, most page figures are computed by the `viewmodels` builders from stored inputs. The baseline is a parallel run with the old serve's page JSON saved each session day; the exporter reads the register's `pages` paths in it. Not verified: how the `pages` paths resolve in the saved JSON (arrays, nested cards); the exporter counts what it cannot find and the compare says `unchecked`, never zero.
3. Simpler, drop the framework. Adopted: `ot account-import` and `ot bar-import` with a read-only `--verify`, in `internal/jobs/cutover`, no new module; the serial `.go-arch-lint.yml` chain is one entry (4). The replay is `ot backfill` plus `ot compare-figures`.
4. Lock-in, the kind column and merge views. Adopted: dropped (Decision 5), because complete-day imports never share a day with a live capture. Kept: the run key `import` of the capture id (it is the id rule of `store.Capture`, no new schema) and the `ot_import` source of bars (entry 5, one enum value; the other choice was to label imported bars `ibkr`, which would be false).
5. False statements, rehearsal 2 and the stale capture, the register count. Adopted: Decision 4 excludes the export day and unfinal bars, so a later export agrees with an earlier one on every shared day; Decision 6 defines `Different` (stored row of the key with another hash, a stale import capture included) and makes each rehearsal run on a fresh scratch database; the register holds 54 figures, 44 `pending-recompute`, not 53 (counted from `docs/conventions/register.toml`).
6. Slip, acceptance on names that do not exist yet and unshown replay against a scratch database. Adopted in part: every wait is by entry name in the Order cell and in the header; the acceptance items name only files of the entry's own Files line, the shipped `internal/jobs/api.go` and, in entries 2, 5 and 7, the types PR 768 creates, which those entries wait for by name. Not adopted: showing that `ot backfill` runs `compute_figures` for a past day against a scratch database with no live source; it is not shown here (the command refuses a day ahead of the cursor, and what a scratch database's cursor is has not been read). It is named under what is unverified, and the runbook step is a window default.
