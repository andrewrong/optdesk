# ot contract

## Responsibility

`ot` is the optdesk binary, the composition root (`cmd/ot`). It selects the profile and the log handler through `internal/profile`, and dispatches to seven subcommands: `calendar`, `migrate`, `page`, `run-once`, `serve`, `setup` and `version`. A subcommand registers itself in an `init()`; a name registered twice panics. Without a command, or with a name that is not registered, `ot` prints `usage: ot <command> [args]` and the sorted list of commands on standard error.

The subcommands:

- `ot run-once [--day DAY]`: opens the stores, wires the `fred_series` job and reconciles the day (`Reconcile`). Real profile only.
- `ot page [--day DAY] [--out FILE]`: builds the macro rates page once and writes its bytes.
- `ot serve [--day DAY] [--addr ADDR]`: serves the macro rates page over HTTP until interrupted.
- `ot setup pg`: sets up the PostgreSQL roles through `store.SetupFunc`, under the administrative DSN.
- `ot version`: prints the version and the profile.
- `ot migrate [--dir DIR]`: applies the migration files of a directory under the administrative DSN, as the owner role.
- `ot calendar load FILE [FILE ...]`: reads session-day calendar files and upserts their rows through the session-day store, as the owner role.

## Inputs

Flags (each command parses its own; an unknown flag or a bad flag value is a flag parse error):

| Flag | Commands | Default |
|---|---|---|
| `--day` | page, run-once, serve | today in the market zone `America/New_York`, formatted `YYYY-MM-DD`; a value that is not `YYYY-MM-DD` is an error |
| `--out` | page | empty: the page goes to standard output; else it is written to that file |
| `--addr` | serve | `127.0.0.1:8080` (the loopback host on port :8080) |
| `--dir` | migrate | `migrations` |

`ot migrate` takes no argument besides `--dir`. `ot setup` takes exactly one argument, `pg`. `ot calendar load FILE [FILE ...]` needs `load` and at least one file; any other form prints the usage line `usage: ot calendar load FILE [FILE ...]` as the error.

Environment names (names only; no value is written in this document, in a log or in an error):

| Name | Source | Read by |
|---|---|---|
| `OT_HOME` | process environment | the profile (all commands); `setup pg` (its home); the `.env` path of run-once, page, serve |
| `OT_PROFILE` | process environment | the profile (all commands) |
| `OT_PG_ADMIN_DSN` | process environment | `ot setup pg`, `ot migrate`, `ot calendar load` |
| `OT_PG_DSN_RUNNER` | `$OT_HOME/.env` | `ot run-once` (session-day store and ledger) |
| `OT_PG_DSN_SERIES_FRED` | `$OT_HOME/.env` | `ot run-once` (series store) |
| `OT_PG_DSN_PAGES` | `$OT_HOME/.env` | `ot page` and `ot serve` (series reader), real profile only |
| `FRED_API_KEY` | `$OT_HOME/.env` | `ot run-once` (FRED source) |

The `OT_PG_` names are the constants `store.AdminDSNEnv`, `store.DSNEnvRunner`, `store.DSNEnvSeriesFred` and `store.DSNEnvPages` of `internal/store/api.go`. A name that is unset or empty is an error that names the variable and no value (`OT_PG_ADMIN_DSN is not set`, `OT_PG_DSN_PAGES is missing in .env`).

`.env` rules (`readEnvFile`):

- The file is `$OT_HOME/.env`; with `OT_HOME` unset the error is `OT_HOME is not set`.
- Lines are `KEY=VALUE`; a line without `=` is skipped. Blank lines and lines starting with `#` are skipped. Key and value are trimmed of white space.
- One pair of surrounding quotes (both `"` or both `'`) is removed from the value; nothing else is unquoted.
- The process environment is untouched: the file is read into a map and no variable is set.
- An error names the path and never a value (`env file PATH cannot be read`).

Profile rules (`internal/profile.Load`, which reads only `OT_HOME` and `OT_PROFILE`):

- The profile is demo when `OT_HOME` is unset, and also when `OT_PROFILE=demo`. With `OT_HOME` set and `OT_PROFILE` unset or `real`, it is real.
- `OT_PROFILE=real` needs `OT_HOME`; without it the profile is bad.
- An `OT_PROFILE` that is neither demo nor real is a bad profile.
- In the demo profile `page` and `serve` use the built-in demo points of DGS10 and open no store; `run-once`, `migrate` and `calendar load` refuse it (an error that says the demo profile opens no store). `version` and `setup pg` do not look at the profile name; `setup pg` still needs `OT_HOME`.

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
- `ot run-once`, `ot setup pg`, `ot migrate` and `ot calendar load` write nothing to standard output; they log one line with `status=ok` on success.
- Error texts of `ot migrate`, `ot calendar load` and `ot setup pg` withhold the underlying error, because it may hold `OT_PG_ADMIN_DSN`: the text says the error text is withheld. The calendar file errors name the path, and the row errors the row index and the rule, never a value.

Exit codes, each tied to its condition:

- Exit code 2: the profile is bad (an error line starting `ot:` on standard error), or the command is unknown or missing (the usage text on standard error).
- Exit code 1: a command fails; the log line is `command failed` with the error text, and this includes a flag parse error.
- Exit code 0: success.

## Invariants

- The profile is chosen once, before the command, and a bad profile stops the start before any command runs: even `version` exits 2.
- Standard error carries logs and usage; standard output carries the output of the command only.
- No value of a DSN, key or password appears in an error, a log line or this document.
- `ot calendar load` reads and checks every file before it opens a store; a bad file writes nothing. It loads the files one after the other in the order given. The load is idempotent: loading a file twice gives the same rows.
- `ot calendar load` opens the store with the owner DSN derived from `OT_PG_ADMIN_DSN` and closes it on every path.
- `ot run-once` closes every store it opened on every path.
- `ot page` and `ot serve` open no store in the demo profile.
- `cmd/ot` calls constructors through the `deps` struct, which a test fills with fakes; no test connects to a live broker or database.

## Version history

- v1: the subcommands `run-once`, `page`, `serve`, `setup`, `version`, `migrate` and `calendar load`.

## Contract tests

- `cmd/ot/main_test.go`: `TestDemoProfile` (the profile is demo without `OT_HOME` and only `OT_HOME` and `OT_PROFILE` are read), `TestUnknownCommandIsUsageError` (exit 2 and the list of commands), `TestVersionCommand`, `TestBadProfileStopsStart`.
- `cmd/ot/serve_test.go`: `TestServeOnBuildError500`, `TestServeOnAnswersBuilderBytes`.
- `cmd/ot/migrate_test.go`: `TestMigrateCallsUp`, `TestMigrateDefaultDir`, `TestMigrateDemoRefused`, `TestMigrateUnsetDSN`, `TestMigrateMalformedDSN`, `TestMigrateWrongPasswordDSN`.
- `cmd/ot/calendar_test.go`: `TestCalendarLoadIdempotent`, `TestCalendarUsage`, `TestCalendarLoadDemoRefused`, `TestCalendarLoadProvenanceRequired`, `TestCalendarLoadProvenanceFields`, `TestCalendarLoadRowOrder`, `TestCalendarLoadRowHours`, `TestCalendarLoadClosedDayShape`, `TestCalendarLoadEmptyFile`.
- `cmd/ot/env_test.go`: `TestReadEnvFileQuotes`, `TestReadEnvFileComments`, `TestReadEnvFileMissing`.
- `cmd/ot/page_test.go`: `TestPageBytesAreBuilderBytes`, `TestPageDemoOpensNoStore`.
- `cmd/ot/runonce_test.go`: `TestRunOnceDayReachesReconcile`, `TestRunOnceDemoRefused`, `TestRunOnceMissingKey`, `TestRunOnceClosesStores`.
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
