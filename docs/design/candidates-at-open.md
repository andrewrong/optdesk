# candidates at the open: option_candidates reads the option_open snapshot of the day and runs when option_open ends, not after the close

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-18
Revision: 2

Owner decision: 2026-11-18, asks optdesk-3564, optdesk-5031 and optdesk-5032 (plan item P2), owner word typed in the dispatcher window: the sell-put scan runs right after the open with the pre-market preparation already built (`option_list_preopen`, `option_open_resolve`, `option_open_plan`), and the candidates are on the page then, not after the close.
Read on origin/main at d4d59e7f: `AGENTS.md`, `docs/design/option-open-pull.md` (Decisions 3, 7, 8, 9, 10), `docs/design/option-chain-daily.md`, `docs/design/console-run-lock.md` (Decision 6, the order of jobs), `docs/design/scheduler.md`, `docs/design/modules.md`, `internal/jobs/optionopen/` (`job_pull.toml`, `job_plan.toml`, `job_resolve.toml`, `job_list_preopen.toml`, `planfunc.go`, `band.go`, `pull.go`), `internal/jobs/optioneod/` (`job.toml`, `run.go`), `internal/jobs/optioncandidates/` (`job.toml`, `job.go`, `input.go`, `calls.go`, `types.go`, `schedule.go`), `internal/compute/optioncand/` (`put.go`, `optioncand.go`), `internal/store/openquote/api.go`, `contracts/datasets/obs.option_open.r1.json`, `obs.option_eod.r1.json`, `cmd/ot/candidates.go`, `docs/contracts/jobs/option_candidates.md`, `docs/contracts/ot/jobs/option_candidates.md`.
Measured on the scheduler log (times in KL): `option_open` of 2026-11-16 ran 22:35:58 to 22:56:45, of 2026-11-17 22:35:18 to 22:56:36, each 40 keys (batches), 3405 and 3434 plan rows of which 1675 and 1961 ok rows; the plan job asks about 2100 contracts a day (`option_open_plan` returned 3514 rows, usable 2076, on 2026-11-18), so the pull asks about 100 contracts a minute. On 2026-11-18 it ran 22:36 to 23:14: batches 8 and 23 failed their first attempts (6 and 3 tries; "0 of 54 planned items ok, under the share 0.50") and were retried. `option_eod` of 2026-11-17 ran at open + 90 minutes and ended 01:16 KL (76 minutes). `option_candidates` takes 25 seconds. The plan job runs in 1 second.

## Goal

`option_candidates` for session day D is on the page about 28 minutes after the US open on a normal day (about 22:58 KL in winter), from the `option_open` snapshot of D, with the open interest of the last session and with a named fallback to `option_eod` when the snapshot is bad. Nothing waits for the close. On a day when a key of the pull is retried (2026-11-18: first pass to 23:05, last key ok 23:14), a first page comes when the first pass ends and the complete page comes by itself when the last key is ok, with no action of the owner (Decision 12).

## Today

- `option_candidates` is `session_close` plus 30 minutes (`job.toml`), `After()` is `compute_figures` (which is after `option_eod`). It reads `obs.option_eod` only: the chain day from the catalog (`OptionEodLatest`), the rows of that day through `OptionEod`, per instrument the row with the greatest generation and then the greatest `CaptureID` (`input.go:157-163`, and the same loop in `calls.go` `chainQuotes`).
- `option_open` (job id, manifest `job_pull.toml`) runs at `session_open` plus 5 minutes (09:35 ET), `After()` `option_open_plan`, 40 keys of at most 120 plan rows in chunks of 30, `run_bound` 25 minutes, `stop_after` 10:00 ET, `min_ok_share` 0.5. It writes `obs.option_open`. `openquote.OptionOpenReader.OptionOpen(ctx, day)` returns the newest row of each instrument of the day (greatest `received_at`, ties by slot and capture id).
- The plan (`job_plan.toml`): puts and held calls, 20 to 45 days to expiry, every listed strike in the band `spot * exp(-1.4 * sigma * sqrt(T))` up to the spot (at least 6 puts per expiry, at most 25 per expiry on a 1 dollar grid), ceilings 350 around the spot, 1400 puts, 1200 calls, 60 per stock; contracts over a ceiling or outside the band at the open spot are planned and stored with status `deferred` and not asked. The stored (resolved) band reaches `exp(-1.75 * sigma * sqrt(T))` or 25 percent below the close, whichever is nearer.
- `option_eod` is the in-session chain snapshot at `session_open` plus 90 minutes (`option-chain-daily.md`), with open interest and the model values; its strikes are the buy-price band (0.80 times the buy price up to the larger of 1.05 times the buy price and the spot, at most 24 strikes), and it resolves every contract itself, one `ResolveContract` at a time (`run.go` `listed`).
- Decision 7 of `option-open-pull.md` kept the open rows out of `option_eod` because the candidates job takes the newest capture by comparing random uuids. That is still the reason the open rows have their own table.
- The scheduler has dependent runs (`console-run-lock.md` Decision 6): a job whose `After()` names an upstream runs on day D at the later of its own slot and the moment its upstream ends for D, never before its own slot; an upstream that failed or ended without ok does not leave the dependent unrun, it runs now on what is stored and its key line carries `ReasonCode` `upstream_missing`. Nothing waits and nothing polls. The runner starts the dependent once: `dependentRun` returns when any key of the dependent has a run for the day (`internal/runner/reconcile/runkey.go`, the check "it ran for the day"), so a retry of the upstream that succeeds later does not start it again (Decision 12 changes this for a dependent marked `upstream_missing`). `trigger` runs after every turn of the upstream that is not cut, also a turn that left keys unrun, so the dependent can start before every key was tried.
- `option_candidates` has one key per issuer (33 keys, `job.go` `Plan`); each key calls `Run`, which ignores the key and writes the rows of every underlying under its own run id (log: every key `returned=3168`, about 1 second each). The page reads the rows of the newest run of the day only (`internal/store/candidate/read.go:16`), so a later run replaces an earlier one as a whole.
- A put row with open interest 0 is not blocked: `put.go` blocks `low_open_interest` only when the open interest is above 0 and below `min_open_interest`, and `optioncand.go` marks `no_open_interest` when it is 0.

## Decisions

### 1 Source and order

1. The put chain of an underlying comes from the `option_open` rows of D when they are usable, else from `option_eod` as today. The open rows are read through `openquote.OptionOpenReader` (dataset `obs.option_open`), so the choice is made by dataset: the reader's own rule (newest `received_at` per instrument) replaces the comparison of uuids for the open rows, and the random capture id of Decision 7 never decides between an open and an eod row. The eod path keeps its rule unchanged (one capture a day since `option-chain-daily.md`; see `## Left out`).
2. Usable open chain of an underlying (a value of `job.toml`, table `[source]`): at least `open_min_rows` (1) rows of the open snapshot of D with a put of 21 to 45 days (the `[params]` window), a status `ok` (1) or `partial` (2) with a bid and an ask, and, when the watch entry has a buy price, a strike at most `need_high` (1.05, the high of the `short_put` `[[need]]` row) times the buy price. Rows with status 7 (`deferred`, not asked) and 4 (`not_listed`) are not quotes and are dropped before the join, so the 1400 or so not-asked contracts a day never become `no_quote` rows; rows with status 3, 5, 6, 8, 9 stay and are `no_quote` as an eod row with no data is today. `QuoteOK` of an open row is status 1, or status 2 with a bid and an ask (the model IV is missing in a `partial` row; the rule uses bid, ask, bid size and the market data type, and the delta and IV are empty text then).
3. An underlying whose open rows are not usable, a run whose open read fails, a day with no open row and a job built with no open reader all take the eod path for that underlying: the newest chain day on or before D, exactly as today, so a name is never left with no candidate because the open failed (owner rule: a chained job collects from the stored input that is left when the first input is bad; the eod rows of D come at 12:16 ET, so at 10:00 ET the fallback is the close archive of the previous session, the same rows the job reads today, and it says so). Per underlying, not per run: a batch that failed in the pull costs only its own names. Every row of such an underlying carries the mark `eod_fallback`; `Skipped()` names the underlying and the chain day it fell back to, and one log line per run counts `open`, `eod_fallback` and `none`. A read error of the open reader is a warning (`component jobs`, `status degraded`), never a failed run.
4. One source per underlying and run, both rights: the covered-call and held-call rows (`NewWithCalls`) use the same source and chain day as the puts of that underlying (the open plan carries calls for held names, `calls = "held"`, so the held names are covered; an underlying that falls back falls back for both rights). `ChainDay` of a row from the open is D.
5. Schedule: `After()` becomes `option_open` (replacing `compute_figures`: the job reads no figure) and the slot `anchor = "session_open"`, `offset = "5m"`, `run_bound` stays 10 minutes. The scheduler supports dependent runs (Decision 6 of `console-run-lock.md`), so the candidates run starts when a turn of `option_open` ends for D, not at a fixed offset; the slot only keeps it from running before the open. A fixed offset (`session_open` plus 30 minutes) would have started a run on a pull that is still going or waited on a pull that ended at minute 21. A pull with a key that is not ok at that moment makes the candidates run start with `upstream_missing` and the open rows of that key missing, which the per-underlying rule of 3 covers, and Decision 12 runs the candidates again by itself when the last key is ok. The owner never presses "Run today" for this. The schedule switch is the last entry of the issues, after the source, the wiring, the rerun and the page hints, so no day runs the job at the open on the old rows only.
6. The owner's wish "5 minutes after the open" is the start of the pull, not of the scan: the scan needs the rows, which take 21 minutes at 100 contracts a minute on a normal day (11-16 ended 22:56:45, 11-17 22:56:36) and 30 minutes on 11-18 (batches 35 to 40 ran 22:59 to 23:05). The earliest honest time is the end of the first pass of the pull. Running the scan on the rows of the first batches while the pull goes on is `## Left out`.

### 2 Coverage

7. What the candidates need and what the plan holds:

| need | open plan | covered |
|---|---|---|
| puts only for the short put | puts and held calls | yes |
| 21 to 45 days | 20 to 45 days (list and resolve bands: 21 to 45 plus 4 days lookahead) | yes |
| every watched underlying incl. the 33 candidate subjects | the watch list and the held underlyings, the same two sources `option_eod` reads (Decision 2 of `option-open-pull.md`) | yes |
| bid, ask, bid size, delta, IV, market data type | all in `obs.option_open` (not `open_interest`) | yes |
| strikes 5 to 15 percent below the spot | the put band reaches about 14 percent at sigma 0.35 and 30 days (the stored band 17.5 percent, at most 25) | yes |
| strikes down to 0.80 times the buy price | only where the buy price is within the put band; below it the contracts are planned `deferred` or not resolved | no, for a name whose buy price is far below the spot |
| open interest | not stored at the open (Decision 7 of `option-open-pull.md`) | no |

8. Open interest. A joined value, not a lapse: for a put of the open snapshot the job takes the open interest of the newest `option_eod` row of the same instrument within the last 3 session days (`oi_lookback_sessions`, read through the `OptionEod` reader the job already holds, the rows of the underlying), and the row carries the mark `oi_prior_session`. Open interest changes once a day (the exchange clearing publishes it before the open), so the value is one or two sessions old at worst and the floor `min_open_interest` 10 stays a coarse liquidity floor on it; the rule is unchanged. A put with no eod row in that window has open interest 0: no block, the mark `no_open_interest` as today. Without this join the floor would be off for every open row and every row would read `no_open_interest`, and the page would show a rule that checks nothing.
9. The need band below the plan band is not widened here. Estimate: the pull asks 100 contracts a minute and ends at minute 21 of its bound of 25 (`stop_after` 10:00 ET), so the room is about 4 minutes, 400 contracts, 13 chunks of 30, and keys stay at 40 (a key holds up to 120 plan rows, deferred ones included). A buy band of 0.80 to 1.05 times the buy price holds about 20 strikes per expiry on a 1 dollar grid and 3 to 4 expiries, 60 to 80 contracts a name, 2000 to 2600 for 33 names, 20 to 26 minutes: it does not fit the bound, and the evening list (`band_k` 1.5, floor 10 percent) and resolve (`store_k` 1.75, `store_drop` 0.25) would have to be widened to the same band first, three jobs of the pre-market chain for names the first run may not need. What replaces it: the per-underlying fallback of Decision 3 (a name with no usable open put at or below 1.05 times its buy price takes the `option_eod` chain, whose strikes are the buy band), and a coverage line in the log of each run that counts, per underlying that took the open, the usable puts at or below 1.05 times the buy price, so the first runs measure how many names live on the fallback. If that count says the fallback is the rule and not the exception, the widening is a later design with numbers (`## Left out`).

### 3 option_eod

10. `option_eod` stays as it is: the in-session snapshot at open + 90 minutes, the record of the day (open interest, IV history and rank, the fallback source and the open-interest source of the candidates). It does not move and it is not retired. Making its `listed` use the resolved con_id cache (`master.option_contract`) is not small: it needs the underlying id and the contract identity of its own strikes in the cache, a new store in the constructors of `optioneod` (`NewWithWatch`, `NewWithSymbols`, the JP and index variants), the not-listed marks, and wiring in `cmd/ot`; and nothing waits on it any more (the candidates run at 22:58 KL, the eod ends 01:16 KL). It is `## Left out`.

### 4 Pages

11. Pages. The rows of D are written when the run ends, with `run_day` D, `ChainDay` D (an underlying on the fallback: the chain day it fell back to), `computed_at` of that run; the pages read them as they read the after-close rows today. The marks `eod_fallback` and `oi_prior_session` are stored text in `marks` and become part of what the page reads: renaming one later is a data and page change (lock-in, accepted). `markHints` of the option page (`internal/pages/option_candidates_page/strings.go`) knows four marks and prints an unknown one as stored (`cells.go` `marksCell`), so without a hint the owner would see the raw word `oi_prior_session` on nearly every open put row and `eod_fallback`, the only warning of a price from the previous close, as an unexplained token. Entry 4 adds the two hints next to the term ("Price is from the last close, the open pull had no usable quote for this stock", "Open interest is from an earlier session").
12. A late success of the pull starts the candidates again, by itself. The runner rule (entry 3, `internal/runner/reconcile`, Decision 6 of `console-run-lock.md` amended by this sentence): a dependent whose every key has, as its newest run of the day, a done run with `ReasonCode` `upstream_missing` is run once more when, by the run ledger, every key of every job in its `After()` is ok for the day. The check is a read of the ledger in `dependentRun`, which every turn of the upstream reaches (also a turn with nothing to run, each tick), so it needs no memory of an event and a restart loses nothing. The rerun is a plain run (reason empty, attempt n+1 through `RunKey`), so the newest run of each key no longer carries the mark and no further run follows; the old runs stay in the ledger. A dependent with no mark, a forced re-run of an upstream (`TestDependentRunsWhenLateUpstreamEnds` pins that it starts nobody) and a dependent that never ran are unchanged. A key of the pull that never recovers (the pull stops at 10:00 ET) leaves its underlyings on the fallback of Decision 3, named in `Skipped()` and the log line of the run; nothing is asked of the owner. Cost: 33 keys of about 1 second, and the page then reads the newest, complete run (see Today). Run per underlying instead was rejected: the job writes every underlying under each key and a pull batch holds issuers unrelated to the 33 keys, so a batch to issuer mapping would be a new join for the gain of 30 seconds.
13. Terms of the open rows. The open rows carry `InstrumentID`; strike, expiry and right come from `Terms.OptionTerms` as on the eod path (`input.go` `quotes`). An open row whose instrument has no terms is dropped from the chain of that underlying and counted in the `open` coverage line (never a failed run); if that leaves the underlying unusable it takes the fallback. The `OutsideBand` and `Deferred` flags of a row are not read: status 7 is the one test for `deferred`, as Decision 2 says.

## Expected timeline after the change (KL, US winter time; one hour earlier in US summer time)

| time KL (ET) | what |
|---|---|
| 19:15 (06:15) | `option_list_preopen` (unchanged) |
| 21:00 (08:00) | `option_open_resolve_gap` (unchanged) |
| 21:45 (08:45) | `option_open_plan`, 1 second (unchanged) |
| 22:30 (09:30) | US open |
| 22:35 (09:35) | `option_open` pulls, about 21 minutes, bound 25 minutes (`stop_after` 10:00 ET = 23:00 KL) |
| about 22:57 (09:57) | `option_open` ends; `option_candidates` starts at once, 25 seconds plus the open-interest join |
| about 22:58 (09:58) | candidates on the page, 28 minutes after the open |
| 00:00 to 01:16 (11:00 to 12:16) | `option_eod` (unchanged) |
| 05:30 (16:30) | the old candidates run; gone |

On 2026-11-18 (first pass 22:36 to 23:05, batches 8 and 23 failed and were retried, the last ok at 23:13:58) the plan would have given a first page when the first pass ended, with `upstream_missing` and the names of the failed keys on the fallback, and the complete page by itself at about 23:14, 38 minutes after the open, with nothing pressed by the owner. Before this change the candidates came at 05:30 KL, about 6.5 hours after the open, and from the eod rows of the day before's close.

## Modules

`docs/design/modules.md` does not change: no entry file, path or dependency is added. Four rows are used: jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entries 1 and 5), ot (`cmd/**`, `docs/contracts/ot/**`; entry 2), runner (`internal/runner/**`, `docs/contracts/runner.md`; entry 3) and pages (`internal/pages/**`; entry 4). `optioncandidates` imports the declarations-only package `internal/store/openquote`, which the jobs row already uses in `internal/jobs/optionopen`, and no other jobs package. No migration: the candidates job reads `rd.option_open_r1` through the role of the existing line `OT_PG_DSN_OPTION_OPEN` (`ot_w_option_open` holds `SELECT` on the view, `migrations/0029_option_open.sql:89`), opened by `cmd/ot` as the open jobs open it; the compute stores' role `ot_runner` has no grant on that view and none is added. `bookrisk` is `After()` `option_candidates` at close plus 45 minutes (`internal/jobs/bookrisk/job.go:168`); it still runs after the candidates of the day and reads nothing the move changes.

## Interface

None. `CandidateStores` (`internal/jobs/optioncandidates/types.go`) gains one optional field `Open openquote.OptionOpenReader` (nil: the job of today); the constructor types `NewOptionCandidatesJobFunc` and `NewOptionCandidatesCallsFunc` keep their signatures, so `internal/jobs/api.go` is not edited. `cmd/ot` gets the reader through a new field of its `deps` (a function value, nil leaves `Open` nil), not through a package variable. `internal/runner/api.go` is not edited: the rule of Decision 12 is inside `reconcile`. `Contracts: unchanged`.

## Issues

Each entry is one module. All entries: `Plan: P2`, `Design: docs/design/candidates-at-open.md#<n>`. The checked head is `origin/main` `d4d59e7f`. Order: 1 and 3 and 4 first (independent), 2 after 1, 5 after 1, 2, 3 and 4 (the switch is last, so no day runs the job at the open on the old rows). No entry edits `internal/jobs/api.go`, `internal/runner/api.go`, `internal/jobs/optionopen`, `internal/jobs/optioneod` or a migration.

### 1 candidates-source

Module: jobs. Size: M. Order: first.
Files: `internal/jobs/optioncandidates/source.go`, `internal/jobs/optioncandidates/source_test.go`, `internal/jobs/optioncandidates/input.go`, `internal/jobs/optioncandidates/calls.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/types.go`, `internal/jobs/optioncandidates/schedule.go`, `internal/jobs/optioncandidates/job.toml`, `internal/jobs/optioncandidates/job_test.go`, `docs/contracts/jobs/option_candidates.md`
The job reads the open snapshot of D per underlying with the fallback and joins the open interest of the last sessions; the schedule is not changed here.

### 2 candidates-open-wire

Module: ot. Size: S. Order: after 1.
Files: `cmd/ot/candidates.go`, `cmd/ot/candidates_test.go`, `cmd/ot/wire.go`, `docs/contracts/ot/jobs/option_candidates.md`
The wiring opens the option open store with `OT_PG_DSN_OPTION_OPEN` through a `deps` function and gives its reader to the job.

### 3 rerun-marked-dependent

Module: runner. Size: S. Order: first.
Files: `internal/runner/reconcile/rerun.go`, `internal/runner/reconcile/rerun_test.go`, `internal/runner/reconcile/runkey.go`, `docs/contracts/runner.md`
A dependent that ran marked `upstream_missing` runs once more when its upstream is ok for every key of the day.

### 4 candidate-mark-hints

Module: pages. Size: S. Order: first.
Files: `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/markhints_test.go`
The option page shows a hint for the marks `eod_fallback` and `oi_prior_session`.

### 5 candidates-at-open-switch

Module: jobs. Size: S. Order: after 1, 2, 3 and 4.
Files: `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/job.toml`, `internal/jobs/optioncandidates/schedule_test.go`, `internal/jobs/optioncandidates/job_test.go`, `docs/contracts/jobs/option_candidates.md`
The job runs after `option_open` at session open plus 5 minutes instead of close plus 30 minutes.

## Owner-visible results

What the owner sees, and what the window (not the owner) checks on the first day after the last entry is deployed, from the scheduler log lines after that deploy and from the page:

- The page of candidates for session day D has rows with `ChainDay` D from about 22:58 KL on a normal day, not at 05:30; the run line of `option_candidates` in the scheduler log starts at the end of the pull, not at close plus 30 minutes.
- On a day with a failed key, a second run of `option_candidates` for D appears in the log after the last key of `option_open` is ok, with no `upstream_missing` on its lines, and the page shows its rows (newest run); the owner pressed nothing.
- A name on the fallback shows the hint "Price is from the last close, ..." in its marks cell; an open put row shows "Open interest is from an earlier session"; no row shows a bare `eod_fallback` or `oi_prior_session` token.
- The log line of each run counts `open`, `eod_fallback` and `none` and the coverage of Decision 9; a day where most names are on the fallback is visible there and is the number the widening design waits for.
- Before the first day, nothing else changes: until entry 5 is merged the job runs at close plus 30 minutes as today.

These are read by the window after deploy; the entries' acceptance items are the code and git facts that make them true.

## Checked by CI

- `ci-go` for the five entries: build, vet, the tests of `internal/jobs/optioncandidates`, `cmd/ot`, `internal/runner/reconcile` and `internal/pages/option_candidates_page`, the architecture check (`optioncandidates` imports no other jobs package). Every new test uses `t.Parallel()`, fake stores and a fake clock, no sleep, no database and no broker.
- The live first run is the window's and is the list above; the dependent start after a multi-key upstream is Decision 6 of `console-run-lock.md` plus Decision 12 here.

## Left out

- Widening the open plan to the need band (0.80 to 1.05 times the buy price) and the list and resolve bands under it: estimate in Decision 9, 20 to 26 minutes for 33 names, over the 25-minute bound; a later design after the coverage line of the first runs gives the number of names that live on the fallback. Raising a class ceiling alone would not help: the contracts of that band are the planned `deferred` ones outside the put band.
- Running the scan per batch of the pull while it goes on (the dependent run starts when the upstream ends, not when a key ends): a runner change.
- A third run after `option_eod` ends (12:16 ET): not designed; the console "Run today" is the owner's way if he wants one and it reads the same rule, the open rows first.
- `option_eod` `listed` through the con_id cache (Decision 10), moving or retiring `option_eod`, and the random uuid tie-break of the eod path (`input.go:157-163`): one capture a day makes it harmless; replacing it by `received_at` is a store-reader matter (`option-chain-daily.md`).
- A legend of all marks on the pages; a grant of `rd.option_open_r1` to `ot_runner` (a migration) in place of reading through the open line.
- Japan: `option_candidates` gives no row for a JP entry (`split` in `job.go`), as today.


## Adversarial

Source: `adversarial-raw.md` in the draft directory (fresh-eyes read of revision 1 against origin/main d4d59e7f; the file is not part of the repository). Each point:

1. Gap: the owner pressed "Run today" after a retried key (Decision 5, Timeline). Adopted: Decision 12 and entry 3, a runner rule that reruns a dependent marked `upstream_missing` when the upstream is ok for the day; Decision 5 and the Timeline lose the owner step.
2. Gap: the 22:57 / 28 minutes claim holds for 11-16 and 11-17 only; 11-18 first pass ended about 23:05 and the last key was ok 23:13:58. Adopted: Goal, Decision 6 and the Timeline say so.
3. Risk: the dependent can start before every key was tried (`trigger` after any uncut turn). Adopted as covered by Decision 12 (the rerun is state-based), not by a start rule; the design no longer claims a start time it cannot prove.
4. Risk: stale fallback rows and bare mark words on the page. Adopted: Decision 11 and entry 4 (two hints), plus an owner-visible result.
5. Today: 33 keys each write every row; the newest run is read. Adopted as a statement in Today and Decision 12 (a rerun is complete).
6. Simpler way: none meets the goal; run per underlying rejected in Decision 12. Not adopted, reason given there.
7. Lock-in: mark names stored and read by the page. Accepted, stated in Decision 11. `bookrisk` order checked, stated in Modules.
8. False claim: the `openOptionOpenStores` package variable in tests breaks the parallel-test rule. Adopted: a `deps` field in entry 2 (`cmd/ot/wire.go`), nil leaves `Open` nil.
9. False claim: "the runner is not changed". Adopted: entry 3 and the Modules paragraph name the runner row.
10. Terms of the open instruments were not covered. Adopted: Decision 13 and an acceptance item in entry 1.
11. Issue cut: the schedule switch before the wiring would run the job at the open on yesterday's archive. Adopted: entry 5 is last and its Order line names 1, 2, 3 and 4; Files list the changed existing tests (`schedule_test.go`, `job_test.go`) and the new rerun tests go in a new file because `reconcile_test.go` has 800 lines and `runkey.go` 763.
