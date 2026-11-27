# ot job `data_check`

Order: 240

Family file: `cmd/ot/wire_datacheck.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/data-check.md` (entry 11).

## Schedule

Once per XNYS session day, 120 minutes after the close (`internal/jobs/datacheck/job.toml`: calendar `XNYS`, anchor `session_close`, zone `America/New_York`, offset `120m`, run bound `10m`). `jobTable` row: any-day, force allowed (a forced run for an older day is how the owner sees the check again after a repair).

## Rules

The job checks stored data for wrong values and silent gaps with six rules; each finding has a severity:

| Rule | Severity |
| --- | --- |
| `reading_repeat` (a reading repeats the previous day) | `error` |
| `option_open_late_quote` (an open quote received on a later local day than its session day) | `error` |
| `candidates_older_than_settings` (candidates computed before a later settings change) | `warn` |
| `capture_gap` (a ledger cell missing or expired) | `warn` |
| `plan_absent` (a dataset with no plan for a session day) | `warn` |
| `gap_unclosed` (an open `ops.job_gap` older than the grace) | `warn` |

A rule that cannot read its input stores one finding of severity `unknown` with the evidence `read_failed`; a run in which every rule is `unknown` is a failed run.

## Reports only

The job repairs nothing and writes no data value but its own run and findings. The repair button of the data health page stays the repair path.

## Lines it reads

- `OT_PG_DSN_DATACHECK`: the data check store (role `ot_w_datacheck`, made by `setup pg`). The same line opens the writer, the probe and the coverage reader. While it is empty the job is held with the reason `OT_PG_DSN_DATACHECK is not set (run setup pg)`.
- `OT_TELEGRAM_TOKEN`, `OT_TELEGRAM_CHAT_ID`: the message. The data health page reads the findings with `OT_PG_DSN_PAGES`.

## Expected datasets

The list for `plan_absent` is built from `dataHealthManifests` for the jobs of `jobTable` that are not in `dataHealthNoLedger` and not in `dataCheckNotDaily` (a fixed list with a reason per job that is not planned on every session day). `TestEveryLedgerJobIsDailyOrReasoned` guards both lists.

## The message

One Chinese message per run when a finding is new (never delivered): at most ten lines, `error` before `warn` before `unknown`, each with the rule label, dataset, day and counts, then "还有 N 条见页面" when more. It names no id of an account, an order or a contract. A send failure is a retryable error and the findings stay new. When the sender cannot be built (a missing token or chat id) the announcer is nil: findings are stored, the page shows them, the run is ok and the job logs "no announcer".

## Version

Introduced with the data check (entry 11 of `docs/design/data-check.md`).
