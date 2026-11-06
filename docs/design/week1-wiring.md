# Week 1 wiring: cmd/ot, the other page schemas, fixture validation

Author: advisor of optdesk (second draft)
Source: `## Left out` of `docs/design/week1.md` (the earlier issues 13 wire-run-once, 15 page-schemas-28, 16
web-fixtures), cut against the code on origin/main at 9b6f5f0. Identifiers below were read there; a name marked
[not landed] belongs to an issue of week1.md that has not merged (store-session-day, store-series-ops,
jobs-fred-series) and is stated here as the design expects it.

Goal of the design: the pieces of week 1 are wired in `cmd/ot` and tested with fakes. They are wired, not run on
real data: `master.session_day` stays empty on a real database (see Left out), so `ot run-once` on a real database
ends in `UnknownCalendar` until a later issue fills the table. The other 29 page schemas are in `contracts/schemas`
and every page fixture of `web/fixtures` is validated against them by a Go test.

## Modules

No change to `docs/design/modules.md`: every path below belongs to an existing row. Construction in `cmd/ot` is
allowed by its row ("the eleven packages named above for construction", line 119 of modules.md, the list of
`checks.yml` at line 43 of week1.md): `internal/clock/realclock`, `internal/clock/calendar`,
`internal/store/sessionday`, `internal/store/series`, `internal/store/ops`, `internal/store/setup`,
`internal/sources/fred`, `internal/jobs/fredseries`, `internal/runner/reconcile`, `internal/pages/macro_rates`,
`internal/pages/embed`; the `ot` row's depends cell adds `repo` for `internal/profile`.

| issue | module |
|---|---|
| store-dsn-names | store (interface) |
| schemas-28 | contracts |
| contracts-fixture-validation | contracts |
| store-wiring-support | store |
| ot-run-once | ot |
| ot-page-serve-setup | ot |

Every entry lies in one module. `store-dsn-names` is the one entry marked (interface): it edits the interface file
`internal/store/api.go`, which `cmd/ot` and `internal/store/setup` both use. `schemas-28` adds new files under
`contracts/schemas`, which are not an interface file of the map; AGENTS.md calls the JSON schema of a page the
TypeScript interface, so that issue carries a Design note and lands before any use (see its entry). No issue of this
design edits `go.mod`, `go.sum`, `package.json` or `pnpm-lock.yaml`; the Go test uses the santhosh-tekuri library
that is already a direct dependency.

## Interface

Frozen names, each read on origin/main. The owner of a name is the module of its file.

Landed and used as they are (no edit):

| name | owner | file | signature as landed |
|---|---|---|---|
| `clock.NewClockFunc` | clock | internal/clock/api.go | `func() Clock`; implemented by `realclock.NewReal` |
| `clock.NewCalendarFunc` | clock | internal/clock/api.go | `func(SessionSource) Calendar`; implemented by `calendar.NewCalendar` |
| `clock.SessionSource` | clock | internal/clock/api.go | `SessionDays(ctx, calendarID string, from, to contracts.Day) ([]contracts.SessionDay, error)` |
| `store.SessionDayReader` | store | internal/store/api.go | the same method, same parameters, same result as `SessionSource` |
| `store.OpenSessionDayFunc`, `OpenSeriesFunc`, `OpenLedgerFunc`, `SetupFunc` | store | internal/store/api.go | `func(ctx, dsn string) (SessionDayStore / SeriesStore / OpsLedger, error)`; `func(ctx, SetupConfig) error`; the stores embed their Reader and Writer and have `Close() error` |
| `store.SetupConfig` | store | internal/store/api.go | `AdminDSN string`, `Home string` |
| `sources.NewFREDFunc`, `sources.FREDConfig` | sources | internal/sources/api.go | `func(FREDConfig) (Source, error)`; `FREDConfig{BaseURL, APIKey string; Clock clock.Clock}` |
| `jobs.NewFredSeriesFunc`, `jobs.Registry` | jobs | internal/jobs/api.go | `func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error)`; `Registry` is `[]contracts.Job` |
| `runner.NewFunc`, `runner.Reconciler` | runner | internal/runner/api.go | `func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) Reconciler`; `Reconcile(ctx, day contracts.Day) error`; implemented by `reconcile.New` (the zero window rule: calendar `XNYS`, no offsets) |
| `pages.BuildMacroRatesFunc`, `pages.NewHandlerFunc` | pages | internal/pages/api.go | as in the wiring table of week1.md; implemented by `macro_rates.Build` and `embed.Handler` |
| `profile.LoadFunc`, `profile.NewLogHandlerFunc` | repo | internal/profile/api.go | already used by `newApp` in `cmd/ot/main.go` |
| `app`, `command`, `register(name, run)`, `newApp(getenv, stdout, stderr)`, `run(args, getenv, stdout, stderr)` | ot | cmd/ot/main.go | `app` has the fields `profile`, `log`, `stdout`; `command` is `func(a *app, args []string) error`; subcommand files register in `init()` |

Consequence of the table: `store.SessionDayReader` and `clock.SessionSource` have one identical method, so a
`store.SessionDayStore` already satisfies `clock.SessionSource`. The adapter named in week1.md is therefore not a
type: `cmd/ot` passes the store to `NewCalendar` and holds `var _ clock.SessionSource = store.SessionDayReader(nil)`
as the compile-time check that the two stay equal.

Added by `store-dsn-names` (the only interface issue of this design), in `internal/store/api.go`:

| name | meaning |
|---|---|
| `DSNEnvRunner`, `DSNEnvSeriesFred`, `DSNEnvPages` | string constants `OT_PG_DSN_RUNNER` (role `ot_runner`), `OT_PG_DSN_SERIES_FRED` (role `ot_w_series_fred`), `OT_PG_DSN_PAGES` (role `ot_ro_pages`): the names of the lines `Setup` writes to `$OT_HOME/.env` and `cmd/ot` reads. Two modules need them, store (the writer) and ot (the reader), so they are declared once, in the interface |
| `AdminDSNEnv` | string constant `OT_PG_ADMIN_DSN`: the process environment variable that carries `SetupConfig.AdminDSN`; it is read from the process environment only, never from `.env` and never from a flag |

Constants named `OT_*` and `*DSN*` in Go files are not environment variables; `scripts/hygiene` scans no identifier
names and the `environment` job of `hygiene.yml` greps the environment of the runner, so the names are free in code. They can never
be set in a workflow `env:` block; the pg tests build their DSN in the test from the `PG*` variables of the test-pg
job.

Names fixed here without a declaration, owned by `cmd/ot`: the key `FRED_API_KEY` of `$OT_HOME/.env` (owner ruling
of week1.md, issue 8); the default FRED base URL, a constant of `cmd/ot` (`https://api.stlouisfed.org/fred`); the
subcommands `run-once`, `setup`, `page` and `serve`; the flags `--day YYYY-MM-DD` (default: the New York date of
`clock.Now()`), `--out` (page), `--addr` (serve, default `127.0.0.1:8080`) and the word `pg` after `setup`.

Not frozen (Left out): `Page` and `Builder` of pages; any window rule.

## Issues

Order by dependency. Wave A (nothing to wait for): `store-dsn-names`, `schemas-28`. Wave B: `contracts-fixture-validation`,
`store-wiring-support`, `ot-run-once`. Wave C: `ot-page-serve-setup`. Items under "Checked by CI" are decided by the
workflow of the head commit (ci-go, checks, hygiene, and the pg job for pg-tagged tests); the facts above them are
read from the diff.

### store-dsn-names

(interface) The constants `DSNEnvRunner`, `DSNEnvSeriesFred`, `DSNEnvPages` and `AdminDSNEnv` of `## Interface`. Declarations only; no module issue waits on behaviour.
  Goal: `Setup` writes, and `cmd/ot` reads, the names of the `OT_PG_DSN_*` lines; the landed interface has no name for them.
  Files: internal/store/api.go, internal/store/api_test.go.
  Design: docs/design/week1-wiring.md#store-dsn-names; the constants are in store and not in `cmd/ot` because `internal/store/setup` writes the same lines and may not import `cmd/ot`.
  Acceptance:
  - `internal/store/api.go` declares the four string constants with the values of `## Interface`, each with a doc comment naming the role it belongs to or, for `AdminDSNEnv`, the process environment.
  - Its import list is unchanged, and the file still holds only package, import, type, alias, const and interface declarations (no `func`, no `var`).
  - `api_test.go` asserts that the three `DSNEnv*` constants are distinct and start with `OT_PG_DSN_`, and that `AdminDSNEnv` is not equal to any of them.
  - Checked by CI: `go build`, `go vet`, `go test` of `internal/store`; go-arch-lint (import list of `api.go` unchanged); checks (`Contracts: unchanged`).
  Depends on: none (everything it edits is landed).
  Size: S

### schemas-28

The other page schemas of the old repository, copied unchanged, with a manifest of the copies.
  Goal: The schemas the migrated front end already follows are in `contracts/schemas`, so their fixtures can be validated. This is the carry-over of week1.md#left-out, not the cleanup pass of week 2.
  Files: contracts/schemas/page.advice_page.v1.json, page.companies_page.v1.json, page.full_scan.v1.json, page.held_companies_summary.v1.json, page.home_page.v1.json, page.household_page.v1.json, page.household_risk.v1.json, page.household_summary.v1.json, page.indicator_detail.v1.json, page.ledger_page.v1.json, page.macro_page.v1.json, page.macro_summary.v1.json, page.market_summary.v1.json, page.market_weather.v1.json, page.option_page.v1.json, page.option_result_page.v1.json, page.options_summary.v1.json, page.positions_page.v1.json, page.positions_page.v2.json, page.positions_page.v3.json, page.report_page.v1.json, page.report_page.v2.json, page.settings_page.v1.json, page.stock_option_page.v1.json, page.structure_detail.v1.json, page.structure_detail.v2.json, page.structure_detail.v3.json, page.structures_list.v1.json, page.tracking.v1.json (all under contracts/schemas/), contracts/schemas/carryover.sha256, internal/contracts/carryover_test.go, docs/contracts/page-schemas.md.
  Design: docs/design/week1-wiring.md#schemas-28; interface note: no file of this issue is an interface file of `docs/design/modules.md`, but AGENTS.md calls the JSON schema of a page an interface, so the issue only adds files, edits no existing schema and lands before any issue that reads them; the content is the bytes of the source, so `$id` still names the old host (`option-trader.invalid`) and the manifest is deleted by the week-2 pass that renames it.
  Acceptance:
  - The Files line lists 29 new schema files, named `page.` plus the name of a file of `docs/schemas/` of the old repository (week1.md says 28; the list holds 29 and the pull request description names the file the earlier count left out).
  - `contracts/schemas/carryover.sha256` has one line per listed schema, in the order of the Files line, each line `<sha256>  contracts/schemas/<file>`; `page.macro_rates.v1.json` is not in it.
  - `carryover_test.go` computes the sha256 of every `page.*.json` of `contracts/schemas` except `page.macro_rates.v1.json`, compares it with the manifest, fails on a file missing from the manifest or a manifest line without a file, and compiles each schema with santhosh-tekuri/jsonschema/v6 (the `x-decimals`, `x-timezone` and `x-rounding` keywords are unknown keywords the compiler ignores). The expected hashes are in the manifest, not computed from the schemas the test checks.
  - `docs/contracts/page-schemas.md` is created (it does not exist on origin/main) with the headings `## Responsibility`, `## Inputs`, `## Outputs`, `## Invariants`, `## Version history`; it lists the 30 schema ids with their `schema_version` const and says in a sentence that the copies are unchanged and the week-2 pass writes new versions only.
  - `page.macro_rates.v1.json`, its fixtures and `MANIFEST.synthetic-v1.json` of `contracts/fixtures` are not in the diff.
  - The pull request body has `Contracts: ` followed by one `page.<name>.vN (new)` entry per file, as the format check of `checks.yml` accepts.
  - Checked by CI: ci-go (the test, which proves that each hash is that of the copy); checks (the Contracts line, generated code unchanged since nothing is generated from these files); hygiene (no account id, path or e-mail); the size rule (JSON data is not counted).
  Depends on: none. Order: before contracts-fixture-validation.
  Size: M

### contracts-fixture-validation

Every page fixture of `web/fixtures` is validated against the schemas by a Go test.
  Goal: The 13 fixture directories were copied by web-migrate; nothing checks them against a schema yet. The test reads `contracts/schemas` and `web/fixtures` and makes the two sides agree. Reading the files of the web module is a read, not an edit; the pull request names it.
  Files: internal/contracts/fixtures_test.go.
  Design: docs/design/week1-wiring.md#contracts-fixture-validation; a Go test with santhosh-tekuri/jsonschema/v6, already a direct dependency of go.mod, so there is no new dependency and no whitelist change. The test is a Go file and not a vitest file because the front end only renders data (AGENTS.md).
  Acceptance:
  - The test holds a table, typed by hand, from fixture directory to schema file: `advice-page`, `companies-page`, `full-scan`, `held-companies-summary`, `household-page`, `household-summary`, `macro-page`, `macro-summary`, `market-weather`, `option-page`, `structures-list`, `tracking` map to the `v1` schema of the same name with `-` as `_`, and `positions-page` maps to `page.positions_page.v3.json`; it fails when a directory of `web/fixtures` has no row.
  - It loops over every `*.json` file of every fixture directory, the five extra files of `market-weather` (`intraday*.json`, `mismatch.json`) included, and skips the `README.md` files; the files named `bad-version.json` must fail validation, with the error on `schema_version`; every other file must validate.
  - The schema is compiled with the `x-decimals`, `x-timezone` and `x-rounding` keywords unknown (ignored); the paths are relative to the test file (`../../web/fixtures`, `../../contracts/schemas`).
  - `t.Parallel()` in the test and in each subtest; no write, no network, no sleep.
  - `web/fixtures` and its `MANIFEST.synthetic-v1.json` are not in the diff.
  - Checked by CI: ci-go. If a fixture does not validate, the pull request reports the fixture and the schema path in its description and leaves both unchanged.
  Depends on: schemas-28. Order: after schemas-28.
  Size: S

### store-wiring-support

The runner role can read the calendar table, and `Setup` writes the DSN lines under the frozen names.
  Goal: Two facts the wiring depends on and week1.md does not state: which role reads `master.session_day` (the roles of store-series-ops grant `ot_runner` SELECT on the series tables only), and that the lines of `.env` carry the names of `store-dsn-names`. `ot run-once` opens the session-day store with the runner DSN.
  Files: migrations/0004_session_day_grant.sql, internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/sessionday/sessionday_pg_test.go, docs/contracts/store.md.
  Design: docs/design/week1-wiring.md#store-wiring-support; one more role would need a new password line for no gain, so the existing runner role gets SELECT. It edits files of two unmerged issues (store-series-ops for `setup.go`, store-session-day for `sessionday_pg_test.go`) and therefore waits for both.
  Acceptance:
  - `migrations/0004_session_day_grant.sql` has an Up and a Down section for goose; Up grants SELECT on `master.session_day` to `ot_runner` and nothing else; Down revokes it.
  - `setup.go` writes the three lines under `store.DSNEnvRunner`, `store.DSNEnvSeriesFred` and `store.DSNEnvPages` (the constants, not string literals) and reads the administrative DSN through nothing but `SetupConfig.AdminDSN`; `setup_test.go` asserts, in a `t.TempDir()`, that the three names appear in the file written, and, with a sentinel password injected, that the sentinel appears in no log line.
  - `sessionday_pg_test.go` (build tag `pg`) builds the DSN of `ot_runner` in the test from generated parts and the `PG*` variables of the job (no literal `postgres://user:password@` string in the file), reads a typed row of `master.session_day`, and asserts that an INSERT as `ot_runner` fails.
  - `docs/contracts/store.md` does not exist on origin/main: this issue creates it with the five headings of `docs/contracts/runner.md` up to `## Version history` if no landed issue has, and lists the three names under `## Inputs`; if the file exists it gains the same entries and no heading changes.
  - Checked by CI: ci-go unit tests; the pg job for the grant (applied after 0001 to 0003).
  Depends on: store-dsn-names, store-session-day [in flight], store-series-ops [not landed] (it edits their `setup.go` and test file). Order: after those three.
  Size: S

### ot-run-once

`ot run-once`: reconcile one session day through the real modules.
  Goal: The first subcommand that runs the slice: clock, calendar, ledger, FRED source, `fred_series` job and reconciler wired in `cmd/ot`. This issue also gives `app` the environment and the dependencies the other subcommands use.
  Files: cmd/ot/main.go, cmd/ot/main_test.go, cmd/ot/wire.go, cmd/ot/env.go, cmd/ot/runonce.go, cmd/ot/wire_test.go, cmd/ot/env_test.go, cmd/ot/runonce_test.go.
  Design: docs/design/week1-wiring.md#ot-run-once; `main.go` is edited because `app` has no environment and no dependencies today and a package-level `deps` would be shared state between parallel tests; own implementation (cmd/ot/env.go, about 30 lines), because it only reads `KEY=VALUE` lines of `$OT_HOME/.env` and the existing dotenv libraries load them into the process environment, which `profile.Load` forbids, and a library that touches secrets needs the owner's word.
  Acceptance:
  - `main.go`: `app` gains `getenv func(string) string` and `deps deps`; `newApp` keeps its signature and sets `getenv`; `run` sets `deps` from the one constructor of `wire.go`; no package-level variable holds dependencies; `TestDemoProfile` is unchanged and passes (the new environment reads happen only inside subcommands).
  - `wire.go` declares one struct `deps` whose fields are the Func types of the wiring table (`clock.NewClockFunc`, `clock.NewCalendarFunc`, `store.OpenSessionDayFunc`, `store.OpenSeriesFunc`, `store.OpenLedgerFunc`, `store.SetupFunc`, `sources.NewFREDFunc`, `jobs.NewFredSeriesFunc`, `runner.NewFunc`, `pages.BuildMacroRatesFunc`, `pages.NewHandlerFunc`) and one function that fills it with `realclock.NewReal`, `calendar.NewCalendar`, `sessionday.Open`, `series.Open`, `ops.Open`, `setup.Setup`, `fred.New`, `fredseries.New`, `reconcile.New`, `macro_rates.Build`, `embed.Handler`; each assignment is typed, so a drift fails the build. It is the only file of `cmd/ot` that imports an implementation package.
  - `env.go` reads `$OT_HOME/.env` (lines `KEY=VALUE`, `#` comments and blank lines skipped, optional surrounding quotes removed) and returns a map; a missing file is an error that names the path and no value.
  - `runonce.go` registers `run-once` in an `init()`. The flow: refuse the `demo` profile; read the runner and series DSNs (by `store.DSNEnvRunner`, `store.DSNEnvSeriesFred`) and `FRED_API_KEY` from the `.env` map; build the clock; open the session-day store with the runner DSN and pass it to `NewCalendar`; open the ledger with the runner DSN and the series store with the series DSN; build the source with the default base URL, the key and the clock; build the job; call the reconciler with the day of `--day` or, by default, the New York date of `clock.Now()`; close the stores in all cases.
  - One slog line per outcome through `a.log` with the day only; no DSN, key or password in a log line or an error text; an error built from a DSN or key names the variable, not the value.
  - `cmd/ot` calls no function of the packages above other than those of the wiring table; `time.Now` and `time.Sleep` do not appear; `import _ "time/tzdata"` makes the New York zone available without a system database.
  - Tests build `deps` from fakes in the test and `app` directly: the day flag reaches `Reconcile`; the default day is the New York date of a fake clock at 2026-11-06T02:00:00Z (that is 2026-11-05); a `demo` profile refuses with no store opened; a missing `FRED_API_KEY` stops before any store is opened; a ledger error carrying a sentinel DSN is returned with the sentinel absent from the error text and the log; `env_test.go` covers comments, quotes and a missing file. Every test calls `t.Parallel()` and uses `t.TempDir()` for any file; none opens a port or a database.
  - Checked by CI: ci-go (build, vet, test, golangci-lint with depguard and forbidigo, go-arch-lint component `ot`); each file stays under 600 lines.
  Depends on: store-dsn-names, store-session-day, store-series-ops, jobs-fred-series (`wire.go` imports `sessionday`, `series`, `ops`, `setup` and `fredseries`, which have not landed; the clock, fred, reconcile, macro_rates and embed packages are on origin/main; the grant of store-wiring-support is a run-time condition, not a dependency). Order: after those.
  Size: M

### ot-page-serve-setup

`ot page` writes the macro_rates view model, `ot serve` serves it in the static page, `ot setup pg` calls `Setup`.
  Goal: Calls of `pages.BuildMacroRatesFunc`, `pages.NewHandlerFunc` and `store.SetupFunc`. Pages stay free of a clock and a request: the caller passes `generatedAt` and the day. `ot setup pg` is the command the owner runs once; this design does not run it and no test touches a real `$OT_HOME`.
  Files: cmd/ot/page.go, cmd/ot/serve.go, cmd/ot/demo.go, cmd/ot/setup.go, cmd/ot/page_test.go, cmd/ot/serve_test.go, cmd/ot/setup_test.go.
  Design: docs/design/week1-wiring.md#ot-page-serve-setup; the three commands share the `deps` of `ot-run-once` and add three files of the same size class, so they are one pull request of seven acceptance items.
  Acceptance:
  - `page.go` registers `page`; it builds the JSON with `generatedAt = clock.Now().UTC()` and the day of `--day` (default as in run-once) and writes it to `--out` or standard output; with a real profile it opens the series store with `store.DSNEnvPages` from `.env` and reads through the `store.SeriesReader` interface only.
  - `serve.go` registers `serve`; it listens on `--addr` (default `127.0.0.1:8080`, loopback), builds the view model anew for each request and answers through `deps.newHandler`; a build error is answered with 500 and logged without a DSN.
  - `demo.go` is a `store.SeriesReader` over three literal points of DGS10 typed in the file, one with a nil value, used when the profile is `demo`; no file outside `cmd/ot` is read at run time.
  - `setup.go` registers `setup`; the argument must be `pg`, else usage and an error; it builds `store.SetupConfig{AdminDSN: <process env store.AdminDSNEnv>, Home: <OT_HOME>}` and calls the `deps.setup` field; an empty administrative DSN or an unset `OT_HOME` is an error that names the variable; no flag takes the DSN; no value of the config appears in a log line or an error text.
  - `page_test.go`, `serve_test.go`: with the fake builder the bytes of `page` are the bytes the builder returned; `serve` is exercised with `httptest.NewServer` (never a fixed port) and a fake handler constructor; a failing builder gives 500; the `demo` profile reaches the demo reader and opens no store.
  - `setup_test.go`: a fake `store.SetupFunc` records its `SetupConfig`, with `OT_HOME` set to a `t.TempDir()`; the config has the two values; an unset variable stops before the fake is called; a fake that returns an error carrying a sentinel DSN makes the command return it with the sentinel absent from the text.
  - Every test calls `t.Parallel()`; none sleeps; each file stays under 600 lines.
  - Checked by CI: ci-go.
  Depends on: ot-run-once, store-dsn-names. Order: after ot-run-once.
  Size: M

## Left out

- TypeScript type generation from the schemas (`json-schema-to-typescript`, a `gen:types` script, the TS step of
  `contracts/generate.sh`, the drift check of `.github/workflows/checks.yml`, `web/src/generated/*`): a later design
  gated by the owner. No consumer exists yet (the migrated pages keep their local types in `web/src/types/*`) and neither
  package is in `frontend-whitelist.json` (revision 2026-11-05.1); this design asks for no whitelist change. When the
  owner opens it, the old generator `web/scripts/gen-validator.mjs` also wrote `*.validate.js` wrappers, which a new script drops.
- The window rule: `ot run-once` uses `runner.NewFunc`, that is `reconcile.New` with the zero rule (calendar `XNYS`,
  no offsets). Passing a rule from a job (`WindowRule`, a `[window]` table in `job.toml`, `reconcile.NewWithRule` behind a
  Func type) waits for a populated calendar and for jobs-fred-series to land; its trigger is the first run on a real database.
- Population of `master.session_day`: the table is empty on a real database in week 1 (the calendar generator is left out
  by week1.md); loading `contracts/fixtures/master.session_day.r1.json` through `store.SessionDayWriter` is one more `ot`
  subcommand and would take the design over the size of this wave. Until then `ot run-once` on a real database returns
  `UnknownCalendar`; its tests use fakes only.
- `docs/contracts/jobs.md`: not touched here; it does not exist on origin/main either.
- Fixtures for the 17 schemas that have no fixture directory (and fixtures regenerated for the carried schemas): the old ones
  are written by a Python registry that is not in this repository.
- The moves of the migrated pages from `web/src/types/*` to generated types, and the week-2 cleanup pass over the schemas.
- `Page` and `Builder` names of pages: `ot page` and `ot serve` call the one `BuildMacroRatesFunc`.
- Embedding the real web build into `internal/pages/embed/dist` (a build step that writes into the pages module and the `vm`
  element contract of the built page): a later issue of pages with the owner.
- A named adapter type from `store.SessionDayReader` to `clock.SessionSource`: not chosen, the store satisfies the interface.
- Calling `reconcile.New`, `reconcile.NewWithRule` or any implementation by name outside `wire.go`: not chosen, the `ot` row
  uses the functions of the wiring table.
- A dotenv library for `.env`: not chosen, it would touch secrets and load them into the process environment.
- The broker, compute and notify wiring: later weeks.

## Adversarial

Source: scratchpad file design-week1-wiring.adversarial.md of the dispatcher window.

- 1a `cmd/ot` cannot take the design (main.go, `app`, no getenv/deps): adopted; main.go and main_test.go are in `ot-run-once` Files and `app` gains `getenv` and `deps`.
- 1b run-once fails on an empty calendar: adopted as wording; the goal says wired, not run on real data, and Left out names the population of `master.session_day`.
- 1c fixture validation covers four files per directory only: adopted; the test loops over every `*.json` and only `bad-version.json` must fail.
- 2a wiring-interface crosses four modules on an unlanded window shape: adopted; the window rule, `runner-window-rule` and `jobs-window-rule` are gone.
- 2a (imports sentence false, `contracts/api.go` imports context): adopted; the acceptance is deleted with the entry.
- 2b store-wiring-support edits files of unmerged issues and cites missing docs: partly adopted; it waits for both issues, and `store.md` is created if absent; the unlanded names stay a risk named in its Design line.
- 2c hygiene names: adopted; the note on `OT_*` constants, the DSN built in the pg test from `PG*` parts, and no literal DSN string.
- 2d interface marks (ts-generate, schemas-28): adopted for schemas-28 (Design note); ts-generate is Left out; the "only interface issue" sentence is limited to `store-dsn-names`; schemas-28 lists explicit file names.
- 2e acceptance not judgeable from the diff: adopted; no source hash or count is judged by running, the manifest content and file list are diff facts, the hash proof is under Checked by CI, the sentinel DSN is tested by injection.
- 2f go-toml promotion and pnpm-lock: adopted by removal; no `[window]` reader, no pnpm-lock edit.
- 3 simpler way (Go test, cut window machinery, defer TS, merge setup): adopted in full; setup is folded into `ot-page-serve-setup` (seven items).
- 4 lock-in (DSN names, manifest churn, generator pin, `[window]` keys): DSN names accepted and frozen in `store-dsn-names`; manifest deletion by week 2 stated; the other two vanish with the cut.
- 5 claims likely false (only interface issue, init registration without main.go, imports sentence, generator options, headings of missing docs): all corrected above.
- 6 ordering of unlanded dependencies: adopted in part; `ot-run-once` depends on the unlanded issues only through `wire.go`'s imports, and no issue waits on the owner.
