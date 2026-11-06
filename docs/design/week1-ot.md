# Week 1 ot: store wiring support, the ot subcommands

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1-wiring.md`, which promises this follow-up for the issues
`store-wiring-support`, `ot-run-once` and `ot-page-serve-setup`. Identifiers were read on main at 9f457d2, except
those of `internal/jobs/fredseries`, which are fixed by the entry `### jobs-fred-series` of `docs/design/week1.md`
and by `internal/jobs/api.go` and are not merged yet.

Goal of the design: `cmd/ot` has the subcommands `run-once`, `page`, `serve` and `setup pg`, each wired through the
function types of the interface files; the role `ot_runner` can read `master.session_day`. The slice is wired and
fake-tested, not runnable against a real database: nothing applies the migrations and the calendar table stays empty
(see `## Left out`).

## Modules

No change to `docs/design/modules.md`: every path below belongs to an existing row. `cmd/**` is the row `ot`; it may
import the root packages of the rows in its depends cell, `internal/profile`, and only the eleven implementation
packages the row lists.

| issue | module |
|---|---|
| store-wiring-support | store |
| ot-wiring | ot |

Every entry lies in one module. No issue edits `go.mod`, `go.sum`, `package.json` or `pnpm-lock.yaml`.

## Interface

No interface change: no interface file of a module (`internal/*/api.go`, `internal/profile/api.go`) is edited, and no
entry is marked (interface). Every name below was read on main.

Names used as they are:

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

Names fixed by `ot-wiring`, owned by `cmd/ot` (unexported):

- `app` gains `getenv func(string) string` and `deps deps`; `command`, `register` and `newApp(getenv, stdout, stderr)`
  keep their signatures.
- `deps` has the eleven fields `newClock`, `newCalendar`, `openSessionDay`, `openSeries`, `openLedger`, `setup`,
  `newFRED`, `newFredSeries`, `newRunner`, `buildMacroRates`, `newHandler`, typed by the Func types above; `newDeps()`
  in `wire.go` fills it.
- `readEnvFile(path string) (map[string]string, error)` in `env.go`; the key `FRED_API_KEY` of `$OT_HOME/.env`; the
  constant `fredBaseURL` (`https://api.stlouisfed.org/fred`).
- `dayOrDefault(clk clock.Clock, flag string) (contracts.Day, error)` in `day.go`: the one helper for `--day`; an empty
  flag gives the date of `clk.Now()` in the constant zone `marketZone` (`America/New_York`), a malformed flag is an
  error. `run-once`, `page` and `serve` all call it.
- The subcommands `run-once`, `page`, `serve`, `setup` (argument `pg`); the flags `--day YYYY-MM-DD`, `--out` (page)
  and `--addr` (serve, default `127.0.0.1:8080`).
- `serveOn(ctx context.Context, ln net.Listener, build func(context.Context) ([]byte, error), newHandler pages.NewHandlerFunc) error`
  in `serve.go`: serves until `ctx` is done; the `serve` command listens on `--addr` and passes a context that ends on
  an interrupt.

## Issues

`store-wiring-support` waits for nothing. `ot-wiring` waits for `jobs-fred-series` of week1.md. Items under `Checked by
CI` are decided by the workflows of the head commit; every other item is a fact read from the diff, with an offline grep
where it is about behaviour.

### store-wiring-support

The runner role can read the calendar table.
  Goal: `ot run-once` opens the session-day store with the runner DSN, but `migrations/0001_master_session_day.sql` has no grant and the grants of 0002 and 0003 name `ot_runner` on `obs`, `rd` and `ops` only, so the role cannot read `master.session_day`. One more role would need a new password line for no gain, so the existing runner role gets SELECT.
  Files: migrations/0004_session_day_grant.sql, internal/store/sessionday/sessionday_pg_test.go, docs/contracts/store.md.
  Design: docs/design/week1-ot.md#store-wiring-support
  Acceptance:
  - `migrations/0004_session_day_grant.sql` has a `-- +goose Up` and a `-- +goose Down` section; Up holds `GRANT USAGE ON SCHEMA master TO ot_runner` and `GRANT SELECT ON master.session_day TO ot_runner` and no other statement; Down revokes the same two.
  - `sessionday_pg_test.go` (build tag `pg`) creates the four roles itself when missing, under an advisory lock, as `series_pg_test.go` does with its private `ensureRoles` (about 25 lines copied), and applies 0001 and 0004 itself; the file holds a function `TestRunnerReadsOnlyPG` and a `t.Parallel()` (offline: `grep -n 'func TestRunnerReadsOnlyPG' internal/store/sessionday/sessionday_pg_test.go` finds it, and `grep -c 'postgres://[^"]*:[^"]*@' internal/store/sessionday/sessionday_pg_test.go` prints 0, the DSN being built from the `PG*` variables).
  - `docs/contracts/store.md` names the grant in `## Invariants` (`ot_runner` has SELECT on `master.session_day` and nothing else there) and adds a line to `## Version history`; no heading changes.
  - No file outside the three listed is in the diff; `setup.go` is not edited (it already writes the three DSN lines under the constants of the interface).
  - Checked by CI: ci-go; the `test-pg` job runs `TestRunnerReadsOnlyPG`, which reads a typed row through the real Reader as `ot_runner` and asserts that an INSERT as `ot_runner` fails.
  Depends on: none. Order: any.
  Size: S

### ot-wiring

The `cmd/ot` scaffold and the subcommands `run-once`, `page`, `serve` and `setup pg`.
  Goal: Calls of the constructors of the wiring table through one `deps` struct: `run-once` reconciles one day through clock, calendar, ledger, FRED source, `fred_series` job and reconciler; `page` writes the macro_rates view model; `serve` serves it; `setup pg` calls `Setup`. Pages stay free of a clock and a request: the caller passes `generatedAt` and the day. The slice is wired and fake-tested; `ot setup pg` is the command the owner runs once and no test touches a real `$OT_HOME`.
  Files: cmd/ot/main.go, cmd/ot/main_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/env.go, cmd/ot/env_test.go, cmd/ot/day.go, cmd/ot/day_test.go, cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/page.go, cmd/ot/page_test.go, cmd/ot/serve.go, cmd/ot/serve_test.go, cmd/ot/demo.go, cmd/ot/setup.go, cmd/ot/setup_test.go.
  Design: docs/design/week1-ot.md#ot-wiring; one issue because every subcommand file uses the scaffold and a split would make one cmd/ot issue wait for a sibling; split by file so each new file stays small. `main.go` is edited because `app` has no environment and no dependencies and a package-level variable would be shared state between parallel tests; own implementation (`env.go`, about 30 lines), because it reads `KEY=VALUE` lines without loading them into the process environment, which `profile.Load` forbids, a dotenv library would touch secrets, and `internal/store/setup` exports no `.env` reader.
  Acceptance:
  - `main.go`: `app` gains `getenv` and `deps`; `newApp` keeps its signature and sets `getenv`; `run` sets `deps` from `newDeps()`; no package-level variable holds dependencies; `TestDemoProfile` of `main_test.go` is unchanged.
  - `wire.go` declares `deps` with the eleven fields of `## Interface` and `newDeps()`, which assigns `realclock.NewReal`, `calendar.NewCalendar`, `sessionday.Open`, `series.Open`, `ops.Open`, `setup.Setup`, `fred.New`, `fredseries.New`, `reconcile.New`, `macro_rates.Build` and `embed.Handler`, each to its field. It imports the root packages of the rows in the depends cell of `ot` that the Func types need, and of the implementation packages only these eleven. `TestNewDepsFilled` of `wire_test.go` asserts no field of `newDeps()` is nil.
  - Offline: `grep -rlE --include='*.go' 'internal/(clock/(realclock|calendar)|store/(sessionday|series|ops|setup)|sources/fred|jobs/fredseries|runner/reconcile|pages/(macro_rates|embed))' cmd/ot | grep -v _test.go` prints `cmd/ot/wire.go` only, and `grep -rn 'internal/' cmd/ot --include=*.go | grep -v '_test.go' | grep -v 'cmd/ot/wire.go'` lists no implementation package.
  - `env.go` reads `$OT_HOME/.env`: lines `KEY=VALUE`, blank lines and `#` lines skipped, one pair of surrounding quotes removed; a missing file is an error that names the path and carries no value (`TestReadEnvFileComments`, `TestReadEnvFileQuotes`, `TestReadEnvFileMissing` of `env_test.go`).
  - `day.go` holds `dayOrDefault`; `TestDayDefaultIsNewYork` uses a fake clock at 2026-11-06T02:00:00Z and expects 2026-11-05; `TestDayFlagWins` and `TestDayMalformed` cover the flag; `day.go` carries `import _ "time/tzdata"`; `run-once`, `page` and `serve` each call `dayOrDefault` (offline: `grep -c 'dayOrDefault(' cmd/ot/runonce.go cmd/ot/page.go cmd/ot/serve.go` prints 1 or more for each file).
  - `runonce.go` registers `run-once` in `init()`. Flow: refuse the `demo` profile before anything is opened; read `store.DSNEnvRunner`, `store.DSNEnvSeriesFred` and `FRED_API_KEY` from the map of `env.go`; build the clock; open the session-day store with the runner DSN and pass it to `newCalendar`; open the ledger with the runner DSN and the series store with the series DSN; build the source with `fredBaseURL`, the key and the clock; build the job; call the reconciler with `dayOrDefault`; close the stores on every path. Tests of `runonce_test.go`: `TestRunOnceDayReachesReconcile`, `TestRunOnceDemoRefused` (no store opened), `TestRunOnceMissingKey` (stops before any store is opened), `TestRunOnceClosesStores`.
  - Errors from a DSN or a key never carry the value: an error from an open function is wrapped by naming the variable and the error text of the open function is dropped, so the DSN value is not in the returned text; `TestRunOnceSentinelDSN` makes a fake `openLedger` return an error whose text holds a sentinel DSN and asserts the sentinel is absent from the error and from the log output. One slog line per outcome through `a.log` with the day only.
  - `page.go` registers `page`: it builds the JSON with `generatedAt = clock.Now().UTC()` and `dayOrDefault`, writes it to `--out` or standard output, and with a real profile opens the series store with the line `store.DSNEnvPages` of `.env` and passes it as a `store.SeriesReader` (`TestPageBytesAreBuilderBytes`, `TestPageDemoOpensNoStore`).
  - `demo.go` is a `store.SeriesReader` over three literal points of `DGS10` typed in the file, one with a nil value, used when the profile is `demo`; no file outside `cmd/ot` is read at run time.
  - `serve.go` registers `serve` and holds `serveOn` of `## Interface`: it builds the view model anew for each request, answers through `newHandler`, answers a build error with status 500 logged without a DSN, and returns when `ctx` is done. `TestServeOnAnswersBuilderBytes` and `TestServeOnBuildError500` run it on a listener the test opens on `127.0.0.1:0` and stop it by cancelling the context; no fixed port.
  - `setup.go` registers `setup`: the argument must be `pg`, else usage and an error; it builds `store.SetupConfig{AdminDSN: getenv(store.AdminDSNEnv), Home: getenv("OT_HOME")}` and calls the `setup` field; an empty administrative DSN or an empty home is an error that names the variable; no flag takes the DSN; the error text of the `setup` field is dropped as above. `TestSetupPassesConfig` (`OT_HOME` a `t.TempDir()`), `TestSetupUnsetStopsBeforeCall`, `TestSetupArgumentMustBePg`, `TestSetupSentinelDSN`.
  - Every test of the listed test files calls `t.Parallel()` (offline: `grep -c 't.Parallel()'` of each `_test.go` file is at least the count of its `func Test` lines); none sleeps, and `time.Now` and `time.Sleep` do not appear in `cmd/ot`.
  - Checked by CI: ci-go (build, vet, test, golangci-lint with depguard, forbidigo, noctx and contextcheck, go-arch-lint component `ot`); each new file stays under 600 lines.
  Note (not acceptance): the grant of `store-wiring-support` is a run-time condition of a real database, not a dependency of the code.
  Depends on: jobs-fred-series of week1.md (`wire.go` imports `internal/jobs/fredseries`). Order: after jobs-fred-series of week1.md.
  Size: L

## Left out

- Applying the migrations and loading `master.session_day`: `setup.Setup` creates the database, the extension and the
  roles and writes the `.env` lines, but runs no migration; the table stays empty, so a real `ot run-once` ends in
  `UnknownCalendar`. Both need `internal/store/migrate` (and a loader of `contracts/fixtures/master.session_day.r1.json`
  through `store.SessionDayWriter`) called from `cmd/ot`, which is not one of the eleven implementation packages on the
  import list of `docs/design/modules.md`: a change of that file and a design of its own. Until then the owner applies
  the migration files by hand.
- Known lock-in: `cmd/ot/env.go` is a second `.env` parser next to the unexported one of `internal/store/setup`, with
  its own quoting rule. A later store issue may export one reader and `env.go` then goes.
- The window rule: `run-once` uses `runner.NewFunc`, that is `reconcile.New` with the zero rule; `reconcile.NewWithRule`
  behind a Func type waits for a populated calendar.
- `docs/contracts/jobs.md` and the contract documents of ot: not touched here.
- Embedding the real web build into `internal/pages/embed/dist`: a later issue of pages with the owner.
- `Page` and `Builder` names of pages: `page` and `serve` call the one `BuildMacroRatesFunc`.
- A named adapter from `store.SessionDayReader` to `clock.SessionSource`: not chosen, the store satisfies the interface.
- The broker, compute and notify wiring: later weeks.

## Adversarial

Source: the scratchpad file week1-ot.adversarial.md of the dispatcher window (kept there, not published).

- 1 gap against the goal: adopted; the goal says wired and fake-tested, and `## Left out` names the follow-up for migrations and the calendar load, which needs a change of the import list.
- 2 biggest risk (unmerged jobs package, helper for the day): adopted; `dayOrDefault` is named in the scaffold and `marketZone` is a constant; the jobs coupling stays as the Order line and the identifiers follow week1.md and `internal/jobs/api.go`.
- 3 simpler way (merge the two ot issues): adopted; one `ot-wiring` issue, split by file.
- 4 lock-in: adopted in part; the second `.env` parser is recorded under `## Left out`; the shared `wire.go` hot spot and the grant on `ot_runner` are accepted as they are.
- 5 false claims: adopted; the import sentence names the root packages and the eleven packages, and the pg claim says the test applies 0001 to 0004 and creates the roles itself.
- 6 slip causes: adopted; no (interface) mark, test functions named with grep substitutes, pg items under Checked by CI, the DSN text dropped from errors, and `serveOn` takes a listener and a context.
