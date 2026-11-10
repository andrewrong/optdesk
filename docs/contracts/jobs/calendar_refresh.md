# calendar_refresh

## calendar_refresh

`calendarrefresh.NewCalendarRefreshFunc(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, calendarrefresh.CalendarObserver)` builds the job `calendar_refresh` (implementation: `internal/jobs/calendarrefresh.New`, a later issue); the design is `docs/design/compute-calendar.md`. `calendarrefresh.CalendarObserver` is `func(clock.SessionDayChange)`.

- Id `calendar_refresh`. Key: one `{master.session_day, "XNYS"}`.
- Span: from the day of the injected clock minus 7 to that day plus 400, inclusive. `Run` reads the stored rows and the fresh rows of the source over the span, takes the diff with the differ, upserts only added and changed rows through the store and deletes none.
- Retry classes: an empty fresh answer is `Permanent` (nothing is overwritten with nothing). A source error keeps the class the source gave: exit 1, exit 2 and any other nonzero exit of the command are `Permanent`, a failure to start or wait for the process (a kill at the run bound included) is `Retryable`. A store error is `Retryable` unless already typed.
- `Returned` and `Usable` are the fresh rows read and checked.
- `Postcondition`: the stored rows cover every day from the as-of day to the span end; false until the first successful refresh, true again on a no-change day.
- Observer: called once per changed day, in day order, with the `clock.SessionDayChange`; the wiring writes one log line per call (`component=calendar_refresh`, day, kind added or changed, before and after). It is the only trace of the diff; no table holds it.

## Version history

- v1: `calendar_refresh`, interface only (`calendarrefresh.CalendarObserver`, `calendarrefresh.NewCalendarRefreshFunc`).
