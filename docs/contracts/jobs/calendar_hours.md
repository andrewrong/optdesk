# calendar_hours

## calendar_hours

`calendarhours.NewCalendarHoursFunc(market string, broker.Broker, clock.SessionSource, clock.IBKRHours, hoursnotice.Store, clock.Clock, *slog.Logger)` builds the job of one market (implementation: `internal/jobs/calendarhours.New`); the design is `docs/design/calendar-ibkr-hours.md` (Decisions 2 to 4, 6, 8). `market` is `XNYS` or `XTKS`; any other market is refused. The package imports no other jobs package.

- Ids `calendar_hours_us` (reference SPY, calendar `XNYS`) and `calendar_hours_jp` (reference 7203 on TSEJ, calendar `XTKS`). Schedule: anchor `session_open` of the job's own calendar, offset `-20m`, `restart_sensitive = false`, run bound 5m. Key: one `{master.session_hours_notice, <calendar id>}`, never a `master.session_day` key, so `calendar_refresh` marking its key present cannot make the runner skip these jobs.
- Decision 2: one `ResolveContract` per run, the only broker method the job calls; the job holds no `SessionDayWriter` and never writes `master.session_day`. A resolve failure (or a reference contract not listed) fails this job only, the error names the market and the code, and the market's rows stay as they were.
- Decision 3: the string is `liquid_hours`, read in `time_zone_id` through `clock.IBKRHours`. A day's open is its first segment's start and its close its last segment's end, so a lunch break is ignored; `CLOSED` is a closed day. A string the parser cannot read fails the run (`Permanent`) and changes no row.
- Decision 4: for each day IBKR lists, ours is the `master.session_day` row. The day differs when `IsSession` differs, or both are open and `open_at` or `close_at` differ by one minute or more. A day with no row of ours, and a day IBKR does not list, is not compared.
- Output: one `slog` line per differing day (`calendar hours differ market= day= ours= ibkr=`, hours as `HHMM-HHMM` in the market zone or `CLOSED`) and `hoursnotice.Store.Replace` of the market's rows with the differing days, so a day that no longer differs is gone. `Postcondition` is true.

## Version history

- v1: `calendar_hours_us` and `calendar_hours_jp`.
