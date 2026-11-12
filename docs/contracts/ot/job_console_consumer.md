# ot job console consumer

Files: `cmd/ot/jobconsole_host.go` (the host and `Start`), `cmd/ot/jobconsole_turn.go` (the turn), `cmd/ot/jobconsole_jobs.go` (the table of the 24 jobs). Design: `docs/design/job-console.md#Decisions` 4, 6, 7, 8, 9 and 10.

The host implements `runner.ConsoleTurner` over a `jobrequest.Consumer`, a `runner.ForceBackfiller` and a `runner.KeyGapSyncer`, all by their interfaces. The job set and the backfiller are those of the last `build` (`set`); a job a probe brought back is in the set. The wiring (entry 9) calls `Start` before the ticker runs and hands over the job set; the host itself calls `Start` again at the start of every `ConsoleTurn` until it has succeeded.

## Start

`Start` ends every request still `running` as `failed` with the reason `restart` (the fence makes this process the only consumer). Until it has succeeded a turn is `took=false`, reads no queue, and one warn line a minute is written (never the error text, which may hold a DSN).

## Order of a turn

1. Read the queued requests, oldest first, under a one-minute limit of the injected clock (each of `Queued`, `Hold` and `Claim` has its own).
2. A request that carries a withdrawal is finished `withdrawn`. A withdrawal of a running request is ignored; a withdrawal inserted after the read and before `Claim` is not seen, so that request runs.
3. Take the first request not held: ask the `SlotHold` with `turnMax(job)`, the start phase (1 minute) plus the `RunBound` of the job (30 minutes without `runner.Bounded`) plus the `Finish` timeout (30 s) plus the `SyncKeys` timeout (2 minutes); 19.5 minutes for a job of 16. A held request gets `Hold(job, at)` written and is passed over; a request that is taken has its hold cleared, then is claimed. A store error is a turn that took nothing and one warn line a minute.
4. Check the request: an id not in the table, a `listed_only` job, or force for a force-no job ends `refused` `not_runnable`; a day that the kind does not allow in the job's zone (any day not after today for `any_day`, today in the job's zone for `today_only`) ends `refused` `not_today`; a job not in the set ends `refused` `left_out`; a job with no dataset on the day ends `refused` `no_key`.
5. `BackfillWith` with the datasets of the job's plan, `Job`, `Hold` and the request's `Force`.
6. `Finish` under a context that the stop of the turn does not cancel, with a 30-second limit.
7. `SyncKeys(day)` under a 2-minute limit only after a `done` or `failed` request that has a line `ran`. A request that only re-ran present keys (`rerun`), or whose lines are `already_present`, `not_due`, `lease_held` or `not_reached`, calls no `SyncKeys`: a bare re-run changes no run row or mark. An interrupted request is synced by the next reconcile turn.

## End states and reasons

Every ended state stores a reason code.

| state | reason |
|---|---|
| `done` | `completed` |
| `withdrawn` | `withdrawn` |
| `interrupted` | `stopped` |
| `refused` | `no_plan`, `no_key`, `not_due`, `left_out`, `not_runnable`, `not_today` |
| `failed` | `slot_near`, `lease_held`, `plan_failed`, `error`, `restart` |

## Precedence of the end state

1. The turn's context is done with the cause `Canceled` (during the plan or the backfill): `interrupted` `stopped`, whatever error text came with it; the lines so far are stored.
2. `Refused` set: `RefusedNoPlan` is `refused` `no_plan`, `RefusedNoKey` is `refused` `no_key`.
3. An error: `Retryable` (the lease is held) is `failed` `lease_held`; a `Plan` error is `failed` `plan_failed`; any other is `failed` `error`.
4. No line `ran` or `rerun` and at least one `not_due`: `refused` `not_due`.
5. A line `not_reached` with the reason `slot_near`: `failed` `slot_near`.
6. Otherwise `done` `completed`.

A request is never run again by itself.
