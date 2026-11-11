# Job console: a page of `ot serve` on which the owner runs one job for one day, with force yes or no, and reads the result per key, also after a reload

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 1

Source: owner asks of 2026-11-12. optdesk-2879: "I don't want to operate in a terminal; I click on web pages; we are a product." optdesk-2881: "every job can be run on its own, and is idempotent." The follow-up named in `docs/design/flex-xml-import.md` (`## Left out`, "Follow-up design `job-console`").
Status: draft. Read on origin/main at 3ee2a21ad79f. "[unverified]" marks a claim not checked in code, docs or a probe.

## Goal

On `/job-console` of `ot serve --owner-edit` the page lists every job the scheduler runs, with what each may be asked: any past day, or today only; force allowed or not. The owner chooses a job, a day and force yes or no, presses `Run`, and the page shows the request as `queued`, `running`, then `done` (or `refused` with the reason), with one line per key: the outcome and a reason code. After a reload the last requests and their per-key lines are still there. The run is done by the scheduler process, which holds the service env and the run lease; `ot serve` only writes a request and reads results. No terminal step.

Idempotent means here: a second run of a job for a day adds no duplicate row and changes no stored row except by the job's own revision or upsert rule on changed content. One of 24 jobs fails that today (`option_candidates`, Decision 8); the console states it per job and does not offer force for it.

## What main holds today

Each line is read on origin/main at 3ee2a21ad79f.

- **`ot serve` has no runner line, no broker for jobs and no lease.** `runServe` opens the site readers and, with `--owner-edit`, one settings writer (`cmd/ot/serve.go:73-82`, `newOwnerEdit` at `cmd/ot/settings.go:53-77`, `ownerEditSecrets` at `cmd/ot/env.go:70-85` reads `OT_FORM_KEY` and `OT_PG_DSN_SETTINGS` only). `--live-quotes` gives it a pyside client for quotes (`cmd/ot/serve.go:48,79-86`); that is not a job path. `store.DSNEnvRunner` is read by `run-once`, `backfill` and `scheduler` (`cmd/ot/runonce.go:70`, `cmd/ot/backfill.go:51`), not by `serve` or `site.go`. The only writes of `serve` are two posts, `/owner-settings` and `/owner-underlyings`, each behind `rd.edit` and `guardOwnerPost` (`cmd/ot/site.go:355-362`, `cmd/ot/settings.go:245-259`): loopback address, Host check, Origin check, MAC token with expiry, body cap 64 KiB.
- **The scheduler process holds what a run needs.** `ot scheduler` takes the instance fence, builds the wiring with the env and runs the ticker (`cmd/ot/scheduler.go:85-91,133-188`). A key run goes through `Reconcile` or `Backfill`, both under `ops.run_lease`; `Backfill` takes the lease under an owner of its own and returns a `Retryable` error while another owner holds it (`internal/runner/reconcile/backfill.go:31-37`, `internal/store/ops/ledger.go:253`).
- **`Backfiller.Backfill(ctx, day, datasets)` has no force.** A key whose mark is `present` is listed in `AlreadyPresent` and not run (`backfill.go:61-62`); a `missing` or `no_value` key is run once its window has closed. The result lists keys in six groups without a reason per key (`internal/runner/api.go:54-70`). `Refused` is a whole-day value with one constant, `RefusedNoPlan` (`api.go:73-81`). `RunKey` marks the key `present` or `no_value` after the verdict, so a forced run that ends `no_value` would overwrite a `present` mark (`internal/runner/reconcile/runkey.go:100-103`, verdict at `:172-198`).
- **Nine job ids answer only for today and return `NoValue "wrong day"` for another day**, and the runner then marks that key `no_value`: `account_capture` (`internal/jobs/accountcapture/job.go:106-107`), `position_marks` (`positionmarks/run.go:122-123`), `quote_snapshot` (`quotesnapshot/run.go:98-101`), `option_eod` and `option_eod_jp` (`optioneod/run.go:119-120`), `option_open` (`optionopen/pull.go:217-218`), `option_open_jp` (`optionopen/jp.go:256-257`), and also `earnings_refresh` (`earningsrefresh/run.go:234-236`) and `dividend_capture` (`dividendcapture/run.go:212-213`). The last two "refuse" a past day, but by the same `NoValue`, so a backfill of a past day marks their key `no_value` as well; the ask counted seven, the code has nine ids. A backfill of a past day therefore records an explained absence for a key that was never tried, and that mark clears the gap of the day (`docs/design/backfill-any-day.md`, Decision 5).
- **The runner already has optional job interfaces of plain values**: `Windowed`, `Scheduled`, `Bounded`, `Postcondition`, `RestartSensitive` (`internal/runner/api.go:87-130`), written so that a job package need not import `runner` (the jobs row of `docs/design/modules.md` does not list `runner` as a dependency).
- **`option_candidates` is not idempotent.** `Run` makes `runID := uuid.NewString()` per run (`internal/jobs/optioncandidates/job.go:172`), so a second run appends a second full set under another run id. The store half is closed (issue 1719: inserting the same rows twice stores nothing new); the job half, a deterministic run id, is a registered job draft and is not in this design.
- **`account_capture` is idempotent.** Its capture id is `alias:day:key` (`job.go:209`) and every insert is `ON CONFLICT DO NOTHING` (`internal/store/account/store.go:20-40`); the `At` of the capture comes from the clock, the id does not. A forced second run the same day adds nothing and does not refresh the stored capture.
- **`internal/store/api.go` is exactly 800 lines**, so a line added to `store.OpsLedger` there needs a ceiling entry in `docs/design/ratchet.json`, which no agent may write. The new store interface is therefore its own `api.go` in the package that implements it, as `docs/design/flex-xml-import.md` Decision 8 does for `optionledger`. `internal/runner/api.go` has 185 lines.
- The other 14 jobs are idempotent on re-run by their own writers (`ON CONFLICT DO NOTHING` on a business key, a revision rule, or an upsert), as read for the audit of 2026-11-12 [inferred for the write path of `iv_history`, and for the day semantics of `option_open_resolve*`; neither was read line by line]. Entry 4 fixes the one set that is not.

## Decisions

1. **The page writes a request; the scheduler runs it.** `ot serve` inserts one row into `ops.job_request` (job id, day, force, time) and does nothing else. The scheduler process, in a goroutine started in `runScheduler` next to the ticker, polls for requests without a state row, claims one, runs `Backfill` for that day and that job's datasets (`jobDatasets`, `cmd/ot/backfill.go:126-165`, is the existing resolution of a job to its datasets), and stores the outcome. Why not `serve` shelling out to `ot backfill`: `serve` would need the runner DSN, the vendor keys and a gateway, which it holds none of; a page that can start a process is a larger authority than a page that can write one row. Why not a second process with its own timer: the scheduler already owns the fence and the lease, and a request must not run beside a tick.
2. **State is append-mostly and the page reads views.** Tables in schema `ops` (migration entry 2): `ops.job_request` (insert-only, written by the web role), `ops.job_request_state` (one row per claimed request, written by `ot_runner`; the primary key on `request_id` makes the claim a single `INSERT ... ON CONFLICT DO NOTHING`), `ops.job_request_key` (one row per key of a finished request), `ops.job_capability` (one row per job, upserted by the scheduler). The page reads three views `rd.job_request_r1`, `rd.job_request_key_r1`, `rd.job_capability_r1` with `OT_PG_DSN_PAGES` (role `ot_ro_pages`, which reads `rd` and nothing in `ops`, `migrations/0003_ops_ledger.sql:87`). States: `queued` (no state row), `running`, `done`, `refused`, `failed`, `interrupted`.
3. **Who may write (second opinion).** The web side writes with the existing role `ot_w_settings` and the line `OT_PG_DSN_SETTINGS` that `--owner-edit` already needs: the migration grants it `INSERT, SELECT` on `ops.job_request` and `USAGE` on schema `ops`, nothing else in `ops`. No new role, no new `.env` line, no `ot setup pg` change. The route exists where the two settings posts exist (`rd.edit` set: `--owner-edit`, loopback, `OT_FORM_KEY`, the settings DSN) and goes through `guardOwnerPost`. What a holder of the page token gains is the ability to queue a run of any job for any day: a run that costs vendor quota and broker pacing and, with force, appends rows, but never deletes or edits one (the role cannot, and the runner path only appends and marks). The token is a settings token and the reverse, as for `/owner-underlyings` (Doubt 3). A new role `ot_w_jobrequest` would narrow the grant to one table; it costs a change of `internal/store/setup`, a new `.env` line and an owner step, and is Doubt 4.
4. **A request is one job and one day; a double press is one request.** `Enqueue` inserts only when no request of the same job and day is without a state row or is `running`; otherwise it returns that request's id with `existing = true`, and the page redirects to it. After `done`, `refused`, `failed` or `interrupted` the owner may ask again. A post is `POST /owner-job-console` (form fields `job`, `day`, `force`, token) and answers 303 to `/job-console?request=<id>`, so a reload never repeats the post.
5. **Force is "run a present key again", with no clearing.** The ask said "clear the present mark under the lease, then run". Clearing opens a window in which a present key looks missing to the gap syncer, and a failed forced run would leave a real key unmarked. Instead `ForceBackfiller.BackfillWith(ctx, day, datasets, BackfillOptions{Force: true})` does not skip a `present` key: it runs it as an extra attempt (`seq = runs+1`, as `Backfill` does at `backfill.go:81`), and if the new mark is not `present` it writes `present` back and reports the key `kept` with the reason of the run. A key forced and `present` again is `filled`. Nothing is deleted; rows are only appended, and a mark is rewritten only to put `present` back (Doubt 11). `BackfillWith` takes the same lease under an owner of its own. `Backfill(ctx, day, datasets)` is unchanged and equals `BackfillWith` with no force.
6. **A per-key outcome with a reason code, and a key the runner will not try.** `BackfillResult` gains `Keys []KeyOutcome` (key, outcome, reason, returned, usable, attempt), one entry per selected key in plan order, next to the six old lists, which stay for `ot backfill`. Outcomes: `filled`, `kept`, `already_present`, `absent`, `missing`, `in_window`, `refused`. The reason is a code, never a text: the `ErrorCode` and `ReasonCode` of the run row (`kind` of the job error, `empty_before_close`, `abandoned_stop`, `backfill`), or for a refusal one of `live_only_past_day`, `force_unsafe`. `absent` has no reason beyond "the source gave nothing": the text of a `NoValue` error is not stored and can hold a value (Doubt 6). A refused key is not run and not marked: neither a `no_value` nor a gap change.
7. **"Today only" is a capability of the job, checked by the runner.** `runner.LiveOnly` (one method, `LiveOnly() bool`, plain values) is declared in `internal/runner/api.go`; the nine jobs of the list above implement it (entry 4). `BackfillWith` refuses a key of a `LiveOnly` job when `day` is not the clock's date in the zone the job's `Schedule()` returns (the zone each of these jobs already compares against: `j.loc` of `account_capture`, the manifest zone of the others); a `LiveOnly` job without `Scheduled` is a wiring error, shown by a test in `cmd/ot` (entry 5). The refusal applies to `Backfill` too, so `ot backfill --day <past>` no longer marks these nine `no_value`: it prints `refused live_only_past_day` for them. The jobs keep their own `wrong day` check as the second guard. A live-only job on today, after its slot, still returns its own explained `NoValue` (`after session`, `missed slot`); that is a verdict of the job and stays.
8. **Force is offered for every job but those that declare `runner.ForceUnsafe`.** `ForceUnsafe() string` returns the reason; only `option_candidates` implements it ("a run adds a second set"), and its entry is removed by the job issue that makes the run id deterministic. Without force, a key that is not `present` is run once, as `Backfill` does today. The runner refuses force on a `ForceUnsafe` job with `force_unsafe`; the page shows the switch off with that reason.
9. **The capability table is published by the scheduler, not kept in the page.** At start and every 60 s of the clock the consumer upserts one `ops.job_capability` row per job of `w.jobs` and per left-out job (`schedulerDisabled`, `cmd/ot/scheduler.go:278`): `live_only`, `live_day` (the clock's date in the job's `Schedule()` zone, empty for a job that is not live-only), `force_unsafe` (reason or empty), `left_out` (reason or empty), `seen_at`. So the page lists exactly what the running scheduler holds, includes the jobs left out with their reason, and says the scheduler is not running when `seen_at` is over 2 minutes older than the page's `generated_at` (the builder compares; the view computes nothing). A live-only job's row offers one button, `Run today`, whose day is the row's `live_day`; the post takes the day from the view, not from a field, and the runner re-checks it.
10. **A console run does not wait for the window.** `Backfill` puts a key whose window has not closed in `InWindow` and does not run it (`backfill.go:62-76`), so "Run today" during the session would do nothing. The consumer sets `BackfillOptions.RunInWindow`: such a key is run now; an empty answer before the close stays unmarked (`empty_before_close`, `runkey.go` verdict), so the scheduler's own slot still fills it. `in_window` is then reported only by a call without the option (`ot backfill`).
11. **The consumer.** Every 5 s of the fake-able clock (`clockwork`; a test advances it, none sleeps) `Next` reads the oldest request with no state row; `Claim` inserts the state row `running`; the job is resolved among `w.jobs` and its datasets by `jobDatasets` (an unknown job or a job with no key on the day finishes `refused` with the code `unknown_job` or `no_key`); `BackfillWith` runs; a `Retryable` error (the lease is held by a tick or a backfill) is retried every 15 s for up to 20 minutes, the state staying `running` with `waiting_for_lease` visible, then the request ends `failed` with the code `lease_held`; a refusal of the day (`RefusedNoPlan`) ends `refused` with the code `no_plan`; otherwise the keys are stored and the state is `done`; after a run the consumer calls `KeyGapSyncer.SyncKeys(day)` as `ot backfill` does (`cmd/ot/backfill.go:68`), so gaps clear at once. At start, every `running` row is set `interrupted` (the fence makes this process the only consumer). A drain cancels the run and the request ends `interrupted`. An interrupted request is never rerun by itself: a forced run is the owner's act.
12. **The page.** `page.job_console.v1`: `title`, `sentence`, `form` (null when the route is off, else `post_path`, `expires`, `mac`), `gaps`, `scheduler` (`running` or `not_running`, `seen_text`), `jobs` (per job: `id`, `kind` `any_day` or `today_only`, `day` for today-only, `force` `allowed` or `unsafe` with `force_reason`, `left_out`), `requests` (the newest 20: `id`, `job`, `day`, `force`, `state`, `state_text`, `at_text`, `refusal`, `keys` with `dataset`, `subject`, `outcome`, `outcome_text`, `reason`, `returned`, `usable`), `selected` (the request id of `?request=`, whose entry is open) and `refresh_seconds` (5 while a shown request is `queued` or `running`, else 0). Every sentence is built in Go; the view computes nothing. A GET with `?request=<id>` shows that request even when it is older than the 20; an unknown id shows the list and the gap `no_such_request`.
13. **The view and the stack.** Plain HTML forms that post to the route of the payload, as the underlyings view does (`web/src/pages/underlyings/view.tsx`); the only script is the reload timer driven by `refresh_seconds`. Lines for the front end follow `docs/adr/0034-frontend-stack.md` and `frontend-whitelist.json` revision 2026-11-06.1:
    - 按 frontend-stack：用 shadcn table, badge and button (`components/ui`) 做 the job list with its per-row form (job and token hidden, day, force, `Run`).
    - 按 frontend-stack：用 shadcn input (`type="date"`) and switch (`components/ui`) 做 the day field of an any-day job and the force switch; no `select`, no calendar (the calendar adds a package and is owner-only).
    - 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 the recent requests, each with its per-key table (shadcn table and badge).
    - 按 frontend-stack：用 shadcn alert and card (`components/ui`) 做 the gap lines, the scheduler line and the refusal text.
    No new package, no hand-built component; the reload is `setTimeout` and `location.reload` in the view, 5 lines.
14. **No CLI of its own.** `ot backfill --day D --job J` exists and keeps working; it gains the refusal lines (entry 5) and no `--force` (the owner works on the page; Left out).
15. **Limits.** The newest 20 requests are listed (Doubt 8); a post body is at most 64 KiB (`ownerBodyMax`); a request of a job and day is refused at once when the day is in the future or not in `YYYY-MM-DD` (422, a fixed text, no row); every other refusal is the runner's, with a code, so there is one source for what a job may do.

## Modules

One edit of `docs/design/modules.md` is made in this pull request: the entry cell of the store row gains `internal/store/ops/api.go` (Decision 2; the same pattern as `docs/design/flex-xml-import.md` Decision 8). The first matching glob of `docs/design/modules.md` for each path below.

| module | paths touched | change |
|---|---|---|
| runner (interface entry 0) | `internal/runner/api.go`, `internal/runner/api_test.go`, `docs/contracts/runner.md` | `LiveOnly`, `ForceUnsafe`, `KeyOutcome`, `BackfillOptions`, `ForceBackfiller`, `BackfillResult.Keys` |
| store (interface entry 1) | `internal/store/ops/api.go`, `internal/store/ops/api_test.go`, `docs/contracts/store.md` | the request types and three interfaces, declarations only |
| store (entry 2) | `migrations/0034_job_request.sql`, `internal/store/ops/jobrequest.go`, `internal/store/ops/jobrequest_test.go`, `internal/store/ops/jobrequest_pg_test.go` | tables, views, grants and the implementation |
| runner (entry 3) | `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/backfill_force.go`, `internal/runner/reconcile/backfill_force_test.go` | `BackfillWith`, per-key outcomes, the two refusals |
| jobs (entry 4) | `capability.go` and `capability_test.go` in each of `internal/jobs/{accountcapture,positionmarks,quotesnapshot,optioneod,optionopen,earningsrefresh,dividendcapture,optioncandidates}/`, `docs/contracts/jobs.md` | `LiveOnly` on nine ids, `ForceUnsafe` on `option_candidates` |
| ot (entry 5) | `cmd/ot/jobconsole_consume.go`, `cmd/ot/jobconsole_consume_test.go`, `cmd/ot/scheduler.go`, `cmd/ot/scheduler_test.go`, `cmd/ot/backfill.go`, `cmd/ot/backfill_test.go`, `docs/contracts/ot/job_console_consumer.md` | the consumer, its start, the refusal lines of `ot backfill` |
| contracts (interface entry 6) | `contracts/schemas/page.job_console.v1.json`, `web/src/generated/job_console.v1.ts`, `contracts/fixtures/page.job_console.v1.*.json`, `internal/contracts/job_console_v1_test.go`, `docs/contracts/page-schemas.md` | the page schema and its type |
| pages (entry 7) | `internal/pages/job_console/**`, `docs/contracts/pages.md` | the builder |
| ot (entry 8) | `cmd/ot/jobconsole_site.go`, `cmd/ot/jobconsole_post.go`, `cmd/ot/jobconsole_site_test.go`, `cmd/ot/jobconsole_post_test.go`, `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/job_console_page.md` | the route and the post |
| web (entry 9) | `web/src/pages/job_console/view.tsx`, `web/src/types/job_console.ts`, `web/test/pages/job_console/view.test.tsx` | the view |

## Interface

- Entry 0, in `internal/runner/api.go`: `type LiveOnly interface{ LiveOnly() bool }`; `type ForceUnsafe interface{ ForceUnsafe() (reason string) }`; `type KeyOutcome struct { Key contracts.ExpectedKey; Outcome KeyResult; Reason string; Returned, Usable, Attempt int }`; `type KeyResult string` with the seven constants of Decision 6; refusal codes `RefusedLiveOnlyPastDay`, `RefusedForceUnsafe` (type `string`, not `BackfillRefusal`, which stays whole-day); `type BackfillOptions struct{ Force, RunInWindow bool }`; `type ForceBackfiller interface{ BackfillWith(ctx context.Context, day contracts.Day, datasets []string, opt BackfillOptions) (BackfillResult, error) }`; the new field `Keys []KeyOutcome` of `BackfillResult`. `api_test.go` has a fake that implements `ForceBackfiller` and a caller that uses it. Owner module: runner.
- Entry 1, in `internal/store/ops/api.go` (owner module: store): types `JobRequest`, `JobRequestRow`, `KeyRow`, `Capability`, `RequestState`; `JobConsoleReader` (`Capabilities`, `Recent(limit)`, `Request(id)`; pages line); `JobRequestWriter` (`Enqueue(ctx, JobRequest) (id int64, existing bool, err error)`; web role); `JobRequestQueue` (`PutCapabilities`, `Next`, `Claim`, `Finish`, `InterruptRunning`; runner role). Plain types and strings, no `runner` import. `api_test.go` has a fake and a caller for each of the three.
- Entry 6: `page.job_console.v1` and the generated type `JobConsole`.
- Not in `internal/jobs/api.go`: the two optional methods are on the job types; no type is shared by two packages.

"Landed" means: the entry's pull request is merged to main and `git grep` of its declaration on origin/main finds it; an entry that uses it is cut only after.

## Issues

Ten entries, 0 to 9. 0, 1 and 6 are interface entries and land first. Order: 0, 1, 6, then 2, 3, 4 and 7 (each needs only interface entries), then 5 (needs 0 to 4) and 8 (needs 1, 6, 7), then 9 (needs 6). Each lists every file it touches and builds alone on main plus the entries it is ordered after; no two entries share a file. Acceptance states results checkable with git or by reading code; a count that can be 0 is a match that does not exist (`git grep -q -e PATTERN -- FILE` exits with status 1); test runs and pull request text are under `## Checked by CI`.

### 0 runner-job-console-api (interface, runner)

Module: runner. Size: S.
Files: `internal/runner/api.go`, `internal/runner/api_test.go`, `docs/contracts/runner.md`
Design: docs/design/job-console.md#Decisions (5, 6, 7, 8)
Contracts: unchanged
Order: first.
Goal: the runner declares what a job may say about the days and forces it accepts, and a backfill answers per key with a reason.
Acceptance:
1. `internal/runner/api.go` declares, each once, `LiveOnly`, `ForceUnsafe`, `KeyOutcome`, `KeyResult`, `BackfillOptions` (fields `Force`, `RunInWindow`) and `ForceBackfiller` with the method set of `## Interface`, and `BackfillResult` has a field `Keys []KeyOutcome`; the six old fields and `Backfiller` are unchanged (`git diff origin/main -- internal/runner/api.go` removes no line that declares them).
2. `KeyResult` has exactly seven constants, `filled`, `kept`, `already_present`, `absent`, `missing`, `in_window`, `refused`, and the file holds the two refusal codes `live_only_past_day` and `force_unsafe`.
3. `internal/runner/api.go` has no function body (declarations only) and stays under 300 lines; `docs/contracts/runner.md` has a paragraph for each of `LiveOnly`, `ForceUnsafe` and `ForceBackfiller` that names the whole-day versus per-key distinction and says a refused key is not marked.
4. `api_test.go` has a fake that implements `ForceBackfiller` and a caller that takes the interface, in the style of the existing cases; each test calls `t.Parallel()`.

### 1 job-request-api (interface, store)

Module: store. Size: S.
Files: `internal/store/ops/api.go` (new), `internal/store/ops/api_test.go` (new), `docs/contracts/store.md`
Design: docs/design/job-console.md#Decisions (2, 3, 4, 9, 11)
Contracts: unchanged
Order: first.
Goal: the request queue, the capability table and the read side of the page are declared in one interface file of the store, outside `internal/store/api.go` (800 lines).
Acceptance:
1. `internal/store/ops/api.go` declares `JobConsoleReader`, `JobRequestWriter`, `JobRequestQueue` with the methods of `## Interface` and the types `JobRequest`, `JobRequestRow`, `KeyRow`, `Capability`, `RequestState`; it has no function body and no import of `internal/runner`.
2. `RequestState` has the constants `queued`, `running`, `done`, `refused`, `failed`, `interrupted` and no other.
3. `git diff origin/main --stat -- internal/store/api.go` prints nothing; `docs/contracts/store.md` says which role reaches which interface (pages line, web role `ot_w_settings`, runner role `ot_runner`).
4. `api_test.go` has, for each of the three interfaces, a fake and a caller; each test calls `t.Parallel()`.

### 2 job-request-store (store)

Module: store. Size: M.
Files: `migrations/0034_job_request.sql` (new; the number is the next free one at the cut), `internal/store/ops/jobrequest.go` (new), `internal/store/ops/jobrequest_test.go` (new), `internal/store/ops/jobrequest_pg_test.go` (new, tag `pg`)
Design: docs/design/job-console.md#Decisions (2, 3, 4)
Contracts: unchanged
Order: after 1. The owner's window runs `ot migrate` after the merge (rebuild, setup pg, migrate, setup pg).
Goal: the three interfaces of entry 1 are implemented over four tables in schema `ops` and three views in `rd`, with the grants of Decision 3.
Acceptance:
1. The migration creates `ops.job_request` (identity id, `job_id`, `session_day date`, `force boolean`, `requested_at timestamptz default now()`), `ops.job_request_state` (primary key `request_id` referencing `job_request`, `state` checked against the five written states, `started_at`, `finished_at`, `refusal text`), `ops.job_request_key` (primary key `request_id, dataset_id, subject_key`; `outcome` checked against the seven outcomes; `reason`, `returned_n`, `usable_n`, `attempt`) and `ops.job_capability` (primary key `job_id`), all owned by `ot_owner`; its Down section drops them and the views.
2. It creates `rd.job_request_r1` (request left join state, so a request without a state row reads `queued`), `rd.job_request_key_r1` and `rd.job_capability_r1`, every column named, and grants `SELECT` on the three to `ot_ro_pages`.
3. The grants of `ot_w_settings` in the file are exactly `USAGE` on schema `ops` and `INSERT, SELECT` on `ops.job_request`; `git grep -n -e "ot_w_settings" -- migrations/0034_job_request.sql` shows no other line with `UPDATE`, `DELETE` or another table; `ot_runner` gets `SELECT` on `ops.job_request` and `SELECT, INSERT, UPDATE` on the other three tables, no `DELETE`.
4. `Enqueue` is one statement that inserts only when no request of the same job and day lacks a state row or is `running`, and returns the id of that open request with `existing` true otherwise; `Claim` is `INSERT ... ON CONFLICT (request_id) DO NOTHING` and reports whether a row was inserted; `Finish` writes the keys and the final state in one transaction; `InterruptRunning` sets every `running` row `interrupted`.
5. `jobrequest_pg_test.go` stores a request, reads it as `queued`, claims it twice (second claim false), finishes it and reads its keys through the view, enqueues twice while open (one row, `existing` true), and shows `ot_w_settings` fails an `UPDATE` of `ops.job_request` and a read of `ops.job_request_state`.
6. `wc -l` of each new Go file prints a number under 600; each test calls `t.Parallel()` and uses `t.TempDir()` where it writes a file.

### 3 backfill-force-refusals (runner)

Module: runner. Size: M.
Files: `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/backfill_force.go` (new), `internal/runner/reconcile/backfill_force_test.go` (new)
Design: docs/design/job-console.md#Decisions (5, 6, 7, 8)
Contracts: unchanged
Order: after 0.
Goal: the reconciler's backfiller implements `ForceBackfiller`: it fills `Keys`, refuses a live-only key on another day and a forced key of an unsafe job without marking it, and runs a present key again when force is set, keeping the old mark if the new verdict is not `present`.
Acceptance:
1. The type returned by `NewBackfillerWithRule` satisfies `runner.ForceBackfiller` (a compile-time assertion line in `backfill_force.go`), and `Backfill` is `BackfillWith` with `BackfillOptions{}`: `git grep -n "func (r \*reconciler) Backfill(" -- internal/runner/reconcile/backfill.go` finds one line whose body calls `BackfillWith`.
2. Every selected key has one entry in `Keys`, in plan order, with the outcome and reason of Decision 6; the six old lists hold what they held before for a call without force (the existing `backfill_test.go` cases are unchanged).
3. A key of a job that implements `runner.LiveOnly`, asked for a day that is not the clock's date in the zone of its `Scheduled.Schedule()`, is `refused` with `live_only_past_day`; no run row, no mark and no window row is written for it; a `LiveOnly` job without `Scheduled` makes `BackfillWith` return an error naming the job id.
4. With `Force`, a `present` key is run as attempt `runs+1`; the result is `filled` when the mark is `present` after, and `kept` when the verdict was `no_value` or empty, in which case the code writes `present` back; no call of the ledger deletes a mark (`git grep -q -e "Delete" -- internal/runner/reconcile/backfill_force.go` exits with status 1). A key of a job that implements `runner.ForceUnsafe` with `Force` set is `refused` with `force_unsafe` and is not run.
5. The reason of a run outcome is the `ErrorCode` or else the `ReasonCode` of the run row of that attempt and never a text of a job error (`git grep -q -e "Err.Error()" -- internal/runner/reconcile/backfill_force.go` exits with status 1).
6. With `RunInWindow`, a key whose window has not closed is run, and an empty answer before the close stays unmarked; without it the key is `in_window` as today.
7. `backfill.go` stays at or under 330 lines and `backfill_force.go` under 600; each test calls `t.Parallel()` and uses the fake clock of the test kit, no sleep.

### 4 jobs-declare-capabilities (jobs)

Module: jobs. Size: M.
Files: `internal/jobs/accountcapture/capability.go`, `internal/jobs/positionmarks/capability.go`, `internal/jobs/quotesnapshot/capability.go`, `internal/jobs/optioneod/capability.go`, `internal/jobs/optionopen/capability.go`, `internal/jobs/earningsrefresh/capability.go`, `internal/jobs/dividendcapture/capability.go`, `internal/jobs/optioncandidates/capability.go`, the eight `capability_test.go` beside them, `docs/contracts/jobs.md` (all new except the last)
Design: docs/design/job-console.md#Decisions (7, 8)
Contracts: unchanged
Order: after 0 (the method set is the one `runner.LiveOnly` and `runner.ForceUnsafe` declare; these packages do not import `runner`).
Goal: the nine ids that answer for today only say so, and the one job whose re-run appends says so.
Acceptance:
1. `LiveOnly() bool` returning true is defined on the job types of `account_capture`, `position_marks`, `quote_snapshot`, `option_eod` and `option_eod_jp` (one type, both manifests), `option_open` and `option_open_jp` (the `pull` type), `earnings_refresh` and `dividend_capture`; none of the other types of those packages (`planJob`, `resolveJob`) has it (`git grep -n "LiveOnly()" -- internal/jobs` lists the seven job types and no other).
2. `ForceUnsafe() string` is defined on the `option_candidates` job only and returns a non-empty reason; `git grep -n "ForceUnsafe()" -- internal/jobs` lists one definition.
3. No file under `internal/jobs` imports `internal/runner` (`git grep -q -e "internal/runner" -- internal/jobs` exits with status 1 for the files of this entry); no existing line of a job's `Run` changes (`git diff origin/main --stat` shows only the files above).
4. Each `capability_test.go` builds the package's job as its other tests do and checks the method answers; `docs/contracts/jobs.md` lists the nine ids and the one unsafe id.

### 5 ot-job-consumer (ot)

Module: ot. Size: L.
Files: `cmd/ot/jobconsole_consume.go` (new), `cmd/ot/jobconsole_consume_test.go` (new), `cmd/ot/scheduler.go`, `cmd/ot/scheduler_test.go`, `cmd/ot/backfill.go`, `cmd/ot/backfill_test.go`, `docs/contracts/ot/job_console_consumer.md` (new)
Design: docs/design/job-console.md#Decisions (1, 9, 10, 11, 14)
Contracts: unchanged
Order: after 0, 1, 2, 3 and 4.
Goal: the scheduler process consumes the requests and publishes the capability table; `ot backfill` prints the refusals.
Acceptance:
1. `runScheduler` starts one goroutine that runs the consumer until `ctx` ends, and the consumer opens `ops.JobRequestQueue` with the runner DSN the scheduler already holds; no new `.env` line (`git grep -n "OT_PG_DSN" -- cmd/ot/jobconsole_consume.go` finds only constants that exist on origin/main).
2. At start the consumer calls `InterruptRunning` once, then `PutCapabilities` with one row per job of the wiring and one per left-out job with its reason, and again every 60 s of the clock; `live_day` is the date in the zone of the job's `Schedule()`.
3. Per request the consumer follows Decision 11: `Claim`, resolve the job and datasets (an unknown job ends `refused` with `unknown_job`, a job with no key on the day `no_key`), `BackfillWith` with `RunInWindow` set, store the keys, `SyncKeys`, `Finish`; a `Retryable` error is retried every 15 s of the clock up to 20 minutes and ends `failed` with `lease_held`; `RefusedNoPlan` ends `refused` with `no_plan`; a drain ends the request `interrupted`.
4. A test in `cmd/ot` asserts that exactly the nine ids of Decision 7 satisfy `runner.LiveOnly` and each also satisfies `runner.Scheduled`, and that exactly `option_candidates` satisfies `runner.ForceUnsafe`, over the jobs the wiring builds.
5. `ot backfill` prints one line `refused <dataset> <reason>` per `refused` key of the result, exits 0 when every key is `filled`, `already_present`, `absent`, `in_window` or `refused`, and the existing exit rules for `missing` and a held lease are unchanged.
6. `docs/contracts/ot/job_console_consumer.md` names the polling interval, the retry bound and the states.
7. Each new Go file is under 600 lines; `scheduler.go` and `backfill.go` stay under 800; each test calls `t.Parallel()` and uses the fake clock, no sleep.

### 6 job-console-page-schema (interface, contracts)

Module: contracts. Size: S.
Files: `contracts/schemas/page.job_console.v1.json` (new), `web/src/generated/job_console.v1.ts` (new, generated), `contracts/fixtures/page.job_console.v1.golden.json` (new), `contracts/fixtures/page.job_console.v1.bad-version.json` (new), `internal/contracts/job_console_v1_test.go` (new), `docs/contracts/page-schemas.md`
Design: docs/design/job-console.md#Decisions (12)
Contracts: page.job_console v1 (new)
Order: first.
Goal: the page's JSON schema and TypeScript type, from which the builder and the view are written.
Acceptance:
1. The schema has `schema_version` 1 and the fields of Decision 12 with `additionalProperties: false`; `state` is an enum of the six states, `outcome` of the seven outcomes, `kind` of `any_day` and `today_only`, `force` of `allowed` and `unsafe`; no field holds a number that the view would compute from another.
2. `web/src/generated/job_console.v1.ts` is the output of `contracts/generate.sh` for the schema (the issue carries source and output together) and declares `JobConsole`.
3. The golden fixture holds one any-day job, one today-only job, one job left out, one `unsafe` job, one `done` request with a `kept` key and a `refused` one with `live_only_past_day`; the bad-version fixture has `schema_version` 2.
4. `docs/contracts/page-schemas.md` lists the page; each new test calls `t.Parallel()`.

### 7 job-console-builder (pages)

Module: pages. Size: M.
Files: `internal/pages/job_console/builder.go` (new), `internal/pages/job_console/text.go` (new), `internal/pages/job_console/builder_test.go` (new), `docs/contracts/pages.md`
Design: docs/design/job-console.md#Decisions (9, 12)
Contracts: unchanged
Order: after 1 and 6.
Goal: a builder turns the reader's capabilities and requests into the view model, with every sentence in Go.
Acceptance:
1. `Build` takes an `ops.JobConsoleReader`, the generated-at instant, the post path with its token fields (or nil for no route), the selected request id and returns the JSON of `page.job_console.v1`; it never reads a clock and has no import of `internal/runner`.
2. The `scheduler` field is `not_running` when the newest `seen_at` is more than 2 minutes before `generated_at` or no capability row exists; a today-only job carries `day` from its row and `kind` `today_only`; an `unsafe` job carries its reason; a left-out job carries its reason and no form.
3. `refresh_seconds` is 5 when a listed request is `queued` or `running`, else 0; the list holds the newest 20, plus the selected request when older; an unknown selected id gives the gap `no_such_request`.
4. With a nil reader the page has the gap `no_reader` and no jobs, status 200 (the rule of the other builders); no sentence contains a number that is not a field of the page.
5. Each new Go file is under 600 lines; each test calls `t.Parallel()`.

### 8 ot-job-console-route (ot)

Module: ot. Size: M.
Files: `cmd/ot/jobconsole_site.go` (new), `cmd/ot/jobconsole_post.go` (new), `cmd/ot/jobconsole_site_test.go` (new), `cmd/ot/jobconsole_post_test.go` (new), `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/job_console_page.md` (new)
Design: docs/design/job-console.md#Decisions (3, 4, 12, 15)
Contracts: unchanged
Order: after 1, 6 and 7.
Goal: `GET /job-console` serves the page and `POST /owner-job-console` queues a request, behind the guard of the other posts.
Acceptance:
1. `jobconsole_site.go` registers the page by `registerPage` with the route `job-console`; the GET reads with the pages line (`OT_PG_DSN_PAGES`) through an opener opened once per process, and shows `form` null when `rd.edit` is nil.
2. `site.go` gains one branch for the route `owner-job-console` next to the two existing ones and no other change; `jobconsole_post.go` calls `guardOwnerPost` before reading any field, answers 403 with no store opened when it fails, and 405 for a method other than POST.
3. The post checks `job` against the capability view (an unknown job: 422, a fixed text, no row), `day` as `YYYY-MM-DD` and not after the market date (422, no row), takes the day of a today-only job from its `live_day`, calls `Enqueue`, and answers 303 to `/job-console?request=<id>`; the writer is opened with `OT_PG_DSN_SETTINGS` per post and closed after it.
4. A post never carries a vendor key, a DSN or an account id in any response, log line or error text (`git grep -n "slog\|a.log" -- cmd/ot/jobconsole_post.go` shows only fixed messages and codes).
5. `check_serve.go` and `check_blocks.go` know the page (`pageMetaKeys` entry as for `underlyings`); `docs/contracts/ot/pages/job_console_page.md` names the routes, lines and gaps.
6. Each new Go file is under 600 lines; each test calls `t.Parallel()`.

### 9 web-job-console-view (web)

Module: web. Size: M.
Files: `web/src/pages/job_console/view.tsx` (new), `web/src/types/job_console.ts` (new), `web/test/pages/job_console/view.test.tsx` (new)
Design: docs/design/job-console.md#Decisions (12, 13)
Contracts: unchanged
Order: after 6 (and 8 for use on the site).
Goal: the owner sees the job list with one form per job, the recent requests and their per-key lines.
Acceptance:
1. The file ends with the `page(...)` call of the shell as `web/src/pages/underlyings/view.tsx` does, and imports only components under `@/components/ui` that exist on main (`table`, `badge`, `button`, `input`, `switch`, `accordion`, `alert`, `card`, `label`) and the generated type; no new entry in `web/package.json` (`git diff origin/main -- web/package.json` prints nothing).
2. Each job row is one `<form method="post">` with hidden `job`, `expires` and `mac`, a date `Input` named `day` for an any-day job (none for a today-only job: the text "today" and the day from the payload), a `Switch` named `force` disabled with the reason when `force` is `unsafe`, and a `Button` `Run`; a job with `left_out` has no form.
3. The view prints every visible value from the payload and computes none: `git grep -q -e "Date\|toFixed\|reduce(" -- web/src/pages/job_console/view.tsx` exits with status 1 (the one `setTimeout` is not matched).
4. When `refresh_seconds` is above 0 the view reloads the page after that many seconds; when 0 it sets no timer; the request whose id is `selected` is the open accordion item.
5. The vitest renders the golden fixture and finds the five states' badge text, a `kept` row and the refusal text.

## Rollout

Entries 0, 1, 3, 4, 6, 7 and 9 need no owner step. Entry 2 holds a migration: after it merges the owner's window runs `ot migrate`; until then the consumer of entry 5 and the route of entry 8 are not merged (their order). After entry 8 merges `ot serve` deploys itself with the other code merges and runs with `--owner-edit` as for `/settings`; entry 5 deploys the scheduler the same way. The owner then opens `/job-console`. No new role, no new `.env` line. A `ot backfill` of a past day stops marking the nine live-only jobs `no_value` from entry 3 on (Decision 7); the owner's merge of this design is his word for that behaviour change, and for the one edit of `docs/design/modules.md`. Between entry 3 and entry 5 `ot backfill` refuses the nine keys without printing a line for them; no exit code changes.

## Checked by CI

`go test` of `internal/runner/...`, `internal/store/ops` (the tag `pg` runs only in GitHub CI), `internal/jobs/...`, `internal/contracts` (entry 6), `internal/pages/job_console`, `cmd/ot`; `tsc`, `eslint`, `vitest`, the build and the `type drift` step of the web workflow; `go run ./scripts/arch --check`; the ratchet; the `Contracts:` line check (entry 6: `Contracts: page.job_console v1 (new)`; every other entry `Contracts: unchanged`; this pull request `Contracts: unchanged`). The behaviours of the consumer under a held lease, an interrupted run and a double press are tested with fakes of the three interfaces and the fake clock; the real tables with the tag `pg` in entry 2.

## Left out

- **`ot backfill --force` and any CLI of the console.** The owner works on the page (Decision 14). If wanted, it is one flag that calls `BackfillWith`.
- **A "give up this gap" action** for a live key whose day is lost (`account_capture` of a day the scheduler was down). The refusal of Decision 7 leaves that gap open instead of closing it with a false `no_value`; closing it honestly is a separate owner act with its own record. Doubt 5.
- **Quota and cost lines per job** (Alpha Vantage 25 a day, Flex generation delay, IB pacing). They need a table of 24 entries that no module owns today; the page shows no warning. A forced run of `earnings_refresh` spends quota.
- **Cancel, schedule-for-later and bulk "all jobs for a day".** One job, one day, now.
- **Result text of a job.** The page shows codes and counts, not the job's error message (it may hold a value, `safeErrText` at `runkey.go:120-131` exists for logs only).
- **The deterministic run id of `option_candidates`** (the registered job draft). Until it lands the page offers no force for that job (Decision 8).
- **A new narrow write role** `ot_w_jobrequest` (Decision 3, Doubt 4).
- **Auto-run of a request after a restart.** An interrupted request is not resumed (Decision 11).

## Adversarial

Fresh read-only agent (full answer in the designer's scratchpad; every answer is repeated here). Nothing needed to read this section is outside the file.
- *Gap against the goal ("Run today" lands in `in_window` and does nothing; "idempotent" overstated for revision and upsert writers; no cancel).* Adopted: true (`backfill.go:62-76`). Decision 10 and `BackfillOptions.RunInWindow` make a console run go now; the Goal's wording of idempotent is narrowed. Cancel stays out (Left out).
- *Biggest unnamed risk (the consumer shares the scheduler process; the forced write-back is a new mark write).* Adopted in part: the mark rewrite is Doubt 11 and the Decision 5 wording is corrected. The shared process is Doubt 2 and Doubt 9; not changed, because the lease already serialises key runs.
- *A simpler way (call a scheduler socket; derive results from run rows, no request table).* Not adopted: a socket gives `serve` the authority to start a run, which Decision 1 rejects, and run rows hold neither refusals nor the owner's intent (force, a request that never ran); reload would show less.
- *Lock-in (page schema and the seven outcomes, `ot_w_settings` widened, job-type methods, consumer in the scheduler).* Adopted as stated: Decisions 3, 6, 7, 12; the role is Doubt 4.
- *Claim most likely false ("Run today" works).* Adopted: see the first point.
- *Likeliest slip (migration number, `ops/api.go` against `scripts/arch`, hidden order, shared files, same-package tests).* Adopted: the number is taken at the cut (entry 2), the arch point is Doubt 12, the entry 3 / entry 5 gap is in Rollout, entry 4 adds only new methods and files so no same-package test uses a changed ident; `site.go` gets one branch in entry 8.

## Doubts (least sure)

1. The consumer and the ticker compete for `ops.run_lease`. A long reconcile (the 40-minute budget of `option_open_resolve`, `docs/design/option-jobs.md` [unverified]) can keep a request `running` and waiting for up to 20 minutes, then `failed` with `lease_held`; the numbers 15 s and 20 minutes are chosen, not measured.
2. Ten new files in `cmd/ot` and a goroutine in `runScheduler`: `scheduler.go` is the one process whose failure stops all data collection. The consumer recovers from a panic per request and logs a fixed message [unverified: the behaviour of `runScheduler` on a panic in a goroutine is not read].
3. The page token is also a settings and underlyings token for 15 minutes, and the reverse (`Check` binds no route). A holder can now queue a run; he could already change settings. Reach is local (loopback, Host and Origin checks).
4. The web role is `ot_w_settings`, widened by one table. A narrower role is cleaner and costs an owner step; the design picks fewer steps. If the owner prefers the role, entry 2 gains a change of `internal/store/setup` and the Rollout one `.env` line.
5. A live key of a lost day stays a gap for good (Left out). Before this design a backfill would have closed it with `no_value`; the gap line of `ot scheduler status` stays red until the owner decides.
6. `absent` carries no reason: the runner stores no text for a `NoValue`. The page says "the source gave nothing", which for a live job on today can also mean "after session" or "missed slot". A reason code on `JobResult` is an interface change of `contracts` and is not taken here.
7. Force on a job whose writer is `ON CONFLICT DO NOTHING` (most of the 24) reports `returned` and `usable` and cannot say that nothing new was stored; the owner may read `filled` as "changed". The page says so in the `sentence`.
8. The list keeps 20 requests; an older one is reachable only by `?request=<id>`.
9. `jobDatasets` (`cmd/ot/backfill.go:126`) plans the job for the day and so runs `Plan` inside the scheduler for a request; a job whose `Plan` is slow or reads the broker blocks the consumer, not the ticker.
10. The audit lines for the 14 idempotent jobs are read, not run; the first forced run of each on a copy of the data is the real test and is not in this design.
11. A forced run rewrites a mark: if the process dies between the run and the write-back of `present`, the key stays `no_value` over good rows (the rows are untouched; the next unforced run of the key repeats it). "Nothing is deleted" holds for rows, not marks. The window is one ledger write.
12. `internal/store/ops` already holds `ledger.go`, `gaps.go` and `scheduler.go`; a declarations-only `api.go` there is allowed by `scripts/arch` as for `optionledger` [checked in the text of `docs/design/modules.md`, not run].
