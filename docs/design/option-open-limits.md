# option_open: ask only the strikes an expiry lists, keep the not-listed marks, and drop every limit that IBKR does not set

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-25
Revision: 1

Source: owner asks optdesk-6202, optdesk-6204, optdesk-6205 and optdesk-6206 (2026-11-25), his words final; the read-only audit `state/optdesk-drafts/designs/option-open-audit/audit-2026-11-25.md` (origin/main 4b28845e). Amends `option-open-pull.md` Decision 5 (triggers (b) and (c) of the early end of a mark) and `## Capacity` (max_new), and `option-open-delta-band.md` Decision 4 (`ceil_total` 2050). Read on origin/main at 4b28845e: `internal/jobs/optionopen/{band.go,listed.go,core.go,pull.go,list.go,list_run.go,manifest.go,job_*.toml}`, `cmd/ot/wire_optionopen.go`, `pyside/src/optdesk/ibkr/{pacing.py,server.py,ibgateway.py,lister.py}`, `internal/broker/{api.go,grpcbroker/dial.go}`. Status: draft.

## Frame

Facts of the audit, re-read in the code:

- The evening resolve and the gap pass ask 1,000 to 4,000 contracts a day, 78 to 100 percent of them answered "no security definition" (gap 2026-11-25: 4,000 asked, 884 found, 3,116 not listed, 21 deferred). `candidates` in `band.go` applies the strike union of one chain (`ListChains` gives one union per trading class) to every expiry of the chain. A weekly has a coarser grid than the monthly, so about half of the pairs asked do not exist.
- The marks that would stop the repeat are deleted at the next run. `endMarks` in `listed.go` removes a mark when the same strike, right and class is stored at any other expiry (trigger (b), true for almost every strike of a liquid name) or, in the gap pass, when the chain union lists the strike (trigger (c), true for every mark of the evening pass because the candidates came from that union). Evening 2026-11-24 wrote 960 marks; 94 survive.
- The cap (`max_new` 4000 per day, `take`) is reached at 08:09 ET, 36 minutes before the plan; real new contracts are deferred behind not-listed ones. `ResolveInFlight` is 1 in `cmd/ot/wire_optionopen.go`, so the asks run one after the other.
- The plan keeps 2,050 items (`ceil_total`, `job_plan.toml`): 929 to 1,438 planned items a day are deferred before the open. The pull sends 30 quotes per call (`chunk`) and the pyside cap of quotes in flight is 50 (`DEFAULT_SNAPSHOTS`), below the 70 lines.
- `MessagePacer` in `pacing.py` counts a cancel as a start and drains cancels through `start()`, so every quote costs two of the 40 starts a second.

## Decisions

### Decision 1 (owner asks optdesk-6202, 6204, 6205, 6206, 2026-11-25): IBKR's limits are the only limits

The owner's words as relayed on 2026-11-25: "IBKR limits only: 100 market-data lines per account (use 60-70 in flight), request pacing 50/s counting only starts (a cancel is not a start; 3 violations end the session). No other self-imposed limits: drop max_new count caps; keep concurrency and pacing; a time budget may stay only where an ordering needs it, and anything not done is listed with its reason (no silent drop)." And: "Subscribe, take the data, cancel at once (current snapshot model) stays."

1. Count caps are removed: `max_new` of the resolve jobs (US 4,000, JP 300), `list_max_new` of the four list manifests, the plan cut `ceil_total`, and the env override `OT_OPEN_RESOLVE_MAX_NEW`. A value 0 means none, as the plan ceilings already say.
2. Concurrency and pacing stay and move to IBKR's numbers: 60 to 70 quotes in flight (chunk 60, pyside cap 70, lines 70), starts counted 40 a second (a margin of 10 under IBKR's 50, kept so that a burst never reaches three violations), a cancel takes no slot.
3. A time bound stays only where a later job needs the run to end: the gap pass ends before the plan at 08:45 ET (`budget_minutes` 40), the list ends before the gap (45 and 25 minutes), the pull stops at its `stop_after`. Each such bound is named in the table of Decision 6 with the job it protects.
4. What a run does not do is listed with its reason: the summary line and the stored reason of every deferred item (Decision 5).
5. The model "subscribe, take the data, cancel at once" is unchanged.

### Decision 2 (fault 1): a strike is asked at an expiry only if that expiry lists it

The candidates of an expiry come from the strikes that expiry holds, not from the union of the chain. `refs` (the stored contracts of the name, read by `ListOptions` in `keyRun.run`) give, for each expiry, the strikes already known listed there. The strikes of an expiry to ask come from `refs` alone (no new port): the strikes stored listed at that expiry define its grid (they are the contracts the list job and earlier resolves stored, so they are cached and not asked again), and the strikes of the chain union asked at that expiry are those that lie on that grid:

1. the step of the expiry is the smallest gap between two stored strikes of that expiry inside the band; the strikes of the union that are multiples of the step are asked, the cached ones skipped;
2. an expiry with no stored strike takes the step of the nearest stored expiry of the same kind (a third-Friday monthly takes the monthly step, any other expiry the weekly step); with no stored expiry at all, every strike of the union is asked, as today, and its answers write the marks of Decision 3.

The held contracts are still asked at their own expiry whatever the grid. Effect: the 3,116 pairs of 2026-11-25 that no weekly lists are not asked; the strikes of a new weekly are asked once, and a wrong grid costs a mark, not a daily repeat.

### Decision 3 (fault 2): the not-listed marks live until retry_at

Triggers (b) and (c) of `option-open-pull.md` Decision 5 are dropped. A mark ends at (a) `retry_at` (7 days, plus or minus 2), (d) the stored close moved 5 percent or more since `checked_at`, or the exact contract is stored listed (then it is cached and never asked). These are option-trader's three early re-ask triggers; a timeout or an ambiguous answer stores nothing (unchanged). A fourth, "a strike newly in the chain since `checked_at`", needs the chain of that day, which is not stored; it is in `## Left out`. A strike listed at another expiry says nothing about this expiry, and the chain union is the very list that produced the candidates. Effect: a pair answered "no security definition" is not asked again for about a week.

### Decision 4 (fault 5): the resolve asks in parallel, up to IBKR's pacing

The resolve jobs ask the candidates of a name with `parallel` workers (manifest key, 5; a manifest value the owner can change), each one fully specified `ResolveContract`; the broker client holds at most `ResolveInFlight` in flight (5 in the wiring, was 1, equal to the lane) and the pyside details lane stays at `DETAILS_MAX` 5 (start 2, 20 s timeout, 30 s pause). Evidence, not our guess: option-trader ran this exact setup (ADR 0003 line 55, #757, #1619, #793, #803; p90 538 ms), and a class-wide or strike-less details request overloaded the gateway twice and a timed-out details request cannot be cancelled. So the lane is the safety bound of the contract-details calls and the Go side must not throttle below it: `parallel` is 5 and `ResolveInFlight` 5. A timeout still counts toward `stopAfterTimeouts` (2 consecutive): the run stops asking and lists the rest as unsent. Marks, stores and counters are written by one goroutine, never by a worker.

### Decision 5 (fault 4): no plan cut, and a deferral always has its reason

`ceil_total` is 0 in `job_plan.toml` and in `DefaultPlanParams` (`planfunc.go`, which the manifest test compares with): the plan keeps every planned item (3,118 items on 2026-11-24, 929 of them cut today). The pull sends 60 quotes per call (was 30) and pyside runs 70 quotes in flight (was 50). An item the pull does not ask ends `deferred` with its code (`BUDGET` of the key's 8-minute lease, now also a code for a stop at `stop_after`), and the summary line of the pull carries the count of deferred items per code. The manifest `stop_after` "1000" is the value that runs; `option-open-pull.md` Decision 11 and `option-open-delta-band.md` Decision 6 name 10:30 only for the gated stage 2 of the delta-band design, so the two do not disagree today and this design does not move the value. Not isolated by the audit: why about 415 planned items inside the old cut ended status 7; the codes of this decision show it on the next night.

### Decision 6 (item 6): the limits that remain

| limit | where | after this design |
|---|---|---|
| `max_new` 4000 / 300 | job_resolve, job_gap, job_resolve_jp | removed (0 none) |
| `list_max_new` 60000 | four list manifests | removed |
| `OT_OPEN_RESOLVE_MAX_NEW` | cmd/ot/wire_optionopen.go | removed |
| `ceil_total` 2050 | job_plan.toml | 0 |
| `ResolveInFlight` 1 | wire_optionopen.go (US resolve and gap) | 5 (the lane); the JP resolve too; the list jobs keep 1 (they ask one at a time) |
| `chunk` 30 | job_pull.toml, job_pull_jp.toml | 60 |
| `DEFAULT_SNAPSHOTS` 50, `OT_IBKR_SNAPSHOTS` 50 | pacing.py, server.py | 70 |
| `DETAILS_MAX` 5, start 2 | pacing.py | kept: option-trader's evidence-based gateway safety (Decision 4) |
| cancel counted as a start | `MessagePacer` | not counted |
| `budget_minutes` gap 40 | job_gap.toml | kept: ends before the plan at 08:45 ET |
| `list_budget_minutes` 45 / 25 | list manifests | kept: the list ends before the gap |
| `budget_minutes` evening resolve 40 | job_resolve.toml | 300 (its window `lateness_hours` 5); no later job needs it earlier |
| `budget_minutes` pull 8 (key lease), `max_per_key` 120 | job_pull.toml | kept: a key is a lease of the runner; nothing is dropped, only deferred with a code |
| `stop_after` "1000" | job_pull.toml | kept: orders the pull before the candidates window; value agrees with the design text (Decision 5) |
| `stopAfterTimeouts` 2, details timeout 20 s, pause 30 s | listed.go, pacing.py | kept: they stop work on a hung gateway, they cap no work |
| `DEFAULT_STARTS_PER_SECOND` 40, `DEFAULT_LINES` 70 | pacing.py | kept: IBKR's 50 and 100 with margin |
| `retry_days` 7, `spot_move` 0.05, `lookahead_days` 4 | resolve manifests | kept: staleness rules of the marks and the window, not limits of work |
| `min_ok_share` 0.5 | job_pull.toml | kept: a check of the result, not a limit |

## Modules

jobs (`internal/jobs/optionopen`), ot (`cmd/ot/wire_optionopen.go`, its two job documents), pyside (`pacing.py`, `server.py` and their tests). No contracts, no migration, no `modules.md` change.

## Interface

None changed. No proto, no `internal/broker/api.go`, no `api.py`: the per-expiry call is not used (`## Left out`). The values the modules agree on are manifest and environment values: `parallel` (jobs reads it), `ResolveInFlight` (ot sets it, the broker client already honours it); `ResolveDeps` gets no new field, `OT_IBKR_SNAPSHOTS` (pyside).

## Capacity

Resolve (from the audit, 2026-11-25): about 3,100 not-listed asks of the gap pass fall away by Decision 2, and the not-listed asks that remain are not repeated for a week by Decision 3. Option-trader's expectation after the first fill is low hundreds of resolves a day. With 5 in flight and its p90 of 538 ms, 884 asks take about 884 / 5 x 0.54 s = 95 s (estimate); today they run one after the other at about 6 a second observed. Pull: 3,118 items at 60 in flight and a hold of 3 s a line is about 52 seconds of line time; the real figure is measured on the first night (estimate, not measured). Lines: 70 of 100, so 30 stay for the account's own use.

## Failure paths

- A weekly whose step is guessed wrong: asked strikes answer "no security definition" and are marked for a week (Decision 3); the cost is one pass, not a daily one.
- IBKR error 100 (message rate): the pacer margin (40 of 50) is the guard; the pyside entry logs the starts and cancels of the last second at each such error, so a violation is seen with its numbers.
- A worker timeout: two in a row stop the run; the rest is `unsent`, the run is Retryable, and the next run asks the same candidates (cache and marks hold the rest).

## Issues

### 1 option-limits-expiry-strikes

Module: jobs. Size: L. Order: first. Files: `internal/jobs/optionopen/band.go`, `internal/jobs/optionopen/listed.go`, `internal/jobs/optionopen/manifest.go`, `internal/jobs/optionopen/job_resolve.toml`, `internal/jobs/optionopen/job_gap.toml`, `internal/jobs/optionopen/job_resolve_jp.toml`, `internal/jobs/optionopen/expiry_grid.go` (new), `internal/jobs/optionopen/expiry_grid_test.go` (new), `internal/jobs/optionopen/listed_test.go`, `internal/jobs/optionopen/fakes_test.go`.

Acceptance:
1. A weekly whose stored strikes are 30, 35 and 40, for a union of 30, 31, 32, 35 and 45, is asked on its step of 5 only: a new test `TestWeeklyIsAskedOnItsStoredGrid` in `expiry_grid_test.go` expects no request for 31 or 32 and a request for 45, and the monthly (stored 30 to 35 in steps of 1) all of them.
2. A held contract is asked at its own expiry whatever the grid, and an expiry with no stored strike and no stored expiry of its kind asks the whole union: two new tests in `expiry_grid_test.go` pin both.
3. A mark is not removed because the same strike is stored at another expiry or because the chain union lists it: `TestStrikeListedAtAnotherExpiryEndsItsMark` and `TestStrikeNewlyInTheChainEndsItsMark` are replaced by `TestMarkSurvivesTheSameStrikeAtAnotherExpiry` (new) and `TestMarkSurvivesTheChainListingTheStrike` (new), and `TestMorningChainListingAMarkedStrikeEndsItsMarkInTheGapPassOnly` is removed; `endMarks` has neither trigger and `otherExpiry` and `chainLists` no longer exist.
4. A mark past `retry_at`, and every mark of a name whose close moved 5 percent, still end: `TestMarkedContractIsNotAskedBeforeItsRetryAt` and `TestSpotMoveOfFivePercentAsksAgain` pass unchanged.
5. A run with a day of 10,000 candidates asks all of them: `take` has no cap when `max_new` is 0, `TestMaxNewOf1500ResolvesAllThirteenHundredInOneRun`, `TestCapOf300StopsAtThreeHundredAndSaysSo` and `TestTheNearestStrikesAreResolvedFirstWhenTheCapBites` are replaced by one new test `TestNoCapAsksEveryCandidate`, and `job_resolve.toml`, `job_gap.toml` and `job_resolve_jp.toml` carry `max_new = 0`; the gap keeps `budget_minutes = 40`, the evening resolve has `budget_minutes = 300`.
6. The candidates of a name are asked by `parallel` workers: a new test `TestResolveAsksInParallelAndWritesInOrder` with 5 workers and a fake broker that holds each call expects up to 5 requests in flight, every answer stored once, the counters of the summary line equal to a one-worker run, and two consecutive timeouts still stop the run with the rest counted `unsent`.
7. The summary line of a resolve run names the number of candidates dropped by the grid (`grid_skipped=` in a new assertion of `TestResolveSlotValues`'s sibling `TestResolveSummaryCarriesGridSkipped` (new)), so a pair not asked is counted, not silent.
8. A contract stored on an earlier day is never asked again: a new test `TestStoredContractOfAnEarlierDayIsNotAsked` expects no request for a key held by the stored refs, whatever the day of the run.
9. Docs: docs/design/option-open-limits.md#1-option-limits-expiry-strikes.

### 2 option-limits-plan-pull

Module: jobs. Size: M. Order: first. Files: `internal/jobs/optionopen/planfunc.go`, `internal/jobs/optionopen/job_plan.toml`, `internal/jobs/optionopen/job_pull.toml`, `internal/jobs/optionopen/job_pull_jp.toml`, `internal/jobs/optionopen/job_list_preopen.toml`, `internal/jobs/optionopen/job_list_zone.toml`, `internal/jobs/optionopen/job_list_preopen_jp.toml`, `internal/jobs/optionopen/list.go`, `internal/jobs/optionopen/list_run.go`, `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/summary.go`, `internal/jobs/optionopen/ceiling_test.go`, `internal/jobs/optionopen/manifest_test.go`, `internal/jobs/optionopen/list_run_test.go`, `internal/jobs/optionopen/core_test.go`, `internal/jobs/optionopen/list_test.go`, `internal/jobs/optionopen/list_fresh_test.go`.

Acceptance:
1. The US plan keeps every planned item: `job_plan.toml` has `ceil_total = 0`, `DefaultPlanParams` in `planfunc.go` has `CeilTotal: 0`, and `TestPlanManifestParamsEqualDefaultPlanParams` passes with both; `TestJPPlanParamsKeepTheirCeilings` is changed to the new default or sets 2050 itself; `TestCeilTotalDefersTheTailOfTheRank` and the other tests of `ceiling_test.go` that pass an explicit ceiling still pass.
2. The pull sends 60 quotes per call: `job_pull.toml` and `job_pull_jp.toml` have `chunk = 60`, and a new test `TestPullSendsSixtyPerCall` in `core_test.go` expects a call of 60 contracts for a 130-item key.
3. A list run has no cap: `list_max_new` is 0 in the four list manifests, `parseList` accepts 0, and a new test `TestListRunWithNoCapListsEveryGroup` in `list_run_test.go` lists 70,000 stored-new contracts in one run.
4. An item the pull does not ask ends `deferred` with a stored code, and the summary line of the pull gives the count per code: a new test `TestDeferredItemsCarryTheirCodeInTheSummary` in `core_test.go` runs a key past its budget and expects every item not asked to hold `BUDGET` and the line to name the count.
5. `stop_after` and `budget_minutes` of `job_pull.toml` are unchanged (`1000`, 8).
6. Docs: docs/design/option-open-limits.md#2-option-limits-plan-pull.

### 3 option-limits-wiring

Module: ot. Size: S. Order: first. Files: `cmd/ot/wire_optionopen.go`, `cmd/ot/wire_optionopen_test.go`, `docs/contracts/ot/jobs/option_open_resolve.md`, `docs/contracts/ot/jobs/option_open_resolve_gap.md`.

Acceptance:
1. The US resolve, the gap pass and the JP resolve dial the broker with `ResolveInFlight: 5` (equal to `parallel` 5 of their manifests); the list jobs keep 1.
2. `OT_OPEN_RESOLVE_MAX_NEW` is not read: `openResolveMaxNew` and `envOpenResolveMaxNew` no longer exist, and a new test `TestResolveJobsAreBuiltWithNoMaxNew` in `wire_optionopen_test.go` expects the options passed to the job constructors to carry no `MaxNew` even when the variable is set (the manifest value is the jobs entry's).
3. The two job documents name no cap and say that `max_new` is 0.
4. Docs: docs/design/option-open-limits.md#3-option-limits-wiring.

### 4 option-limits-pyside-pacing

Module: pyside. Size: M. Order: first. Files: `pyside/src/optdesk/ibkr/pacing.py`, `pyside/src/optdesk/ibkr/server.py`, `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/tests/ibkr/test_pacing.py`, `pyside/tests/ibkr/test_server.py`, `pyside/tests/ibkr/test_ibgateway.py`.

Acceptance:
1. A cancel takes no slot: `test_pacer_counts_cancels` is replaced by a new test `test_a_cancel_takes_no_start_slot` that sends 40 starts and 40 cancels in one second and expects all cancels sent at once and the 41st start waiting; the docstring of `MessagePacer` says a cancel is not a start, and a cancel in a finally still never waits (`test_cancel_in_finally_does_not_block` passes).
2. Quotes in flight are 70: `DEFAULT_SNAPSHOTS` is 70, the default of `OT_IBKR_SNAPSHOTS` in `server.py` is 70, and `test_snapshot_cap_is_fifty` is replaced by a new `test_snapshot_cap_is_seventy`.
3. The details lane is unchanged: `DETAILS_MAX` stays 5 and `test_lane_never_rises_above_max` and `test_lane_starts_at_two` pass unchanged.
4. A gateway error 100 is logged with the starts and the cancels of the last second (a new test `test_error_100_logs_the_last_second` pins the line), and nothing else changes in the handling of errors.
5. `AmbiguousResolve` and `resolve` are unchanged.
6. Docs: docs/design/option-open-limits.md#4-option-limits-pyside-pacing.

## Checked by CI

- `go test ./internal/jobs/optionopen/... ./cmd/ot/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass; `uv sync --locked`, `ruff check`, `mypy --strict` and `pytest -n auto` pass in `pyside/`.
- The tests are parallel with a temporary directory per file, a fake clock, no sleep, no live broker and no PostgreSQL.
- None of these is an acceptance item above.

## Left out

- A per-expiry resolve (one strike-less `reqContractDetails` per name and expiry): rejected, open requests overload the gateway (option-trader #776, ADR 0003: a details request with strike or right left open burst 148 open requests). Every resolve stays fully specified; the saving comes from knowing the listed strikes per expiry first (Decision 2). The proto `ContractsService.ListContracts` that already exists is not used by the resolve jobs.
- A fourth early end of a mark, "a strike newly in the chain since `checked_at`": the chain of the mark's day is not stored, so it cannot be tested; a new strike of an expiry is asked anyway as a candidate of Decision 2.
- Item 8 of the audit: `runDependent` writes no `ops.job_run` row; it is a separate registered draft.
- Reading the per-expiry records of the list store in the resolve (a listing as the first source of the strikes of an expiry): it needs a new port in `ResolveDeps` and its wiring in `cmd/ot`, two modules; not cut, the stored contracts (`refs`) give the grid.
- The overlap of the list job and the resolve (the same band asked twice, by two jobs): one band for both is a decision for the owner, not cut here.
- F4 (the cap counter resets on restart; the gap run at 16:00 ET) falls away with the cap; the 16:00 run itself is not touched.
- F8 (the second `option_list_preopen` run lists nothing): the audit is uncertain; needs its own look.
- JP resolve: it runs the same code, so every change applies to it and no JP-specific work is cut: the cap is removed (`max_new` 0), triggers (b) and (c) of the marks are dropped, the grid rule of Decision 2 applies, `parallel` is 5 and `ResolveInFlight` is 5. Its budget (20 minutes) is unchanged.

## Adversarial

Self-attack by the author on six questions, no agent spawned.

- Gap against the goal: the owner wants no silent drop; the grid rule drops candidates that are never asked. Adopted: acceptance 7 of issue 1 counts them in the summary line (`grid_skipped`). Not adopted: writing a mark for each, because a mark would claim IBKR answered.
- Biggest unnamed risk: cancels not counted. If IBKR in fact counts every message, 40 starts and up to 40 cancels a second is 80 and three violations end the session. The owner decided "starts only"; adopted as written, with the 40 margin kept and the error 100 log of issue 4 so the first violation shows its numbers. Not adopted: a cancel counter that slows starts, since the owner said no.
- Simpler way: ask only the strikes stored at that expiry (no grid guess). Not adopted: new real contracts (884 on 2026-11-25) would never be found. Another simpler way, one band for the list job and the resolve, is in `## Left out` for the owner.
- Lock-in: `parallel` and `ResolveInFlight` are values, not a shape; the per-expiry call stays possible later because no interface was closed.
- Claim most likely false: the grid rule of Decision 2. It rests on the one NVDA example of the audit (12-24 stored 38 calls and 38 puts, 32 and 31 not listed) and on "weeklies are coarser"; it is not checked on the other 52 names. A wrong guess costs one pass of marks.
- Likeliest slip: issue 1 is size L in two files `listed.go` (629 lines) and `band.go`; a new file `expiry_grid.go` carries the grid so that `listed.go` does not grow past 800 lines; if it still does, the pull request reports it.
