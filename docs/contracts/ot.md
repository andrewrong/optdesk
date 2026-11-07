# ot contract

## Responsibility

`ot` is the optdesk binary, the composition root (`cmd/ot`). It selects the profile and the log handler through `internal/profile`, and dispatches to eight subcommands: `backfill`, `calendar`, `migrate`, `page`, `run-once`, `serve`, `setup` and `version`. A subcommand registers itself in an `init()`; a name registered twice panics. Without a command, or with a name that is not registered, `ot` prints `usage: ot <command> [args]` and the sorted list of commands on standard error.

The subcommands:

- `ot run-once [--day DAY]`: opens the stores and the broker, wires the three jobs `fred_series`, `option_eod` and `quote_snapshot` and reconciles the day (`Reconcile`) with them, in one call; `ot` passes the zero `runner.WindowRule{}` and each job states its window through `runner.Windowed`. Real profile only. A day at or before the day of the ledger cursor is an error (exit 1) that names both days and logs no `status=ok`: `Reconcile` walks only days after the cursor, so there is nothing to reconcile; the error adds the hint `use ot backfill --day <day>`.
- `ot backfill --day DAY [--dataset ID]`: opens the stores and the broker, wires the same three jobs and calls `Backfill` for DAY: it re-runs the keys of that day that are missing or marked `no_value`, and moves neither the cursor, a plan nor a window. `--day` is required. Real profile only (the demo profile is refused with `backfill needs a real profile; the demo profile opens no store`). A day ahead of the cursor is refused with `day <day> is ahead of the cursor <cursor>: use run-once` (the cursor is `(none)` when empty); a day with no stored plan is refused. The error text of the runner is withheld because it may hold a DSN or a key.
- `ot page [--day DAY] [--out FILE]`: builds the macro rates page once and writes its bytes.
- `ot serve [--day DAY] [--addr ADDR]`: serves the macro rates page over HTTP until interrupted.
- `ot setup pg`: sets up the PostgreSQL roles through `store.SetupFunc`, under the administrative DSN.
- `ot version`: prints the version and the profile.
- `ot migrate [--dir DIR]`: applies the migration files of a directory under the administrative DSN, as the owner role.
- `ot calendar load FILE [FILE ...]`: reads session-day calendar files and upserts their rows through the session-day store, with the DSN of `ot_loader` from `OT_PG_DSN_LOADER`.

## Inputs

Flags (each command parses its own; an unknown flag or a bad flag value is a flag parse error):

| Flag | Commands | Default |
|---|---|---|
| `--day` | backfill (required, no default), page, run-once, serve | today in the market zone `America/New_York`, formatted `YYYY-MM-DD`; a value that is not `YYYY-MM-DD` is an error |
| `--dataset` | backfill | empty: every key of the day; else one dataset id |
| `--out` | page | empty: the page goes to standard output; else it is written to that file |
| `--addr` | serve | `127.0.0.1:8080` (the loopback host on port :8080) |
| `--dir` | migrate | `migrations` |

`ot migrate` takes no argument besides `--dir`. `ot setup` takes exactly one argument, `pg`. `ot calendar load FILE [FILE ...]` needs `load` and at least one file; any other form prints the usage line `usage: ot calendar load FILE [FILE ...]` as the error.

Environment names (names only; no value is written in this document, in a log or in an error):

| Name | Source | Read by |
|---|---|---|
| `OT_HOME` | process environment | the profile (all commands); `setup pg` (its home); the `.env` path of run-once, page, serve and calendar load |
| `OT_PROFILE` | process environment | the profile (all commands) |
| `OT_ZONE` | process environment | the profile (real profile only; unset or invalid stops the start) |
| `OT_PG_ADMIN_DSN` | process environment | `ot setup pg`, `ot migrate` |
| `OT_PG_DSN_RUNNER` | `$OT_HOME/.env` | `ot run-once`, `ot backfill` (session-day store and ledger) |
| `OT_PG_DSN_SERIES_FRED` | `$OT_HOME/.env` | `ot run-once`, `ot backfill` (series store) |
| `OT_PG_DSN_OPTION_EOD` | `$OT_HOME/.env` | `ot run-once`, `ot backfill` (option_eod writer) |
| `OT_PG_DSN_QUOTE_SNAPSHOT` | `$OT_HOME/.env` | `ot run-once`, `ot backfill` (quote_snapshot writer) |
| `OT_PG_DSN_MASTER` | `$OT_HOME/.env` | `ot run-once`, `ot backfill` (instrument store) |
| `OT_PG_DSN_LOADER` | `$OT_HOME/.env` | `ot calendar load` (session-day store) |
| `OT_PG_DSN_PAGES` | `$OT_HOME/.env` | `ot page` and `ot serve` (series reader), real profile only |
| `FRED_API_KEY` | `$OT_HOME/.env` | `ot run-once`, `ot backfill` (FRED source) |

All six lines read by `ot run-once` and `ot backfill` (the three above, `OT_PG_DSN_RUNNER`, `OT_PG_DSN_SERIES_FRED` and `FRED_API_KEY`) are required; a missing one is an error that names the line.

The broker socket is `$OT_HOME/run/ibkr.sock`, created by the pyside service; `ot` only connects to it and creates, writes and chmods nothing there. A path of 104 bytes or more is refused before any store is opened with the error `socket path too long` (the text holds no path). The dial is made once, before `Reconcile`; when it fails, `ot` logs the failure once (step name only) and the two option jobs get a stand-in broker whose every call answers a retryable error `broker is not reachable`, so the option keys end retryable while `fred_series` still runs. The runs inside a day are several calls of `ot run-once` a day by the owner's scheduler; the cursor holds while a slot key is open.

The `OT_PG_` names are the constants `store.AdminDSNEnv`, `store.DSNEnvRunner`, `store.DSNEnvSeriesFred` and `store.DSNEnvPages` and `store.DSNEnvLoader` of `internal/store/api.go`. A name that is unset or empty is an error that names the variable and no value (`OT_PG_ADMIN_DSN is not set`, `OT_PG_DSN_PAGES is missing in .env`).

`.env` rules (`setup.ReadEnv`, called through `deps.readEnv`):

- The file is `$OT_HOME/.env`; with `OT_HOME` unset the error is `OT_HOME is not set`.
- Lines are `KEY=VALUE`; a line without `=` is skipped. Blank lines and lines starting with `#` are skipped. Key and value are trimmed of white space.
- One pair of surrounding quotes (both `"` or both `'`) is removed from the value; nothing else is unquoted.
- The process environment is untouched: the file is read into a map and no variable is set.
- An error names the path and never a value (`env file PATH cannot be read`).

Profile rules (`internal/profile.Load`, which reads `OT_HOME` and `OT_PROFILE`, and `OT_ZONE` for the real profile only):

- The profile is demo when `OT_HOME` is unset, and also when `OT_PROFILE=demo`. With `OT_HOME` set and `OT_PROFILE` unset or `real`, it is real.
- `Load` reads `OT_ZONE` for the real profile: it names the owner's zone, and unset or invalid is a bad profile. The demo profile has the zone `UTC` and does not read `OT_ZONE`.
- `OT_PROFILE=real` needs `OT_HOME`; without it the profile is bad.
- An `OT_PROFILE` that is neither demo nor real is a bad profile.
- In the demo profile `page` and `serve` use the built-in demo points of DGS10 and open no store; `run-once`, `backfill`, `migrate` and `calendar load` refuse it (an error that says the demo profile opens no store). `version` and `setup pg` do not look at the profile name; `setup pg` still needs `OT_HOME`.

Calendar file format (`ot calendar load`): one JSON object per file with two keys.

- `provenance`: an object with five keys, all non-empty strings: `generator`, `generator_version`, `source_library`, `source_library_version` and `generated_at`. `generated_at` is a time in RFC 3339. The loader invents none of them; a missing `provenance` is an error.
- `rows`: a non-empty list of session days, each with `calendar_id`, `session_day`, `is_session`, `open_at`, `close_at` and `early_close`.

Row rules, checked for every file before anything is written:

- `calendar_id` is set.
- `session_day` is strictly ascending within one `calendar_id`.
- On a session day (`is_session` true), `open_at` is before `close_at`.
- On a closed day (`is_session` false), `open_at` equals `close_at` and `early_close` is false.

## Outputs

- `ot version` writes `ot dev (profile demo)` to standard output: the format is `ot <version> (profile <profile>)` and a newline; the version is `dev` unless set at build time with `-ldflags "-X main.version=..."`; the profile is `demo` or `real`.
- `ot page` writes the page bytes the builder returned to standard output, or to the file of `--out` (mode 0644).
- `ot serve` listens on `--addr` and logs `serve listening`. It builds the page per request: the view model is built anew for each request, with the day of `--day` or else today in the market zone at that request. When the build fails it answers 500 with the fixed text `internal error`; no error text reaches the client. A failed listen is an error that names the address. On interrupt it shuts down with a 5 second grace.
- `ot run-once`, `ot backfill`, `ot setup pg`, `ot migrate` and `ot calendar load` write nothing to standard output; they log one line with `status=ok` on success (`ot run-once` only when `Reconcile` ran and returned nil). `ot backfill` logs `backfill <day> done` with `status=ok` and the counts `already_present`, `filled`, `absent` and `missing`; it exits 1 when the day was refused or any selected key is `Missing` (the message names the count); `Absent` is not an error, exit 0.
- Error texts of `ot migrate` and `ot setup pg` withhold the underlying error, because it may hold `OT_PG_ADMIN_DSN`: the text says the error text is withheld.
- The error texts of the open and load steps of the calendar loader keep that fixed text and say it is withheld because it may hold `OT_PG_DSN_LOADER`.
- An error at the command boundary never wraps a pgx error. This holds for every command that takes a DSN (`backfill`, `calendar`, `migrate`, `setup`, `page`, `run-once`, `serve`): the error is a new text of fixed wording that names a stage and a line, never the text of the connection error. The calendar file errors name the path, and the row errors the row index and the rule, never a value.

Exit codes, each tied to its condition:

- Exit code 2: the profile is bad (an error line starting `ot:` on standard error), or the command is unknown or missing (the usage text on standard error).
- Exit code 1: a command fails; the log line is `command failed` with the error text, and this includes a flag parse error.
- Exit code 0: success.

## Invariants

- The profile is chosen once, before the command, and a bad profile stops the start before any command runs: even `version` exits 2.
- Standard error carries logs and usage; standard output carries the output of the command only.
- No value of a DSN, key or password appears in an error, a log line or this document.
- `ot calendar load` reads and checks every file before it opens a store; a bad file writes nothing. It loads the files one after the other in the order given. The load is idempotent: loading a file twice gives the same rows.
- `ot calendar load` opens the store with the DSN of `ot_loader` from `OT_PG_DSN_LOADER` and closes it on every path. It reads no administrative DSN and has no fallback: a missing or empty line is an error that names the line and says `ot setup pg` creates it. Every check of the files comes before `.env` is read.
- `ot run-once` and `ot backfill` close every store they opened on every path.
- `ot page` and `ot serve` open no store in the demo profile.
- `--name` of `ot page` and `ot serve` accepts `macro_rates` (default) and `macro_page`; `macro_page` is built in the zone of the profile (`pageNamed`, `cmd/ot/pagelist.go`), an unknown name is an error that lists both, and the run id `ot-page` reaches no ledger row.
- `cmd/ot` calls constructors through the `deps` struct, which a test fills with fakes; no test connects to a live broker or database.

## Version history

- v4: `ot run-once` and `ot backfill` build `fred_series`, `option_eod` and `quote_snapshot`, need the lines `OT_PG_DSN_OPTION_EOD`, `OT_PG_DSN_QUOTE_SNAPSHOT` and `OT_PG_DSN_MASTER`, dial `$OT_HOME/run/ibkr.sock` and pass `runner.WindowRule{}`.

- v1: the subcommands `run-once`, `page`, `serve`, `setup`, `version`, `migrate` and `calendar load`.
- v3: `ot backfill` and the hint in the cursor error of `run-once`.
- v2: `ot calendar load` takes the DSN of `ot_loader` from the line `OT_PG_DSN_LOADER` of `$OT_HOME/.env` and needs no administrative DSN, which only `ot setup pg` and `ot migrate` read; `.env` is read by one function, `setup.ReadEnv`, through `deps.readEnv`; the rule that an error at the command boundary never wraps a pgx error is written once under Outputs. `calendar_test.go` holds 19 tests after the change. A real `ot calendar load` needs `store-loader-role` merged and `ot setup pg` then `ot migrate` run once more on an existing database; the tests of this version need neither.

## Contract tests

- `cmd/ot/main_test.go`: `TestDemoProfile` (the profile is demo without `OT_HOME` and only `OT_HOME` and `OT_PROFILE` are read), `TestUnknownCommandIsUsageError` (exit 2 and the list of commands), `TestVersionCommand`, `TestBadProfileStopsStart`.
- `cmd/ot/serve_test.go`: `TestServeOnBuildError500`, `TestServeOnAnswersBuilderBytes`, `TestServeMacroPage`.
- `cmd/ot/pagelist_test.go`: `TestBuildIDOf`, `TestPageNamedUnknown`, `TestPageNamedNilZone`, `TestMacroPageRealBuild`.
- `cmd/ot/migrate_test.go`: `TestMigrateCallsUp`, `TestMigrateDefaultDir`, `TestMigrateDemoRefused`, `TestMigrateUnsetDSN`, `TestMigrateMalformedDSN`, `TestMigrateWrongPasswordDSN`.
- `cmd/ot/calendar_test.go`: `TestCalendarLoadIdempotent`, `TestCalendarUsage`, `TestCalendarLoadDemoRefused`, `TestCalendarLoadProvenanceRequired`, `TestCalendarLoadProvenanceFields`, `TestCalendarLoadRowOrder`, `TestCalendarLoadRowHours`, `TestCalendarLoadClosedDayShape`, `TestCalendarLoadEmptyFile`, `TestCalendarLoadNoLoaderLine`, `TestCalendarLoadNeverFallsBack`, `TestCalendarLoadChecksBeforeEnv`.
- `cmd/ot/env_test.go`: `TestEnvFileHomeUnset`, `TestEnvFilePassesPath`, `TestEnvFileReturnsReaderError`, `TestEnvFileNilReaderFallsBack`, `TestNoCommandLeaksDSN`, `TestLeakTableCoversCommands`.
- `cmd/ot/page_test.go`: `TestPageBytesAreBuilderBytes`, `TestPageDemoOpensNoStore`, `TestPageMacroPage`, `TestPageMacroRatesIgnoresZone`, `TestPageMacroPageNilZoneOpensNoStore`, `TestMacroPageDemoGolden`.
- `cmd/ot/runonce_test.go`: `TestRunOnceDayReachesReconcile`, `TestRunOnceDemoRefused`, `TestRunOnceMissingKey`, `TestRunOnceClosesStores`; `cmd/ot/runonce_option_test.go`: `TestRunOnceWiresOptionJobs`, `TestRunOnceRule`, `TestRunOnceCloses`, `TestRunOnceErrors`, `TestRunOnceErrorsRefused`, `TestRunOnceDialFails`, `TestRunOnceSocketPathLength`.
- `cmd/ot/backfill_test.go`: `TestBackfillFlags`, `TestBackfillCallsRunner`, `TestBackfillErrors`, `TestBackfillNoSecretInErrors`.
- `cmd/ot/setup_test.go`: `TestSetupPassesConfig`, `TestSetupArgumentMustBePg`.
- `cmd/ot/day_test.go`: `TestDayDefaultIsNewYork`, `TestDayFlagWins`, `TestDayMalformed`.

There is no test yet (checked by `grep` in the test files under `cmd/ot`) for: the default `--addr` value `127.0.0.1:8080`; the exit 1 of a flag parse error; the demo profile chosen by `OT_PROFILE=demo` with `OT_HOME` set; and `ot page` and `ot serve` in the real profile reading `OT_PG_DSN_PAGES` when it is missing from `.env`.

## Forbidden

- Writing a value of a DSN, API key or password to a log line, an error, standard output or a file.
- Setting a process environment variable from `.env`.
- Opening a store in the demo profile for `page` or `serve`, or running `run-once`, `migrate` or `calendar load` in it.
- Returning the text of a build error to an HTTP client.
- Inventing provenance for a calendar file, or writing a row of a file that failed a check.
- A test that connects to a live broker or database, or sleeps.
