# Macro events: a stored list of scheduled releases and meetings, shown on the macro page

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 2 (revision 2 answers the adversarial read, see `## Adversarial`: one table, kind-only rows with the texts mapped at read time, no delete on a short answer, per-kind staleness, the release call inside the job package, four entries, funding rows)

Source: owner priority 2026-11-14 ("a macro event list on the macro page: date, name and a conventional time, especially those before an expiry I am about to sell"); advisor proposal 2026-11-14 (FRED release dates plus a hand-kept file; display only) and advisor 2026-11-14 (funding calendar rows for the liquidity work). `docs/design/alignment-v25.md` lists the events as a gap and `docs/design/macro-page-wiring.md` leaves the page `events` array empty.

Goal: `macro_page` shows the scheduled US releases (CPI, employment situation, PCE, GDP, PPI, retail sales, JOLTS, H.4.1), FOMC decisions and minutes, BoJ decisions, quarter-ends and US corporate tax dates of the next 30 days, each with its date, its name and a conventional time. Display only: marking an option candidate whose expiry spans an event is a later step (Left out).

## Today

All paths at origin/main be0e4e4e.

- `contracts/schemas/page.macro_page.v1.json` has `events` (at most 20 `Event` objects of `date`, `when_text`, `name_text`, `previous`; `previous` is a string or a `Gap`) and `events_title`. `internal/pages/macro_page/builder.go` builds `events: []` (field type `[]struct{}`), the title "This week and next", the gap "Event calendar: no data yet" and two sentence constants that say "events: no data yet". `Config.Read` is its one optional reader.
- No table holds events; `calendar_refresh` and `calendar_hours` store sessions and hours. The last migration is `0045_reading_key.sql`.
- `internal/sources/fred` calls `/fred/series/observations` with a key from `sources.FREDConfig`. `fred_series` is the model job (manifest `job.toml`, console row, by-key upsert, `docs/design/series-registry.md`). `cmd/ot` wires jobs by `registerJob(jobFamily{...})` (`wire_fred.go`); `build` gets `jc.cfg.fredKey`, and `fredBaseURL` is in `cmd/ot/env.go`. The console table `jobTable` in `cmd/ot/jobconsole_jobs.go` holds 31 jobs, and `cmd/ot/jobconsole_jobs_test.go` pins the count and the exact Force list (`fred_series`, `compute_figures`, `calendar_refresh`, `fx_refresh`, `bars_refresh`). The page opens a read-only DSN (`store.DSNEnvPages`) in `cmd/ot/pagelist.go` and builds through `a.deps.macroPage`, whose signature the tests fix.
- FRED documentation, page `fred/release/dates` (fetched 2026-11-14, no API call made): `include_release_dates_with_no_data` defaults to false, "The defalut value 'false' excludes release dates that do not have data. In particular, this excludes future release dates which may be available in the FRED release calendar." The default `realtime_end` is 9999-12-31, `limit` default 10000, `sort_order` default asc; each `release_dates` entry has `release_id` and `date`. The page does not say that true returns future dates, nor how far ahead, and says release dates "do not necessarily represent when data will be available". The premise is implied, not stated.

## Decisions

1. **Two feeds, one table.** Dates come from FRED (`fred/release/dates`, release ids CPI 10, Employment Situation 50, PCE 54, GDP 53, PPI 46, Retail Sales 9, JOLTS 192, H.4.1 20; query `release_id`, `realtime_start` = the run day, `include_release_dates_with_no_data=true`, `sort_order=asc`, `file_type=json`) and from a hand-kept file: FOMC decisions and minutes, BoJ decisions, quarter-ends and US corporate tax dates. Same host and key as `fred_series`; no new host, key or library.
2. **A row is a kind and a date, nothing else.** Name, time text and importance are not stored: the page maps a kind to them at read time through one table of conventions in `internal/pages/macro_page/events.go`, so a changed convention needs no rerun. Conventions (all [O], the designer's knowledge of the usual time, not read from a source, except the two BLS times that the BLS schedule page states): CPI, PPI, employment situation, GDP, PCE, retail sales "usually 8:30 ET"; JOLTS "usually 10:00 ET"; H.4.1 "usually 4:30 pm ET" [O]; FOMC decision and minutes "usually 2:00 pm ET"; BoJ decision "usually around midday JST" [O]; quarter-end and tax dates "date only". Importance is a fixed label per kind, shown as "high" in `when_text` after the time, no tag in the name: high for CPI, employment situation, PCE, GDP, FOMC decision and BoJ decision, none for the rest.
3. **Which calendar a date is in.** The stored date is the publisher's own calendar day: the US releases, FOMC and the US tax dates in ET, BoJ in JST. `when_text` names the zone ("usually 8:30 ET", "usually around midday JST, date in JST"), and the page shows the date as stored with no conversion; the front end computes nothing. A BoJ row is the decision day, the second day of the meeting. For FOMC the file lists the last day of the meeting and the job derives minutes = that day + 21 days as kind `fomc_minutes`.
4. **Storage (schema v2: by-key upsert, repairable, rerunnable).** One migration creates `obs.macro_event (kind text, event_date date, source text check in ('fred','file'), source_ref text, run_day date, primary key (kind, event_date))`. `source_ref` is the FRED release id or the URL of the file row; `run_day` is the run day that last returned the row. The migration takes the next free number of `migrations/` when the issue is registered (0046 and 0047 are taken by designs in flight) and holds auto-deploy until the window runs `setup pg` then `migrate`. No feed table: the observed horizon and the freshness of a kind are `max(event_date)` and `max(run_day)` of its rows, read by `Reader.Kinds`.
5. **Run, repair, rerun, and what a run never deletes.** Job `macro_events` takes a run day (the wall day by schedule; any day by the console row, Force). `ReplaceFuture` upserts the returned rows with `run_day` set to the run day (a second run of the same day changes no row) and deletes a stored row of the same kind and source only when all of these hold: the answer is not empty; its latest date is not before the latest stored date of that kind and source; the row's date is after the run day (a row of the run day or earlier is never deleted, so a Force rerun on the day keeps today's event); the row is not in the answer. When a rule refuses the delete, the rows are still upserted and the result says "kept stored rows of <kind>: answer shorter or empty". A fetch error leaves the kind unchanged and is reported; the other kinds are still written. Without Force a run day that equals the newest `run_day` of every FRED kind is a no-op and calls no source.
6. **Horizon and the fallback.** Expected horizon (an inference from the docs, not checked live): the schedule the publisher has announced, commonly the rest of the year. The job records what it saw: the result line names the last date per kind and `Reader.Kinds` shows it. A kind whose answer is empty or has no date after the run day, and that has no stored FRED row after the run day, takes its rows from the hand-kept file (the file may carry any kind) and the result says "fred: no future dates for <kind>, file used". If every kind does so, the whole list comes from the file and nothing else changes: that is the fallback if the first live run returns no future dates.
7. **The hand-kept file.** `internal/jobs/macroevents/hand_events.toml`, embedded with `go:embed` (no environment, no path): one `[[source]]` per kind and year with `url` (the publication page: Fed calendar, BoJ schedule, IRS dates), `read_day` (the day a person read the page) and `dates`. Quarter-end and tax dates are listed as the IRS page gives them; Treasury settlement sizes are out (a new host). A test checks that every row has both `url` and `read_day`. A person edits the file when a year is announced, by pull request; it changes about twice a year.
8. **Staleness and "not entered", per kind.** For each FRED kind the page reads `Kinds`; when the newest `run_day` of a kind is more than 10 days before the page day it adds a gap `{reason: stale, text: "CPI dates fetched <day>, older than 10 days"}`, one gap per kind. For each file kind (`fomc`, `boj`, `quarter_end`, `us_tax`) with no `file` row in the year of the page day, and in the next year when the window reaches it, it adds `{reason: no_data, text: "FOMC <year>: not entered"}`.
9. **Window and truncation.** The page shows the events from the page day through day + 30, ascending, first 20 (the schema maximum). The page cannot know the expiry of the candidates cheaply, so the window stays 30 days; when more than 20 rows fall in it the page adds `{reason: missing_input, text: "Events after <date> not shown: 20 of <n> shown"}`. `previous` is the gap `{reason: not_built, text: "previous value: not built"}`.
10. **Interfaces and own code.** The store sits behind `macroevent.Writer` and `macroevent.Reader`. The release-dates call is one GET: it lives in the job package behind a local interface (`ReleaseSource` in `types.go`), own implementation because the repository has no FRED release-dates client and nothing else uses one; the `fred` package and `internal/sources/api.go` stay unchanged, so no module crossing and no change of the factory type `sources.Source`. A library or a `sources` package can replace it later.
11. **Wiring.** The job family `macro_events` (new file) builds its source from `jc.cfg.fredKey` and `fredBaseURL`, the same two values `fred_series` uses, so the key is built in one place; `startOff` gives `reasonNoFredKey` when the key is empty. The writer opens with the DSN of the series store (`store.DSNEnvSeriesFred`, the role of `fred_series`) and is closed through `jc.extra`. The page gets a reader-only opener on `store.DSNEnvPages`, like `openReadingView`, nil when the line is empty or the profile is demo; `pagelist.go` calls a new `newMacroPageEvents` only when the reader is non-nil and `a.deps.macroPage` as today otherwise, so `deps`, `wire.go` (796 lines) and the existing tests keep their signatures.

## Modules

- store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`): `internal/store/macroevent` (interface), `internal/store/macroeventpg` (implementation), the migration. Entry 1.
- jobs (`internal/jobs/**`, `docs/contracts/jobs/**`): `internal/jobs/macroevents`, imports `macroevent`. Entry 2.
- pages (`internal/pages/**`): `internal/pages/macro_page`, imports `macroevent`. Entry 3.
- ot (`cmd/**`): wiring of job, console row and page reader. Entry 4.

`docs/design/modules.md` row `store` gains `internal/store/macroevent` in its entry cell in this design's commit; no other cell changes and no issue edits `modules.md`. All edges follow the layer rule of `modules.md`.

## Interface

`internal/store/macroevent/api.go`, declarations only:

```go
type Event struct {
	Kind, Source, SourceRef string
	Date, RunDay            contracts.Day
}
type Kind struct { // one row per kind and source, derived from the rows
	Kind, Source      string
	LastRunDay        contracts.Day // max run_day
	LastDate          contracts.Day // max event_date: the observed horizon
	Rows              int
}
type Outcome struct {
	Written, Deleted int
	Held             bool // a delete rule of Decision 5 refused; rows were still upserted
}
type Writer interface {
	ReplaceFuture(ctx context.Context, kind, source string, runDay contracts.Day, events []Event) (Outcome, error)
}
type Reader interface {
	Between(ctx context.Context, from, to contracts.Day) ([]Event, error) // date then kind order
	Kinds(ctx context.Context) ([]Kind, error)
}
type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, io.Closer, error)
type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, io.Closer, error)
```

`internal/jobs/macroevents/types.go` declares `ReleaseSource` (`Dates(ctx, releaseID string, from contracts.Day) ([]contracts.Day, error)`, typed errors only, never the key) and `NewMacroEventsFunc`.

## Issues

Build order: every entry compiles and its tests pass against main (BASE is `git merge-base origin/main HEAD`). Entry 1 holds the auto-deploy (migration). `Contracts:` line of every pull request: `unchanged`. Test runs are in `## Checked by CI`.

### 1 macro-events-store (interface)

Module: store. Size: M.
Order: first.
Files: `internal/store/macroevent/api.go`, `internal/store/macroevent/api_test.go`, `internal/store/macroeventpg/store.go`, `internal/store/macroeventpg/store_test.go`, `internal/store/macroeventpg/store_pg_test.go`, `migrations/NNNN_macro_event.sql`, `docs/contracts/store.md`
Design: docs/design/macro-events.md#Interface
`NNNN` is the next free number of `migrations/` when this entry is registered.
Acceptance:
1. `api.go` declares `Event`, `Kind`, `Outcome`, `Writer`, `Reader`, `OpenWriterFunc` and `OpenReaderFunc` as in the Interface section, declarations only, and is the only non-test `.go` file of its package.
2. `api_test.go` has the new contract test `TestMacroEventContract` (new; a fake pair: the same call twice writes 0 the second time; `Between` is in date then kind order; `Kinds` gives the latest `run_day` and date of each kind), `t.Parallel()`.
3. The migration creates `obs.macro_event` with the columns, the `source` check and the key of Decision 4, owned and granted like `migrations/0043_series_registry.sql` (rw role writes, ro role selects), with a Down section.
4. `store.go` implements `Writer` and `Reader` and the two openers; `ReplaceFuture` runs in one transaction and applies the four delete rules of Decision 5 in this order: empty answer, latest date below the stored latest, row date not after the run day, row in the answer.
5. `store_test.go` has no build tag, as `internal/store/markpg/store_test.go`, and has the new tests `TestReplaceFutureEmptyAnswerDeletesNothing` (new), `TestReplaceFutureShorterAnswerHeld` (new), `TestReplaceFutureKeepsRunDayRow` (new; a row dated the run day survives a rerun whose answer lacks it), `TestReplaceFutureDeletesMovedDate` (new), exercising the SQL arguments and the rules without a database; `store_pg_test.go` carries the build tag `pg` and has the new test `TestMacroEventRoundTrip` (new).
6. `docs/contracts/store.md` states the table and the four delete rules.

### 2 macro-events-job

Module: jobs. Size: L.
Order: after macro-events-store.
Files: `internal/jobs/macroevents/job.toml`, `internal/jobs/macroevents/types.go`, `internal/jobs/macroevents/run.go`, `internal/jobs/macroevents/release.go`, `internal/jobs/macroevents/hand.go`, `internal/jobs/macroevents/hand_events.toml`, `internal/jobs/macroevents/run_test.go`, `internal/jobs/macroevents/release_test.go`, `internal/jobs/macroevents/hand_test.go`, `internal/jobs/macroevents/testdata/release_dates.json`, `internal/jobs/macroevents/testdata/release_dates_empty.json`, `internal/jobs/macroevents/testdata/hand_bad_no_url.toml`, `docs/contracts/jobs/macro_events.md`
Design: docs/design/macro-events.md#Decisions
Acceptance:
1. `job.toml` has `version = 1`, `id = "macro_events"` and the `[schedule]` keys of `internal/jobs/fredseries/job.toml` (calendar XNYS, anchor time, zone America/New_York, offset 6h, run_bound 5m, restart_sensitive false) and no `[window]`.
2. `types.go` declares `ReleaseSource` and `NewMacroEventsFunc` (a `ReleaseSource`, a `macroevent.Writer`, the clock); `release.go` implements `ReleaseSource` over `net/http` with the query of Decision 1, drops dates before `from`, maps 429 and 5xx to Retryable and other non-200 to Permanent, and no error text holds the key or the URL.
3. `run.go` fetches the eight release ids one at a time, calls `ReplaceFuture` per kind with source `fred`, leaves a kind with a fetch error unchanged and reports it, takes the file rows for a kind as in Decision 6, writes `fomc_minutes` as meeting end + 21 days, skips a day that equals the newest `run_day` of every FRED kind unless forced, and names the last date per kind and every held kind in its result text.
4. `hand.go` reads `hand_events.toml` through `go:embed` and rejects a source without `url` or `read_day`; `hand_events.toml` has FOMC, BoJ, quarter-end and `us_tax` sources with the `url` and `read_day` of the page each was read from.
5. Tests, all `t.Parallel()` with a fake source, a fake writer and the fake clock of the test kit, no sleep: `TestRunWritesKinds` (new), `TestRunEmptyAnswerFallsBackToFile` (new), `TestRunShortAnswerHeld` (new), `TestRunForceSameDayKeepsToday` (new), `TestRunOneKindFails` (new), `TestRunSameDayIsNoop` (new), `TestHandRejectsNoURL` (new, reads `testdata/hand_bad_no_url.toml`), `TestHandRowsHaveURLAndReadDay` (new, every row of the embedded file), `TestMinutesAre21DaysAfter` (new), `TestReleaseQuery` (new; an httptest server checks the query), `TestReleaseErrorNeverHoldsKey` (new).
6. `docs/contracts/jobs/macro_events.md` states the manifest, the kinds, the fallback, the delete rules by reference to `docs/contracts/store.md`, and the Force rule.

### 3 macro-events-page

Module: pages. Size: M.
Order: after macro-events-store.
Files: `internal/pages/macro_page/builder.go`, `internal/pages/macro_page/events.go`, `internal/pages/macro_page/events_test.go`, `internal/pages/macro_page/builder_test.go`, `cmd/ot/testdata/macro_page_demo.golden.json`
Design: docs/design/macro-events.md#Decisions (Decisions 2, 3, 8 and 9)
`builder.go` is 276 lines; the logic goes in `events.go`.
Acceptance:
1. `Config` gains an optional `Events macroevent.Reader`; `viewModel.Events` changes from `[]struct{}` to a slice of the event row type of `events.go`; `eventsTitle` becomes "Next events"; with a non-nil reader the two sentence constants drop the words "events: no data yet" and with nil they stay.
2. `events.go` holds the one table of conventions of Decision 2 (name, `when_text`, importance per kind), builds the rows from `Reader.Between(day, day+30)` as in Decisions 3 and 9, and the `previous` gap of Decision 9.
3. `events.go` adds the per-kind `stale` gaps, the `no_data` "not entered" gaps and the truncation gap of Decisions 8 and 9; an error of the reader is an error of `Build`, never a gap.
4. `events_test.go` has the new tests `TestEventsConventionTexts` (new; no importance tag in the name, zone in `when_text`), `TestEventsWindow` (new; 21 rows give 20 and the truncation gap), `TestEventsNilReaderKeepsGap` (new), `TestEventsStalePerKind` (new), `TestEventsNotEnteredGap` (new), `TestEventsReaderErrorFailsBuild` (new), with a fake reader and a fixed page day, `t.Parallel()`; `builder_test.go` changes only where the title or the sentence changed.
5. `cmd/ot/testdata/macro_page_demo.golden.json` changes only in `events_title`.

### 4 macro-events-wire

Module: ot. Size: M.
Order: after macro-events-job, macro-events-page.
Files: `cmd/ot/wire_macro_events.go`, `cmd/ot/macro_events_page.go`, `cmd/ot/macro_events_wire_test.go`, `cmd/ot/pagelist.go`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`
Design: docs/design/macro-events.md#Decisions (Decision 11)
`cmd/ot/wire.go` (796 lines) is not edited.
Acceptance:
1. `wire_macro_events.go` registers the family `macro_events` as in Decision 11 (key and base URL from `jc.cfg.fredKey` and `fredBaseURL`, `startOff` returns `reasonNoFredKey` for an empty key, writer on `store.DSNEnvSeriesFred` closed through `jc.extra`) and adds no field to `jobFactory` and no hook to `deps`.
2. `jobconsole_jobs.go` appends the row `{ID: "macro_events", Kind: kindAnyDay, Force: true}` to `jobTable`, whose count becomes 32 (the comment "the 31 jobs" becomes "the 32 jobs"); `jobconsole_jobs_test.go` is edited to the count 32 and to the Force list `fred_series`, `compute_figures`, `calendar_refresh`, `fx_refresh`, `bars_refresh`, `macro_events`, and its test `TestForceYesIsFredCalendarFxBarsAndComputeFigures` is renamed `TestForceYesListsMacroEvents` (new name).
3. `macro_events_page.go` holds `newMacroPageEvents` (the page built with the reader) and the reader opener on `store.DSNEnvPages`, nil for the demo profile or an empty line; `pagelist.go` uses them only when the reader is non-nil.
4. `macro_events_wire_test.go` has the new tests `TestMacroEventsRegistered` (new; the job is in the wired list with the id, and off with `reasonNoFredKey` when the key is empty), `TestMacroPageEventsNilWithoutDSN` (new; `deps.macroPage` is called) and `TestMacroPageEventsUsedWithReader` (new), `t.Parallel()`, `t.TempDir()`, no live call.

## Checked by CI

- `go build ./...`, `go vet ./...`, `go test ./...` and `scripts/arch` pass for entries 1 to 4; the `pg` test of entry 1 runs in GitHub CI only.
- `golangci-lint run` and the size check pass; no file of `docs/design/ratchet.json` grows.

## Left out

- Marking an option candidate whose expiry spans an event (the later step; it reads `macroevent.Reader`), and a window set by the latest short-put expiry.
- Treasury auctions and settlement sizes (a new host), ISM, Japan CPI, unscheduled events, BEA and BLS calendars, vendor calendars.
- Reading times from a source; previous values and actuals; a new schema field for importance; a console action that edits the hand-kept file.

## Doubts

A. FOMC, BoJ, quarter-end and tax dates are entered by a worker from their publication pages, so the file is only as right as that reading. Recommend yes: each row carries its url and read day, and the owner can check one page per year. A parser of the Fed and BoJ pages would add two hosts and is not recommended.

## Adversarial

The read is `events-adversarial.md` in the dispatcher window's scratchpad. Each point:

- B1 adopted: entry 4 lists `jobconsole_jobs.go` and its test, states the count 32 and the new Force list.
- B2 adopted, by the second route: the release call is inside the job package, so no `fredrelease` source, no factory field, no `deps` hook; the key comes from `jc.cfg.fredKey` in one place (Decisions 10 and 11).
- B3 adopted: Decision 5 never deletes on an empty answer, on an answer whose latest date is below the stored latest, or a row dated the run day or earlier; new tests in entries 1 and 2.
- B4 adopted: per-kind staleness from `max(run_day)` (Decision 8).
- Q1 adopted: BoJ is the second day of the meeting; the calendar of each kind is stated (Decision 3). Not adopted: a window that reaches the latest short-put expiry, because the page cannot know it cheaply; the window stays 30 days with a truncation gap at 20 (Decision 9).
- Importance adopted: no tag in the name; only the kind is stored and the texts are mapped at read time (Decision 2).
- Q3 adopted: no feed table, `Kinds` is derived; entries 1 and 3 of revision 1 merged and the sources entry dropped, four entries.
- Q5 adopted: the real changes to `builder.go` (field type, title and sentence constants), manifest keys as `fred_series`, a reader-only opener, an untagged `store_test.go`. Not adopted as asked: the page reaches the opener through a new function in `cmd/ot` rather than a changed `deps.macroPage`, to leave the tests that fix its signature alone.
- Q6 adopted: no entry edits `modules.md` (this commit does); "no date typed from memory" is dropped for a test that every row has `url` and `read_day`.
- Migration adopted: "the next free number of `migrations/` when the issue is registered".
- Funding rows adopted: `quarter_end` and `us_tax` kinds from the file; settlement sizes left out.
- H.4.1 and BoJ times: tagged [O] in Decision 2.
