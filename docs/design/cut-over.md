# Cut-over: `ot import` and `--verify`, the kind='import' merge, the replay tool, check-every-figure, allowlists to zero, week-13 closure

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Status: draft, not yet a pull request. Builds on main at 7959e72: `store.AccountWriter`, `store.AccountReader`, `store.Capture` and migration `0019_account.sql` (the `account` schema and the `rd.account_*_r1` views), `store.OptionLedgerWriter`, `store.OptionLedgerReader` and `jobs.ImportOptionTraderFunc` (declared; its implementation is in `docs/design/option-ledger.md` entry 10), `store.ReadingReader` (`LatestReadings`), `contracts.Job`, `clock.SessionSource`, `tools/ot-flex-export/`, `docs/conventions/register.toml` (44 entries with `source_hash = "pending-recompute"`), `internal/compute/rules/rules_test.go`, `web/no-compute-allow.txt` (empty), `github.com/pelletier/go-toml/v2` (direct dependency in `go.mod`), and `docs/design/alignment-v25.md` Decision 3 item 9 and the rows "ot import + import --verify", "replay tool", "kind='import' merge" and "stop-the-world import, replay last N sessions, cut-over".
Not on main yet, so some entries wait: the account store implementation `internal/store/account` (`docs/design/account-data.md` entry 8 `account-store-impl`); the bar and rate store `internal/store/bar` and its datasets (`docs/design/market-bars.md`, PR 768, entries 1 and 3); the ledger import and store (`docs/design/option-ledger.md` entries 8, 10, 12); `compute-stage2` (follow-on, not yet drafted), which rehashes the 44 register entries. The plan text is not in the repository; the scope below is read from the alignment index and the gap table of 2026-11-09, and the detail the plan holds that those do not (note 5 on `kind='import'`, the meaning of the two rehearsals) is a window default.
Needs the owner (owner-only paths): none. No entry edits `contracts/proto/**`, `.github/**`, `docs/design/ratchet.json`, `services.toml` or `pyside/src/optdesk/ibkr/transport.py`. `internal/store/api.go` is 799 lines on main and no entry edits it: the cut-over adds its types in the new package `internal/cutover` and reaches the existing store interfaces unchanged.
Source: owner 2026-11-08/09: options trading first and complete; all account data before cut-over (Flex and daily FX included); only the main IBKR account must work; no silent drops; own glue code sits behind an interface; prefer a mainstream library; the web renders data and computes nothing. Plan v2.5 section 11, week 7 (`ot import`, `--verify`, the replay tool, note 5), week 12 (stop-the-world import, replay of the last N sessions, cut-over, check-every-figure) and week 13 (allowlist to zero, enforce mode, runbook).

## Goal

On the day the owner chooses, the old system (option-trader, a DuckDB file) is stopped for good and everything it knows that the new system needs is in the new database, proven by a report; the last N sessions are replayed through the new compute and every figure is compared with the old value; each difference is explained by the owner or fixed; the two lists that let figures pass unchecked (the 44 `pending-recompute` hashes and `web/no-compute-allow.txt`) are zero and CI refuses them from then on; and one runbook says the steps in order. This design stores and reports. It builds no page, no job and no figure.

## Decisions

1. What is imported, from where. A dev-only exporter `tools/ot-history-export/` (its own uv project with a committed `uv.lock`, not run in CI, imported by nothing at run time, as AGENTS.md allows; the mainstream, MIT `duckdb` package, as `tools/ot-flex-export/` does) reads a copy of option-trader's DuckDB file, read-only, and writes one JSON Lines file per dataset into a directory: `account.jsonl` (one line per alias and session day: the last snapshot of that day, with its summary values and positions), `price_bar.jsonl` (daily bars), `fx_rate.jsonl` (daily USD.JPY), `figure.jsonl` (the old figure values, for the replay) and, from the existing `tools/ot-flex-export/`, `ledger.jsonl` (every revision of the Flex trade rows, unchanged). The exporter refuses a table whose columns differ from the ones it was written for, and it refuses a file in which any value of an alias column looks like an IBKR account id (letters then digits): only aliases leave the old system, never an account number.
2. The import uses existing writers and invents none. Account lines go through `store.AccountWriter.WriteCapture` (role `ot_w_account`), ledger lines through `jobs.ImportOptionTraderFunc` and `store.OptionLedgerWriter.ImportFlexTradeRows` (source `ot_import`, the rule of `option-ledger.md` Decision 6), bar and rate lines through the writers of `internal/store/bar` once PR 768 lands. All are insert-only and idempotent, so a second run counts `Same` and writes nothing. Counts only are printed; no row, no account number.
3. Alias map. `ot import DIR [--map from=to ...]`: the default is the same label; when at least one `--map` is given, an alias in a file that no `--map` names is refused, and the whole import is refused before any write (the rule of `option-ledger.md` Decision 7). Both gateways' aliases are imported when the owner maps them; only the main account must be right.
4. One import capture per alias and session day. The old system keeps several snapshots a day; the import keeps the last of each day, with the capture id `<alias>:<session_day>:import` (the id rule of `store.Capture`: `<alias>:<session_day>:<run key>`, run key `import`). Earlier snapshots of a day are not imported (Left out).
5. kind='import' and its merge (window default — owner may change). The table `account.capture` gets a generated column `kind` that is `import` when the capture id ends in `:import` and `live` otherwise; no interface changes, because the id already carries the run key. New views `rd.account_capture_r2`, `rd.account_position_r2`, `rd.account_value_r2` show, for each alias and session day, the live captures when there is one and the import capture only when there is no live capture of that day: live wins, import fills the days before the first live capture. Executions need no kind: their key `(alias, exec_id)` makes an imported and a live row for one execution the same row (the first stays, `ON CONFLICT DO NOTHING`). The reader of `internal/store/account` reads the r2 views. `rd.*_r1` stay as they are, with their grants. Ledger rows already carry `source = ot_import` and bars and rates carry it after entry 7, so only the account tables need a kind.
6. `ot import --verify DIR` (two rehearsals). Verify reads the same files and asks each dataset's reader whether every line is stored with the same content (the hash of the canonical line against the hash of the stored row): it reports `Missing` and `Different` per dataset and exits 0 only when both are zero and no file was skipped. It writes a report file (`--report FILE`: the dataset, the SHA-256 of the input file, the counts, `ok`; no rows). Verify is for the time before the first live capture of the new system: a day that has a live capture hides its import capture by Decision 5. A rehearsal (window default — owner may change) is an import and a verify against a scratch database named by `--env FILE`: rehearsal 1 on an empty scratch database; rehearsal 2 on the same database from an export taken later, with `Conflict` 0 in every dataset and `Inserted` only for the days after the first export. The final import is the same command against the live environment, with its own verify report. Three reports with `ok` are the evidence; the runbook keeps them (Decision 11).
7. The replay tool. `ot replay --sessions N --old DIR/figure.jsonl --env FILE`: for each of the last N session days before today (`clock.SessionSource`, `ot` passes the calendar id), it plans and runs the jobs of the registry that make figures (default `compute_figures`) as `contracts.Job` against the scratch database, then reads every reading back with `store.ReadingReader.LatestReadings` and sets it beside the old value of `figure.jsonl` (name, subject kind, subject, day, value as decimal text). A row is `same`, `different`, `missing_new` (the old value exists, the new reading does not) or `new_only`. `ot` refuses `--env` equal to the live environment file, so a replay never writes into the live database. N is 20 session days (window default — owner may change). Equality is exact on the decimal text after rounding both to the places the old display used, which the exporter writes beside each old value; no tolerance (window default — owner may change). A difference is listed, never hidden.
8. Check-every-figure. `ot check-figures --replay REPORT --register docs/conventions/register.toml --explained docs/deploy/cut-over-figures.toml` reads the register (`github.com/pelletier/go-toml/v2`, already a direct dependency) and, for every `[[figure]]` id and version, sets the replay rows beside it. A figure is `checked` (all its rows `same`), `explained` (it has an entry in the owner's file with a reason and the owner's date), `unexplained` (a `different` or `missing_new` row without an entry) or `unchecked` (no old value, e.g. a new figure of the 12 added; the reason is printed, `new_only`). The command exits 0 only when `unexplained` is 0 and every `unchecked` figure has an `agreed_in` other than `carried over from option-trader` (a figure made in optdesk has no old value to meet). The tool never writes the explanations file; the owner does. It checks values, not pixels: how a page looks against its design is the owner's comparison of screenshots (Left out), and the web renders the stored text and computes nothing (Decision 9).
9. Allowlists to zero and enforce mode (window default — owner may change: these two lists). (a) The 44 `source_hash = "pending-recompute"` entries of the register become real hashes in `compute-stage2` (not here); this design flips the test that accepts the word (`rules_test.go` fails on any entry with it) and waits for that rehash. (b) `web/no-compute-allow.txt` is empty on main but no test reads it; a vitest reads it, fails when it holds a line, and fails when a file under `web/src/` outside `web/src/generated/` holds `toFixed`, `toLocale`, `Intl.` or `Math.`. "Enforce mode" is these two tests failing the CI, nothing more; `web/deps-allowlist.txt` is a dependency list, not a pass list, and stays.
10. Stop-the-world and the runbook (window default — owner may change: the restore drill). The old serve is not restarted and the collection gap between the last export and the first new capture is accepted (alignment index, Left out; plan line 25). The runbook `docs/deploy/cut-over.md` has these steps, each a command or an owner act: back up the new database and restore the backup into a scratch database, comparing row counts (the restore drill the alignment index defers to this design); stop the old collectors (owner); copy the old DuckDB file (a copy, never the live file); run the three exporters; rehearse twice (Decision 6); replay N sessions; check figures; the final import and its verify; start the new scheduler (owner); close (Decision 11). The old serve stays stopped.
11. Week-13 closure. `docs/deploy/cut-over-record.md` is the owner's record: the dates of the two rehearsals and of the final import, the SHA-256 of each of the three verify reports, the replay and check commands' exit codes, and the two greps that show both lists at zero (`grep -c 'pending-recompute' docs/conventions/register.toml` prints 0; `web/no-compute-allow.txt` has no line). The cut-over is closed when the owner fills the record and merges it. The old DuckDB copy is kept until then.
12. Library or own code. The JSON Lines reading, the canonical hash and the comparison by key are own code on the standard library (`encoding/json`, `bufio`, `crypto/sha256`): no mainstream library does an import keyed by dataset with a per-line content hash, and a diff library would not know the keys; own implementation, because of that, and it sits behind `cutover.Importer`, `cutover.Replayer` and `cutover.Checker`, so a library can replace any of them. The register is read with go-toml/v2, a mainstream, MIT library already in `go.mod`. Python uses `duckdb` (MIT), as `ot-flex-export` does. No new direct dependency.
13. Tests. No test touches option-trader's database, a live broker or service data: the exporter's test builds a small DuckDB file in a temporary directory with invented rows; every Go test reads fixture lines with invented aliases and values; the PostgreSQL tests carry the build tag `pg`. Tests run in parallel (`t.Parallel()`, `t.TempDir()`), none sleeps, and time comes from `clock.Clock` with a fixed value. No source file reaches 600 lines.
14. Nothing here goes to Telegram. The cut-over prints counts and statuses to the terminal and to report files only; the margin cushion and the assignment cash appear in no report (the account lines carry summary tags as the old store held them, and the report holds no value).

## Modules

One row added, one row changed, in `docs/design/modules.md` of this pull request (with `arch.toml` regenerated by `go run ./scripts/arch`, which the check in `checks.yml` compares):

| module | paths | entry | tests | depends |
|---|---|---|---|---|
| cutover | internal/cutover/** docs/contracts/cutover.md | internal/cutover/api.go | internal/cutover/api_test.go | contracts clock store |
| ot (changed: `tools/ot-history-export/**` added to the paths, `cutover` to depends) | cmd/** docs/contracts/ot.md tools/ot-flex-export/** tools/ot-history-export/** |  | cmd/ot/main_test.go | contracts clock sources broker store runner jobs notify pages repo compute gaphistory cutover |

The second table gets the row `cutover` (outside the layers, like `ot`: it may import no layer module above the three it lists, and no `jobs`, `pages` or `compute`; jobs reach it as `contracts.Job` values that `ot` passes in). The list of implementation packages `ot` may import gains `internal/cutover/importer`, `internal/cutover/barsink`, `internal/cutover/replay` and `internal/cutover/figcheck`. The new sub-packages are components `cutover/<sub>` that import the root `cutover`, `contracts`, `clock` and `store`; `internal/cutover/barsink` also imports `internal/store/bar`, a declarations-only package of the store row that PR 768 introduces. `docs/deploy/cut-over*.md` and `docs/deploy/cut-over-figures.toml` belong to docs. `.go-arch-lint.yml` is generated from the directories that exist, so each entry that creates a package carries it (the licensed pair with repo) and the five of them are one serial chain, 1, 3, 8, 10, 11.

The paragraph of PR 768 on declarations-only packages, copied from `docs/design/modules.md` of `origin/design/market-bars` verbatim, because `barsink` depends on it:

> A declarations-only package named in an entry cell (today `internal/store/bar`) holds declarations only (one `api.go` and its test, no logic) and is listed so that other modules import an interface without importing its implementation; the implementation (`internal/store/barpg`) is a separate package that is not an entry. It is imported like the root package of its row: the root component and the sub-packages of any row that has its row in the depends cell may import it (jobs and `jobs/*` import `store/bar`). This is the general import rule for declarations-only packages, and this paragraph is its one statement; the `depends` paragraph below refers to it. `scripts/arch` derives the rule from the table plus one list of its own, `declarationsOnly` (today `internal/store/bar`), because the table cannot tell a declarations-only package from the test kit: a root component receives an entry-listed package only when the package is in that list, so `internal/compute/contracttest` and `internal/contracts/genpb` never go to a root component. A sub-package component may also import a declarations-only package named in the entry cell of its own row and on that list (a row whose design adds such a package adds it to its entry cell and to the list in the same pull request). The sub-packages of a dependent row may import every entry-listed package; the generated sub-packages themselves (`contracts/gen`, `contracts/genpb`) import nothing and keep an empty import list. That includes `contracttest`, which a sub-package of jobs, notify or pages may therefore import (`_test.go` files, as above). `scripts/arch` follows neither part yet (a root component receiving a declarations-only package, a sub-package receiving the declarations-only packages of its own row); the interface entry `bar-store-api` of `docs/design/market-bars.md` carries the change (its Files line names `scripts/arch/main.go`).

The wait: entry 8 (`barsink`) imports `internal/store/bar`, which does not exist on main and which `scripts/arch` does not yet let a `cutover` sub-package import. Entry 8 starts only after the entry `bar-store-api` of `docs/design/market-bars.md` has merged, and after entry 7 below. This design does not edit that paragraph or `internal/store/bar`; if PR 768 changes the paragraph, this copy is brought in line when it merges.

## Interface

- cutover, `internal/cutover/api.go` (entry 1, declarations only):
  - `type Dataset string` with constants `DatasetAccount` ("account"), `DatasetPriceBar` ("price_bar"), `DatasetFxRate` ("fx_rate"), `DatasetLedger` ("ledger"), `DatasetFigure` ("figure"), and the file names `FileAccount` ("account.jsonl") and the like, one per dataset.
  - `type Line struct { Dataset Dataset; Number int; Key string; SHA256 string; Raw []byte }`: one parsed line (after the alias map), its stable key, the SHA-256 of its canonical text and its text.
  - `type Counts struct { Lines, Inserted, Same, Conflict int }`; `type Mismatch struct { Missing, Different int }`.
  - `type Target interface { Dataset() Dataset; Put(ctx context.Context, lines []Line, now time.Time) (Counts, error); Check(ctx context.Context, lines []Line) (Mismatch, error) }`: the side of one dataset in the store; `Put` is idempotent.
  - `type DatasetReport struct { Dataset Dataset; File string; FileSHA256 string; Counts Counts; Mismatch Mismatch; Skipped bool }`; `type Report struct { Mode string; Datasets []DatasetReport; OK bool }` (Mode `import` or `verify`; no row and no value).
  - `type Importer interface { Import(ctx context.Context, dir fs.FS, aliasMap map[string]string, now time.Time) (Report, error); Verify(ctx context.Context, dir fs.FS, aliasMap map[string]string) (Report, error) }` and `type NewImporterFunc func(targets []Target) (Importer, error)` (`internal/cutover/importer.New`).
  - `type FigureRow struct { Name, SubjectKind, Subject string; Day contracts.Day; Old, New *string; Status string }` (Status `same`, `different`, `missing_new`, `new_only`); `type ReplayInput struct { Jobs []contracts.Job; Sessions clock.SessionSource; CalendarID string; Through contracts.Day; N int; Old io.Reader; Readings store.ReadingReader }`; `type ReplayReport struct { Days []contracts.Day; Rows []FigureRow }`; `type Replayer interface { Replay(ctx context.Context, in ReplayInput) (ReplayReport, error) }`; `type NewReplayerFunc func() Replayer` (`internal/cutover/replay.New`).
  - `type FigureResult struct { ID string; Version int; Status string; Reason string }` (Status `checked`, `explained`, `unexplained`, `unchecked`); `type FigureCheck struct { Results []FigureResult; OK bool }`; `type Checker interface { Check(ctx context.Context, register, explained io.Reader, replay ReplayReport) (FigureCheck, error) }`; `type NewCheckerFunc func() Checker` (`internal/cutover/figcheck.New`).
- importer (entries 3, 4): `importer.New(targets []cutover.Target) (cutover.Importer, error)`; `importer.NewAccountTarget(store.AccountWriter, store.AccountReader) cutover.Target`; `type LedgerImportFunc func(ctx context.Context, r io.Reader, w store.OptionLedgerWriter, aliasMap map[string]string, now time.Time) (LedgerCounts, error)` (the shape of `jobs.ImportOptionTraderFunc`, which `ot` passes, because `cutover` may not import `jobs`) and `importer.NewLedgerTarget(LedgerImportFunc, store.OptionLedgerWriter, store.OptionLedgerReader) cutover.Target`.
- barsink (entry 8): `barsink.NewPriceBarTarget(bar.PriceBarWriter, bar.PriceBarReader) cutover.Target`, `barsink.NewFxRateTarget(bar.FxRateWriter, bar.FxRateReader) cutover.Target`.
- contracts (entry 7): `source` of `obs.price_bar` r1 and `obs.fx_rate` r1 also accepts `ot_import`; generated `contracts.PriceBar` and `contracts.FxRate` regenerate.
- store (entries 5, 6): migration `NNNN_account_import_kind.sql` (generated column `account.capture.kind`, views `rd.account_capture_r2`, `rd.account_position_r2`, `rd.account_value_r2`, grants as the r1 views); no Go interface changes.
- web (entry 14) and compute (entry 13): tests only.
- ot (entries 9, 12): subcommands `import`, `replay`, `check-figures`, each a file that registers itself with `register(...)` as `backfill` does; `docs/contracts/ot.md` documents their flags.

## Issues

| # | issue | module | order |
|---|---|---|---|
| 1 | cutover-api (interface) | cutover, plus repo (licensed pair) | first |
| 2 | history-exporter | ot | after 1 |
| 3 | import-core | cutover, plus repo (licensed pair) | after 1 |
| 4 | import-account-ledger-targets | cutover | after 1, 3 |
| 5 | account-import-kind-migration | store | first |
| 6 | account-merge-read | store | after 5 and `account-store-impl` |
| 7 | bar-source-import (interface) | contracts | after `bar-datasets` (PR 768) |
| 8 | import-bar-fx-targets | cutover, plus repo (licensed pair) | after 3, 7, `bar-store-api` |
| 9 | import-wire | ot | after 3, 4, 6, 8 |
| 10 | replay-tool | cutover, plus repo (licensed pair) | after 1, 8 |
| 11 | figure-check | cutover, plus repo (licensed pair) | after 10 |
| 12 | replay-wire | ot | after 9, 10, 11 |
| 13 | enforce-register-hash | compute | after `compute-stage2` rehash |
| 14 | enforce-web-no-compute | web | first |
| 15 | cutover-runbook | docs | after 9, 12 |

### 1 cutover-api (interface)

Module: cutover, plus repo (licensed pair: `.go-arch-lint.yml` is generated from the directory). Size: M.
Files: `internal/cutover/api.go`, `internal/cutover/api_test.go`, `docs/contracts/cutover.md`, `.go-arch-lint.yml`
Design: docs/design/cut-over.md#Interface
Order: first. It shares `.go-arch-lint.yml` with 3, 8, 10 and 11, which come after it in that chain (1, 3, 8, 10, 11).
Goal: the declarations of the Interface section, one file, no logic, with the contract test from both sides and the contract document that also names the line format of each export file.
Acceptance:
1. `internal/cutover/api.go` declares the types `Dataset`, `Line`, `Counts`, `Mismatch`, `Target`, `DatasetReport`, `Report`, `Importer`, `FigureRow`, `ReplayInput`, `ReplayReport`, `Replayer`, `FigureResult`, `FigureCheck`, `Checker` and the three `New...Func` types: `grep -cE '^type (Dataset|Line|Counts|Mismatch|Target|DatasetReport|Report|Importer|FigureRow|ReplayInput|ReplayReport|Replayer|FigureResult|FigureCheck|Checker|NewImporterFunc|NewReplayerFunc|NewCheckerFunc) ' internal/cutover/api.go` prints 18.
2. `internal/cutover/api.go` has no function body: `grep -c '^func ' internal/cutover/api.go` prints 0; it imports only the standard library, `internal/clock`, `internal/contracts` and `internal/store`.
3. `docs/contracts/cutover.md` names the five file names `account.jsonl`, `price_bar.jsonl`, `fx_rate.jsonl`, `ledger.jsonl` and `figure.jsonl`, and for each the key of a line and the fields of `account.jsonl` (`alias`, `session_day`, `captured_at`, `values`, `positions`); `grep -c 'jsonl' docs/contracts/cutover.md` is at least 5.
4. `internal/cutover/api_test.go` has a fake of `Target` and of `Importer` and a caller that uses them (the contract from both sides), and `grep -c 't.Parallel()' internal/cutover/api_test.go` is at least 2.
5. `.go-arch-lint.yml` has a component `cutover` whose `in` is `internal/cutover`: `grep -c 'internal/cutover$' .go-arch-lint.yml` prints at least 1.
Checked by CI: `go test ./internal/cutover -race -count=1`, `go run ./scripts/arch --check`, golangci-lint, go-arch-lint.

### 2 history-exporter

Module: ot. Size: L.
Files: `tools/ot-history-export/pyproject.toml`, `tools/ot-history-export/uv.lock`, `tools/ot-history-export/README.md`, `tools/ot-history-export/src/ot_history_export/__init__.py`, `tools/ot-history-export/src/ot_history_export/__main__.py`, `tools/ot-history-export/src/ot_history_export/accounts.py`, `tools/ot-history-export/src/ot_history_export/bars.py`, `tools/ot-history-export/src/ot_history_export/figures.py`, `tools/ot-history-export/tests/test_export.py`
Design: docs/design/cut-over.md#Decisions (Decisions 1, 4, 12, 13)
Order: after 1 (the line formats are in `docs/contracts/cutover.md`). It shares no file with another entry.
Goal: `python -m ot_history_export COPY.duckdb OUTDIR` writes `account.jsonl`, `price_bar.jsonl`, `fx_rate.jsonl` and `figure.jsonl` as `docs/contracts/cutover.md` gives them, read-only, printing counts only.
Acceptance:
1. `tools/ot-history-export/pyproject.toml` names `duckdb` as the one runtime dependency and the project is its own uv project: `grep -c 'duckdb' tools/ot-history-export/pyproject.toml` is at least 1, `tools/ot-history-export/uv.lock` is in the diff, and no file outside `tools/ot-history-export/` imports `ot_history_export`.
2. `accounts.py` opens the file with `read_only=True` and keeps the last snapshot of each alias and session day (a `max(finished_at)` or equivalent per day), and writes the capture of the line with the alias only: `grep -n 'read_only=True' tools/ot-history-export/src/ot_history_export/*.py` finds a line, and no column named `account` of an IBKR id is written (the exporter refuses a value matching letters then digits, in a function that the test calls).
3. Each exporter refuses a table whose columns differ from the expected ones, with a message naming the table and no row: `grep -c 'refuse\|Refuse' tools/ot-history-export/src/ot_history_export/accounts.py tools/ot-history-export/src/ot_history_export/bars.py tools/ot-history-export/src/ot_history_export/figures.py` is at least 1 for each file.
4. `figures.py` writes with each old value the number of decimal places the old display used (the field `places`), and a missing old value is not written as zero.
5. `tools/ot-history-export/tests/test_export.py` builds its DuckDB file in `tmp_path` with invented aliases and values, opens no other file and has no `sleep`: `grep -c 'tmp_path' tools/ot-history-export/tests/test_export.py` is at least 1, `grep -c 'sleep' tools/ot-history-export/tests/test_export.py` prints 0.
6. `tools/ot-history-export/README.md` says to use a copy of the file, never the live one, and has no absolute path and no account id.
Checked by CI: none (a dev-only tool is not run in CI, AGENTS.md); the owner runs `uv run --project tools/ot-history-export pytest` by hand.

### 3 import-core

Module: cutover, plus repo (licensed pair: `.go-arch-lint.yml`). Size: L.
Files: `internal/cutover/importer/importer.go`, `internal/cutover/importer/jsonl.go`, `internal/cutover/importer/alias.go`, `internal/cutover/importer/verify.go`, `internal/cutover/importer/importer_test.go`, `internal/cutover/importer/jsonl_test.go`, `internal/cutover/importer/alias_test.go`, `internal/cutover/importer/verify_test.go`, `.go-arch-lint.yml`
Design: docs/design/cut-over.md#Decisions (Decisions 2, 3, 6, 12)
Order: after 1. It shares `.go-arch-lint.yml` with 1, 8, 10 and 11 (the chain 1, 3, 8, 10, 11); 4 adds files to the same package after it.
Goal: `importer.New(targets)` returns a `cutover.Importer` that reads the five files of a directory in the order account, ledger, price_bar, fx_rate, applies the alias map, hands lines to the target of each dataset, and builds the report; `Verify` writes nothing.
Acceptance:
1. `importer.New` rejects two targets for one dataset and a target whose `Dataset()` is `figure`: `grep -rn 'DatasetFigure' internal/cutover/importer/importer.go` finds the check, and the package has no import of `internal/jobs`, `internal/pages` or `internal/compute`.
2. The alias map is applied to the `alias` field of a line before it reaches a target, and, when the map is not empty, an alias that the map does not name refuses the whole import before any `Put`: the refusal is in `alias.go` and `importer.go` calls it before the first `Put` call (`grep -n 'refus' internal/cutover/importer/alias.go` finds it).
3. `Verify` calls `Check` only: `grep -c '\.Put(' internal/cutover/importer/verify.go` prints 0, and the report's `OK` is true only when every dataset has `Missing` 0, `Different` 0 and `Skipped` false.
4. A file that is absent gives `Skipped: true` and is named in the report; a line that is not valid JSON is an error that names the file and the line number and no content: `grep -n 'Number' internal/cutover/importer/jsonl.go` finds the line number carried into the error.
5. The canonical hash is the SHA-256 over the line with keys sorted and no space, in `jsonl.go`; `crypto/sha256` is the only hash import and no float type appears in the package: `grep -rn 'float64\|float32' internal/cutover/importer/*.go` prints nothing.
6. Every test file calls `t.Parallel()` in each test, uses `t.TempDir()` or an in-memory `fstest.MapFS` for files, and no test file contains `Sleep` or a host and port.
Checked by CI: `go test ./internal/cutover/... -race -count=1`, go-arch-lint, golangci-lint.

### 4 import-account-ledger-targets

Module: cutover. Size: M.
Files: `internal/cutover/importer/account.go`, `internal/cutover/importer/ledger.go`, `internal/cutover/importer/account_test.go`, `internal/cutover/importer/ledger_test.go`
Design: docs/design/cut-over.md#Decisions (Decisions 2, 4, 5)
Order: after 1, 3 (the same package; it adds files only).
Goal: `NewAccountTarget` and `NewLedgerTarget` as the Interface section gives them.
Acceptance:
1. `account.go` builds a `store.Capture` with `ID` equal to alias, session day and the text `import` joined by colons (the id rule of `store.Capture`) and calls `WriteCapture` once per line: `grep -n '":import"\|"import"' internal/cutover/importer/account.go` finds the run key, and `grep -c 'WriteCapture' internal/cutover/importer/account.go` is at least 1.
2. `Check` of the account target reads `Positions` and `Values` of the alias and day and compares the canonical hash of the rebuilt line with the line's: `grep -c 'Positions(\|Values(' internal/cutover/importer/account.go` is at least 2.
3. `ledger.go` does not import `internal/jobs`; it calls the `LedgerImportFunc` it was given, and `Check` reads `FlexTradeRows` and `StoredAliases` of the `store.OptionLedgerReader` and treats a line as present when the stored row of its key has the line's `attributes_sha256` or a higher revision: `grep -c 'FlexTradeRows' internal/cutover/importer/ledger.go` is at least 1.
4. Neither file writes an account number, a token or a value to an error or a log line: `grep -n 'slog\|fmt.Errorf' internal/cutover/importer/account.go` shows messages that name the alias, the day and a count only.
5. The tests use fakes of `store.AccountStore` and `store.OptionLedgerStore` defined in the test files with invented aliases, and `grep -c 't.Parallel()' internal/cutover/importer/account_test.go internal/cutover/importer/ledger_test.go` is at least 1 for each file.
Checked by CI: `go test ./internal/cutover/... -race -count=1`, go-arch-lint, golangci-lint.

### 5 account-import-kind-migration

Module: store. Size: S.
Files: `migrations/NNNN_account_import_kind.sql` (NNNN the next free number at cut time)
Design: docs/design/cut-over.md#Decisions (Decision 5)
Order: first. No other entry edits a migration of the account schema; `account-store-impl` of `docs/design/account-data.md` does not edit this file.
Goal: the generated column and the three merge views of Decision 5, with their grants and a Down that drops them.
Acceptance:
1. The file adds the column `kind` to `account.capture` as `GENERATED ALWAYS AS` an expression that is `'import'` when `capture_id LIKE '%:import'` and `'live'` otherwise: `grep -n 'GENERATED ALWAYS AS' migrations/*_account_import_kind.sql` finds it.
2. The file creates `rd.account_capture_r2`, `rd.account_position_r2` and `rd.account_value_r2`, each hiding an import capture of an alias and session day that has a live capture: `grep -c 'CREATE VIEW rd.account_.*_r2' migrations/*_account_import_kind.sql` prints 3, and the text of each view names `kind = 'live'` or `kind = 'import'`.
3. The views are owned by `ot_owner` and granted to `ot_w_account` and `ot_ro_pages` as the r1 views are in `migrations/0019_account.sql`: `grep -c 'GRANT SELECT ON rd.account_.*_r2' migrations/*_account_import_kind.sql` is at least 2 per role.
4. `rd.account_*_r1` and the table grants are untouched: the file contains no `DROP VIEW rd.account_capture_r1` and no `ALTER TABLE account.position`.
5. The `+goose Down` section revokes the grants, drops the three views and then the column, in that order.
Checked by CI: the migration tests of `internal/store/migrate` and the `pg` tests in GitHub CI.

### 6 account-merge-read

Module: store. Size: S.
Files: `internal/store/account/read.go`, `internal/store/account/merge_pg_test.go`
Design: docs/design/cut-over.md#Decisions (Decision 5)
Order: after 5 and after `account-store-impl` of `docs/design/account-data.md` (entry 8), which creates `internal/store/account/read.go`. It shares `read.go` with that entry, which lands first.
Goal: the Reader of the account store reads the r2 views, so a day with a live capture shows the live rows and a day without shows the import rows.
Acceptance:
1. `read.go` selects from `rd.account_position_r2` and `rd.account_value_r2` and no longer from the r1 views: `grep -c 'account_position_r2\|account_value_r2' internal/store/account/read.go` is at least 2 and `grep -c 'account_position_r1\|account_value_r1' internal/store/account/read.go` prints 0.
2. `Executions` still reads `rd.account_execution_r1` (an execution has no kind).
3. `merge_pg_test.go` carries the build tag `pg` (`grep -c '//go:build pg' internal/store/account/merge_pg_test.go` prints 1) and has a test for each of: import only, live only, both on one day (live shown), import day before the first live day (import shown).
4. `internal/store/api.go` is not in the diff.
Checked by CI: `go test -tags pg ./internal/store/account -race -count=1` in GitHub CI, go-arch-lint.

### 7 bar-source-import (interface)

Module: contracts. Size: M.
Files: `contracts/datasets/obs.price_bar.r1.json`, `contracts/datasets/obs.fx_rate.r1.json`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/obs.price_bar.md`, `docs/contracts/datasets/obs.fx_rate.md`
Design: docs/design/cut-over.md#Interface
Order: after `bar-datasets` (entry 1 of `docs/design/market-bars.md`, PR 768), which creates these files (names as that design gives them; re-checked when it lands). It shares them and `records.go` with that entry, which lands first.
Goal: the `source` of both datasets also accepts `ot_import`, with the generated Go record and the dataset documents committed together.
Acceptance:
1. `grep -c 'ot_import' contracts/datasets/obs.price_bar.r1.json contracts/datasets/obs.fx_rate.r1.json` prints at least 1 for each file, and the value `ibkr` and `ibkr_midpoint` are still allowed.
2. `bash contracts/generate.sh && git diff --exit-code internal/contracts/gen` leaves no change (the generated record and the schemas agree).
3. `docs/contracts/datasets/obs.price_bar.md` and `docs/contracts/datasets/obs.fx_rate.md` each say that an imported row has source `ot_import` and when it is written (only by `ot import`).
4. No file other than the five of the Files line is in the diff (`git diff --name-only $(git merge-base HEAD origin/main)` lists only them).
Checked by CI: `go test ./internal/contracts -race -count=1`, the generate check of `checks.yml`; the pull request body carries `Contracts: changed`.

### 8 import-bar-fx-targets

Module: cutover, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/cutover/barsink/bar.go`, `internal/cutover/barsink/fx.go`, `internal/cutover/barsink/bar_test.go`, `internal/cutover/barsink/fx_test.go`, `.go-arch-lint.yml`
Design: docs/design/cut-over.md#Modules (the wait on PR 768) and #Decisions (Decision 2)
Order: after 3 (the chain 1, 3, 8, 10, 11 on `.go-arch-lint.yml`), after 7, and after the entry `bar-store-api` of `docs/design/market-bars.md`, which creates `internal/store/bar` and teaches `scripts/arch` the import rule of the paragraph copied above.
Goal: `NewPriceBarTarget` and `NewFxRateTarget` as the Interface section gives them: lines become `contracts.PriceBar` and `contracts.FxRate` with source `ot_import`, written through `InsertPriceBars` and `InsertFxRates`.
Acceptance:
1. `bar.go` sets `Source` to `ot_import` and `Basis` to `trades` and `AdjustEpoch` to 1 on every row it builds: `grep -c 'ot_import' internal/cutover/barsink/bar.go internal/cutover/barsink/fx.go` is at least 1 for each file; the basis and the epoch are constants named in the file.
2. `Put` returns `Inserted` from `InsertPriceBars` and `Same` as lines minus inserted; it never updates a row (the interface offers no update): the package calls no method other than `InsertPriceBars`, `InsertFxRates`, `PriceBars`, `FxRates`.
3. `Check` reads `PriceBars` and `FxRates` for the range of the lines and compares open, high, low, close, volume and the rate as decimal text, not as numbers: `grep -rn 'float64\|strconv.ParseFloat' internal/cutover/barsink` prints nothing.
4. The package imports `internal/store/bar`, `internal/contracts` and `internal/cutover` only (the standard library aside), and no `internal/store/barpg`: `grep -rn 'store/barpg' internal/cutover` prints nothing.
5. Tests use fakes of the bar writers and readers and `t.Parallel()` in each test.
Checked by CI: `go test ./internal/cutover/... -race -count=1`, go-arch-lint (after the rule of PR 768), golangci-lint.

### 9 import-wire

Module: ot. Size: M.
Files: `cmd/ot/import.go`, `cmd/ot/import_test.go`, `docs/contracts/ot.md`
Design: docs/design/cut-over.md#Decisions (Decisions 2, 3, 6)
Order: after 3, 4, 6, 8, and after the account wire and the ledger wire of `docs/design/account-data.md` (entry 10) and `docs/design/option-ledger.md` (entry 12), whose constructors it uses. It shares `docs/contracts/ot.md` with 12, which comes after it.
Goal: `ot import DIR [--map from=to ...] [--env FILE] [--verify] [--report FILE]` registers itself (`register("import", ...)`) and builds the importer from the account, ledger, price bar and rate stores.
Acceptance:
1. `grep -c 'register("import"' cmd/ot/import.go` prints 1, and `cmd/ot/import.go` imports no `pgx` package (the stores are opened through the `Open...Func` constructors of `internal/store`).
2. `--verify` calls `Verify` and never `Import`: in `import.go` the two calls are in different branches (`grep -c '\.Verify(' cmd/ot/import.go` is 1 and `grep -c '\.Import(' cmd/ot/import.go` is 1).
3. The ledger target receives `jobs.ImportOptionTraderFunc` as its `LedgerImportFunc`: `grep -n 'ImportOptionTrader' cmd/ot/import.go` finds one use.
4. The command exits non-zero when `Report.OK` is false and prints counts and the dataset names only: `grep -n 'Printf\|Fprintf' cmd/ot/import.go` shows format strings with `%d` and dataset names, no `%s` of a line's content.
5. `--report FILE` writes the report as JSON with the keys `mode`, `datasets` and `ok`, and the file name is taken from the flag, never fixed: `grep -c '"/' cmd/ot/import.go` prints 0.
6. `docs/contracts/ot.md` has a section for `ot import` with its flags, the file names of the directory and the three-report evidence of Decision 6.
Checked by CI: `go test ./cmd/ot -race -count=1`, go-arch-lint, golangci-lint.

### 10 replay-tool

Module: cutover, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/cutover/replay/replay.go`, `internal/cutover/replay/old.go`, `internal/cutover/replay/diff.go`, `internal/cutover/replay/replay_test.go`, `internal/cutover/replay/diff_test.go`, `.go-arch-lint.yml`
Design: docs/design/cut-over.md#Decisions (Decision 7)
Order: after 1, 8 (the chain 1, 3, 8, 10, 11 on `.go-arch-lint.yml`).
Goal: `replay.New()` returns a `cutover.Replayer` that, for the last N session days before `Through`, plans and runs each job, reads the readings back and builds the rows of Decision 7.
Acceptance:
1. The session days are the last `N` of `clock.SessionSource.SessionDays` up to `Through` (not including it); no call of `time.Now`, `time.Sleep` or `time.After` appears in the package: `grep -rn 'time\.Now\|time\.Sleep\|time\.After' internal/cutover/replay` prints nothing.
2. For each day the package calls `Plan` and then `Run` of each `contracts.Job` with the planned keys, and a day on which a job returns a failed or partial result is named in `ReplayReport.Days` with the job id (the type of `JobResult` is the one of `internal/contracts`): `grep -c '\.Plan(\|\.Run(' internal/cutover/replay/replay.go` is at least 2.
3. Rows are `same` only on equal decimal text after both sides are rounded to `places` of the old line, half-even, with `math/big`; no `float` type appears: `grep -rn 'float64\|float32' internal/cutover/replay` prints nothing.
4. A reading of the new system with no old line is a row `new_only`; an old line with no reading is `missing_new`; the counts of the four statuses are the sum of `Rows` (checked by a test that builds rows and counts them).
5. The package imports no `internal/jobs`, `internal/compute`, `internal/pages` or `internal/store/*` sub-package, and writes nothing itself: the jobs write through the stores `ot` gave them.
Checked by CI: `go test ./internal/cutover/... -race -count=1`, go-arch-lint, golangci-lint.

### 11 figure-check

Module: cutover, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/cutover/figcheck/register.go`, `internal/cutover/figcheck/check.go`, `internal/cutover/figcheck/explained.go`, `internal/cutover/figcheck/check_test.go`, `internal/cutover/figcheck/testdata/`, `.go-arch-lint.yml`
Design: docs/design/cut-over.md#Decisions (Decision 8)
Order: after 10 (the end of the chain 1, 3, 8, 10, 11 on `.go-arch-lint.yml`).
Goal: `figcheck.New()` returns a `cutover.Checker` with the four statuses of Decision 8 for every `[[figure]]` of the register.
Acceptance:
1. `register.go` reads the register with `github.com/pelletier/go-toml/v2` and takes `id`, `version` and `agreed_in` of each `[[figure]]`: `grep -c 'go-toml/v2' internal/cutover/figcheck/register.go` is 1.
2. `Check` returns `OK` true only when no figure is `unexplained` and every `unchecked` figure has an `agreed_in` other than `carried over from option-trader` (the rule is in `check.go` under a name that says so): `grep -n 'OK' internal/cutover/figcheck/check.go` shows the one place it is set.
3. `explained.go` reads the owner's file as TOML with entries `id`, `reason`, `date` and refuses an entry with an empty reason; the package never opens a file for writing: `grep -rn 'os.Create\|os.OpenFile\|WriteFile' internal/cutover/figcheck` prints nothing.
4. `testdata/` holds a register of three invented figures and an explained file; the test covers `checked`, `explained`, `unexplained` and `unchecked` once each.
5. The package imports the standard library, go-toml/v2, `internal/contracts` and `internal/cutover` only.
Checked by CI: `go test ./internal/cutover/... -race -count=1`, go-arch-lint, golangci-lint.

### 12 replay-wire

Module: ot. Size: M.
Files: `cmd/ot/replay.go`, `cmd/ot/replay_test.go`, `cmd/ot/checkfigures.go`, `cmd/ot/checkfigures_test.go`, `docs/contracts/ot.md`
Design: docs/design/cut-over.md#Decisions (Decisions 7, 8)
Order: after 9, 10, 11. It shares `docs/contracts/ot.md` with 9, which comes before it.
Goal: `ot replay --sessions N --old FILE --env FILE [--report FILE]` and `ot check-figures --replay FILE --register FILE --explained FILE` register themselves and call the replayer and the checker.
Acceptance:
1. `grep -c 'register("replay"' cmd/ot/replay.go` and `grep -c 'register("check-figures"' cmd/ot/checkfigures.go` print 1 each.
2. `replay.go` refuses an `--env` equal to the live environment file before it opens any store: the refusal is a named function and its call is above the first store constructor in the file (`grep -n 'refuseLive' cmd/ot/replay.go` finds the definition and the call).
3. `--sessions` defaults to 20 as a named constant: `grep -n 'defaultReplaySessions *= *20' cmd/ot/replay.go` finds one line.
4. `check-figures` exits non-zero when `FigureCheck.OK` is false and prints one line per figure that is not `checked`, with id, version, status and reason; it prints no value.
5. `docs/contracts/ot.md` documents the two commands, their flags and the exit codes.
6. The jobs given to the replayer are built from the registry by id with the scratch stores, never the live ones: `replay.go` builds its stores from the `--env` file only (`grep -c 'OT_PG_DSN' cmd/ot/replay.go` prints 0, the line names come through the existing env loader).
Checked by CI: `go test ./cmd/ot -race -count=1`, go-arch-lint, golangci-lint.

### 13 enforce-register-hash

Module: compute. Size: S.
Files: `internal/compute/rules/rules_test.go`
Design: docs/design/cut-over.md#Decisions (Decision 9a)
Order: after `compute-stage2` (follow-on, not yet drafted), which rehashes the 44 entries; this entry must not land before the register holds no `pending-recompute`. It shares no file with another entry of this design.
Goal: the test that compares the register with the rules fails for any entry whose `source_hash` is `pending-recompute`.
Acceptance:
1. `grep -n 'pending-recompute' internal/compute/rules/rules_test.go` finds only a failure message or a comparison that reports an error, no branch that accepts the word (the branch of the current line that says `else if f.SourceHash != "pending-recompute"` is gone).
2. `grep -c 'pending-recompute' docs/conventions/register.toml` prints 0 (the rehash of `compute-stage2` is in the base).
3. The test still asserts that each register entry has a rule with the same id and version (`TestRulesMatchRegister` keeps its name and body for that part).
Checked by CI: `go test ./internal/compute/rules -race -count=1`.

### 14 enforce-web-no-compute

Module: web. Size: S.
Files: `web/test/scripts/noCompute.test.ts`
Design: docs/design/cut-over.md#Decisions (Decision 9b)
Order: first. It shares no file with another entry.
Goal: a vitest that fails when `web/no-compute-allow.txt` has a line, and when a source file under `web/src/` other than generated ones holds a computing call.
Acceptance:
1. The test reads `web/no-compute-allow.txt` and expects no non-empty line: `grep -c 'no-compute-allow' web/test/scripts/noCompute.test.ts` is at least 1.
2. The test scans the files under `web/src/` except `web/src/generated/` for `toFixed`, `toLocale`, `Intl.` and `Math.`: `grep -c 'toFixed' web/test/scripts/noCompute.test.ts` is at least 1, and so for the other three patterns.
3. The test names the offending file and the pattern and no line content, and uses no network and no fixed port.
4. `web/no-compute-allow.txt` and `web/deps-allowlist.txt` are not in the diff.
Checked by CI: `pnpm --dir web run test` (vitest), `tsc`, `eslint` and the build.

### 15 cutover-runbook

Module: docs. Size: M.
Files: `docs/deploy/cut-over.md`, `docs/deploy/cut-over-record.md`, `docs/deploy/cut-over-figures.toml`
Design: docs/design/cut-over.md#Decisions (Decisions 10, 11)
Order: after 9, 12 (it names their flags). It shares no file with another entry.
Goal: the steps of Decision 10 in order, the closure record of Decision 11 as a blank form the owner fills, and the owner's explanation file, empty.
Acceptance:
1. `docs/deploy/cut-over.md` has the steps of Decision 10 as a numbered list in this order: backup and restore drill, stop the old collectors, copy the DuckDB file, run the exporters, rehearsal 1, rehearsal 2, replay, check figures, final import and verify, start the new scheduler, close; `grep -c '^[0-9]*\. ' docs/deploy/cut-over.md` is at least 11.
2. Each command line in it uses a flag that exists in `docs/contracts/ot.md` (`--map`, `--env`, `--verify`, `--report`, `--sessions`, `--old`, `--replay`, `--register`, `--explained`) and no absolute path (no word that starts with a slash and a directory name), no account id, no token.
3. `docs/deploy/cut-over-record.md` has one line for each item of Decision 11 (two rehearsal dates, the final import date, three report hashes, the two exit codes, the two greps) with an empty value.
4. `docs/deploy/cut-over-figures.toml` holds comments that name the keys `id`, `reason`, `date` and no `[[explained]]` entry: `grep -c '^\[\[' docs/deploy/cut-over-figures.toml` prints 0.
5. The runbook says that the old serve is not restarted and that the old DuckDB copy is kept until the record is merged.
Checked by CI: none (text only).

## Left out

- Execution rows of the old system. The old account store holds snapshots, positions, summaries, orders and prices (`account_snapshot`, `account_position`, `account_summary`, `account_order`, `account_price`), and no table of executions; trade history comes through the Flex rows (`ledger.jsonl`). Why: nothing to import. `store.AccountWriter.WriteExecutions` is not called by the cut-over.
- Earlier snapshots of a day, working orders (`account_order`) and the fallback marks of the old account store. Why: the new datasets hold one capture per run and no orders; the day's last snapshot is what a page reads (Decision 4). Asked as a one-word question if the owner wants the orders kept.
- Pixel comparison of the pages with the v2.2 design images and of the rendered web with the old web. Why: the owner compares the screenshots himself and explains each difference (owner 2026-11-09); the tool compares values (Decision 8).
- Importing the old readings as readings of the new system. Why: the new readings come from the new rules; the old values are the thing to compare with, not to store. `figure.jsonl` is read by the replay and never written to the database.
- The weekly `adjusted_last` history of the old system. Why: the old system kept no adjust epoch; the new bars job re-pulls it from IBKR (`docs/design/market-bars.md`). Only `trades` bars are imported, as epoch 1.
- Timescale tick tables, the Stooq layer, the 15 s FX stream, the 15:10 premarket capture, the scanner: dropped or deferred by the owner and not in the old store this design reads (alignment index, Left out).
- A restart of the old serve and the gap between the last export and the first new capture: accepted (plan line 25). The restore drill is a runbook step (Decision 10), not a tool.
- Verify after the new scheduler's first capture. Why: by Decision 5 a day that has a live capture hides its import capture, so verify is defined for the time before it; a day-wise verify with the r1 views is a later design if the owner wants it.
- Raising a ratchet ceiling or splitting a large file: none is needed; `internal/store/api.go` is untouched.
- Not chosen by this design: one import command for each dataset (one `ot import DIR` with a report per dataset is smaller and the verify reads the same directory); putting `kind` into the `Capture` type (an edit of the 799-line `internal/store/api.go` and a new field for what the id already says).
