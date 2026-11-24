# option_eod US slot moves to close-120m

The scheduled US run of option_eod starts 120 minutes before the close (issue #3637), so that a slow
pull still ends before the close. Moving the slot changes values that two modules pin in tests: the
schedule test of the job (module jobs) and the wiring test of the scheduler (module ot), whose test
clock sits before the old 15:45 slot and after the new 14:00 one.

## Modules

No row added or changed.

## Interface

No interface changes. The slot is a value of the embedded manifest `internal/jobs/optioneod/job.toml`;
`cmd/ot` reads it through the existing schedule loader. Only the test clock of the wiring test moves.

## Issues

### 1. Move the slot and its pinned tests

Files: `internal/jobs/optioneod/job.toml`, `docs/contracts/jobs/option_eod.md`, `internal/jobs/optioneod/rerun_test.go`, `internal/jobs/optioneod/schedule_test.go`, `cmd/ot/wire_optioneod_calendar_test.go`

Acceptance:
1. `job.toml` row `us` has `slot = "close-120m"` and `offset = "-120m"`; `job_index.toml` and `job_jp.toml` are unchanged.
2. `TestEmbeddedSchedule` expects the offset `-120m`; the `us` row of `rerun_test.go` expects `close-120m`, `1400` and the half-day `1100`.
3. The test clock of `cmd/ot/wire_optioneod_calendar_test.go` is before 14:00 New York time, so the test still sees the run as not yet due.
4. `docs/contracts/jobs/option_eod.md` names the 14:00 slot.

## Left out

`cmd/ot/data_health_site.go` still lists `1545` for option_eod; it is a display table, changed by its own issue.
