# Week 1 ot: store wiring support, the ot subcommands

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1-wiring.md`, which promises this follow-up for the issues
`store-wiring-support`, `ot-run-once` and `ot-page-serve-setup`. Identifiers were read on main at 9f457d2, except
those of `internal/jobs/fredseries`, which are fixed by the entry `### jobs-fred-series` of `docs/design/week1.md`
and by `internal/jobs/api.go` and are not merged yet.

Goal of the design: `cmd/ot` has the subcommands `run-once`, `page`, `serve` and `setup pg`, each wired through the
function types of the interface files and tested with fakes; the role `ot_runner` can read `master.session_day`. The
pieces are wired, not run on real data (see `## Left out`).

## Modules

No change to `docs/design/modules.md`: every path below belongs to an existing row. `cmd/**` is the row `ot`; its
construction imports are the eleven implementation packages the row lists, and this design imports no other package of
a module from `cmd/ot`.

| issue | module |
|---|---|
| store-wiring-support | store |
| ot-run-once | ot (interface of this cut) |
| ot-page-serve-setup | ot |

Every entry lies in one module. No issue edits `go.mod`, `go.sum`, `package.json` or `pnpm-lock.yaml`.

## Interface

No interface change: no interface file of a module (`internal/*/api.go`, `internal/profile/api.go`) is edited, and every
name below was read on main. The row `ot` has no interface file, but `cmd/ot` has a scaffold that the subcommand files
share, and a second `cmd/ot` issue may not depend on a sibling that is not the (interface) entry. So `ot-run-once`,
the entry that fixes the scaffold, is the one entry marked (interface) under `## Issues`, and `ot-page-serve-setup`
uses the scaffold as it lands.

Names used as they are on main:

| name | owner | file | signature |
|---|---|---|---|
| `clock.NewClockFunc`, `clock.NewCalendarFunc` | clock | internal/clock/api.go | `func() Clock`; `func(SessionSource) Calendar`; implemented by `realclock.NewReal` and `calendar.NewCalendar` |
| `store.SessionDayReader` | store | internal/store/api.go | `SessionDays(ctx, calendarID string, from, to contracts.Day)`, the method of `clock.SessionSource`, so a `store.SessionDayStore` is passed to `NewCalendar` as it is |
| `store.OpenSessionDayFunc`, `store.OpenSeriesFunc`, `store.OpenLedgerFunc`, `store.SetupFunc`, `store.SetupConfig` | store | internal/store/api.go | `func(ctx, dsn string) (SessionDayStore / SeriesStore / OpsLedger, error)`; `func(ctx, SetupConfig) error`; `SetupConfig{AdminDSN, Home string}`; implemented by `sessionday.Open`, `series.Open`, `ops.Open`, `setup.Setup` |
| `store.DSNEnvRunner`, `store.DSNEnvSeriesFred`, `store.DSNEnvPages`, `store.AdminDSNEnv` | store | internal/store/api.go | the names `OT_PG_DSN_RUNNER`, `OT_PG_DSN_SERIES_FRED`, `OT_PG_DSN_PAGES` of the `.env` lines and `OT_PG_ADMIN_DSN` of the process variable |
| `sources.NewFREDFunc`, `sources.FREDConfig` | sources | internal/sources/api.go | `func(FREDConfig) (Source, error)`; `FREDConfig{BaseURL, APIKey string; Clock clock.Clock}`; implemented by `fred.New` |
| `jobs.NewFredSeriesFunc` | jobs | internal/jobs/api.go | `func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error)`; implemented by `fredseries.New` (`### jobs-fred-series` of week1.md) |
| `runner.NewFunc`, `runner.Reconciler` | runner | internal/runner/api.go | `func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) Reconciler`; `Reconcile(ctx, day contracts.Day) error`; implemented by `reconcile.New` |
| `pages.BuildMacroRatesFunc`, `pages.NewHandlerFunc` | pages | internal/pages/api.go | `func(ctx, store.SeriesReader, contracts.Day, generatedAt time.Time) ([]byte, error)`; `func(viewModel []byte) http.Handler`; implemented by `macro_rates.Build` and `embed.Handler` |
| `profile.LoadFunc`, `profile.NewLogHandlerFunc`, `profile.Profile` | repo | internal/profile/api.go | `Profile.Name` is `demo` or `real` |

Names of the `cmd/ot` scaffold, fixed by `ot-run-once` and owned by `cmd/ot` (not exported, no file declares them
apart from the code of that issue):

- `app` gains `getenv func(string) string` and `deps deps`; `command`, `register` and `newApp(getenv, stdout, stderr)`
  keep their signatures.
- `deps` is a struct with the eleven fields `newClock`, `newCalendar`, `openSessionDay`, `openSeries`, `openLedger`,
  `setup`, `newFRED`, `newFredSeries`, `newRunner`, `buildMacroRates`, `newHandler`, typed by the Func types above, and
  `newDeps() deps` in `wire.go` fills it.
- `readEnvFile(path string) (map[string]string, error)` in `env.go`; the key `FRED_API_KEY` of `$OT_HOME/.env` (owner
  ruling of week1.md); the constant `fredBaseURL` (`https://api.stlouisfed.org/fred`).
- The subcommands `run-once`, `page`, `serve`, `setup` (argument `pg`); the flags `--day YYYY-MM-DD` (default the New
  York date of `clock.Now()`), `--out` (page) and `--addr` (serve, default `127.0.0.1:8080`).

## Issues

Wave A: `store-wiring-support` and `ot-run-once` (the second after `jobs-fred-series` of week1.md). Wave B:
`ot-page-serve-setup`, after `ot-run-once`. Items under `Checked by CI` are decided by the workflows of the head
commit; every other item is a fact read from the diff.

### store-wiring-support

The runner role can read the calendar table.
  Goal: `ot run-once` opens the session-day store with the runner DSN, but `migrations/0001_master_session_day.sql` has no grant and the grants of 0002 and 0003 name `ot_runner` on `obs`, `rd` and `ops` only, so the role cannot read `master.session_day`. One more role would need a new password line for no gain, so the existing runner role gets SELECT.
  Files: migrations/0004_session_day_grant.sql, internal/store/sessionday/sessionday_pg_test.go, docs/contracts/store.md.
  Design: docs/design/week1-ot.md#store-wiring-support
  Acceptance:
  - `migrations/0004_session_day_grant.sql` has a `-- +goose Up` and a `-- +goose Down` section; Up holds `GRANT USAGE ON SCHEMA master TO ot_runner` and `GRANT SELECT ON master.session_day TO ot_runner` and no other statement; Down revokes the same two.
  - `sessionday_pg_test.go` (build tag `pg`) creates the roles the way `series_pg_test.go` does (advisory lock, create when missing), applies 0001 and 0004, builds the DSN of `ot_runner` from the `PG*` variables in the test (no literal `postgres://` string with a password), reads a typed row through the real Reader as `ot_runner`, and asserts that an INSERT as `ot_runner` fails.
  - `docs/contracts/store.md` names the grant in `## Inputs` or `## Invariants` (`ot_runner` has SELECT on `master.session_day` and nothing else there) and adds a line to `## Version history`; no heading changes.
  - No file outside the three listed is in the diff; `setup.go` is not edited (it already writes the three DSN lines under the constants of the interface).
  - Checked by CI: ci-go; the pg job applies 0001 to 0004 in order and runs the new test.
  Depends on: none. Order: any.
  Size: S

### ot-run-once

(interface) `ot run-once`: reconcile one session day through the real modules, and the scaffold the other subcommands use.
  Goal: The first subcommand that runs the slice: clock, calendar, ledger, FRED source, `fred_series` job and reconciler wired in `cmd/ot`. This issue also gives `app` the environment and the dependencies, so that the later subcommand issue adds files and edits none of these.
  Files: cmd/ot/main.go, cmd/ot/main_test.go, cmd/ot/wire.go, cmd/ot/env.go, cmd/ot/runonce.go, cmd/ot/wire_test.go, cmd/ot/env_test.go, cmd/ot/runonce_test.go.
  Design: docs/design/week1-ot.md#ot-run-once; `main.go` is edited because `app` has no environment and no dependencies today and a package-level variable would be shared state between parallel tests; own implementation (`env.go`, about 30 lines), because it reads `KEY=VALUE` lines without loading them into the process environment, which `profile.Load` forbids, and a dotenv library would touch secrets.
  Acceptance:
  - `main.go`: `app` gains `getenv` and `deps`; `newApp` keeps its signature and sets `getenv`; `run` sets `deps` from `newDeps()`; no package-level variable holds dependencies; `TestDemoProfile` is unchanged.
  - `wire.go` declares `deps` with the eleven fields of `## Interface` and `newDeps()`, which assigns `realclock.NewReal`, `calendar.NewCalendar`, `sessionday.Open`, `series.Open`, `ops.Open`, `setup.Setup`, `fred.New`, `fredseries.New`, `reconcile.New`, `macro_rates.Build` and `embed.Handler`, each to a field of the Func type, so a drift fails the build. It is the only file of `cmd/ot` that imports an implementation package, and these eleven are all it imports from `internal/`.
  - `env.go` reads `$OT_HOME/.env`: lines `KEY=VALUE`, blank lines and `#` lines skipped, one pair of surrounding quotes removed; a missing file is an error that names the path and carries no value.
  - `runonce.go` registers `run-once` in `init()`. Flow: refuse the `demo` profile before anything is opened; read `store.DSNEnvRunner`, `store.DSNEnvSeriesFred` and `FRED_API_KEY` from the map of `env.go`; build the clock; open the session-day store with the runner DSN and pass it to `newCalendar`; open the ledger with the runner DSN and the series store with the series DSN; build the source with `fredBaseURL`, the key and the clock; build the job; call the reconciler with the day of `--day` or the New York date of `clock.Now()`; close the stores on every path.
  - One slog line per outcome through `a.log` with the day only; an error built from a DSN or key names the variable, never the value.
  - `time.Now` and `time.Sleep` do not appear in `cmd/ot`; `import _ "time/tzdata"` is in `runonce.go`; `cmd/ot` calls no function of an implementation package other than the eleven constructors of `wire.go`.
  - Tests build `deps` from fakes and `app` directly: `--day` reaches `Reconcile`; the default day is the New York date of a fake clock at 2026-11-06T02:00:00Z, which is 2026-11-05; a `demo` profile is refused with no store opened; a missing `FRED_API_KEY` stops before any store is opened; a ledger error carrying a sentinel DSN comes back with the sentinel absent from the error text and from the log; `env_test.go` covers comments, quotes and a missing file. Each test calls `t.Parallel()` and uses `t.TempDir()` for a file; none opens a port or a database.
  - Checked by CI: ci-go (build, vet, test, golangci-lint with depguard and forbidigo, go-arch-lint component `ot`); each new file stays under 600 lines.
  Note (not acceptance): the grant of `store-wiring-support` is a run-time condition of a real database, not a dependency of the code. The calendar table is empty on a real database in week 1, so a real run ends in `UnknownCalendar` (see `## Left out`).
  Depends on: jobs-fred-series of week1.md (`wire.go` imports `internal/jobs/fredseries`). Order: after jobs-fred-series of week1.md.
  Size: M

### ot-page-serve-setup

`ot page` writes the macro_rates view model, `ot serve` serves it, `ot setup pg` calls `Setup`.
  Goal: Calls of `pages.BuildMacroRatesFunc`, `pages.NewHandlerFunc` and `store.SetupFunc` through the scaffold of `ot-run-once`. Pages stay free of a clock and a request: the caller passes `generatedAt` and the day. `ot setup pg` is the command the owner runs once; this design does not run it and no test touches a real `$OT_HOME`.
  Files: cmd/ot/page.go, cmd/ot/serve.go, cmd/ot/demo.go, cmd/ot/setup.go, cmd/ot/page_test.go, cmd/ot/serve_test.go, cmd/ot/setup_test.go.
  Design: docs/design/week1-ot.md#ot-page-serve-setup; the four files only add subcommands to the scaffold and edit none of `ot-run-once`.
  Acceptance:
  - No file of the diff exists in `ot-run-once`; `main.go`, `wire.go` and `env.go` are not in the diff.
  - `page.go` registers `page`; it builds the JSON with `generatedAt = clock.Now().UTC()` and the day of `--day` (default as in run-once) and writes it to `--out` or standard output; with a real profile it opens the series store with the line `store.DSNEnvPages` of `.env` and passes it as a `store.SeriesReader`.
  - `serve.go` registers `serve`; it listens on `--addr` (default `127.0.0.1:8080`), builds the view model anew for each request and answers through `newHandler`; a build error is answered with status 500 and logged without a DSN.
  - `demo.go` is a `store.SeriesReader` over three literal points of `DGS10` typed in the file, one with a nil value, used when the profile is `demo`; no file outside `cmd/ot` is read at run time.
  - `setup.go` registers `setup`; the argument must be `pg`, else usage and an error; it builds `store.SetupConfig{AdminDSN: getenv(store.AdminDSNEnv), Home: getenv("OT_HOME")}` and calls the `setup` field; an empty administrative DSN or an empty home is an error that names the variable; no flag takes the DSN; no value of the config appears in a log line or an error text.
  - `page_test.go` and `serve_test.go`: with a fake builder the bytes of `page` are the bytes the builder returned; `serve` runs on `httptest.NewServer` with a fake handler constructor; a failing builder gives 500; the `demo` profile reaches the demo reader and opens no store.
  - `setup_test.go`: a fake `store.SetupFunc` records its config with `OT_HOME` set to a `t.TempDir()`; both values arrive; an unset variable stops before the fake is called; a fake error carrying a sentinel DSN is returned with the sentinel absent from its text.
  - Every test calls `t.Parallel()`; none sleeps or listens on a fixed port.
  - Checked by CI: ci-go; each new file stays under 600 lines.
  Depends on: ot-run-once (the (interface) entry of this cut). Order: after ot-run-once.
  Size: M

## Left out

- Applying the migrations on a real database: `setup.Setup` creates the database, the extension and the roles and writes
  the `.env` lines, but runs no migration, and `internal/store/migrate` is not one of the eleven packages the row `ot`
  may import, nor does the repository embed `migrations/`. A later design decides where `migrate.Up` is called (inside
  store, or a twelfth package on the import list, which only the owner widens). Until then the owner applies the
  files with goose by hand.
- Population of `master.session_day`: the table is empty on a real database in week 1 (the calendar generator is not
  built), so a real `ot run-once` ends in `UnknownCalendar`; the tests use fakes. Loading
  `contracts/fixtures/master.session_day.r1.json` through `store.SessionDayWriter` is one more subcommand for a later
  design.
- The window rule: `run-once` uses `runner.NewFunc`, that is `reconcile.New` with the zero rule; `reconcile.NewWithRule`
  behind a Func type waits for a populated calendar.
- `docs/contracts/jobs.md` and the contract documents of ot: not touched here.
- Embedding the real web build into `internal/pages/embed/dist`: a later issue of pages with the owner.
- `Page` and `Builder` names of pages: `page` and `serve` call the one `BuildMacroRatesFunc`.
- A named adapter from `store.SessionDayReader` to `clock.SessionSource`: not chosen, the store satisfies the interface.
- A dotenv library for `.env`: not chosen, it would touch secrets and load them into the process environment.
- The broker, compute and notify wiring: later weeks.

## Adversarial
