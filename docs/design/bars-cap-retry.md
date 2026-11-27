---
type: design
title: "bars cap retry: a key the first-pull cap left behind is a retryable outcome, so it shows as expired on /data-health and the catch-up pass pulls it"
status: accepted
date: "2026-11-27"
need: [none]
none_reason: "owner goal 2026-11-17 (data must be as complete as the source allows) and docs/design/auto-catch-up.md Left out item 3; no need record exists"
---

# bars cap retry: a key the first-pull cap left behind is a retryable outcome, so it shows as expired on /data-health and the catch-up pass pulls it

Author: the optdesk designer
Owner approved: (pending)
Revision: 1
Plan: P1

摘要：bars 每次运行最多给 10 个新名字做首次长回补（`backfill_names_per_run = 10`），超出的名字这次不请求、也不报错，所以账本把它们记成 no_value（"来源说没有数据"），页面显示成无数据，补数据的自动流程也不碰 no_value。改法只在 `internal/jobs/barsrefresh`：被上限挡下的名字返回一个可重试（Retryable）的结果。账本已有的规则会把它在窗口关闭时记成 expired，页面已经把 expired 画成可补的格子，自动补数流程（catch-up pass）已经会补 expired 的日子，每一轮再拉 10 个，直到每个名字都有 K 线或来源真的回答没有数据。不改接口、不改 runner、不改页面。

Source: `docs/design/auto-catch-up.md` Left out item 3 ("keys past it end `no_value` and the pass never takes `no_value`; needs its own design"); owner words 2026-11-17 (ask optdesk-4677: the collector must keep data as complete as it can). Read from origin/main 94fce574.

Needs the owner: nothing. No outside service, key or account; the only change is the outcome code of one job.

## Frame

How a key past the cap ends today:

- `internal/jobs/barsrefresh/run.go` `runKey` (line 98): a key with no stored bar in `backfill_days` asks `claimFirstPull` (line 82), which lets at most `backfill_names_per_run` keys per run day start a first pull (`started` map, `job.go:76`, in memory). A denied key returns `0, 0, nil` ("past the cap: no request, no error").
- The runner runs one key per `Run` call (`internal/runner/reconcile/runkey.go:114`). `verdict` (`runkey.go:239`): an empty answer with no error before the window close is "no explanation, retry" (`empty_before_close`); at or after the close it is `no_value` (`runkey.go:260`). The job has no `Postcondition`. A day's first run of the 11th name is therefore retried while the window is open (the in-memory cap still denies it) and then closes `no_value` with a clean finish.
- `coverage.StateExpired` is "marked no_value, and the newest run of the key is not a clean finish" (`internal/store/coverage/api.go:35`); a plain `no_value` is the state the page prints as "the source answered no data". So a key the cap left behind looks like a real no-data answer.
- `GapDays` takes `not_run`, `missing` and `expired` cells with a button, not `no_value` (`internal/pages/data_health_page/repair.go:120-131`), and the catch-up pass (`cmd/ot/catchup.go:64`) enqueues `GapDays` of every dataset (docs/design/auto-catch-up.md Decision 1). So the pass never repairs a capped key.
- A run of a later day pulls 10 more first-pull names (`started` is per day), so the backlog also shrinks by 10 a day through the scheduled runs, but the earlier days keep their `no_value` cell for ever, and nothing tells the owner that they are "not pulled yet" rather than "no data".

Result for the owner: on `/data-health` a day whose bars key was left behind by the cap shows as `expired` with a repair button, never as `no_value`. The catch-up pass queues one request for it per calendar day; each request pulls up to 10 more keys, so N waiting keys are all pulled within about N/10 calendar days, and a key whose source really has no bars still ends `no_value`.

### Decisions

#### Decision 1: a cap-denied key returns a Retryable error

`runKey` returns `newErr(contracts.Retryable, "first-pull cap reached")` for a key `claimFirstPull` denied; it makes no broker request and writes no row. The text names no key value beyond what other errors name. `Run` already keeps a Retryable error over a Permanent one (`run.go:41-43`), so one key per call needs no change there. Own implementation, because no library is involved; the outcome code is the one the ledger already reads.

Why this and not the others:

- **Mark the key `missing` from the job.** Impossible without a per-key result: `JobResult` carries only the totals and one error (`internal/contracts`), and the ledger marks, not the job. A new field is an interface change in contracts (a shared interface, a generated output, jobs and runner both read it).
- **Let the catch-up pass take `no_value` cells.** It would pull every real no-data key again each day (a delisted name, a name the source has no bars for) and loses the difference between "not pulled" and "no data" on the page; it also changes `GapDays` and the page verdict (pages module) and `cmd/ot`.
- **Lift the cap for repair runs.** Pacing is the reason for the cap (docs/design/market-bars.md, Decision 7); the owner's no-guard rule applies to past-day repair, which Decision 10 of docs/design/data-repair-bars.md already handles for names that have rows, but a first pull of hundreds of names in one run is the pacing the cap exists to bound. Not changed.
- **A `Postcondition` on the job.** It would turn a denied key into `missing` for ever (no close), so the key would never leave the open state and the page would never show it as closed.

#### Decision 2: the window and the rest of the ledger rules are not touched

A Retryable run is retried before the close (bounded by the runner's own cap, `retryBeforeCloseCap`) and, at the close, `closeRow` row 3 marks the key `no_value` with reason `expired` (`reconcile.go:501`). That is the state `expired`; nothing in `internal/runner`, the store or the page changes. A later request of the same day (the catch-up pass or the owner's button) finds the key without bars again, `claimFirstPull` of that request gives it one of its 10 places, and a key that got bars from a run of another day is "backfilled", asks only its gap and ends `present`.

#### Decision 3: no page, runner, store or contract change

The page already prints `expired` as a repairable gap (`GapDays`), and the pass already queues it. `backfill_names_per_run` stays 10 (a trial value in `job.toml`).

## Modules

jobs only: `internal/jobs/barsrefresh/**` and `docs/contracts/jobs/bars_refresh.md` (row `jobs` of `docs/design/modules.md`). No row changes and no dependency changes.

## Interface

None. No `api.go`, `api.py`, JSON schema or generated type is edited; `contracts.JobResult`, `runner.Postcondition` and the page payload are used as they are.

## Failure paths

- A key past the cap with its window still open: the run row is Retryable; the runner retries it a bounded number of times, the in-memory cap still denies it, and at the close the key is `expired`. The job's day result carries a Retryable error (the same kind a rate-limited pull already gives).
- The process restarts: `started` is empty, so a retry of the same day may start different keys; the stored bars decide, not the set.
- Today's cell: a key waiting for its first pull shows `missing` until the window close, then `expired`; the owner's button works before the close too (verdict of `missing`).
- A key whose source has no bars: it takes a place of the 10, the pull returns no bar and ends `no_value` as before (`TestNoBarsIsUnusableNotZero` is unchanged).
- Carried decisions: Decision 1 in issue 1; Decisions 2 and 3 have no entry of their own (no code beyond Decision 1).

## Issues

### 1 bars-cap-retryable

Module: jobs. Size: S. Order: first. Design: docs/design/bars-cap-retry.md#decisions. Files: `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/run_test.go`, `internal/jobs/barsrefresh/cap_retry_test.go`, `docs/contracts/jobs/bars_refresh.md`.

Acceptance:
1. A key that `claimFirstPull` denies makes `Run` return a result with an error of kind Retryable, no broker request and no stored row for that key; the keys that were let through still pull and store their bars, and the result of a call with both kinds keeps the Retryable error and counts only the pulled bars.
2. A key that began a first pull earlier on the same day still pulls on a retry of that day, and a key that has stored bars in the window is never denied by the cap.
3. A key the source answers with no bars still ends with the `no_value` kind as before (`TestNoBarsIsUnusableNotZero` keeps its meaning), so a Retryable error comes only from the cap.
4. `TestBackfillCapDefersWithoutRequest` is changed to expect the Retryable error of the denied name (and to assert that the allowed names have none) instead of an empty answer without error.
5. `TestCapDeniedKeyPullsOnALaterRun` (new) runs the same key again after the other keys have been pulled by a run of another day and sees its bars stored and no error, and `TestCapDeniedKeyRetryable` (new) is its denied counterpart; both are in `cap_retry_test.go` and call `t.Parallel()`.
6. `docs/contracts/jobs/bars_refresh.md` says, in the `Run` item, that a key past the first-pull cap returns a Retryable error and not an empty answer, and that the ledger then closes it as `expired`, which the page shows as a repairable gap.
7. `run.go` stays under 600 lines.
8. Docs: docs/design/bars-cap-retry.md#decisions.

## Checked by CI

- `go test ./internal/jobs/barsrefresh/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass.
- The tests are parallel, use fake clocks and sources, no sleep, no live service and no PostgreSQL.
- None of these is an acceptance item above.

## Checked by owner

On `/data-health` after the next bars run, a day whose bars keys the cap left behind shows `expired` with the repair button (not `no_value`); after the catch-up pass the count of such keys falls by up to 10 per queued request, and `/job-console` shows the requests.

## Left out

- A page text "not pulled yet, the cap is 10 a run" for these cells: the page shows `expired` and the existing repair text; a mark that tells the cap apart from other expiries needs a per-key reason in the page payload (a contract change). Not cut.
- Raising or removing `backfill_names_per_run`: a pacing value, the owner's word is not given.
- The deepen cap `deepen_names_per_run` and the adjusted cadence: other limits of the same job, not part of this outcome.
- Marking the key `missing` by a per-key `JobResult` field: an interface change across contracts, jobs and runner; see Decision 1.
- Pulling `no_value` keys in the catch-up pass: see Decision 1.

## Adversarial

Self-written, no agent spawned.

- Gap against the goal ("every key has bars or a real no-data answer"): a key still waiting stays `expired` until a request gives it a place; the pass queues at most one request per (job, day) per calendar day (docs/design/auto-catch-up.md Decision 3), so N waiting keys need about N/10 days if only one day is a gap, fewer with more gap days since each request has its own 10. Accepted: owner's no-time-guard dev mode, and the owner's button works at any time.
- Biggest unnamed risk: a Retryable error in a job result may be counted by something that alerts on retryable job failures. The runner reads it per key row, and the pull-pacing error (`TestPacedIsRetryable`) already uses the same kind, so no new alert path is opened; issue 1's worker confirms by the tests of the runner staying unchanged (no file of the runner is edited).
- Simpler way: none that touches fewer modules; the alternatives of Decision 1 each cross a row.
- Claim most likely false: that `closeRow` row 3 gives `expired` for a run whose error is Retryable and which ended after the close. It is read from `reconcile.go:479-505` (a Retryable newest run that did not end before the close); the case of runs that all ended before the close falls to "no row matches" and the key stays `missing`, which is also a repairable gap, so either state is shown with a button.
- Likeliest slip: the in-window retries of a denied key make several runs per day for the same key; they cost no broker request (the denial is before the pull), only the conid and stored-bar reads.
