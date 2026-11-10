# Jobs interface slimmed: `internal/jobs/api.go` keeps only the types that two packages share

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: the window's measurement of the issue queue (16 of the last 150 issues touched `internal/jobs/api.go`; 18 of its last 20 commits were one job's own constructor or stores type, each an interface change that the next job issue had to wait for). `docs/design/modules.md` (jobs row, the entry-cell paragraph, the jobs row of the table of what a module may import), `docs/design/account-data.md#Decisions` (Decision 8 and the account-wire entry: the `accountcapture` constructor lives in `internal/jobs/accountcapture/job.go`, "so no interface issue is added"), `docs/design/parallel-hotspots.md` (the tree, not the cells, says what `cmd/ot` may import).

Read on origin/main at 9737c61: `internal/jobs/api.go` (334 lines), `internal/jobs/*_test.go`, `internal/jobs/*/` (15 job packages besides `accountcapture`), `cmd/ot/*.go`, `docs/contracts/jobs.md`, `docs/design/ratchet.json`, `scripts/arch/main.go`.

Measured at that revision (a selector `jobs.X` in a non-test Go file outside `cmd/ot` and the root of `internal/jobs`; run again at cut time):

- `api.go` declares 47 exported types (and five `FigureStatus` constants).
- 42 are used by exactly one job package (`NewFredSeriesFunc` by `fredseries`, `ComputeStores` by `computefigures`, `OptionLedgerStores` by `optionledger`, `CandidateStores` by `optioncandidates`, `FigureStatus` by `cutover`, and so on); `cmd/ot` uses most of them too.
- 3 are used by two or more job packages: `WatchEntry` (six: `barsrefresh`, `dividendcapture`, `earningsrefresh`, `fundamentals`, `optioncandidates`, `optioneod`), `ImportResult` (`cutover`, `optionledger`), `CaptureID` (`optioneod`, `quotesnapshot`).
- 2 are used by no package: `Registry` (tests and `docs/contracts/jobs.md` only) and `NewOptionEodWatchWindowFunc` (a test only; `optioneod.NewWithWatchWindow` is not typed by it).
- No package of compute, pages, store, runner, notify or broker uses any of them. The root of `internal/jobs` holds no non-test code besides `api.go`; its tests (`api_test.go` 800 lines, and four more) are the only other users.
- Every job package that has a shared type imports the root `internal/jobs` (so the root cannot import a job package: an alias in `api.go` would be an import cycle).

Needs the owner: nothing. No `contracts/` file, no migration, no change of behaviour.

## Goal

A job issue that adds or changes its own constructor type, stores struct, config or result touches only its own job package and `cmd/ot`; it never edits `internal/jobs/api.go`, so no interface issue is added and two job issues no longer wait for each other. `api.go` shrinks to the types that two packages share, and a test keeps it that way without a reviewer having to notice.

## Decisions

1. A job's own types live in its package. The constructor func type (`NewFredSeriesFunc`), the stores, config and result structs and interfaces (`ComputeStores`, `OptionLedgerConfig`, `FigureReport`, `SeriesHistoryLoader`, the `FigureStatus` constants and so on), and a function type of the job's helper (`CompareFiguresFunc`, `ImportAccountFunc`) are declared in an ordinary file of the job package, `types.go` (the file this design moves them into; a later type goes in any ordinary file). The name stays (`fredseries.NewFredSeriesFunc`); renaming to `fredseries.NewFunc` is a separate change and is not made here (Left out). `cmd/ot` imports the job package directly, as it already does for the constructor (the tree rule of `parallel-hotspots.md` lets `ot` import every implementation package of the rows in its depends cell, so `modules.md` needs no new import path: every package that receives a type `cmd/ot` names is among the 12 job packages it imports today). A constructor type declared in the job package is not an interface file, so declaring or changing one is not an interface change; this is the `account-data.md` precedent made the rule.
2. `internal/jobs/api.go` keeps exactly the types that two or more packages other than `cmd/ot` and the root of `internal/jobs` use: after the move `WatchEntry`, `ImportResult` and `CaptureID`. A type of `api.go` that falls below two such packages is moved to its one user (or deleted if it has none). `Registry` is deleted (no code uses it; `docs/contracts/jobs.md` loses the sentence), `NewOptionEodWatchWindowFunc` moves to `optioneod`. A type that a third party starts to share moves up into `api.go` by an interface issue, as today. Decision 1 alone already stops new jobs from touching `api.go`; the move of the 43 existing types only removes what is there.
3. The rule is a test, not a sentence. A test in the root of `internal/jobs` (`api_users_test.go`) parses the repository's non-test Go files with `go/parser`, finds every exported type declared in `api.go`, counts the distinct directories (other than `cmd/ot` and the root `internal/jobs`) that refer to it as `<name of the jobs import>.<Type>`, and fails, naming the type and its users, when a type has fewer than two, unless the comment of the type carries `shared-pending: docs/design/<name>.md` naming a design file that exists (the one escape, because AGENTS.md lands an interface issue before the issues that use it, so a new shared type has no user yet; the mark is removed by the issue that adds the second user, and a type that loses a user to a move is handled by moving it or marking it in the same change). The same file tests the counter on a tree written under `t.TempDir()`. It reads files only: no network, no sleep, no shared state, `t.Parallel()`.
4. How the move compiles in steps. A type cannot move in one pull request of one module: `cmd/ot` names `jobs.X` today (22 files), and an alias in `api.go` would import the job package from the root, which is a cycle for every job package that imports the root. So the move is three entries: (1) each job package gets its own alias `type X = jobs.X` (and `const FigureChecked = jobs.FigureChecked`) in a new `types.go`, an edit to no interface file; (2) `cmd/ot` stops writing `jobs.X` for a type that moves and writes `<pkg>.X`; (3) the interface entry replaces each alias by the declaration moved out of `api.go`, trims `api.go`, moves the contract tests, adds the test of Decision 3 and the contract document. After entry 1 and after entry 2 the tree builds and `api.go` is unchanged; entry 3 is the only edit of `api.go` and carries the `(interface)` mark and the Design line. The cost, accepted: one extra pass over the 15 packages, a day of aliases that name a type twice. The alternative, one cross-module entry (jobs and ot together, about 40 files), is rejected for review size and because it cannot be retried by module.
5. The tests of a moved type move with it. `api_test.go` and the four other root tests are contract tests of the types they use (a fake on one side, a caller on the other); each test of a type that moves becomes `internal/jobs/<pkg>/types_test.go` (package `<pkg>_test`), so the root keeps only the tests of the three shared types and of Decision 3. This also takes `api_test.go` from 800 lines down; it is not in `docs/design/ratchet.json` and is not allowed to grow, and it does not.
6. The rule line of `modules.md` (written in this pull request, see `## Modules`) and the sentence of AGENTS.md (a separate follow-up entry, below, as AGENTS.md changes are issues of their own) say the same thing in one sentence each.

## Modules

No new row. One edit of `docs/design/modules.md`, made in this pull request: a bullet after the paragraph on entry files, "In the jobs row the entry file holds only the types that two or more packages other than `cmd/ot` use (checked by `internal/jobs/api_users_test.go`); a job's constructor type, stores, config and result types live in the ordinary files of its package, and `cmd/ot` imports the package." The jobs row of the table is unchanged (paths, entry `internal/jobs/api.go`, test `internal/jobs/api_test.go`, depends). The entry-cell test column keeps `api_test.go`, which after entry 3 holds the contract test of the three shared types.

## Interface

jobs, `internal/jobs/api.go` after entry 3 (declarations only; the `NewXFunc` types listed here are the ones that leave):

- stays: `type WatchEntry struct{...}` (unchanged), `type ImportResult struct{...}` (unchanged), `type CaptureID = uuid.UUID`.
- leaves for the package of its one user, same declaration, same name: `fredseries`: `NewFredSeriesFunc`; `optioneod`: `NewOptionEodFunc`, `NewOptionEodSymbolsFunc`, `NewOptionEodWatchFunc`, `NewOptionEodWatchSymbolsFunc`, `NewOptionEodHeldFunc`, `OptionEodWindow`, `NewOptionEodWatchWindowFunc`; `quotesnapshot`: `NewQuoteSnapshotFunc`; `positionmarks`: `NewPositionMarksFunc`; `computefigures`: `ComputeStores`, `NewComputeFiguresFunc`, `NewComputeFiguresEaseFunc`; `serieshistory`: `SeriesLoaded`, `SeriesHistoryLoader`, `NewSeriesHistoryFunc`; `calendarrefresh`: `CalendarObserver`, `NewCalendarRefreshFunc`; `earningsrefresh`: `NewEarningsRefreshFunc`, `NewEarningsRefreshSymbolsFunc`; `optionledger`: `OptionLedgerStores`, `FlexRun`, `OptionLedgerConfig`, `NewOptionLedgerJobFunc`, `ImportOptionTraderFunc`, `FlexAttributesSHA256Func`; `cutover`: `VerifyResult`, `ImportAccountFunc`, `VerifyAccountFunc`, `FigureStatus` with its five constants, `FigureResult`, `FigureReport`, `CompareFiguresFunc`; `optioncandidates`: `CandidateStores`, `NewOptionCandidatesJobFunc`, `NewOptionCandidatesCallsFunc`, `SettingsVersionFunc`; `barsrefresh`: `NewBarsRefreshFunc`; `fxrefresh`: `NewFxRefreshFunc`; `dividendcapture`: `NewDividendCaptureFunc`; `fundamentals`: `NewFundamentalsFunc`, `FundamentalsSkip`, `FundamentalsSkips`. That is 43 types (the 42 with one user and `NewOptionEodWatchWindowFunc`); 47 less 43 less `Registry` leaves the three that stay.
- deleted: `Registry`.
- `internal/jobs/api_users_test.go`: `TestAPITypeHasTwoUserPkgs` (the repository test of Decision 3), `TestUsersCounterOnTempTree`, `TestUsersIgnoresOtAndRoot` (and the escape mark is tested there too).

Landed in: not yet.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, uses a fake clock, never sleeps, opens no port, and touches no live broker or database. Acceptance is checked offline by reading, grep and git; test runs are under "Checked by CI". `BASE` is `git merge-base HEAD origin/main`.

### 1 jobs-types-alias

Module: jobs. Size: L (mechanical).
Files: `internal/jobs/barsrefresh/job.go`, `internal/jobs/barsrefresh/types.go`, `internal/jobs/calendarrefresh/job.go`, `internal/jobs/calendarrefresh/types.go`, `internal/jobs/computefigures/ease_test.go`, `internal/jobs/computefigures/ivinput.go`, `internal/jobs/computefigures/ivinput_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/subjects.go`, `internal/jobs/computefigures/types.go`, `internal/jobs/cutover/accountimport.go`, `internal/jobs/cutover/accountverify.go`, `internal/jobs/cutover/classes_test.go`, `internal/jobs/cutover/compare.go`, `internal/jobs/cutover/compare_test.go`, `internal/jobs/cutover/doc.go`, `internal/jobs/cutover/types.go`, `internal/jobs/dividendcapture/run.go`, `internal/jobs/dividendcapture/run_test.go`, `internal/jobs/dividendcapture/types.go`, `internal/jobs/earningsrefresh/run.go`, `internal/jobs/earningsrefresh/types.go`, `internal/jobs/fredseries/run.go`, `internal/jobs/fredseries/types.go`, `internal/jobs/fundamentals/run.go`, `internal/jobs/fundamentals/run_test.go`, `internal/jobs/fundamentals/types.go`, `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/types.go`, `internal/jobs/optioncandidates/calls.go`, `internal/jobs/optioncandidates/calls_test.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/job_test.go`, `internal/jobs/optioncandidates/types.go`, `internal/jobs/optioneod/held.go`, `internal/jobs/optioneod/jp.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/types.go`, `internal/jobs/optioneod/watch.go`, `internal/jobs/optioneod/watch_test.go`, `internal/jobs/optionledger/doc.go`, `internal/jobs/optionledger/hash.go`, `internal/jobs/optionledger/hash_test.go`, `internal/jobs/optionledger/importfile.go`, `internal/jobs/optionledger/job.go`, `internal/jobs/optionledger/job_test.go`, `internal/jobs/optionledger/sections.go`, `internal/jobs/optionledger/sections_test.go`, `internal/jobs/optionledger/types.go`, `internal/jobs/positionmarks/run.go`, `internal/jobs/positionmarks/types.go`, `internal/jobs/quotesnapshot/run.go`, `internal/jobs/quotesnapshot/types.go`, `internal/jobs/serieshistory/loader.go`, `internal/jobs/serieshistory/loader_test.go`, `internal/jobs/serieshistory/types.go`
Design: docs/design/jobs-api-slim.md#Decisions (Decision 4)
Order: first.
Goal: in each of the 15 job packages one new file `types.go` holding an alias `type X = jobs.X` for each type that `## Interface` sends to that package (and `const FigureChecked = jobs.FigureChecked` and its four siblings in `cutover`); and in the 41 other files listed, every `jobs.X` for such a type is written `X` (a package refers to its own type by the local name; `jobs.WatchEntry`, `jobs.ImportResult` and `jobs.CaptureID` stay). It builds at once because the alias keeps both names valid. `api.go` and `cmd/ot` are not edited.
Acceptance:
1. `git diff --name-only BASE` lists only files under `internal/jobs/*/`, and `git diff --name-only BASE -- internal/jobs/api.go cmd` is empty.
2. `grep -rhoE "jobs\.[A-Za-z0-9]+" internal/jobs/*/ --include='*.go' | grep -vE "jobs\.(WatchEntry|ImportResult|CaptureID)$" | grep -v "types.go" | wc -l` prints 0 (outside `types.go`, the only `jobs.` names left in job packages are the three shared types).
3. `grep -c "^type .* = jobs\." internal/jobs/cutover/types.go` is at least 6, `grep -c "FigureChecked" internal/jobs/cutover/types.go` is at least 1, and `grep -c "^type .* = jobs\." internal/jobs/optioneod/types.go` is 7.
4. `grep -c "^func " internal/jobs/*/types.go` prints 0 for every file.

### 2 ot-use-job-types

Module: ot. Size: M.
Files: `cmd/ot/buildcause_test.go`, `cmd/ot/candidates.go`, `cmd/ot/candidates_test.go`, `cmd/ot/comparefigures.go`, `cmd/ot/comparefigures_test.go`, `cmd/ot/dryrun.go`, `cmd/ot/dryrun_test.go`, `cmd/ot/env_test.go`, `cmd/ot/jp_wire_test.go`, `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_option_test.go`, `cmd/ot/runonce_test.go`, `cmd/ot/scheduler_test.go`, `cmd/ot/seriesload_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`
Design: docs/design/jobs-api-slim.md#Decisions (Decisions 1 and 4)
Order: after 1. `api.go` is not edited by any other issue between entries 1 and 3 (a design that must add a shared type waits, or is rebased onto entry 3 by its author).
Goal: every `jobs.X` in `cmd/ot` for a type that leaves `api.go` is written `<pkg>.X` (`computefigures.ComputeStores`, `cutover.FigureReport`, `calendarrefresh.CalendarObserver`, `optionledgerjob.OptionLedgerStores` (the job package is imported as `optionledgerjob` because `internal/compute/optionledger` holds the name; each file keeps its existing import name), `optioncandidates.CandidateStores`, `serieshistory.SeriesLoaded` and so on); `jobs.WatchEntry` and `jobs.ImportResult` stay; no behaviour changes and no new import path (each package is one `cmd/ot` imports today).
Acceptance:
1. `grep -rhoE "jobs\.[A-Za-z0-9]+" cmd/ot | grep -vE "jobs\.(WatchEntry|ImportResult)$" | wc -l` prints 0 (the only `jobs.` names left in `cmd/ot` are the two shared types `cmd/ot` uses; `CaptureID` is not named there).
2. `grep -rhoE "optdesk/internal/jobs/[a-z]+" cmd/ot | sort -u` names only packages that exist on BASE in that list (`git diff BASE -- cmd/ot | grep -c "^+.*optdesk/internal/jobs/"` lists no path outside `calendarrefresh`, `computefigures`, `cutover`, `dividendcapture`, `earningsrefresh`, `fredseries`, `optioncandidates`, `optioneod`, `optionledger`, `quotesnapshot`, `serieshistory`, `accountcapture`).
3. `git diff --name-only BASE` names only files under `cmd/ot/`.

### 3 jobs-api-slim (interface)

Module: jobs. Size: L (about 40 files, mechanical: a declaration moves from `api.go` into the alias line that replaces it).
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/api_fundamentals_test.go`, `internal/jobs/flex_section_api_test.go`, `internal/jobs/optionledger_api_test.go`, `internal/jobs/settings_version_test.go`, `internal/jobs/api_users_test.go`, `docs/contracts/jobs.md`, `internal/jobs/barsrefresh/types.go`, `internal/jobs/calendarrefresh/types.go`, `internal/jobs/computefigures/types.go`, `internal/jobs/cutover/types.go`, `internal/jobs/dividendcapture/types.go`, `internal/jobs/earningsrefresh/types.go`, `internal/jobs/fredseries/types.go`, `internal/jobs/fundamentals/types.go`, `internal/jobs/fxrefresh/types.go`, `internal/jobs/optioncandidates/types.go`, `internal/jobs/optioneod/types.go`, `internal/jobs/optionledger/types.go`, `internal/jobs/positionmarks/types.go`, `internal/jobs/quotesnapshot/types.go`, `internal/jobs/serieshistory/types.go`, `internal/jobs/barsrefresh/types_test.go`, `internal/jobs/calendarrefresh/types_test.go`, `internal/jobs/computefigures/types_test.go`, `internal/jobs/cutover/types_test.go`, `internal/jobs/dividendcapture/types_test.go`, `internal/jobs/earningsrefresh/types_test.go`, `internal/jobs/fredseries/types_test.go`, `internal/jobs/fundamentals/types_test.go`, `internal/jobs/fxrefresh/types_test.go`, `internal/jobs/optioncandidates/types_test.go`, `internal/jobs/optioneod/types_test.go`, `internal/jobs/optionledger/types_test.go`, `internal/jobs/positionmarks/types_test.go`, `internal/jobs/quotesnapshot/types_test.go`, `internal/jobs/serieshistory/types_test.go`
Design: docs/design/jobs-api-slim.md#Interface
Order: after 2.
Goal: `api.go` declares only `WatchEntry`, `ImportResult` and `CaptureID`; every other type of `## Interface` is declared in the `types.go` of its package in place of its alias; `Registry` is gone; the contract tests of the moved types are in the `types_test.go` of their package (`TestRegistryContract` is deleted with `Registry`; `TestSymbolCtorTypes`, which spans `optioneod` and `earningsrefresh`, is split one half per package; the helpers `fakeJob`, `fakeBroker`, `planAndRun` and `captureOf` are copied into each `types_test.go` that needs them, a test file never imports another's); `api_users_test.go` holds the test of Decision 3 and fails on a type with fewer than two user packages; `docs/contracts/jobs.md` (its test list at the `TestRegistryContract` line included) names each moved type with its package (`fredseries.NewFredSeriesFunc`) and no longer mentions `jobs.Registry` or `jobs.NewXFunc` for a moved type.
Acceptance:
1. `grep -c "^type " internal/jobs/api.go` prints 3, and `grep -c "^const\|^var\|^func " internal/jobs/api.go` prints 0.
2. `grep -rE "^type [A-Za-z0-9]+ = jobs\." internal/jobs | wc -l` prints 0 (no alias of entry 1 is left).
3. `grep -c "FigureStatus" internal/jobs/cutover/types.go` is at least 2 and `grep -c "^type NewFredSeriesFunc" internal/jobs/fredseries/types.go` is 1.
4. `grep -c "Registry" internal/jobs/api.go docs/contracts/jobs.md` prints 0 for both files.
5. `internal/jobs/api_users_test.go` names `TestAPITypeHasTwoUserPkgs`, `TestUsersCounterOnTempTree` and `TestUsersIgnoresOtAndRoot` once each, imports `go/parser` and no package outside the standard library, and `grep -c "time.Sleep\|net/http\|os.Getenv" internal/jobs/api_users_test.go` prints 0.
6. `wc -l internal/jobs/api_test.go` is below 800 and no file of the entry is over 600 lines unless it was over 600 on BASE.
7. `git diff --name-only BASE` names only files under `internal/jobs/` and `docs/contracts/jobs.md`.

### 4 agents-jobs-api-rule

Module: repo. Size: S.
Files: `AGENTS.md`
Design: docs/design/jobs-api-slim.md#Decisions (Decision 6)
Order: after 3.
Goal: one sentence under "Interfaces" of AGENTS.md: a job's constructor and stores types live in its package, and `internal/jobs/api.go` holds only types that two packages other than `cmd/ot` use (enforced by `internal/jobs/api_users_test.go`).
Acceptance:
1. `grep -c "api_users_test.go" AGENTS.md` is 1.
2. `git diff --name-only BASE` prints only `AGENTS.md`.

## Checked by CI

`go test ./internal/jobs/... ./cmd/ot/...` (after entry 1 with the aliases, after entry 2 with `cmd/ot` rewritten, after entry 3 including `TestAPITypeHasTwoUserPkgs`), `go vet ./...`, `go run ./scripts/arch --check` prints ok (no new import edge: a job package imports `internal/jobs` as before, `cmd/ot` imports packages it already imports), the `test` check, the design verdict.

## Left out

- Renaming the constructor types to the package-local form (`fredseries.NewFunc`): the move keeps every name so that it stays mechanical; a rename is a later change with its own callers.
- Splitting `docs/contracts/jobs.md` (205 lines) into one document per job package: step c, a later design; this design only changes the names inside it.
- `accountcapture` and any job added after this design already follow Decision 1; nothing to move.
- Remaining serialisation points this design does not remove: `docs/contracts/jobs.md` (touched by every job issue; split per job package is step c, a later design), `cmd/ot/wire.go` and `cmd/ot/runonce.go` (every new job registers there), and `WatchEntry` (shared by six jobs, stays in `api.go`).
- Lock-in named: `cmd/ot` and the job packages now agree on a type by identity of the alias, so a later rename (above) touches both; a constructor change no longer shows in `api.go` at design review, it shows in the pull request of the job package.
- A mechanical check of aliases and re-exports: a type kept in `api.go` by a reference from two packages that do not really use it (a dummy selector) is not detected by the test; review reads the pull request that adds the second user.
- The tests of the three shared types stay in `api_test.go`; their further split is not needed (the file shrinks well below 800 lines).

## Adversarial

Answers to the read-only attack, kept in the dispatcher scratchpad (jobs-api-slim.adversarial.md, not in the repository), items 1 to 6.

1. Gap versus the goal: adopted in part. The remaining serialisation points (`docs/contracts/jobs.md`, `cmd/ot/wire.go`, `runonce.go`, `WatchEntry`) are named under Left out; `api.go` is frozen between entries 1 and 3 (Order line of entry 2).
2. Biggest risk, the two-user test against "interface lands first": adopted. Decision 3 has the `shared-pending: docs/design/<name>.md` escape and a test of it.
3. Simpler way: not adopted. Decision 1 alone stops new churn, and Decision 2 now says so, but the goal also names the 43 types that are there; the three entries stay.
4. Lock-in: adopted, named under Left out (identity of the alias, a constructor change no longer visible in `api.go`).
5. Claim most likely false: adopted. Entry 3 did not compile because 41 files of the job packages write `jobs.X`; their rewrite is now in entry 1 with a grep, entry 3 lists no such file. Also fixed: the `optionledgerjob` import name, entry 2's Files (files that name only shared types removed), `TestRegistryContract` and `TestSymbolCtorTypes` and the shared test helpers, the `jobs.md` test list.
6. Likeliest slip, an edit outside the Files list to make entry 3 compile: removed by the same fix; entry 2's acceptance now greps every `jobs.` name left in `cmd/ot`.
