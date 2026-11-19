# base data alerts: /base-data shows red only for a real unrepaired fault

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-19

Source: the read-only check of `/base-data` of 2026-11-19 (five alerts) and its amendment notes, read on origin/main. Owner principles used: dev mode has no time guards beyond the one same-job concurrency guard; a normal state is never shown as an alarm; the front end computes nothing; every visible decision is its own acceptance item. Every path and line below was read on origin/main after `git fetch`; nothing was run and no database was read.

## What you see

On `/base-data` a row is red only when a key is really missing after the runner has had its last chance, or a value is really wrong. Four causes of false red or false amber go away:

- A key whose runs so far ended retryable before the day closed (the account capture, the position marks, the daily bars of a day) gets one more run after the close, and only if that run fails too is the key shown as expired.
- The option open row no longer counts the contracts the job chose not to ask (status 7, deferred) as errors; they sit in the number "not judged".
- The rates row (FRED publishes a day late) is judged up to the session before the newest closed one, so a normal state is not red.
- The days at the start of a row that can never be fetched again (chains, contracts, calendar, flex, open, from the first week) are no longer counted as missing days; the row shows one note line "history, not repairable" (历史缺口，无法补采) with the number of such days.

## Goal

`/base-data` shows red only for a real unrepaired fault:

1. A key whose only runs are retryable and ended before the window close gets its late run (reason `late`) before it is closed as `expired` (scheduler.md Decision 8, Close table row 3).
2. Open `errors` are statuses 5, 6, 8 and 9; status 7 (deferred) is in `not_judged` (base-data-page.md Decision 7).
3. The rates row judges `coverage_latest` and `coverage_gap` up to the session before the newest closed one.
4. History days that cannot be refetched are not counted in `coverage_gap`, so they are not amber, and are shown as a note named `history_lost` with a Chinese hint.

## Today

- `internal/runner/reconcile/runkey.go:745` calls `expireIfClosed` before `nextAttempt` (line 750). `expireIfClosed` (`reconcile.go`, after `closeRow`) returns at once only for `len(runs) == 0`, an empty or not yet closed window.
- `closeRow` (`reconcile.go:440-480`) row 3 is `newest >= 0 && isRetryable(runs[newest])`: it does not look at when that run ended. The default window rule closes at the session close (`CloseOffset` 0, `cmd/ot/scheduler.go:282`; no `Window()` on `account_capture` or `position_marks`), while their slots are close + 30 min and close + 40 min. A key with one retryable run before the close is therefore closed `expired` at the first tick after the close and gets no late run; a key with no earlier run (`len(runs) == 0`) gets the late run. This is why `account` was red for the last day and why the `bars` run of a day that ended retryable after the "no data" change is expired at the close with no late run.
- `migrations/0057_base_fact_option.sql:106` counts `errors` of the open view as `status IN (5, 6, 7, 8, 9)` and `not_judged` as `(2, 4)`. Status 7 is a row the job chose not to ask (`internal/jobs/optionopen/core.go:240-243`); on the read day it was most of the rows counted as errors.
- `internal/compute/basecheck/eval.go:270-317` `ledgerCheck`: `coverage_latest` reads the cells of `in.Days[firstCalendar(info)]` (XNYS when the row has no calendar, `age.go:39-49`); `coverage_gap` skips cells before one global `HistoryStart` (`rules.go:13`, 2026-11-07) and after the last closed session. The rates row (FRED series, no calendar) therefore expects the newest closed session, which FRED has not published yet.
- `history_lost` is written only in `docs/design/base-data-page.md` (Checks, `coverage_gap`); no code emits it. The real days that cannot be refetched run longer than 2026-11-07 for five rows (chains, contracts, calendar, flex, open).
- `basecheckapi.Rules` (`internal/compute/basecheckapi/api.go:93-197`) lists rules per row with a source mark; `flex_section_absent` is the one `note` rule today. A hint and a label for every rule id is a test of the pages package (`zh_texts_test.go`; `builder.go:20-26`, `text.go:62`).

## Decisions

1. Close table row 3. Old sentence (scheduler.md Decision 8, row 3): "else the key is retryable (runner v3: its last run is a retryable run that is not `crashed`)". New: row 3 applies only when that last retryable run ended at or after the window close (`FinishedAt` not before `closeAt`); a key whose only runs are retryable and ended before the close is not closed by the table: `nextAttempt` starts one late run (reason `late`), and if that run ends retryable, the next tick closes it by row 3 as `expired`. Rows 1, 2 and 4 are unchanged. Why: the close is a fixed instant, the slot of these jobs is after it, and a run before the close never had its chance; no time guard is added. `scheduler.md` and `docs/contracts/runner.md` say the new rule in the same branch.
2. Open `errors`. Old sentence (base-data-page.md Decision 7): "error (5 for chains and quotes, 5 to 9 for open) in `errors`; not listed and stale (3 and 4 for chains and quotes, 4 for open, 2 partial of open) are counted in the fact `not_judged`". New: for open, `errors` are statuses 5, 6, 8 and 9, and status 7 (deferred: a row the job chose not to ask) is counted in `not_judged` with 2 and 4. The view `rd.base_fact_option_r1` is replaced by a new migration; 0057 stays unedited. The rule table line of `open` says the same.
3. Rates publication lag. Old sentence (base-data-page.md, Checks, `coverage_latest`): "Bad = missing + expired keys of the newest closed session of the row's first subject calendar (XNYS if none)". New: for a row marked with a publication lag, `coverage_latest` and `coverage_gap` judge up to the session before the newest closed one (the lag is 1 session); the newer day is shown in no verdict. The mark is a per-row table in `rules.go` (`PublishLag`, row id to sessions), and only `rates` has an entry (FRED publishes the day after). The lagged day is found in `in.Sessions` of the row's calendar; when the closed session is the first of the list, the row is not judged by these two rules (no check, like today's missing cell).
4. History days. Old sentence (base-data-page.md, Checks, `coverage_gap`): "Days before 2026-11-07 are shown apart as the fact `history_lost` (audit gap 6)". New: `HistoryStart` is a per-row first counted day (`rules.go` table `HistoryStart`, row id to day, trial values from the read of 2026-11-19: chains 2026-11-11, contracts 2026-11-13, calendar 2026-11-11, flex 2026-11-11, open 2026-11-10, every other row 2026-11-07, the old constant as the default). Days before it are not counted in `coverage_gap`. A row whose day is later than the default gets the note check `history_lost`: N = days from 2026-11-07 to the day before its `HistoryStart` that have ledger cells, Bad = those with missing keys, Example = the newest of them, Level empty (never judged, like `flex_section_absent`). The Chinese label and hint are in the pages package.
5. Where the new rule id lives. `history_lost` is a rule of `basecheckapi.Rules` with source `note` for the five rows above, and its Chinese hint is in `zh_texts.go`: an interface change, the first entry. The `Check` type, the JSON schema and the web are not changed: the web already lists every check of a row with its hint.
6. Own code, said as the rules ask: the per-row tables are glue inside the existing package `basecheck` behind `basecheckapi.Func`; no library fits a project table of days.

## Modules

None. Every path below is already in a row of `docs/design/modules.md` (runner, store, compute, pages).

## Interface

Entry 1 only: `internal/compute/basecheckapi/api.go`, the table `Rules` gains five `note` entries, `{"chains", "history_lost", "note"}`, `{"contracts", "history_lost", "note"}`, `{"calendar", "history_lost", "note"}`, `{"flex", "history_lost", "note"}`, `{"open", "history_lost", "note"}`. No type changes. Why an interface change: `Rules` is the declarations file of compute's check table, read by the pages test and the contract test of `cmd/ot`.

## Issues

Build order: entry 1 first; entries 2 and 3 have no dependency and run in parallel with 1; entries 4 and 5 run after 1 and in parallel with each other. No entry is merged with another (four modules).

### 1 basedata-alerts-history-rule (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/basecheckapi/api.go`, `internal/compute/basecheckapi/api_test.go`, `internal/compute/basecheckapi/table_test.go`
Contracts: unchanged
Design: docs/design/basedata-alerts.md#Interface
Acceptance:
1. `Rules` has exactly one new entry per row of chains, contracts, calendar, flex and open, rule `history_lost`, source `note`; no other entry and no type is changed (new test `TestRulesHistoryLostNotes`).
2. The existing table tests pass unchanged: every row has `rows` and `newest`, every rule id is unique per row.
3. matches docs/design/basedata-alerts.md#Interface

### 2 basedata-alerts-runner

Module: runner. Size: M.
Order: independent.
Files: `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/retry_test.go`, `internal/runner/reconcile/reconcile_test.go`, `internal/runner/reconcile/runkey_test.go`, `internal/runner/reconcile/backfill_stop_test.go`, `internal/runner/reconcile/orderedstop_test.go`, `docs/contracts/runner.md`
Contracts: unchanged
Design: docs/design/basedata-alerts.md#Decisions
Acceptance:
1. A missing key whose only run is retryable and finished before `closeAt` is not closed at the first tick after the close: no `MarkPresent` call, the run is not rewritten, and a run with reason `late` starts (new test `TestRetryableBeforeCloseGetsLateRun`).
2. The same key whose late run ends retryable is closed at the next tick by row 3: key `no_value`, the late run's reason `expired` (new test `TestLateRetryableRunIsExpired`).
3. A key whose newest retryable run finished at or after `closeAt` is closed by row 3 at once, as today (new test `TestRetryableAfterCloseExpires`).
4. Rows 1, 2 and 4 give the same index and reason as before for every case of the existing close table tests (the existing tests of `closeRow` pass with the new argument only).
5. `docs/contracts/runner.md` says row 3 with the "ended at or after the close" condition and the late run (its version line is bumped by one).
6. matches docs/design/basedata-alerts.md#Decisions (Decision 1)

### 3 basedata-alerts-open-errors

Module: store. Size: S.
Order: independent.
Files: `migrations/<next free number>_base_fact_option_open.sql`, `internal/store/basefactpg/option_pg_test.go`
Contracts: unchanged
Design: docs/design/basedata-alerts.md#Decisions
Acceptance:
1. The new migration replaces `rd.base_fact_option_r1` (`CREATE OR REPLACE VIEW`, same columns and same `-- rule` comment lines) and counts open `errors` with `status IN (5, 6, 8, 9)` and open `not_judged` with `status IN (2, 4, 7)`; chains and quotes are unchanged (new test `TestOpenDeferredIsNotJudged`, build tag `pg`: a day with a deferred row gives `errors` 0 and `not_judged` 1).
2. A contract with no status-1 row whose newest row has status 8 is still an error and one with status 6 too (new test `TestOpenErrorStatusesStillCount`, tag `pg`).
3. `migrations/0057_base_fact_option.sql` is unchanged, and the hold on migrations is named in the pull request description (owner runs migrate after the merge).
4. matches docs/design/basedata-alerts.md#Decisions (Decision 2)

### 4 basedata-alerts-compute

Module: compute. Size: M.
Order: after 1.
Files: `internal/compute/basecheck/eval.go`, `internal/compute/basecheck/rules.go`, `internal/compute/basecheck/eval_test.go`, `internal/compute/basecheck/rules_test.go`, `internal/compute/basecheck/testdata/facts.json`
Contracts: unchanged
Design: docs/design/basedata-alerts.md#Decisions
Acceptance:
1. For the rates row, `coverage_latest` reads the cells of the session before the newest closed session of its calendar (new test `TestRatesLatestJudgesPreviousSession`: a missing key on the newest closed day gives no Bad, a missing key on the day before gives Bad 1).
2. For the rates row, `coverage_gap` ignores cells after that session (new test `TestRatesGapStopsAtLag`); every other row still judges up to the newest closed session (the existing tests pass unchanged).
3. `coverage_gap` skips cells before the row's `HistoryStart`; a row with no entry uses 2026-11-07 (new test `TestGapSkipsDaysBeforeRowHistoryStart`; the existing test `TestCoverageGapSkipsDaysBeforeHistoryStart` passes unchanged).
4. The five rows of entry 1 get a `history_lost` check with Level empty, N, Bad and Example as Decision 4 says; a row with no later day has none (new test `TestHistoryLostNote`: two ledger days before the row day, one with missing keys, gives N 2, Bad 1, Level empty, and the row verdict does not change).
5. `HistoryStart` and `PublishLag` are tables in `rules.go` only, and no other file holds a day or a lag number (new test `TestRowTablesCoverKnownRows`: every key is a row id of `basecheckapi.Rows`).
6. matches docs/design/basedata-alerts.md#Decisions (Decisions 3, 4)

### 5 basedata-alerts-pages

Module: pages. Size: S.
Order: after 1.
Files: `internal/pages/base_data_page/builder.go`, `internal/pages/base_data_page/text.go`, `internal/pages/base_data_page/zh_texts.go`, `internal/pages/base_data_page/zh_texts_test.go`, `internal/pages/base_data_page/builder_test.go`, `internal/pages/base_data_page/testdata/ok.json`, `internal/pages/base_data_page/testdata/gap.json`
Contracts: unchanged
Design: docs/design/basedata-alerts.md#Decisions
Acceptance:
1. The rule id `history_lost` has the English label "History lost" and a Chinese hint that says these days cannot be fetched again and need no repair ("历史缺口，无法补采，无需处理") (new test `TestHistoryLostHint`; the existing test that requires a hint for every rule id of `basecheckapi.Rules` passes).
2. A row's `history_lost` check is built into the page with its hint and an empty level, and the row's own level is unchanged (new test `TestHistoryLostCheckNotJudged`).
3. The hint of `errors` for open mentions that a contract the job chose not to ask is not an error (new test `TestOpenErrorsHint`); the pinned page files carry the new line only where a row has the note.
4. matches docs/design/basedata-alerts.md#Decisions (Decisions 4, 5)

## Checked by CI

- ci-go runs `go test ./internal/compute/... ./internal/runner/... ./internal/pages/... ./internal/store/...`, `scripts/arch` and the lint.
- The `pg` tests of entry 3 run in GitHub CI only.
- Not CI, a read after the deploy by the window: `/base-data` shows no red row for the day above, and the page time is read once.

## Adversarial

to be filled

## Left out

- A new Check field and a web column for history days (the amendment's "Check.HistoryLost" with contracts schema, generated types, pages and web): a note rule shows the same number with no schema change, so the larger four-module chain is a later design if the owner wants a dedicated column.
- Rejecting the alert text "badly worded" only by wording: a lagged judge is a verdict change, not a hint.
- Rejected: making the global `HistoryStart` later. It would hide real gaps in rows that were fetched from 2026-11-07.
- Rejected: moving the window close of `account_capture` and `position_marks` after their slot (`Window()` on the job). It fixes two jobs by schedule, leaves bars and every future job with the same rule fault, and adds a time guard the owner does not want in dev mode.
- Rejected: counting status 7 as `empty`. It is not a missing quote; the job did not ask.
- The bars classifier ("no data" is retryable) is already on main; no bars issue.
- A threshold change for bars `coverage_latest` (4 keys of 75 trip 5%): the cause was the missing late run, so no threshold is touched.
