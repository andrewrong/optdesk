# Job console: a page of `ot serve` on which the owner runs one job for one day, with force yes or no, and reads the result per key, also after a reload

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 2 (the advisor's round 1 is folded in and answered in `## Adversarial`)

Source: owner asks of 2026-11-12. optdesk-2879: "I don't want to operate in a terminal; I click on web pages; we are a product." optdesk-2881: "every job can be run on its own, and is idempotent." The follow-up named in `docs/design/flex-xml-import.md` (`## Left out`, "Follow-up design `job-console`").
Status: draft. Read on origin/main at 7dd0952af7eb. "[unverified]" marks a claim not checked in code, docs or a probe.

## Goal

On `/job-console` of `ot serve --owner-edit` the page lists the jobs, and for each says whether it runs for any past day or for today only. The owner chooses a job, a day and force yes or no, presses `Run`, and the page shows the request as `queued`, `running`, then `done`, `refused` or `failed`, with one line per key: what happened (stored, already present, nothing found, partial, failed, refused) and a reason code. After a reload the last requests and their per-key lines are still there. The run is done by the scheduler process, which holds the service env and the run lease; `ot serve` only writes one request row and reads results. No terminal step.

Idempotent means, here and in `## Idempotency of the jobs`: a second run of a job for a day adds no duplicate row; a row is changed only by the job's own revision or upsert rule on changed content. The table names the rule of each job and says which lines were read; the page does not claim more (Doubt 7).

## What main holds today

Each line is read on origin/main at 7dd0952af7eb unless marked.

- **`ot serve` has no runner line, no broker for jobs and no lease.** `runServe` opens the site readers and, with `--owner-edit`, one settings writer (`cmd/ot/serve.go:74-82`, `newOwnerEdit` at `cmd/ot/settings.go:53-77`, `ownerEditSecrets` at `cmd/ot/env.go:70-85`, which reads `OT_FORM_KEY` and `OT_PG_DSN_SETTINGS` only). `--live-quotes` gives it a pyside client for quotes (`cmd/ot/serve.go:48`, block at `:83-91`); that is not a job path. `store.DSNEnvRunner` is read by `run-once`, `backfill` and `scheduler` (`cmd/ot/runonce.go:70`, `cmd/ot/backfill.go:51`), not by `serve` or `site.go`. The only writes of `serve` are two posts, `/owner-settings` and `/owner-underlyings`, each behind `rd.edit` and `guardOwnerPost` (`cmd/ot/site.go:355-362`, `cmd/ot/settings.go:245-259`): loopback address, Host check, Origin check, MAC token with expiry, body cap 64 KiB.
- **The scheduler process holds what a run needs.** `ot scheduler` takes the instance fence, builds the wiring and runs the ticker (`cmd/ot/scheduler.go:85-91,133-188`). Every key run goes through `Reconcile` or `Backfill`, both under `ops.run_lease`; `Backfill` holds the lease for the whole call, under an owner of its own, and returns a `Retryable` error while another owner holds it (`internal/runner/reconcile/backfill.go:31-41`, `internal/store/ops/ledger.go:253`).
- **The ticker does not retry a held lease.** `reconcileAndSync` logs `reconcile failed` and returns (`internal/runner/sched/ticker.go:440-460`); the loop sets `mark = now` once `submit` accepted the turn (`ticker.go:236-247`), so the slot counts as covered, and the next try is the next 15-minute tick (`tickEvery`, `ticker.go:32`). A console run that holds the lease at a slot instant therefore delays that slot's keys by up to 15 minutes (Decision 8).
- **`Backfiller.Backfill(ctx, day, datasets)` has no force.** A `present` key is listed in `AlreadyPresent` and not run (`backfill.go:61-62`); a `missing` or `no_value` key is run once its window has closed, a key inside its window is only listed in `InWindow` and not run (`backfill.go:63-72`). The result has six lists without a reason per key (`internal/runner/api.go:54-70`); `Refused` is a whole-day value with one constant, `RefusedNoPlan` (`api.go:73-81`).
- **`RunKey` writes `no_value` for what the job calls "no value".** The verdict returns `no_value` for a job error of kind `NoValue` (`internal/runner/reconcile/runkey.go:176-177`) and `RunKey` then writes the mark (`runkey.go:100-103`), over a `present` mark too. The run row of the attempt (`store.Run`: `Status`, `ReturnedN`, `UsableN`, `ErrorCode`, `ReasonCode`, `internal/store/api.go:285-294`) says what the attempt did; the mark says only the verdict. An error that is not `NoValue` leaves `ErrorCode` set and no mark (`runkey.go:178-181`).
- **Nine job ids answer only for today and return `NoValue "wrong day"` for another day**, and the runner then marks that key `no_value`: `account_capture` (`internal/jobs/accountcapture/job.go:106-107`), `position_marks` (`positionmarks/run.go:122-123`), `quote_snapshot` (`quotesnapshot/run.go:98-101`), `option_eod` and `option_eod_jp` (`optioneod/run.go:119-120`, one type), `option_open` and `option_open_jp` (`optionopen/pull.go:217-218`, `jp.go:256-257`; `jpPull` embeds `*pull`), `earnings_refresh` (`earningsrefresh/run.go:234-236`) and `dividend_capture` (`dividendcapture/run.go:212-213`). The ask counted seven; the last two "refuse" a past day by the same `NoValue`, so they are marked too. A backfill of a past day thus records an explained absence for a key that was never tried, and that mark clears the gap of the day (`docs/design/backfill-any-day.md`, Decision 5).
- **The runner already has optional job interfaces of plain values**: `Windowed`, `Scheduled`, `Bounded`, `Postcondition`, `RestartSensitive` (`internal/runner/api.go:87-130`), written so that a job package need not import `runner` (the jobs row of `docs/design/modules.md` does not list `runner` as a dependency).
- **`option_candidates` is idempotent on main.** Its run id is derived from the day and the rows (`internal/jobs/optioncandidates/job.go:209`, `setRunID`), not from a random value (issue 1725); the earlier plain-INSERT premise of the audit of 2026-11-12 is stale. The job is therefore like the others: force is offered for every job.
- **`account_capture` is idempotent.** Its capture id is `alias:day:key` (`job.go:209`) and every insert is `ON CONFLICT DO NOTHING` (`internal/store/account/store.go:20-40`); a forced second run the same day adds nothing and does not refresh the stored capture.
- **`internal/store/api.go` is exactly 800 lines** (no growth without a ceiling in `docs/design/ratchet.json`, which no agent may write) and **`internal/store/ops` is an implementation package** (`ledger.go`, `gaps.go`, `scheduler.go`): `scripts/arch` calls a package declarations-only only when `api.go` is its one non-test file (`scripts/arch/main.go:150-175`), and `docs/design/modules.md` puts every implementation in a package apart from the one that declares it (`barpg`, `ownersetpg`, `underlyingpg`; the paragraph at line 27). So the new interface is its own declarations-only package, `internal/store/jobrequest`, with the implementation in `internal/store/jobrequestpg`.
- **`ot_w_settings` already writes the schema `settings`** (`migrations/0025*.sql`, `0033_underlying.sql`: `USAGE` on the schema, `INSERT, SELECT` on a table, `SELECT` on the `rd` view), and `ot_ro_pages` and `ot_runner` read its `rd` views. `cmd/ot/wire.go` holds every opener in `deps` and is 797 lines; `cmd/ot/scheduler_test.go` is 798 lines; `cmd/ot/site.go` is 594.

## Idempotency of the jobs

Read for the audit of 2026-11-12 on 3ee2a21ad79f; only `option_candidates` was re-read at 7dd0952af7eb. "Read" means the write path was read in code; "inferred" means from the package doc or the manifest.

| jobs | write rule | status |
|---|---|---|
| `account_capture`, `quote_snapshot`, `option_eod`, `option_eod_jp` | deterministic capture or snapshot id, `ON CONFLICT DO NOTHING` (`store/account/store.go:24-39`, `snapshot/quotesnapshot.go:24`, `snapshot/optioneod.go:28`) | read |
| `position_marks`, `earnings_refresh`, `dividend_capture` | key with an attempt counter, `DO NOTHING`; a re-run adds one row under the next attempt (`markpg/store.go:27-30`, `events/earnings.go:32`, `events/dividends.go:26`) | read; a forced run adds a row, not a duplicate |
| `option_open`, `option_open_jp`, `option_open_plan`, `option_open_plan_jp` | `plan_id` is a SHA1 of day, slot, symbol; `DO NOTHING` on the key (`optionopen/plan.go:252`, `queries.go:19,104`) | read |
| `option_open_resolve`, `option_open_resolve_jp`, `option_open_resolve_gap`, `underlying_resolve` | upsert on the contract key, settle each name once (`optionopen/queries.go:251-254`, `underlyingresolve/job.go:100`) | read for the upsert; day semantics of the resolve jobs inferred |
| `option_candidates` | deterministic run id, `ON CONFLICT DO NOTHING` (`job.go:209`, store half issue 1719) | read |
| `compute_figures`, `iv_history`, `bars_refresh`, `fx_refresh` | revision rule or `DO NOTHING` on `(instrument, day, basis, adjust_epoch)` (`computefigures/steps.go:250-252`, `reading/writer.go:15`, `barpg/store.go:27,30`) | read; the `iv_history` write path is inferred (same writer) |
| `fred_series`, `fundamentals` | `DO NOTHING` on the full business key; a vendor revision is a new row (`series/writer.go:29`, `fundamental/store.go:35`) | read |
| `calendar_refresh`, `option_ledger` | upsert (`sessionday/writer.go:19-21`); revision only when the attribute hash differs (`flexsectionpg/queries.go:12-20`) | read |

Nothing here was run twice against a database. A per-job "run twice, row counts equal" test on the `pg` tag is the proof the owner's sentence needs; it is not in this design (Left out) and the page does not say "idempotent" for a job.

## Decisions

1. **The page writes a request; the scheduler runs it.** `ot serve` inserts one row into `settings.job_request` (job id, day, force). The scheduler process, in a goroutine started in `runScheduler` next to the ticker, claims the oldest queued request, runs `Backfill` for that day and the job's datasets (`jobDatasets`, `cmd/ot/backfill.go:126-165`, resolves a job to its datasets), and stores the result in the same row. Why not `serve` shelling out to `ot backfill`: `serve` would need the runner DSN, the vendor keys and a gateway, which it holds none of; a page that can start a process is a larger authority than a page that can write one row. Why not a second process: the scheduler already owns the fence and the lease.
2. **One table, constraints enforced by the table.** `settings.job_request`: `id` (identity), `job_id`, `session_day`, `force`, `requested_at` (default `now()`), and the columns of the consumer: `state` (`queued` default, `running`, `done`, `refused`, `failed`), `started_at`, `finished_at`, `reason` (a code), `keys jsonb` (one object per key: `dataset`, `subject`, `group`, `run_status`, `error_code`, `reason_code`, `returned`, `usable`, `attempt`). A partial unique index `(job_id, session_day) WHERE state IN ('queued','running')` makes a double press one request: the insert is `ON CONFLICT DO NOTHING` followed by a read of the open row. The queue cap is in the insert itself: it inserts only while fewer than 5 requests are `queued` or `running`. The page reads the view `rd.job_request_r1` (every column named) with `OT_PG_DSN_PAGES`.
3. **Who may act, said plainly (second opinion).** Today the page token of `/owner-settings` lets whoever holds it change the owner's rules and names. After this design the same token also lets him queue a run of any job for any day in the last 60 days, which spends vendor quota (Alpha Vantage 25 a day), broker pacing and time, and with force appends rows; it can never delete or edit a row (the web role cannot). The token binds no route (`Check` has no route argument), so one token works for the three posts for 15 minutes. Reach is local: the remote address must be loopback, the Host a loopback name with the listen port, a cross-origin post is refused, and `serve` refuses `--owner-edit` on another address (`serve.go:52-54`). The web role is the existing `ot_w_settings` and the line `OT_PG_DSN_SETTINGS`, widened by `USAGE` on nothing new and `INSERT, SELECT` on one table in a schema it already holds; the runner role `ot_runner` gets `SELECT` and a column-level `UPDATE` (`state`, `started_at`, `finished_at`, `reason`, `keys`) on it. No new role, no new `.env` line, no `ot setup pg` change. A role of its own (`ot_w_jobrequest`) would narrow the web grant to the one table; it costs a change of `internal/store/setup`, an `.env` line and an owner step (Doubt 4). The owner's merge of this design is his word for the widened authority.
4. **Force is "run a present key again", and a console run never writes `no_value`.** `ForceBackfiller.BackfillWith(ctx, day, datasets, BackfillOptions{Force, RunInWindow})` does not skip a `present` key: it runs it as an extra attempt (`seq = runs+1`, `backfill.go:81`). The run is started with the reason code `console`, and `RunKey`, for that reason, writes the mark `present` when the verdict is present and writes nothing otherwise: a `present` mark is never overwritten, a key never tried stays unmarked, and no `no_value` is written by the console for any job. One conditional in `runkey.go`, no second write, no window in which a crash leaves `no_value` over good rows. Nothing is deleted. The ask said "clear the mark, then run"; clearing opens a window in which a present key looks missing to the gap syncer.
5. **The outcome of a key is read from the run row of the attempt, not from the mark.** With the mark kept, a forced run that fails, is cut or has a false postcondition leaves the old `present` untouched, so the mark cannot tell. `BackfillResult` gains `Keys []KeyRun`: key, group (the six old lists plus `refused`), and for a key that was run the `Status`, `ErrorCode`, `ReasonCode`, `ReturnedN`, `UsableN` and attempt of its new run row. The builder derives the one text per key in Go (Decision 12). The reason is always a code, never the text of a job error (`safeErrText`, `runkey.go:121-135`, exists for logs only).
6. **"Today only" is a capability of the job, checked by the runner.** `runner.LiveOnly` (one method, `LiveOnly() bool`, plain values) is declared in `internal/runner/api.go`; the nine ids implement it (entry 3). `BackfillWith` and `Backfill` refuse a key of a `LiveOnly` job when `day` is not the clock's date in the zone of the job's `Schedule()` (the zone each of these jobs compares against: `j.loc`, the manifest zone), with the code `live_only_past_day`; the key is not run and not marked. So `ot backfill --day <past>` no longer marks these nine `no_value`: it prints `refused live_only_past_day`. A live-only job on today after its slot or stop still returns its own `NoValue` (`after session`, `missed slot`); with Decision 4 the console shows it as nothing found and marks nothing. The jobs keep their own check as the second guard.
7. **A console run does not wait for its window.** `Backfill` lists a key inside its window as `InWindow` and does not run it (`backfill.go:63-72`), so "Run today" during the session would do nothing. The consumer sets `BackfillOptions.RunInWindow`; an empty answer before the close stays unmarked (`empty_before_close`, `runkey.go` verdict), so the scheduler's own slot still fills it. `in_window` is reported only by a call without the option.
8. **The consumer is bounded and yields to the schedule.** It is its own goroutine, not the ticker's worker, so its only protection of the morning chain is what it does itself. The facts: `Backfill` holds the lease for its whole call; a held lease costs a slot up to one tick (15 minutes); a key run is cut at `max(close, now) + bound` with a default bound of 30 minutes (`runkey.go:25-26,239-259`), so a run in window can outlast any wait. Rules, each a constant of the file with the reason beside it: (a) **a bound per request** of 10 minutes: the context of `BackfillWith` ends then, `RunKey` cuts the run (`partial` or `abandoned`), and the request ends `failed` with the code `cut` and the keys done so far; (b) **a pre-open rule**: the consumer claims nothing while any slot start of an enabled job (`sched.NextStarts` over the `runner.Slot` of the jobs) falls within the next 15 minutes, which is one tick, so a request started outside that margin has ended (10 minutes) before the slot instant and never holds the lease then; the request stays `queued` and is taken after the slot's Reconcile; (c) **a queue cap** of 5 open requests (Decision 2); (d) **a floor on the day**: a day more than 60 days before the market date is refused by the post (422, no row); (e) **a held lease** (`Retryable`) puts the request back to `queued` and the consumer polls again after 15 seconds of the clock; a request still queued 60 minutes after `requested_at` ends `failed` with the code `expired`. The numbers 10, 15, 60 and 5 are chosen, the relation between 10 and 15 is argued above; they are not measured (Doubt 1).
9. **The consumer follows one order.** Every 15 s of the fake-able clock (`clockwork`; a test advances it, none sleeps): `Claim` (`UPDATE ... SET state='running' WHERE id=$1 AND state='queued'`, one row or none); the job is resolved among the jobs the scheduler runs (the enabled set, `scheduler.go:146-155`; a job it holds out ends `refused` with `left_out`, an unknown id `unknown_job`, a job with no key on the day `no_key`); `BackfillWith`; `RefusedNoPlan` ends `refused` with `no_plan`; otherwise the keys are stored and the state is `done` (also when every key is refused or empty: the page shows the per-key lines, Decision 12); then `KeyGapSyncer.SyncKeys(day)` as `ot backfill` does (`cmd/ot/backfill.go:68`). At start, every `running` row is set `failed` with the code `restart` (the fence makes this process the only consumer); a drain ends the running request the same way. A request is never rerun by itself.
10. **The job list is static, checked by a test.** `cmd/ot` holds one table of job id and kind (`any_day`, `today_only`); a test over the jobs the wiring builds asserts that the ids are the same set and that exactly the nine of Decision 6 are `LiveOnly`. The scheduler does not publish a capability table (Left out), so the page cannot show beforehand that the scheduler holds a job out: the request ends `refused` `left_out` and says so. "Scheduler not running" is shown when the newest request is `queued` for over 2 minutes, which the builder reads from `requested_at` and its `generated_at`.
11. **A double press, a post and a reload.** `POST /owner-job-console` (fields `job`, `day`, `force`, token) answers 303 to `/job-console?request=<id>`, so a reload never repeats the post. A today-only job has no day field: the post takes today of the job's zone [the market date of `marketZone` for US jobs, the Tokyo date for the `_jp` ids, both computed in `cmd/ot`], and the runner re-checks it.
12. **The page.** `page.job_console.v1`: `title`, `sentence`, `form` (null when the route is off, else `post_path`, `expires`, `mac`), `gaps`, `jobs` (`id`, `kind`, `day` for a today-only job), `requests` (the newest 20: `id`, `job`, `day`, `force`, `state`, `state_text`, `at_text`, `reason`, `keys`), `selected` (the id of `?request=`, shown even when older than the 20; an unknown id gives the gap `no_such_request`) and `refresh_seconds` (5 while a shown request is `queued` or `running`, else 0). A key line has `dataset`, `subject`, `outcome` (`stored`, `already_present`, `nothing_found`, `partial`, `failed`, `refused`, `waiting`), `outcome_text` and `reason`, derived in Go from the group and the run row: `stored` is group `filled` with `usable` above 0; `nothing_found` is group `absent`, or a done run with `returned` 0 and no error; `partial` is a run cut with rows; `failed` carries the `error_code`; `refused` carries `live_only_past_day`; `waiting` is `in_window`. A request that is `done` with every key `refused`, `nothing_found` or `failed` shows a request-level line "nothing was stored" in `sentence`. The view computes nothing.
13. **The view and the stack.** Plain HTML forms that post to the route of the payload, as the underlyings view does (`web/src/pages/underlyings/view.tsx`); the only script is the reload timer driven by `refresh_seconds`. Lines for the front end follow `docs/adr/0034-frontend-stack.md` and `frontend-whitelist.json` revision 2026-11-06.1:
    - 按 frontend-stack：用 shadcn table, badge and button (`components/ui`) 做 the job list with its per-row form (job and token hidden, day, force, `Run`).
    - 按 frontend-stack：用 shadcn input (`type="date"`) and switch (`components/ui`) 做 the day field of an any-day job and the force switch; no `select`, no calendar (the calendar adds a package and is owner-only).
    - 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 the recent requests, each with its per-key table (shadcn table and badge).
    - 按 frontend-stack：用 shadcn alert and card (`components/ui`) 做 the gap lines and the request-level line.
    No new package, no hand-built component; the reload is `setTimeout` and `location.reload`.
14. **No CLI of its own.** `ot backfill --day D --job J` exists and keeps working; it gains the refusal lines and, like the console, no `--force` (the owner works on the page; Left out).

## Modules

One edit of `docs/design/modules.md` is made in this pull request: the entry cell of the store row gains `internal/store/jobrequest` (Decision 2; a directory, as the other cells do). The first matching glob of `docs/design/modules.md` for each path below.

| module | paths touched | change |
|---|---|---|
| runner (interface entry 0) | `internal/runner/api.go`, `internal/runner/api_test.go`, `docs/contracts/runner.md` | `LiveOnly`, `BackfillOptions`, `ForceBackfiller`, `KeyRun`, `BackfillResult.Keys` |
| store (interface entry 1) | `internal/store/jobrequest/api.go`, `internal/store/jobrequest/api_test.go`, `docs/contracts/store.md` | the request types and one interface, declarations only |
| runner (entry 2) | `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/backfill_force.go`, `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/backfill_force_test.go`, `internal/runner/reconcile/runkey_console_test.go` | `BackfillWith`, per-key rows, the refusal, the console reason |
| jobs (entry 3) | `capability.go` and `capability_test.go` in each of `internal/jobs/{accountcapture,positionmarks,quotesnapshot,optioneod,optionopen,earningsrefresh,dividendcapture}/`, `docs/contracts/jobs.md` | `LiveOnly` on nine ids |
| store (entry 4) | `migrations/0034_job_request.sql`, `internal/store/jobrequestpg/open.go`, `internal/store/jobrequestpg/store.go`, `internal/store/jobrequestpg/store_test.go`, `internal/store/jobrequestpg/store_pg_test.go`, `internal/store/setup/settings_pg_test.go`, `internal/store/setup/setup.go` | table, view, grants, implementation, the role comment and test |
| ot (entry 5) | `cmd/ot/jobconsole_consume.go`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_consume_test.go`, `cmd/ot/jobconsole_scheduler_test.go`, `cmd/ot/scheduler.go`, `cmd/ot/wire.go`, `cmd/ot/backfill.go`, `cmd/ot/backfill_refused_test.go`, `docs/contracts/ot/job_console_consumer.md` | the consumer, its start, the job table, the refusal lines |
| contracts and pages (interface entry 6) | `contracts/schemas/page.job_console.v1.json`, `web/src/generated/job_console.v1.ts`, `contracts/fixtures/page.job_console.v1.golden.json`, `contracts/fixtures/page.job_console.v1.bad-version.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/job_console_v1_test.go`, `docs/contracts/page-schemas.md`, `internal/pages/job_console/builder.go`, `internal/pages/job_console/text.go`, `internal/pages/job_console/builder_test.go`, `docs/contracts/pages.md` | the page schema, its type and its builder |
| ot and web (entry 7) | `cmd/ot/jobconsole_site.go`, `cmd/ot/jobconsole_post.go`, `cmd/ot/jobconsole_site_test.go`, `cmd/ot/jobconsole_post_test.go`, `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/job_console_page.md`, `web/src/pages/job_console/view.tsx`, `web/src/types/job_console.ts`, `web/test/pages/job_console/view.test.tsx` | the route, the post and the view |

## Interface

- Entry 0, in `internal/runner/api.go` (owner module: runner): `type LiveOnly interface{ LiveOnly() bool }`; `type KeyRun struct { Key contracts.ExpectedKey; Group string; Ran bool; Status, ErrorCode, ReasonCode string; Returned, Usable, Attempt int }` where `Group` is one of `already_present`, `filled`, `absent`, `missing`, `in_window`, `refused` and `Status` is the string form of `store.RunStatus`; refusal code constant `RefusedLiveOnlyPastDay` (a string, not a `BackfillRefusal`, which stays whole-day); `type BackfillOptions struct{ Force, RunInWindow bool }`; `type ForceBackfiller interface{ BackfillWith(ctx context.Context, day contracts.Day, datasets []string, opt BackfillOptions) (BackfillResult, error) }`; the new field `Keys []KeyRun` of `BackfillResult`. `api_test.go` has a fake that implements `ForceBackfiller` and a caller that uses it.
- Entry 1, in `internal/store/jobrequest/api.go` (owner module: store): types `Request`, `Row`, `KeyLine`, `State` and one interface `Requests` with `Enqueue(ctx, Request) (id int64, existing bool, err error)`, `Recent(ctx, limit int)`, `Get(ctx, id int64)` (web and pages roles), and `Next(ctx)`, `Claim(ctx, id, now)`, `Release(ctx, id)`, `Finish(ctx, id, State, reason string, keys []KeyLine, now)`, `FailRunning(ctx, reason string, now)` (runner role). Plain types and strings, no `runner` import. `api_test.go` has a fake and a caller.
- Entry 6: `page.job_console.v1` and the generated type `JobConsole`.

"Landed" means: the entry's pull request is merged to main and `git grep` of its declaration on origin/main finds it; an entry that uses it is cut only after.

## Issues

Eight entries, 0 to 7. 0, 1 and 6 are interface entries and land first. Order: 0, 1, 6, then 2, 3 and 4, then 5 (needs 0 to 4), then 7 (needs 1, 4, 5 and 6). Each lists every file it touches and builds alone on main plus the entries it is ordered after; no two entries share a file. Acceptance states results checkable with git or by reading code; a match that must not exist is `git grep -q -e PATTERN -- FILE` exiting with status 1 and is always paired with an item that the file exists (`git ls-files FILE` prints it); no item is a judgement or a count of diff lines; test runs are under `## Checked by CI`. Entries 6 and 7 each name two modules because the generated type and its builder, and the route and its view, are one result each; the cut asks the advisor's one-issue form.

### 0 runner-job-console-api (interface, runner)

Module: runner. Size: S.
Files: `internal/runner/api.go`, `internal/runner/api_test.go`, `docs/contracts/runner.md`
Design: docs/design/job-console.md#Decisions (4, 5, 6, 7)
Contracts: unchanged
Order: first.
Goal: the runner declares what a job may say about days, how a backfill is forced and run in its window, and a per-key answer with the run row's facts.
Acceptance:
1. `git grep -n -e "^type LiveOnly interface" -e "^type KeyRun struct" -e "^type BackfillOptions struct" -e "^type ForceBackfiller interface" -- internal/runner/api.go` prints four lines; `BackfillResult` has a field `Keys []KeyRun`; `Backfiller` and the six old fields are unchanged (`git diff origin/main -- internal/runner/api.go` has no removed line that holds `Backfill(` or one of the six field names).
2. `BackfillOptions` has exactly the fields `Force` and `RunInWindow`; the file holds the constant `RefusedLiveOnlyPastDay` with the value `live_only_past_day`.
3. `internal/runner/api.go` has no function body: `git grep -n -e "^func " -- internal/runner/api.go` prints no line (and the file exists).
4. `docs/contracts/runner.md` has a heading line `### LiveOnly` and one `### ForceBackfiller` with the text "a refused key is not marked" under them.
5. `api_test.go` has a fake type that implements `ForceBackfiller` and a function that takes `runner.ForceBackfiller`; each test calls `t.Parallel()`.

### 1 jobrequest-api (interface, store)

Module: store. Size: S.
Files: `internal/store/jobrequest/api.go` (new), `internal/store/jobrequest/api_test.go` (new), `docs/contracts/store.md`
Design: docs/design/job-console.md#Decisions (1, 2, 3, 8, 9)
Contracts: unchanged
Order: first.
Goal: the request queue and the read of the page are declared in a declarations-only package of their own.
Acceptance:
1. `git ls-files internal/store/jobrequest` lists `api.go` and `api_test.go` and no other non-test file; `api.go` declares the interface `Requests` with the eight methods of `## Interface` and the types `Request`, `Row`, `KeyLine`, `State`, and has no line that starts with `func ` and no import of `internal/runner`.
2. `State` has exactly the constants `queued`, `running`, `done`, `refused`, `failed`.
3. `git diff origin/main --stat -- internal/store/api.go` prints nothing; `docs/contracts/store.md` names which role reaches which methods (web role `ot_w_settings` and pages line for the first three, runner role `ot_runner` for the rest).
4. `api_test.go` has a fake that implements `Requests` and a function that takes it; each test calls `t.Parallel()`.

### 2 backfill-force-console (runner)

Module: runner. Size: M.
Files: `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/backfill_force.go` (new), `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/backfill_force_test.go` (new), `internal/runner/reconcile/runkey_console_test.go` (new)
Design: docs/design/job-console.md#Decisions (4, 5, 6, 7)
Contracts: unchanged
Order: after 0.
Goal: the backfiller implements `ForceBackfiller`; a console run never writes `no_value`, a live-only key on another day is refused without a mark, and every key has a row with the facts of its attempt.
Acceptance:
1. `git grep -n -e "runner.ForceBackfiller = " -- internal/runner/reconcile/backfill_force.go` prints one line; `Backfill` in `backfill.go` is `BackfillWith` with the zero options (the one `func (r *reconciler) Backfill(` calls `BackfillWith`).
2. For a call without options the six old lists hold what they held before: the cases of `backfill_test.go`, `backfill_addkeys_test.go` and `backfill_window_test.go` are unchanged (`git diff origin/main --stat` lists none of them).
3. Every selected key has one entry of `Keys`, in plan order; for a key that was run the entry carries the `Status`, `ErrorCode`, `ReasonCode`, `Returned`, `Usable` and attempt of the run row of that attempt, read from the ledger's `Runs` and never from the mark.
4. A key of a job that implements `runner.LiveOnly`, asked for a day that is not the clock's date in the zone of its `Scheduled.Schedule()`, gets group `refused` and the reason `live_only_past_day`; for it no run row, mark or window row is written; a `LiveOnly` job without `Scheduled` makes `BackfillWith` return an error that names the job id.
5. With `Force`, a `present` key is run as attempt `runs+1`. With `RunInWindow`, a key inside its window is run, and an empty answer before the close stays unmarked.
6. `RunKey` with the reason `console` writes the mark `present` for a present verdict and writes no mark for any other verdict; a `present` mark is never overwritten by a console run; with any other reason the code is as before: `git grep -n -e "reasonConsole" -- internal/runner/reconcile/runkey.go` prints the constant and its one use, and `git diff origin/main -- internal/runner/reconcile/runkey.go` removes no line.
7. `git ls-files internal/runner/reconcile/backfill_force.go` prints the file; `git grep -q -e "DeleteMark" -e "Err.Error()" -- internal/runner/reconcile/backfill_force.go` exits with status 1; `backfill.go` is at most 330 lines and each new file under 600 (`wc -l`).
8. Each new test calls `t.Parallel()` and uses the fake clock of the test kit; the tests cover a forced run that fails, one that is cut, one with a false postcondition, one with `no_value`, a live-only refusal and a key in window.

### 3 jobs-declare-live-only (jobs)

Module: jobs. Size: S.
Files: `internal/jobs/accountcapture/capability.go`, `internal/jobs/positionmarks/capability.go`, `internal/jobs/quotesnapshot/capability.go`, `internal/jobs/optioneod/capability.go`, `internal/jobs/optionopen/capability.go`, `internal/jobs/earningsrefresh/capability.go`, `internal/jobs/dividendcapture/capability.go` (all new), the seven `capability_test.go` beside them (new), `docs/contracts/jobs.md`
Design: docs/design/job-console.md#Decisions (6)
Contracts: unchanged
Order: after 0 (the method is the one `runner.LiveOnly` declares; these packages do not import `runner`).
Goal: the nine ids that answer for today only say so.
Acceptance:
1. `git grep -n "LiveOnly() bool" -- internal/jobs` lists exactly seven definitions: on the job types of `accountcapture`, `positionmarks`, `quotesnapshot`, `optioneod`, `earningsrefresh`, `dividendcapture` and on `pull` of `optionopen` (`jpPull` embeds it); none on `planJob` or `resolveJob`; each returns true.
2. `git grep -q -e "internal/runner\"" -- internal/jobs/accountcapture/capability.go internal/jobs/positionmarks/capability.go internal/jobs/quotesnapshot/capability.go internal/jobs/optioneod/capability.go internal/jobs/optionopen/capability.go internal/jobs/earningsrefresh/capability.go internal/jobs/dividendcapture/capability.go` exits with status 1, and `git ls-files` prints all seven files.
3. `git diff origin/main --name-only` lists only the files above: no line of any job's `Run` changes.
4. `docs/contracts/jobs.md` lists the nine job ids under a heading `### LiveOnly`.
5. Each `capability_test.go` builds the package's job as its other tests do and checks the method; each test calls `t.Parallel()`.

### 4 jobrequest-store (store)

Module: store. Size: M.
Files: `migrations/0034_job_request.sql` (new; the number is the next free one at the cut), `internal/store/jobrequestpg/open.go` (new), `internal/store/jobrequestpg/store.go` (new), `internal/store/jobrequestpg/store_test.go` (new), `internal/store/jobrequestpg/store_pg_test.go` (new, tag `pg`), `internal/store/setup/settings_pg_test.go`, `internal/store/setup/setup.go`
Design: docs/design/job-console.md#Decisions (2, 3, 8, 9)
Contracts: unchanged
Order: after 1. The owner's window runs `ot migrate` after the merge (rebuild, setup pg, migrate, setup pg).
Goal: `jobrequest.Requests` is implemented over one table in schema `settings` and one view in `rd`, with the grants of Decision 3.
Acceptance:
1. The migration creates `settings.job_request` with the columns of Decision 2, `state` checked against the five states, owned by `ot_owner`, and the partial unique index on `(job_id, session_day) WHERE state IN ('queued','running')`; its Down section drops the view and the table.
2. It creates `rd.job_request_r1` with every column named and grants `SELECT` on it to `ot_ro_pages`, `ot_w_settings` and `ot_runner`.
3. The grants to `ot_w_settings` in the file are `INSERT, SELECT` on `settings.job_request` and nothing else (`git grep -n "ot_w_settings" -- migrations/0034_job_request.sql` shows no line with `UPDATE` or `DELETE` and no other table); `ot_runner` gets `SELECT` and `UPDATE (state, started_at, finished_at, reason, keys)` on the table, no `DELETE`, and no grant in the file touches a table of schema `ops`.
4. `Enqueue` is `INSERT ... ON CONFLICT DO NOTHING` guarded by the cap of 5 open requests in the same statement, followed by a read of the open row for the same job and day; `Claim` is `UPDATE ... WHERE id = $1 AND state = 'queued'` and reports whether a row changed; `Finish` writes `state`, `finished_at`, `reason` and `keys` in one statement; `FailRunning` sets every `running` row `failed`.
5. `store_pg_test.go` stores a request and reads it `queued`, enqueues twice while open (one row, `existing` true), enqueues a sixth while five are open (refused), claims twice (second false), finishes it and reads its keys through the view, and shows `ot_w_settings` fails an `UPDATE` of `settings.job_request`.
6. `internal/store/setup/settings_pg_test.go` has a case that `ot_w_settings` can insert into `settings.job_request` and read nothing in schema `ops`, and the role comment at `setup.go:53` names the new table; each new Go file is under 600 lines and each test calls `t.Parallel()` and uses `t.TempDir()` where it writes a file.

### 5 ot-job-consumer (ot)

Module: ot. Size: L.
Files: `cmd/ot/jobconsole_consume.go` (new), `cmd/ot/jobconsole_jobs.go` (new), `cmd/ot/jobconsole_consume_test.go` (new), `cmd/ot/jobconsole_scheduler_test.go` (new), `cmd/ot/scheduler.go`, `cmd/ot/wire.go`, `cmd/ot/backfill.go`, `cmd/ot/backfill_refused_test.go` (new), `docs/contracts/ot/job_console_consumer.md` (new)
Design: docs/design/job-console.md#Decisions (1, 7, 8, 9, 10, 14)
Contracts: unchanged
Order: after 0, 1, 2, 3 and 4.
Goal: the scheduler process consumes the requests under the bounds of Decision 8, and `ot backfill` prints the refusals.
Acceptance:
1. `runScheduler` starts one goroutine that runs the consumer until `ctx` ends; the queue opener is one new field of `deps` in `wire.go` and one line in its default value, opened with the runner DSN the scheduler already holds, so `wire.go` is at most 800 lines (`wc -l`) and no new `.env` line exists (`git grep -n "OT_PG_DSN" -- cmd/ot/jobconsole_consume.go cmd/ot/jobconsole_jobs.go` prints no constant that is not on origin/main).
2. `jobconsole_consume.go` declares the constants of Decision 8 (`requestBound` 10 minutes, `yieldMargin` 15 minutes, `queueCap` 5, `dayFloor` 60 days, `expireAfter` 60 minutes, `pollEvery` 15 seconds), each with a comment that gives its reason.
3. A request is not claimed while a slot start of an enabled job (from `sched.NextStarts` over the `runner.Slot` values of the jobs) is within `yieldMargin`; a `Retryable` error puts the request back to `queued`; a request queued longer than `expireAfter` ends `failed` with `expired`; the context of `BackfillWith` ends at `requestBound` and the request ends `failed` with `cut`.
4. The order of Decision 9 holds: `Claim`, resolve among the enabled jobs (`left_out`, `unknown_job`, `no_key`), `BackfillWith` with `RunInWindow` and the options of the request, `no_plan` as `refused`, `Finish`, `SyncKeys`; at start `FailRunning` is called once with `restart`.
5. `jobconsole_jobs.go` holds the table of job ids and kinds; `jobconsole_consume_test.go` asserts, over the jobs the wiring builds as the other `wire_*_test.go` files do, that the ids of the table are the same set, that exactly the nine ids of Decision 6 satisfy `runner.LiveOnly` and each also satisfies `runner.Scheduled`.
6. `ot backfill` prints one line `refused <dataset> <reason>` per refused key; its exit rules are unchanged, and an invocation whose every selected key is refused prints the line `nothing was run` (`backfill_refused_test.go`).
7. The tests of the consumer are in the two new test files; `scheduler_test.go` is not in the diff (`git diff origin/main --name-only` does not list it); each new Go file is under 600 lines and `scheduler.go`, `backfill.go` and `wire.go` are at most 800; each test calls `t.Parallel()` and uses the fake clock, no sleep.

### 6 job-console-page (interface, contracts and pages)

Module: contracts and pages. Size: M.
Files: `contracts/schemas/page.job_console.v1.json` (new), `web/src/generated/job_console.v1.ts` (new, generated), `contracts/fixtures/page.job_console.v1.golden.json` (new), `contracts/fixtures/page.job_console.v1.bad-version.json` (new), `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/job_console_v1_test.go` (new), `docs/contracts/page-schemas.md`, `internal/pages/job_console/builder.go` (new), `internal/pages/job_console/text.go` (new), `internal/pages/job_console/builder_test.go` (new), `docs/contracts/pages.md`
Design: docs/design/job-console.md#Decisions (10, 12)
Contracts: page.job_console v1 (new)
Order: after 1 (the builder takes `jobrequest.Requests`).
Goal: the page's JSON schema and TypeScript type, and the builder that fills them, land together.
Acceptance:
1. The schema has `schema_version` 1 and the fields of Decision 12 with `additionalProperties: false`; `state` is an enum of the five states, `outcome` of `stored`, `already_present`, `nothing_found`, `partial`, `failed`, `refused`, `waiting`; `kind` of `any_day` and `today_only`.
2. `web/src/generated/job_console.v1.ts` is the output of `contracts/generate.sh` for the schema (source and output in one pull request) and declares `JobConsole`; `contracts/fixtures/MANIFEST.synthetic-v1.json` has a row with the sha256 of each of the two new fixtures.
3. The golden fixture holds one any-day job, one today-only job, a `done` request with a `stored` key, an `nothing_found` key and a `partial` key, a `refused` request with `live_only_past_day`, and a `failed` request with `cut`; the bad-version fixture has `schema_version` 2.
4. `Build` takes a `jobrequest.Requests` (or nil), the generated-at instant, the post path with its token fields (or nil), the selected id; it has no import of `internal/runner` and no call of a clock (`git grep -q -e "time.Now" -e "internal/runner" -- internal/pages/job_console` exits with status 1, and `git ls-files internal/pages/job_console` lists the three files).
5. With a nil reader the page has the gap `no_reader` and no request, status 200; `scheduler` is shown not running when the newest request is `queued` and older than 2 minutes at `generated_at`; `refresh_seconds` is 5 when a listed request is `queued` or `running`, else 0; a request with every key `refused`, `nothing_found` or `failed` yields the "nothing was stored" line (`builder_test.go` has a case for each).
6. Each new Go file is under 600 lines; each test calls `t.Parallel()`.

### 7 job-console-route-view (ot and web)

Module: ot and web. Size: M.
Files: `cmd/ot/jobconsole_site.go` (new), `cmd/ot/jobconsole_post.go` (new), `cmd/ot/jobconsole_site_test.go` (new), `cmd/ot/jobconsole_post_test.go` (new), `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/job_console_page.md` (new), `web/src/pages/job_console/view.tsx` (new), `web/src/types/job_console.ts` (new), `web/test/pages/job_console/view.test.tsx` (new)
Design: docs/design/job-console.md#Decisions (3, 10, 11, 12, 13)
Contracts: unchanged
Order: after 1, 4, 5 and 6.
Goal: `GET /job-console` serves the page and `POST /owner-job-console` queues a request behind the guard of the other posts; the view shows jobs, forms, requests and key lines.
Acceptance:
1. `jobconsole_site.go` registers the page by `registerPage` with the route `job-console`; its reader is a new field of `siteReaders` in `site.go`, opened once per process with `OT_PG_DSN_PAGES`; `site.go` gains that field and one branch for the route `owner-job-console` next to the two existing ones, and stays at most 620 lines.
2. `jobconsole_post.go` calls `guardOwnerPost` before it reads any field (the first call after the method check), answers 403 with no store opened when it fails and 405 for a method other than POST; the writer is opened with `OT_PG_DSN_SETTINGS` per post through the `deps` opener of entry 4's package and closed after the post.
3. The post answers 422 with a fixed text and no row for an unknown `job` (not in the table of entry 5), a `day` that is not `YYYY-MM-DD`, after the market date or more than 60 days before it; it takes the day of a today-only job from the job's zone, calls `Enqueue`, and answers 303 to `/job-console?request=<id>`; with a full queue it answers 429 with a fixed text.
4. No response, log line or error of the post carries a DSN, a key or an account id: `jobconsole_post_test.go` posts with a fake store whose errors hold a DSN-shaped text and asserts the response and the captured log lines hold none of it.
5. `check_serve.go` and `check_blocks.go` know the page (the `pageMetaKeys` entry as for `underlyings`); `docs/contracts/ot/pages/job_console_page.md` names the routes, lines and gaps.
6. `view.tsx` ends with the `page(...)` call of the shell as `web/src/pages/underlyings/view.tsx` does and imports only components of `@/components/ui` that exist on main (`table`, `badge`, `button`, `input`, `switch`, `accordion`, `alert`, `card`, `label`) and the generated type; `git diff origin/main -- web/package.json` prints nothing and `git ls-files web/src/pages/job_console/view.tsx` prints the file.
7. Each job row is one `<form method="post">` with hidden `job`, `expires`, `mac`, a date `Input` named `day` for an any-day job (for a today-only job the text "today" and the day of the payload), a `Switch` named `force`, and a `Button` `Run`; the view shows every key line's `outcome_text` and `reason`; `view.test.tsx` renders the golden fixture and finds the five state badges, the `partial`, `nothing_found` and `refused` lines with their reasons, and the "nothing was stored" line.
8. The view computes nothing: `git grep -q -e "toFixed" -e "reduce(" -e "new Date" -- web/src/pages/job_console/view.tsx` exits with status 1; when `refresh_seconds` is above 0 it reloads after that many seconds and when 0 it sets no timer (`view.test.tsx`).
9. Each new Go file is under 600 lines; each test calls `t.Parallel()`.

## Rollout

Entries 0, 1, 2, 3 and 6 need no owner step. Entry 4 holds a migration: after it merges the owner's window runs `ot migrate`; entries 5 and 7 are ordered after it. After entry 7 merges `ot serve` deploys itself with the other code merges and runs with `--owner-edit` as for `/settings`; entry 5 deploys the scheduler the same way. The owner then opens `/job-console`. No new role, no new `.env` line. From entry 2 on, `ot backfill` refuses the nine live-only keys of a past day (no `no_value`); between entry 2 and entry 5 it refuses without printing a line for them, and no exit code changes. The owner's merge of this design is his word for the widened authority of the settings token (Decision 3), for that behaviour change and for the one edit of `docs/design/modules.md`.

## Checked by CI

`go test` of `internal/runner/...`, `internal/store/jobrequestpg` and `internal/store/setup` (the tag `pg` runs only in GitHub CI), `internal/jobs/...`, `internal/contracts` (entry 6), `internal/pages/job_console`, `cmd/ot`; `tsc`, `eslint`, `vitest`, the build and the `type drift` step of the web workflow; `go run ./scripts/arch --check` (the new packages `internal/store/jobrequest` and `internal/store/jobrequestpg` are read by the tree); the ratchet; the `Contracts:` line check (entry 6: `Contracts: page.job_console v1 (new)`; every other entry and this pull request `Contracts: unchanged`). The bounds of the consumer (yield margin, cut, expiry, held lease, restart) are tested with fakes and the fake clock.

## Left out

- **A capability table published by the scheduler** (`job_capability`, a 60-second publisher, left-out jobs shown beforehand, `live_day`). The static job table with its drift test replaces it; the cost is Decision 10's `left_out` answer after the post.
- **A seven-valued outcome in the runner** and **`ForceUnsafe`**: the six old groups plus `refused` and the run row's facts are enough, and no job needs `ForceUnsafe` after issue 1725. An `interrupted` state: a restart is `failed` with `restart`.
- **A per-job "run twice, counts equal" test** on the `pg` tag, which is the proof of "idempotent" for the 24 jobs. It is one test per job writer and belongs to its own issues; until it exists the table above says what was read.
- **`ot backfill --force` and any CLI of the console.** The owner works on the page (Decision 14). If wanted, it is one flag that calls `BackfillWith`.
- **A "give up this gap" action** for a live key of a lost day. The refusal of Decision 6 leaves that gap open instead of closing it with a false `no_value`; closing it honestly is a separate owner act with its own record (Doubt 5).
- **Quota and cost lines per job** (Alpha Vantage, Flex generation delay, IB pacing): a table of 24 entries no module owns today. A forced run of `earnings_refresh` spends quota.
- **Cancel, schedule-for-later and bulk "all jobs for a day".** One job, one day, now.
- **The text of a job error** on the page: it may hold a value; codes and counts only.
- **A new narrow write role** `ot_w_jobrequest` (Decision 3, Doubt 4).

## Adversarial

Round 0 (a fresh read-only agent on revision 1; nothing needed to read this section is outside the file):
- *Gap ("Run today" lands in `in_window`).* Adopted: Decision 7, `RunInWindow`.
- *Biggest risk (the consumer in the scheduler process; a mark write-back).* Adopted: Decisions 4 and 8; the write-back is gone.
- *Simpler way (a scheduler socket; results from run rows, no table).* Not adopted: a socket gives `serve` the authority to start a run (Decision 1) and run rows hold neither refusals nor the owner's intent. Revision 2 cut the tables to one.
- *Lock-in, claim most likely false, likeliest slip.* Adopted: Decision 3, the window gate, migration number at the cut.
Round 1 of the advisor (F1 to F9 and the notes), revision 2:
- *F1, option_candidates is idempotent on main (`job.go:209`).* Adopted: `ForceUnsafe`, `force_unsafe`, `unsafe` and the "one of 24 fails" sentence are gone; the Goal is restated; "What main holds" says the audit's premise is stale.
- *F2, the consumer can delay the morning chain.* Adopted: Decision 8, argued from `backfill.go:31-41`, `ticker.go:32,236-247,440-460` and `runkey.go:25-26,239-259`: a per-request bound (10 minutes), a pre-open yield (15 minutes, one tick), a queue cap (5), a floor on the day (60 days), expiry; the numbers are chosen (Doubt 1) and the relation between two of them is argued.
- *F3, "Run today" after the slot marks `no_value` for a never-tried key.* Adopted: Decision 4: a console run (reason `console`) writes the mark `present` or nothing; no `no_value` from the console for any job.
- *F4, the outcome must come from the run row, not the mark; the crash window of the write-back.* Adopted: Decision 5 (`KeyRun` carries the run row's facts) and Decision 4 (no write-back, one conditional in `runkey.go`, which is now in entry 2's Files).
- *F5, `Enqueue` contradicts the grants; no unique index; the `ops` invariant.* Adopted: the table is in schema `settings` with a partial unique index and the cap in the insert; the web role gets `INSERT, SELECT` on that one table and nothing in `ops` (Decision 2, 3; entry 4 items 1 to 4).
- *F6, `internal/store/ops` is an implementation package.* Adopted: a declarations-only `internal/store/jobrequest` and `internal/store/jobrequestpg` (checked against `scripts/arch/main.go:150-175` and `docs/design/modules.md` line 27); the modules.md edit names the directory; Doubt 12 of revision 1 is gone.
- *F7, Files.* Adopted: `wire.go` (one field, one line, at most 800), `scheduler_test.go` untouched (new test files), `MANIFEST.synthetic-v1.json`, `setup.go` and `settings_pg_test.go`, `siteReaders` in `site.go`.
- *F8, refused, empty and partial must be visible.* Adopted: Decision 12 gives seven outcomes derived in Go with reasons, a request-level "nothing was stored" line, and `ot backfill` prints `nothing was run`.
- *F9, vacuous greps and judgement items.* Adopted: every "exits with status 1" item is paired with a `git ls-files` item; judgement items (fixed messages, paragraphs, no computed number) are replaced by results a test shows or a grep finds.
- *N1, the cut.* Adopted: eight entries; `job_capability`, the seven-valued `KeyResult`, `ForceUnsafe`, `interrupted` are in `## Left out`; one interface, one table.
- *N6, left-out jobs.* Adopted: the consumer resolves among the enabled set; a held-out job ends `refused` `left_out` (Decision 9, 10).
- *N7, idempotency per job.* Adopted: `## Idempotency of the jobs` names the rule and what was read; the test per job is Left out and the page makes no claim.
- *N2, serve.go cite.* Adopted: `serve.go:83-91`; the writer-per-post wording follows `underlyings_post.go` through a `deps` opener (entry 7 item 2).
- *N4, N5, the token.* Adopted: Decision 3 states plainly that the settings token now also queues runs.
No second fresh agent was run on revision 2; the whole file was read against origin/main at 7dd0952af7eb.

## Doubts (least sure)

1. The numbers of Decision 8 (10 minutes per request, 15-minute margin, 5 open, 60 days, 60 minutes to expire, 15 s poll) are chosen, not measured. The relation between the 10 minutes and the 15-minute margin is argued from `ticker.go:32,236-247,440-460`; whether 10 minutes is enough for a slow job is not known (a cut run is `failed` `cut` and can be asked again for the keys that remain).
2. A goroutine in `runScheduler`: a panic in it ends the process that runs all data collection [unverified: `runScheduler`'s behaviour on a panic in a goroutine is not read]. The consumer recovers per request and logs a fixed message.
3. The token is also a settings and underlyings token for 15 minutes, and the reverse (`Check` binds no route); a holder can queue runs. Stated as the change of who may act in Decision 3.
4. The web role is `ot_w_settings`, widened by one table. A narrower role is cleaner and costs an owner step; the design picks fewer steps.
5. A live key of a lost day stays a gap for good (Left out); the gap line of `ot scheduler status` stays red until the owner decides.
6. A console run writes no `no_value` for any job (Decision 4): a past-day key that genuinely has nothing at the source stays `missing` and its gap stays open, where `ot backfill` would have closed it; the page shows `nothing_found` and the owner can run `ot backfill` for that case.
7. "Idempotent" is stated per job from reading and inferred for four writers; nothing was run twice. A forced run of a `DO NOTHING` job reports `returned` and `usable` and cannot say that nothing new was stored; the page's `sentence` says so.
8. The page lists 20 requests; an older one is reachable only by `?request=<id>`.
9. `jobDatasets` runs `Plan` inside the scheduler for a request (`backfill.go:143`); a slow `Plan` that reads the broker blocks the consumer, not the ticker.
10. The job table (Decision 10) is a second list of job ids; the drift test is the only guard, and a new job is invisible on the page until the table gets a line.
11. A crash after `BackfillWith` and before `Finish` leaves a `running` row that the next start turns `failed` `restart`; the keys stored by that run are present, the per-key lines are lost.
12. The post's day for a today-only `_jp` job is the Tokyo date computed in `cmd/ot`; a mismatch with the zone of the job's `Schedule()` is refused by the runner (`live_only_past_day`), not stored as a wrong run.
