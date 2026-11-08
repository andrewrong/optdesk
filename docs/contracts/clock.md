# clock contract

## Responsibility

Read-only time and session calendar. `clock.Clock` gives the current time through an injected clock (`realclock.NewReal` in production, clockwork's fake in tests). `clock.Calendar` answers `Session` and `NextSession` from a `clock.SessionSource`, and package `calendar` converts instants to America/New_York, Asia/Tokyo and a caller-given owner zone. The Go side never hand-writes open or close times (ADR 0004).

## Inputs

- A `clock.SessionSource`: `SessionDays(ctx, calendarID, from, to)` returns `master.session_day r1` rows, inclusive. A known calendar has a row for every day, session or not.
- Days as `contracts.Day` (`YYYY-MM-DD`, market zone).
- For zone conversion, an instant and an IANA zone name; the owner zone is given by the caller and no zone is hard-coded for it.

- A `clock.SessionDayDiffer`: `Diff(old, fresh)` compares two session-day slices by day (`calendar.NewDiffer`, a `clock.NewSessionDayDifferFunc`).
- A `clock.CommandFunc`: `func(ctx, argv) (exitCode, err)` runs a command; `err` is non-nil only when it could not start or be waited for. `clock.NewCommandSessionSourceFunc(argv, workDir, run)` builds a `SessionSource` that runs the pyside generator through it (`argv` is the command prefix, `workDir` the directory for its temporary file).

## Outputs

- `clock.Session`: calendar id, day, `IsSession`, `OpenAt`, `CloseAt` (UTC instants from the source row), `EarlyClose`.
- `NextSession`: the first session strictly after the day.
- `clock.SessionDayChange`: day, `Before`, `After` (`*contracts.SessionDay`). `Diff` returns changes sorted by day, empty when the slices agree; a day only in `fresh` has `Before` nil, a day only in `old` has `After` nil.
- Errors are `contracts.TypedError`: `UnknownCalendar` when the source has no rows for the calendar, `NoValue` when no session is found in the searched span (56 days), `Retryable` when the source fails, `Permanent` for a bad day or zone name.

## Invariants

- Open and close instants are copied from source rows; the package declares none.
- DST is carried by the instants: 2026-10-30 opens 13:30 UTC, 2026-11-02 opens 14:30 UTC.
- No `time.Now` or `time.Sleep` outside `internal/clock/realclock/realclock.go`.
- The root package `internal/clock` holds only the interface files.

## Version history

- v1: Session, NextSession, ET/JST/InZone conversions.
- v2: SessionDayChange, SessionDayDiffer, NewSessionDayDifferFunc, CommandFunc, NewCommandSessionSourceFunc (declarations).

## Contract tests

- `internal/clock/api_test.go`: the interface from both sides.
- `internal/clock/calendar/calendar_test.go`: `TestSessionTypedRows`, `TestDSTOpenAt`, `TestNextSessionSkipsHolidays`, `TestUnknownCalendar`, `TestZoneConversions`; the typed rows are expectations, not a source.
- `internal/clock/realclock/realclock_test.go`: the constructor type.

## Jobs and slots

None. The clock is read by jobs and runner; it runs nothing and owns no slot.

## Forbidden

- Writing to the source or any store.
- Hand-written open or close times, holiday lists or half-day rules in Go.
- Guessing a calendar the source does not have.
- Reading the system time anywhere but the real-clock adapter.
