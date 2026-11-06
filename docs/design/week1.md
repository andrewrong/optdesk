# Week 1: the vertical slice

Author: advisor of optdesk (first draft)
Source: advisor week-1 list of 2026-11-05 with its two addenda, cut by the module map. Items marked [guess] are
the advisor's assumptions, kept as written. Owner choice 2026-11-05: option A, vertical slice first. The slice:
calendar contract and clock, one FRED series through sources, store and the ops ledger, one page builder with its
schema, the migrated front end. The wiring of these pieces in `cmd/ot`, the other page schemas and the page fixtures are cut
by a follow-up design (see Left out).

## Modules

This design adds no module of its own: the rows it relies on are the rows added by `docs/design/modules.md` in
the same pull request. Which row each issue below edits:

| issue | module |
|---|---|
| 1 | contracts, clock, store, sources, runner, jobs, pages and repo: the one declarations-only crossing |
| 2 | repo and ot (`cmd/ot/main.go` and `version.go` of the `ot` row: the registry and the version command; go.mod, go.sum) |
| 3 | ci |
| 4 | contracts and repo (go.mod, go.sum only) |
| 5 | contracts |
| 6 | store and repo (go.mod, go.sum only) |
| 7 | clock |
| 8 | store and repo (go.mod, go.sum only) |
| 9 | sources |
| 10 | runner |
| 11 | jobs and repo (go.mod, go.sum only) |
| 12 | pages |
| 14 | web |
| 17 | docs |

Every other entry lies in one module. The entries that change the Go dependencies after wave 0 (4, 6, 8, 11) are judged
two-module entries because `go.mod` and `go.sum` belong to repo and the AGENTS.md rule is that the files that
change are named; each says so, and each is the issue that first imports the dependency (issue 1 adds the
test-kit clock, the generator tools and the runtime schema validator, so 5, 7, 9, 10 and 12 need no change of go.mod; 11 adds the TOML reader of `job.toml`; 6 and 8
both use pgx and goose, and each lists go.mod and go.sum and requires them, so 8 lands after 6 and finds them
there, and its go.mod change is empty or a trivial rebase). Issues that list
go.mod or go.sum (1, 2, 4, 6, 8, 11: the serialised set, listed here once) land one after another in that order, each naming every earlier one in `Order: after`, so their rebases do not conflict. The
advisor's original list is cut into the issues of `## Issues` (the numbers 13, 15 and 16 are left
unused so that the anchors of the others stay stable; their work is in Left out). The interface skeleton is issue 1; the dataset slice
(calendar contract, store and clock were one issue) is cut by module; the owner added the carry-over of the binding decisions (issue 17).
Store and pages provide library functions; the subcommands of `cmd/ot` that call them are cut by the follow-up design.

## Interface

Landed in #7

Types, functions and protocols the modules agree on. Each has exactly one owning module and lives in that
module's interface file (declarations only: types, constants, aliases, interfaces, signatures). Each interface file may import the standard library and the `api.go` of the modules in the depends cell of its row of `docs/design/modules.md`, and nothing else of the repository; the one exception is `internal/contracts/api.go`, which also imports its own generated package `internal/contracts/gen`. That package is generated only (every file carries the generated-code header, and the drift check of `contracts/generate.sh` rewrites it; the generator's own package-level `enumValues_<Type>` variables are allowed, nothing hand-written is) and imports nothing of the repository; it is exempt from the declarations-only rule for that reason. The Go output of protoc-gen-go (issue 4) has package-level variables and an `init` function, so it goes to its own package `internal/contracts/genpb` (owned by contracts, generated only, imports nothing of the repository, imported by no `api.go` in week 1), never into `gen`. The actual imports, per file: `internal/contracts/api.go` imports `internal/contracts/gen` (the aliases `type SessionDay = gen.SessionDay`); `internal/clock/api.go`, `internal/store/api.go`: `internal/contracts`; `internal/sources/api.go`: `internal/contracts` and `internal/clock`; `internal/runner/api.go`: `internal/contracts`, `internal/clock`, `internal/store`; `internal/jobs/api.go`: `internal/contracts`, `internal/clock`, `internal/sources`, `internal/store`; `internal/pages/api.go`: `internal/contracts`, `internal/store` (and the standard library `time`); `internal/profile/api.go`: none.
Issue 1 lands all of them; no module issue of this design edits an interface file; `web/src/generated/index.ts` (generated from contracts) is a stub of issue 1 that the follow-up design regenerates.
Only names that a week-1 issue calls are frozen: `OpsLedger` is called by the runner (issue 10) and implemented
by store (issue 8); `Reconciler` is implemented by the runner (10) and called by the wiring (follow-up design); `Registry` is
filled by the follow-up design (wire-run-once) and read by the wiring; the others are named in the entries that use them. A window
rule is internal to the runner and is not frozen. The constructors that `cmd/ot` calls are frozen too, as the wiring table below.

The owner of each type of the wiring table is the module of its prefix, and every type or struct a signature of the table or of the name table mentions is in the name table with its owner. Wiring table. `cmd/ot` is the composition root, wired by the follow-up design; this design freezes the types it will use. An interface file is declarations only, so it cannot hold a
function body; each constructor is therefore declared there as a function type, and its owner implements one exported
function of exactly that type in the package named in the last column. `cmd/ot` calls a module through its interface
file and, for construction, through the one function of this table, and uses no other name of those packages. The
owner's issue holds the check `var _ <module>.<Type> = <pkg>.<Func>` in a test (compile-time, so a drift fails
`go build`), and the follow-up design assigns each function to its type in `cmd/ot/wire.go`. `cmd/ot` is the go-arch-lint component `ot` of `docs/design/modules.md`, whose allowed imports are exactly the eleven implementation packages of this table, `internal/profile` (row repo, whose two functions below are implemented in its root package, which no module imports) and the interface files (the composition root's list, checked).

| type (interface file) | signature | implemented in | function |
|---|---|---|---|
| `clock.NewClockFunc` | `func() Clock` | internal/clock/realclock | `NewReal` |
| `clock.NewCalendarFunc` | `func(SessionSource) Calendar` | internal/clock/calendar | `NewCalendar` |
| `store.OpenSessionDayFunc` | `func(ctx, dsn string) (SessionDayStore, error)`; `SessionDayStore` embeds the Reader and the Writer | internal/store/sessionday | `Open` |
| `store.OpenSeriesFunc` | `func(ctx, dsn string) (SeriesStore, error)`; `SeriesStore` embeds the Reader and the Writer | internal/store/series | `Open` |
| `store.OpenLedgerFunc` | `func(ctx, dsn string) (OpsLedger, error)` | internal/store/ops | `Open` |
| `store.SetupFunc` | `func(ctx, SetupConfig) error`; `SetupConfig` is declared in the same file | internal/store/setup | `Setup` |
| `sources.NewFREDFunc` | `func(FREDConfig) (Source, error)` | internal/sources/fred | `New` |
| `jobs.NewFredSeriesFunc` | `func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error)` | internal/jobs/fredseries | `New` |
| `runner.NewFunc` | `func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) Reconciler` | internal/runner/reconcile | `New` |
| `pages.BuildMacroRatesFunc` | `func(ctx, store.SeriesReader, contracts.Day, generatedAt time.Time) ([]byte, error)`: `generated_at` of the page is `generatedAt` in UTC, format RFC 3339, and the builder never reads a clock (the caller passes the time, a test passes a fixed one) | internal/pages/macro_rates | `Build` |
| `pages.NewHandlerFunc` | `func(viewModel []byte) http.Handler` | internal/pages/embed | `Handler` |
| `profile.LoadFunc` | `func(getenv func(string) string) (Profile, error)`: selects `demo` when `OT_HOME` is unset, reads only `OT_HOME` and `OT_PROFILE` | internal/profile | `Load` |
| `profile.NewLogHandlerFunc` | `func(Profile, slog.Handler) slog.Handler`: passes only the whitelisted log fields | internal/profile | `NewLogHandler` |

| name | owner | interface file | meaning |
|---|---|---|---|
| `SessionDay` record | contracts | internal/contracts/api.go | an alias `type SessionDay = gen.SessionDay` of the struct generated from dataset master.session_day r1 (calendar_id, session_day, is_session, open_at, close_at, early_close); no hand-written copy |
| `SeriesPoint` record | contracts | internal/contracts/api.go | an alias of the struct generated from obs.series_point r1 (series_id, observation_date, realtime_start, realtime_end, value as pointer, status, unit enum, currency, published_at, source, source_revision); no hand-written copy |
| `Day` | contracts | internal/contracts/api.go | a calendar date, `type Day string` in the form YYYY-MM-DD (not a `time.Time`, so no zone is implied); the type of every `sessionDay` parameter and of `Plan(ctx, day)` |
| `JobResult` | contracts | internal/contracts/api.go | what `Job.Run` returns: the number of records returned and the number usable, and a `TypedError` or nil |
| `ExpectedKey` | contracts | internal/contracts/api.go | dataset_id and subject_key, opaque to runner |
| `Job` interface | contracts | internal/contracts/api.go | `ID`, `Plan(ctx, day)` returning the `ExpectedKey` set and a plan hash, `Run(ctx, day, keys)` returning a `JobResult`; `day` is a `Day` |
| `ErrorKind` and `TypedError` | contracts | internal/contracts/api.go | kinds Retryable, Permanent, UnknownCalendar, NoValue; `TypedError` is `error` plus `Kind()` |
| `Clock`, `Session`, `Calendar` | clock | internal/clock/api.go | `Clock.Now()` (a subset of clockwork.Clock, so the fake satisfies it); `Calendar.Session` and `Calendar.NextSession` by calendar id and day; unknown calendar returns a `TypedError` of kind UnknownCalendar |
| `SessionSource` | clock | internal/clock/api.go | what clock needs from storage: session days of a calendar between two days; its one implementer is the adapter in `cmd/ot` (follow-up design) from `store.SessionDayReader`, so clock never imports store and store never imports clock |
| `SessionDayWriter`, `SessionDayReader` | store | internal/store/api.go | upsert and read `SessionDay` |
| `SeriesWriter`, `SeriesReader` | store | internal/store/api.go | insert (idempotent) and read `SeriesPoint` by series and date range |
| `SessionDayStore`, `SeriesStore` | store | internal/store/api.go | interfaces that embed the Reader and the Writer of their dataset (the return types of `OpenSessionDayFunc` and `OpenSeriesFunc`) |
| `SetupConfig` | store | internal/store/api.go | the struct of the arguments of `SetupFunc`: the administrative DSN, the path of `$OT_HOME` |
| `PresentStatus`, `Cursor`, `Lease`, `Window` | store | internal/store/api.go | the value types of the `OpsLedger` methods: a present key's status (`present`, `no_value`), the reconcile cursor (a `Day`), the run lease (owner, expiry), the dataset window (open and close instants) |
| `OpsLedger` | store | internal/store/api.go | expected keys per session day and plan hash, present keys with status, dataset window, reconcile cursor, run lease; the only way runner touches ops tables |
| `FREDConfig` | sources | internal/sources/api.go | the struct of the arguments of `NewFREDFunc`: base URL, API key (a string passed in by the caller), the `clock.Clock` |
| `Source`, `FetchPlan` | sources | internal/sources/api.go | given a plan of series ids and a date range, return `SeriesPoint` records with provenance; typed errors only |
| `Reconciler` | runner | internal/runner/api.go | `Reconcile(ctx, day)` with `day` a `contracts.Day` for the registered jobs |
| `Registry` | jobs | internal/jobs/api.go | the jobs the binary runs, as `[]contracts.Job` |
| `Profile` | repo | internal/profile/api.go | the profile name (`demo` or `real`) and the whitelist of log fields; `LoadFunc` and `NewLogHandlerFunc` of the wiring table are declared in the same file |
| JSON Schema files and proto | contracts | contracts/schemas, contracts/datasets, contracts/proto | the single source of Go structs, TS types and validators; generated files are never edited by hand |

Contract documents. Each `docs/contracts/<module>.md` of this design has exactly these second-level headings: `## Responsibility`, `## Inputs`, `## Outputs`, `## Invariants`, `## Version history`, `## Contract tests`, `## Jobs and slots`, `## Forbidden`; `grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' <file>` prints 8.

Migration numbering, fixed here so that no store issue waits on another: `migrations/0001_master_session_day.sql` (issue 1), `0002_series.sql` and `0003_ops_ledger.sql` (issue 8); the next free number belongs to the issue that adds a table.

Not in week 1: interface files of broker, compute, notify and pyside (no issue of this week touches them; their
rows in modules.md keep entry cells that name files created in the weeks stated there).

### Libraries

New direct dependencies of week 1, with licences (AGENTS.md: a new direct dependency is named with its licence).
None of them touches secrets, the network or the sandbox except pgx (database driver, used only in `internal/store`) and gitleaks (CI only: it reads the pull request commits and prints nothing secret, `--redact`).
Licences are from memory of the upstream LICENSE files and are unverified: the module cache of this machine holds
none of these modules, so the reviewer checks each against the LICENSE in the module cache or on pkg.go.dev.

| dependency | use | licence | issue |
|---|---|---|---|
| github.com/jonboulle/clockwork | fake clock of the test kit | Apache-2.0, unverified | 1 |
| github.com/atombender/go-jsonschema | Go structs from JSON Schema (`tool` directive); the published module path of the project once named omissis/go-jsonschema | MIT, unverified | 1 |
| github.com/bufbuild/buf and google.golang.org/protobuf (protoc-gen-go) | proto lint and Go generation (`tool` directives, runtime) | Apache-2.0, unverified | 4 |
| github.com/santhosh-tekuri/jsonschema/v6 | runtime validation in Go | Apache-2.0, unverified | 1 |
| github.com/pelletier/go-toml/v2 | reads `job.toml` | MIT, unverified | 11 |
| github.com/jackc/pgx/v5 | PostgreSQL driver, only in `internal/store` | MIT, unverified | 6 or 8, whichever lands first |
| github.com/pressly/goose/v3 | migrations | MIT, unverified | 6 or 8, whichever lands first |
| go-arch-lint and golangci-lint (`tool` directives) | architecture and lint checks | MIT and GPL-3.0 as a stand-alone tool, unverified | 2 |
| gitleaks (a release binary or gitleaks-action in `hygiene.yml`, not a go.mod entry) | secret scan of the pull request commits | MIT, unverified | 3 |

golangci-lint is used as a tool and linked into no binary of ours; the reviewer confirms that the licence of each
tool allows that use.

## Issues

Waves. Wave 0 is three issues landed in this order: issue 1 (the interface files, the generated records, the
datasets, the macro_rates page schema with its input and golden fixtures, the migration numbering in
`## Interface`), issue 2 (the repo skeleton: scripts, profile, subcommand registry, lint and arch files) and issue 3 (the
workflows ci-go, ci-web, hygiene and checks, owner-merged). Wave 1 is issues 4 to 12, 14 and 17: each needs only landed wave-0 work (the interface files, the shared inputs
named above, the workflows) and tests against fakes; none needs the output of a sibling of wave 1. Issues of wave 1 that change go.mod or
go.sum (the serialised set of `## Modules`) are ordered 4, 6, 8, 11 by their `Order: after` lines because of rebases, not because one needs the other's
output. There is no wave 2: what would need the merged output of wave 1 is cut by a follow-up design (see Left out). owner_only (the owner merges): 3 (`.github/`, wave 0, so every wave-1 issue starts after it) and 4
(`contracts/proto/`, the whole pull request). Several steps need the owner (owner-first items, Probe 0 of the
advisor list, the private-repository copies of 14 and 17); the chain 1, 2, 3 and 14 is the web slot's long pole.

Items marked [CI] run `go build`, `go vet`, `go test`, `go list`, `go run`, `node`, `pnpm` or a pg-tagged test. A sandboxed
verify has no network, no module cache, no pnpm store and no live service, so those items are decided by the
GitHub CI of the head commit (ci-go, ci-web, the pg job; pg-tagged tests run only there, AGENTS.md). Those
workflows exist from the landing of issue 3, which every wave-1 issue depends on. Issues 1, 2 and 3 land before them: for
those three the offline substitute alone decides a [CI] item. Every [CI]
item carries an offline substitute that verify can run: a grep or a file read named in the item (for a test result, `grep -n 'func TestName'` of the named function in the named test file) or, where none is
named, `grep -c 't.Parallel()'` on the named test files (at least 1) and `gofmt -l` on the Go files named in
Files (prints nothing). Items not marked are file reads and greps. Owner steps and pull-request-description
requests are notes below the acceptance list, not acceptance items.

### interface-skeleton

1. (interface) The interface files of the slice, with the generated records they alias. This is the judged cut crossing modules: contracts, clock, store, sources, runner, jobs, pages, repo. It adds declarations and generated code only, so no module issue waits on another module's behaviour.
  Goal: Land every name of `## Interface`, the two dataset schemas and the Go structs generated from them, the first page schema with its input and golden fixtures (so the pages issue needs no sibling), (so `SessionDay` and `SeriesPoint` are aliases, never hand-written), the stub of the TS entry file (`web/src/generated/index.ts`), and `go.mod` with the test-kit clock, the generator tool and the runtime schema validator santhosh-tekuri/jsonschema/v6 (used by the test of the page schema below), so the later issues compile against it. The migration `0001_master_session_day.sql` is part of this issue: it is the DDL of the dataset schema, so issues 6 and 8 need no order between them.
  Files: go.mod, go.sum, migrations/0001_master_session_day.sql, internal/contracts/page_schema_test.go, contracts/generate.sh, contracts/schemas/page.macro_rates.v1.json, contracts/fixtures/page.macro_rates.v1.input.json, contracts/fixtures/page.macro_rates.v1.golden.json, contracts/fixtures/MANIFEST.synthetic-v1.json, contracts/datasets/master.session_day.r1.json, contracts/datasets/obs.series_point.r1.json, contracts/gendoc/main.go, contracts/gendoc/main_test.go, docs/contracts/datasets/master.session_day.md, docs/contracts/datasets/obs.series_point.md, internal/contracts/gen/records.go, web/src/generated/index.ts, internal/contracts/api.go, internal/contracts/api_test.go, internal/clock/api.go, internal/clock/api_test.go, internal/store/api.go, internal/store/api_test.go, internal/sources/api.go, internal/sources/api_test.go, internal/runner/api.go, internal/runner/api_test.go, internal/jobs/api.go, internal/jobs/api_test.go, internal/pages/api.go, internal/pages/api_test.go, internal/profile/api.go, internal/profile/api_test.go.
  Design: docs/design/week1.md#interface-skeleton; own implementation (contracts/gendoc), because no maintained library renders our dataset page template from a dataset schema
  Acceptance:
  - [CI for build, vet and test; the greps are offline] Dependencies: `grep -cE 'jonboulle/clockwork v|atombender/go-jsonschema v|santhosh-tekuri/jsonschema/v6 v' go.mod` prints 3 (all pinned to a version, go-jsonschema also in a `tool` directive) and `grep -E '^\s*github.com\S* v[0-9]' go.mod | grep -vc '// indirect'` prints 3 (the three modules above, which are the direct requirements issue 1 adds; only lines with a version are counted, so `tool` lines without one are not, and the transitive requirements are `// indirect` lines and are not counted). Each requirement is used: `internal/contracts/page_schema_test.go` imports santhosh-tekuri/jsonschema/v6 (`grep -c 'tekuri/jsonschema/v6' internal/contracts/page_schema_test.go` is at least 1) and `internal/clock/api_test.go`, the interface test of clock, imports clockwork for its fake (`grep -c 'jonboulle/clockwork' internal/clock/api_test.go` is at least 1), so `go mod tidy` keeps both. The checks that need modules from the network (`go build ./... && go vet ./...` and `go test`) are not decided by a sandboxed verify (see the note on [CI] items at the top of this section; issue 1 lands before `ci-go`, so the offline substitute alone decides); the offline substitute is `gofmt -l internal contracts` printing nothing.
  - `master.session_day r1` has columns calendar_id text, session_day date, is_session boolean, open_at timestamptz, close_at timestamptz, early_close boolean, primary key (calendar_id, session_day); session_day is the market-zone date, not a UTC date. `obs.series_point r1` has the columns series_id text, observation_date date, realtime_start date, realtime_end date, value numeric (nullable), status text, unit (a `series_unit` enum of percent, ratio, ccy_amount and count [guess]), currency text (not null when unit is ccy_amount), published_at timestamptz, source text, source_revision text; primary key (series_id, observation_date, realtime_start). Each dataset file has `additionalProperties: false`.
  - [CI] `sh contracts/generate.sh` (go tools only, no `npx`, no download) rewrites `internal/contracts/gen/records.go` and the two dataset pages and leaves `git diff --exit-code` clean; `gen/records.go` carries a generated header (offline: `grep -c 'DO NOT EDIT' internal/contracts/gen/records.go` prints 1); `grep -n 'type SessionDay = \|type SeriesPoint = ' internal/contracts/api.go` prints two lines and no struct named so is declared by hand.
  - `internal/contracts/gen` imports nothing of the repository: `grep -rn 'optdesk/internal' internal/contracts/gen` prints nothing; and holds only generated code: `grep -L 'DO NOT EDIT' internal/contracts/gen/*.go` prints nothing (every file carries the header), and `grep -rnE '^var ' internal/contracts/gen | grep -v 'var enumValues_'` prints nothing (the only variables are the generator's enum-value lists).
  - `web/src/generated/index.ts` is a stub with a generated header (`export {};`); the follow-up design regenerates it.
  - Schema `page.macro_rates.v1` has fields `schema_version`, `generated_at`, `session_day`, `series[]` (id, unit, points[] of date plus value-or-null), `gaps[]` with a reason code; `additionalProperties: false` at every object; no field outside this list. The `input` fixture holds 3 observations of the series DGS10 (one value null); the `golden` file `contracts/fixtures/page.macro_rates.v1.golden.json` exists and its `generated_at` is `2000-01-01T00:00:00Z` (`grep -c '"generated_at": *"2000-01-01T00:00:00Z"' contracts/fixtures/page.macro_rates.v1.golden.json` prints 1; that it is typed by hand from the input is a note of the pull request, not acceptance); `MANIFEST.synthetic-v1.json` lists both, and no fixture contains a marker field.
  - `migrations/0001_master_session_day.sql` creates the table of `master.session_day r1` exactly (`grep -c 'PRIMARY KEY (calendar_id, session_day)' migrations/0001_master_session_day.sql` prints 1) with an Up and a Down section for goose (`grep -c '+goose' migrations/0001_master_session_day.sql` is at least 2); numbering is fixed in `## Interface` (0001 here, 0002 and 0003 in issue 8), so issues 6 and 8 need no order between them.
  - [CI] `go test ./internal/contracts -run PageSchema` validates the golden file against the page schema with jsonschema/v6 and shows that a copy with an unknown extra field fails (offline: `grep -n 'func TestPageSchema' internal/contracts/page_schema_test.go` finds the function and `grep -c 't.Parallel()'` of the file is at least 1).
  - `contracts/generate.sh` runs `contracts/generate-proto.sh` when that file exists (`grep -n 'generate-proto.sh' contracts/generate.sh` finds the line), so issue 4 needs no edit of the script.
  - Each `api.go` holds only package, import, type, alias, const and interface declarations: `grep -nE '^func |^var ' internal/*/api.go` prints nothing.
  - Each `api_test.go` calls `t.Parallel()` (`grep -L 't.Parallel()' internal/*/api_test.go` prints nothing) and tests the contract from both sides: a fake in the test implements the interface, a caller in the test uses it through the interface only [CI for the run].
  - Every name of the `## Interface` table and every type of the wiring table exists in its owner's file: `grep -n` of the name in its owner's `api.go` finds it.
  - Imports of the `api.go` files are exactly those named per file in the rule at the top of `## Interface`: `grep -hE '^\s+"[^"]*/internal/' internal/clock/api.go internal/store/api.go` shows only `internal/contracts`; the same grep on `sources`, `runner`, `jobs`, `pages` shows only the lists given there; `internal/profile/api.go` has no `internal/` import and `internal/contracts/api.go` only `internal/contracts/gen`. [CI for `go list`; the grep is the offline decision.]
  Depends on: none. Order: first.
  Size: L

### repo-skeleton

2. Repo skeleton, derived arch.toml, ratchet check, hygiene, demo profile. Second interface-level issue: the scripts, the `profile` package and the subcommand registry that wave 1 builds on.
  Goal: The mechanical rules the later issues are checked against. A binary `ot` starts under `OT_PROFILE=demo` when `$OT_HOME` is absent and loads no real value. `docs/design/modules.md` is the single source of dependencies; `arch.toml` and `.go-arch-lint.yml` are generated from its depends column.
  Files: go.mod, go.sum, cmd/ot/main.go, cmd/ot/version.go, cmd/ot/main_test.go, internal/profile/profile.go, internal/profile/logging.go, internal/profile/profile_test.go, internal/profile/logging_test.go, scripts/arch/main.go, scripts/arch/main_test.go, arch.toml, .go-arch-lint.yml, .golangci.yml, scripts/ratchet/main.go, scripts/ratchet/main_test.go, scripts/hygiene/main.go, scripts/hygiene/main_test.go, .gitignore, README.md.
  Design: docs/design/week1.md#repo-skeleton; own implementation (scripts/arch, scripts/ratchet, scripts/hygiene), because no maintained library derives go-arch-lint configuration from the module table, reads docs/design/ratchet.json, or checks this repository's account-id, path and fixture-manifest rules (go-arch-lint and golangci-lint are used for the rest)
  Acceptance:
  - [CI] `go build ./... && go vet ./...` pass; `go run ./cmd/ot version` prints a version; a unit test shows `ot` with no `$OT_HOME` selects profile `demo` and reads no environment variable except `OT_HOME` and `OT_PROFILE` (offline: `grep -n 'func TestDemoProfile' cmd/ot/main_test.go` finds the function and `grep -c 't.Parallel()' cmd/ot/main_test.go` is at least 1).
  - [CI] `go run ./scripts/arch --check` exits 0 on the committed `arch.toml` and `.go-arch-lint.yml` and exits 1 when a test copy of `docs/design/modules.md` gains a depends edge the files lack; the test uses a temporary directory. The component `ot` is generated with exactly the eleven implementation packages listed in the text of `docs/design/modules.md` (the test compares the generated list with the list it reads from that text; no package named there has to exist), and a test builds in its `t.TempDir()` a scratch module tree (`cmd/ot`, `internal/store/migrate`, `internal/store/sessionday` and scratch interface packages) and shows that a `cmd/ot` file importing `internal/store/migrate` fails the generated go-arch-lint configuration and one importing `internal/store/sessionday` passes. The same tree shows that a scratch sub-package of `jobs` importing a sub-package of `store` fails, and one importing the root package of `store` passes (each sub-package is its own component, as the derivation of modules.md states). A test shows that the component `profile` is generated with an empty import list (the rule at `internal/profile` in modules.md) and that a scratch package of the same temporary tree importing a forbidden edge makes the generated go-arch-lint configuration fail. go-arch-lint and golangci-lint are `tool` directives of go.mod, pinned. Offline: `grep -n 'func TestArchCheck' scripts/arch/main_test.go` finds the functions and `grep -c 't.Parallel()' scripts/arch/main_test.go` is at least 1.
  - [CI for the test run] `scripts/ratchet` reads `docs/design/ratchet.json` (owner-only; a missing file is an empty map): it counts source files only (extensions .go .py .ts .tsx .js .mjs .sh) and skips lockfiles, JSON, TOML and Markdown data and any file whose first five lines carry a generated-code header, so generated types, schemas, fixtures and lockfiles are not counted; it applies the size rule of `docs/design/modules.md`: it fails on a source file new in the change over 600 lines (copies count as new), on a file of the base revision over 800 lines that `ratchet.json` does not list, on a listed file over its ceiling, and on a ceiling higher than in the base revision; each case has a test with a temporary repo (offline: `grep -n 'func TestRatchet' scripts/ratchet/main_test.go` finds one function per case, `TestRatchetNewFileOver600`, `TestRatchetUnlistedOver800`, `TestRatchetOverCeiling`, `TestRatchetCeilingRaised`, and `grep -c 't.Parallel()' scripts/ratchet/main_test.go` is at least 1). This replaces size.toml.
  - `grep -n 'func register(' cmd/ot/main.go` finds one line; `grep -n 'func init()' cmd/ot/version.go` and `grep -n 'register("version"' cmd/ot/version.go` each find one line (`version.go` registers itself in its `init()`).
  - Note (not acceptance): later subcommand files of `cmd/ot` register themselves in an `init()`, so a later issue does not edit `main.go`; the import limit of `cmd/ot` is the go-arch-lint component `ot` of the arch item above.
  - `internal/profile` implements the two wiring-table types owned by repo, and `cmd/ot` selects the profile and builds its slog handler only through them: `grep -n 'var _ profile.LoadFunc = profile.Load' internal/profile/profile_test.go` and `grep -n 'var _ profile.NewLogHandlerFunc = profile.NewLogHandler' internal/profile/logging_test.go` each find one line.
  - `.golangci.yml` enables depguard (only `internal/store` may import pgx), noctx, contextcheck, forbidigo (bans `time.Sleep` and `time.Now` outside `internal/clock`), exhaustive, gocyclo, funlen, lll.
  - [CI for the test run] `scripts/hygiene` scans every file that `git ls-files` lists except the exempt paths `docs/design/**`, `scripts/hygiene/**` (its tests included), `pnpm-lock.yaml`, `go.sum` and `uv.lock`, and rejects an account-id shape `\b[DU]U?\d{6,8}\b`, `/Users/`, `/Volumes/`, a non-noreply email, and a file in a `fixtures/` directory that the `MANIFEST.synthetic-v1.json` of that directory does not list (the manifest has the form `{"marker":"synthetic-v1","files":[...]}` and is itself exempt; a fixture holds no marker field, so `additionalProperties: false` and golden files are not disturbed); the hygiene substitute (offline, the one stated once and used by issues 2, 14 and 17): `git ls-files -- <paths> | grep -vE '^(docs/design/|scripts/hygiene/)|^(go\.sum|pnpm-lock\.yaml|uv\.lock)$|(^|/)MANIFEST\.synthetic-v1\.json$' | xargs grep -nE '\b[DU]U?[0-9]{6,8}\b|/Users/|/Volumes/'` prints nothing, with `<paths>` the paths of the pull request (`.` for the whole tree); it checks the account-id shape and the two paths only, not the email or manifest rules; each pattern, the manifest rule and the exempt-path rule has one test whose offending input is built at run time in the test, not written in a file (offline: `grep -n 'func TestHygiene' scripts/hygiene/main_test.go` finds `TestHygieneAccountID`, `TestHygieneUsersPath`, `TestHygieneVolumesPath`, `TestHygieneEmail`, `TestHygieneManifestRule`, `TestHygieneExemptSkipped` (a file under `docs/design/` holding an id-shaped value, built at run time in a temporary tree, yields no finding), and `grep -c 't.Parallel()' scripts/hygiene/main_test.go` is at least 1).
  - [CI for the test run] The slog handler of `internal/profile` passes only whitelisted fields; a test feeds a raw id-shaped value and asserts it is dropped (offline: `grep -n 'func TestLoggingDropsRawID' internal/profile/logging_test.go` finds the function and `grep -c 't.Parallel()' internal/profile/logging_test.go` is at least 1).
  - `README.md` (exists; edited) names the project and the licence (`grep -ci 'optdesk' README.md` and `grep -ci 'licen[cs]e' README.md` are each at least 1); `go run ./scripts/hygiene` exits 0 on the tree of the pull request [CI] (offline: the hygiene substitute above with `<paths>` = `.`); every Go test calls `t.Parallel()` and uses `t.TempDir()`.
  Note (not acceptance): a disclaimer text is the owner's; none exists in the repository, so this issue adds none.
  Depends on: owner-first items 1-3 and Probe 0 of the advisor list (dispatcher steps, not issues). Order: after 1.
  Size: M

### ci-workflows

3. GitHub workflows: ci-go, checks, ci-web, hygiene, Dependabot (owner_only).
  Goal: All week-1 CI in one pull request so the owner merges `.github` once. `checks.yml` runs on every pull request with no paths filter, because the crew gate counts only workflows that ran on the head commit and reads zero checks as FAIL. Its job stays named `test` (the required check).
  Files: .github/workflows/checks.yml, .github/workflows/ci-go.yml, .github/workflows/ci-web.yml, .github/workflows/hygiene.yml, .github/dependabot.yml, .github/PULL_REQUEST_TEMPLATE.md.
  Design: docs/design/week1.md#ci-workflows
  Acceptance:
  - `.github/workflows/checks.yml` has exactly one job whose id is `test`: `grep -c '^  test:' .github/workflows/checks.yml` prints 1; `grep -nE '^\s*paths(-ignore)?:' .github/workflows/checks.yml` prints nothing; the three existing steps of checks.yml are kept.
  - `checks.yml` also runs `go run ./scripts/ratchet`, `go run ./scripts/hygiene`, `go run ./scripts/arch --check` (arch.toml agrees with the depends column of modules.md), the generated-code drift check (`sh contracts/generate.sh` then `git diff --exit-code`) and a check that the pull request body carries `Contracts: unchanged | <id> rN->rN+1` matching the `contracts/` diff; each step is a `run:` line that appears in the file.
  - `ci-go.yml`: paths filter plus `go list -deps` for affected packages; golangci-lint, go-arch-lint, `go test ./...`; a job with `services: timescaledb` runs `-tags pg` tests. Runner GitHub-hosted Linux; action versions pinned to full SHAs [guess]: `grep -hE 'uses:' .github/workflows/*.yml | grep -vcE '@[0-9a-f]{40}'` prints 0.
  - `ci-web.yml`: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; triggers only on `web/` and root pnpm file changes; an empty `web/` must not fail the other workflows (the file carries a guard step, checked by reading it).
  - `hygiene.yml` also runs gitleaks over the commits of the pull request, pinned and with `--redact` so that no secret is printed: the pinned version string appears (`grep -nE 'gitleaks' .github/workflows/hygiene.yml` finds the step and `grep -cE 'v8\.[0-9]+\.[0-9]+' .github/workflows/hygiene.yml` is at least 1 for a release binary, which is verified by its checksum, or the action is pinned by a 40-hex commit SHA); `grep -c -- '--redact' .github/workflows/hygiene.yml` is at least 1.
  - `hygiene.yml` asserts no `OT_*` or DSN variable in the environment; the repository has zero repository secrets (the file reads none: `grep -rn 'secrets\.' .github` prints nothing).
  - Dependabot covers gomod, npm, github-actions. Secret scanning and push protection are owner settings, not files.
  Depends on: 2 (an interface-level issue, landed). Order: after 2, the last of wave 0.
  Note: every item above is decided by reading the workflow files; the workflows' own results are the CI of the head commit.
  Size: M

### contracts-tree

4. Contracts tree: the proto file and its generation (owner_only: the whole pull request is merged by the owner).
  Goal: Create `contracts/proto/` with exactly one proto file in week 1 (pyside waits) and its generated Go; generated files land in the same pull request as their source. This entry is the judged cut crossing contracts and repo: it adds Go dependencies and tools, so it lists go.mod and go.sum.
  Files: go.mod, go.sum, contracts/proto/ibkr/v1/session.proto, buf.yaml, buf.gen.yaml, contracts/generate-proto.sh, contracts/README.md, internal/contracts/genpb/session.pb.go, docs/contracts/00-protocol.md.
  Design: docs/design/week1.md#contracts-tree
  Acceptance:
  - Exactly one `.proto` file exists under `contracts/proto/` (`find contracts/proto -name '*.proto' | wc -l` prints 1); it defines one rpc `Ping` and no account, symbol or order message and no order-placing rpc (`grep -inE 'order|account|symbol' contracts/proto/ibkr/v1/session.proto` prints nothing).
  - [CI; offline: `grep -nE '^tool|^\s+github.com/bufbuild/buf|protobuf/cmd/protoc-gen-go' go.mod` shows both tools, `grep -c 'DO NOT EDIT' internal/contracts/genpb/session.pb.go` prints 1, `grep -n 'go tool buf lint' contracts/generate-proto.sh` finds the line] `buf` and `protoc-gen-go` are `tool` directives of go.mod, pinned; `sh contracts/generate-proto.sh` (called by `contracts/generate.sh` of issue 1) runs `go tool buf generate`, rewrites `internal/contracts/genpb/session.pb.go` (generated header present; `grep -n 'internal/contracts/genpb' buf.gen.yaml` finds the output path and `grep -rn 'optdesk/internal' internal/contracts/genpb` prints nothing) and leaves `git diff --exit-code` clean; `contracts/generate-proto.sh` runs `go tool buf lint` before it generates, and the lint passes; no `buf breaking` step; no `npx`.
  - `docs/contracts/00-protocol.md` holds the one-paragraph version rule: record rN, page schema vN, proto v1, job manifest version; any incompatible or additive change bumps the version in its own issue.
  - This issue adds no workflow file: its Files line names no path under `.github/`. [CI] `go build ./...` passes; offline substitute: `gofmt -l internal` prints nothing.
  Note (not acceptance): the `Contracts:` line of the pull request reads `ibkr.v1 (new)`.
  Depends on: 1, 2, 3. Order: after 1, 2, 3.
  Size: S

### week1-schemas

5. Contracts: the dataset fixtures of the slice and the validation tests.
  Goal: Put the dataset fixtures the slice is checked against in `contracts/` (the page schema and its fixtures are in issue 1), the synthetic symbol list, and one validation test over all fixtures. The validator library comes from the go.mod of issue 1, so this issue stays in contracts.
  Files: contracts/fixtures/master.session_day.r1.json, contracts/fixtures/obs.series_point.r1.json, contracts/fixtures/symbols.txt, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/schema_test.go.
  Design: docs/design/week1.md#week1-schemas
  Acceptance:
  - Fixture of master.session_day for `calendar_id='XNYS'` [guess id], values typed as literals, not computed: 2026-11-26 `is_session=false` (Thanksgiving); 2026-11-27 `early_close=true`, close 13:00 America/New_York; 2026-12-25 `is_session=false`; an ordinary day closes 16:00 ET. DST rows: open_at on 2026-11-02 and 2026-11-06 is 14:30 UTC, on 2026-10-30 it is 13:30 UTC.
  - The typed calendar values of the fixture above are test expectations only, never a source of market hours; no non-test Go file reads `contracts/fixtures/master.session_day.r1.json` (`grep -rln 'master.session_day.r1.json' --include=*.go .` lists only `_test.go` files).
  - The obs.series_point fixture holds 3 DGS10-shaped observations, one with value null and status `no_value`, each value a literal of the file. `contracts/fixtures/symbols.txt` is a synthetic symbol list (`grep -c . contracts/fixtures/symbols.txt` prints at least 1; it holds only the tickers `SYN1` to `SYN9` [guess]). `MANIFEST.synthetic-v1.json` lists every other file of the directory with marker `synthetic-v1` (issue 1 created it with the page files; this issue adds its own); no fixture contains a marker field.
  - [CI] `go test ./internal/contracts/...` validates each dataset fixture against its dataset schema with santhosh-tekuri/jsonschema/v6 (the golden file is validated by issue 1), and checks that a copy with an unknown extra field fails; every test calls `t.Parallel()`; offline substitute: `grep -n 'func TestDatasetFixtures' internal/contracts/schema_test.go` finds the function and `grep -c 't.Parallel()' internal/contracts/schema_test.go` is at least 1.
  - [CI] Every JSON file parses (`python3 -m json.tool` on each prints no error); `sh contracts/generate.sh` leaves `git diff --exit-code` clean.
  Note (not acceptance): the owner approves the symbol list (owner-first item 3); the `Contracts:` line of the pull request reads `master.session_day r1 (new), obs.series_point r1 (new), page.macro_rates v1 (new)` for issue 1 and `unchanged` for this one.
  Depends on: 1, 3. Order: after 1, 3.
  Size: M

### store-session-day

6. Store: session_day migration and Reader/Writer, migration runner.
  Goal: The Writer and Reader of `master.session_day` behind the interfaces of `internal/store/api.go`. Only store opens the database. This entry is the judged cut crossing store and repo: it adds pgx and goose, so it lists go.mod and go.sum.
  Files: go.mod, go.sum, internal/store/migrate/migrate.go, internal/store/migrate/migrate_test.go, internal/store/sessionday/writer.go, internal/store/sessionday/reader.go, internal/store/sessionday/sessionday_test.go, internal/store/sessionday/sessionday_pg_test.go, docs/contracts/store.md.
  Design: docs/design/week1.md#store-session-day
  Acceptance:
  - The migration `migrations/0001_master_session_day.sql` is landed by issue 1; this issue's pg test applies it with a helper of its own, and the SQL strings of the Writer name exactly its columns (`grep -c 'calendar_id' internal/store/sessionday/writer.go` is at least 1).
  - [CI] `go test ./internal/store/...` passes without Docker (offline substitute: `grep -c 't.Parallel()'` of each unit test file is at least 1): the unit tests use an in-test fake and check the SQL strings and the migrate runner against a fake driver; the real round trip (real Writer, real Reader, equal to rows typed in the test) is in `sessionday_pg_test.go`, whose first line is `//go:build pg` (`head -1 internal/store/sessionday/sessionday_pg_test.go` prints it) and which runs only in GitHub CI (AGENTS.md); its result is a CI item, not a verify item.
  - The wiring constructor exists with its declared type: `grep -rn 'var _ store.OpenSessionDayFunc = Open' internal/store/sessionday` prints one line.
  - Offline: `grep -rl 'jackc/pgx' --include=*.go .` lists only files under `internal/store/`; depguard still bans pgx elsewhere.
  - `docs/contracts/store.md` has the eight headings of the contract-document rule in `## Interface` (`grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/store.md` prints 8), and its `## Inputs` names `master.session_day r1` (`grep -c 'master.session_day r1' docs/contracts/store.md` is at least 1).
  Note (not acceptance): if this issue lands before 8, its pull request description names pgx and goose with their licences (`### Libraries` in this design).
  Depends on: 1, 3. Order: after 1, 2, 3, 4.
  Size: M

### clock-reader

7. Clock: injected clock and calendar reader.
  Goal: A read-only `clock` that answers Session, NextSession and ET/JST/owner-zone conversions from a `SessionSource`. The Go side never hand-writes open or close times (ADR 0004). The fake clock (clockwork) comes from the go.mod of issue 1, so this issue stays in one module.
  Files: internal/clock/realclock/realclock.go, internal/clock/calendar/calendar.go, internal/clock/realclock/realclock_test.go, internal/clock/calendar/calendar_test.go, docs/contracts/clock.md. The root package `internal/clock` keeps only `api.go` and `api_test.go`; the two sub-packages import it (the component rule of `docs/design/modules.md`).
  Design: docs/design/week1.md#clock-reader
  Acceptance:
  - [CI] Tests use clockwork's fake clock through the `Clock` interface and an in-test `SessionSource` fake whose rows are literals typed in the test: Thanksgiving 2026-11-26 not a session; 2026-11-27 closes 13:00 America/New_York; an ordinary day closes 16:00 ET. Offline substitute: `grep -n 'func TestSessionTypedRows' internal/clock/calendar/calendar_test.go` finds the function (and `grep -c 't.Parallel()' internal/clock/calendar/calendar_test.go` is at least 1).
  - The rows typed in these tests are expectations, not a source; the package reads market hours only through a `SessionSource`.
  - [CI] DST: open_at for 2026-11-02 and 2026-11-06 is 14:30 UTC and for 2026-10-30 is 13:30 UTC when converted from the typed rows. Offline substitute: `grep -n 'func TestDSTOpenAt' internal/clock/calendar/calendar_test.go` finds the function (and `grep -c 't.Parallel()' internal/clock/calendar/calendar_test.go` is at least 1).
  - [CI] `NextSession(day)` skips weekend and holidays; asking for a calendar absent from the source returns a `TypedError` of kind UnknownCalendar, not a guess; the ET, JST and owner-zone conversions have one table-driven test. Offline substitute: `grep -n 'func TestNextSessionSkipsHolidays' internal/clock/calendar/calendar_test.go`, `grep -n 'func TestUnknownCalendar' internal/clock/calendar/calendar_test.go`, `grep -n 'func TestZoneConversions' internal/clock/calendar/calendar_test.go` finds the functions (and `grep -c 't.Parallel()' internal/clock/calendar/calendar_test.go` is at least 1).
  - Every test calls `t.Parallel()`; no `time.Sleep` and no `time.Now` in the package (`grep -rn 'time.Sleep\|time.Now' internal/clock --include=*.go` prints nothing outside the one real-clock adapter file `internal/clock/realclock/realclock.go`).
  - `docs/contracts/clock.md` has the eight headings of the contract-document rule in `## Interface` (`grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/clock.md` prints 8).
  - The wiring constructors exist with their declared types: `grep -n 'var _ clock.NewClockFunc = NewReal' internal/clock/realclock/realclock_test.go` and `grep -n 'var _ clock.NewCalendarFunc = NewCalendar' internal/clock/calendar/calendar_test.go` print one line each.
  - The root package holds no implementation: `ls internal/clock/*.go` prints only `internal/clock/api.go` and `internal/clock/api_test.go`.
  Depends on: 1, 3. Order: after 1, 3.
  Size: S

### store-series-ops

8. Store: obs.series_point, the ops ledger tables, the database setup function.
  Goal: This entry is the judged cut crossing store and repo: it needs pgx and goose, so it lists go.mod and go.sum and requires them itself (no dependency on issue 6). The minimal ledger the first job needs: series tables and `ops.capture_run`, `ops.capture_expected_key`, `ops.dataset_window`, `ops.reconcile_cursor`, with one-writer-one-reader roles; and a `Setup` function (called by `ot setup pg` once the follow-up wiring lands) that creates the database and the roles on the owner's machine.
  Files: go.mod, go.sum, migrations/0002_series.sql, migrations/0003_ops_ledger.sql, internal/store/series/writer.go, internal/store/series/reader.go, internal/store/series/series_test.go, internal/store/series/series_pg_test.go, internal/store/ops/ledger.go, internal/store/ops/ledger_test.go, internal/store/ops/ledger_pg_test.go, internal/store/setup/setup.go, internal/store/setup/envfile.go, internal/store/setup/setup_test.go.
  Design: docs/design/week1.md#store-series-ops; own implementation (internal/store/setup/envfile.go, about 80 lines), because a dotenv library rewrites the whole file and loses comments and order, and this writer must keep other lines and set mode 0600 [unverified]
  Acceptance:
  - `obs.series_definition` and `obs.series_point` have the primary key `(series_id, observation_date, realtime_start)`, `series_unit` enum, CHECK that a `ccy_amount` series has a currency (grep of the migration for each).
  - `ops.capture_run` has `CHECK (status <> 'running' OR lease_until IS NOT NULL)`; `ops.capture_expected_key` primary key includes `plan_hash` and `subject_key`; `ops.dataset_window` primary key `(dataset_id, session_day)` (grep).
  - [CI] Roles: `ot_w_series_fred` has INSERT on series_point and only REFERENCES and SELECT on `ops.capture_run`; `rd.series_point_r1` (a view created by `0002_series.sql`: `grep -c 'rd.series_point_r1' migrations/0002_series.sql` is at least 1) is read-only; `grep -ohE 'ot_owner|ot_runner|ot_w_series_fred|ot_ro_pages' migrations/0002_series.sql migrations/0003_ops_ledger.sql | sort -u | wc -l` prints 4 (the grants and owners of the four roles of week 1 are in these two files); a `//go:build pg` test connects as each role and asserts the forbidden statement fails: GitHub CI decides it; offline, `head -1` of `internal/store/series/series_pg_test.go` and `internal/store/ops/ledger_pg_test.go` prints the build tag, and `grep -c 'ot_w_series_fred' internal/store/series/series_pg_test.go` is at least 1. The pg tests apply `0002_series.sql` and `0003_ops_ledger.sql` (this issue's own files) by a helper of their own; no table of these two files refers to `master.session_day`, so the tests need neither 0001 nor the migration runner of issue 6.
  - [CI] The Writer rejects a sentinel value (-1, NaN, +Inf) by storing NULL plus a status code (CHECK mirrors it); one table-driven unit test with 3 rows; replaying the same capture is idempotent (second insert adds no row; unit test on the fake, pg test on the real one). Offline substitute: `grep -n 'func TestSentinelRejected' internal/store/series/series_test.go`, `grep -n 'func TestReplayIdempotent' internal/store/series/series_test.go` finds the functions (and `grep -c 't.Parallel()' internal/store/series/series_test.go` is at least 1).
  - [CI] `setup.Setup` creates database `optdesk` on the shared pg service, the Timescale extension, the four roles of week 1 (`ot_owner`, the owner of the migrations and used by goose only; `ot_runner`, the only writer of `ops.*` with INSERT, and UPDATE limited to the columns status, finished_at, lease_until, heartbeat_at, returned_n, usable_n, error_code, reason_code of `ops.capture_run`, and SELECT on the series tables; `ot_w_series_fred`; `ot_ro_pages`, with USAGE on schema `rd` and SELECT on its views only) each with a generated password (`grep -c 'ot_owner\|ot_runner\|ot_w_series_fred\|ot_ro_pages' internal/store/setup/setup.go` is at least 4); later issues add the other roles with their tables, and writes or updates the `OT_PG_DSN_*` lines of `$OT_HOME/.env`. The env-file writer (`envfile.go`) is tested without PG in a `t.TempDir()`: file mode 0600, existing non-PG lines kept, second run changes nothing; no password appears in any log line (test asserts). The SQL side is tested with a throwaway PG in CI (pg tag). Offline substitute: `grep -n 'func TestEnvfile' internal/store/setup/setup_test.go` finds the function (and `grep -c 't.Parallel()' internal/store/setup/setup_test.go` is at least 1).
  - depguard still bans pgx outside `internal/store`.
  - The wiring constructors exist with their declared types: `grep -rn 'var _ store.OpenSeriesFunc\|var _ store.OpenLedgerFunc\|var _ store.SetupFunc' internal/store` prints three lines.
  Owner's word: given 2026-11-05 (writing generated passwords into `$OT_HOME/.env` allowed). After the follow-up wiring lands the owner runs `ot setup pg` once on the owner's machine; it generates passwords and writes them into `$OT_HOME/.env`; this design does not run it and no test touches a real `$OT_HOME`. `$OT_HOME/.env` holds secrets only: the PG DSNs this command writes, FRED_API_KEY (week 1) and IB_FLEX_TOKEN (week 5); gateway host/port/client id, Flex query ids, account aliases, limits and pools are settings (code defaults until the settings page exists), not lines of `.env` (relayed owner ruling, 2026-11-05).
  Note (not acceptance): if this issue lands before 6, its pull request description names pgx and goose with their licences (`### Libraries` in this design).
  Depends on: 1, 3. Order: after 1, 2, 3, 4, 6.
  Size: M

### sources-fred

9. Sources: FRED client for one series (DGS10).
  Goal: A source package with the fixed files (client, mapping, pacing, fixtures) that fetches one FRED series and maps it to typed series records with provenance. It opens no database and does not know the runner.
  Files: internal/sources/fred/client.go, internal/sources/fred/mapping.go, internal/sources/fred/pacing.go, internal/sources/fred/client_test.go, internal/sources/fred/mapping_test.go, internal/sources/fred/pacing_test.go, internal/sources/fred/fixtures/observations.json, internal/sources/fred/fixtures/MANIFEST.synthetic-v1.json, docs/contracts/sources.md.
  Design: docs/design/week1.md#sources-fred; own implementation (pacing.go, about 30 lines), because the limiter must run on the injected Clock interface of this repository and golang.org/x/time/rate takes its own time source [unverified]
  Acceptance:
  - The wiring constructor exists with its declared type: `grep -rn 'var _ sources.NewFREDFunc = New' internal/sources/fred` prints one line.
  - [CI] Pure `net/http`; base URL injectable; tests use `httptest` with the fixture and open no external connection. The API key is passed in by the caller and never appears in a log line or error text (a test asserts redaction). Offline substitute: `grep -n 'func TestRedaction' internal/sources/fred/client_test.go` finds the function (and `grep -c 't.Parallel()' internal/sources/fred/client_test.go` is at least 1).
  - [CI] Mapping: FRED value `"."` becomes nil with status `no_value`; unit `percent` is set in `mapping.go` only, with a comment at the conversion boundary; `realtime_start`, `realtime_end` and `published_at` are kept (vintage). Offline substitute: `grep -n 'func TestMapping' internal/sources/fred/mapping_test.go` finds the function (and `grep -c 't.Parallel()' internal/sources/fred/mapping_test.go` is at least 1).
  - [CI] `internal/sources/fred/fixtures/observations.json` has 3 observations of the form `{"realtime_start","realtime_end","date","value"}`, one with value `"."`, and holds no marker field; `internal/sources/fred/fixtures/MANIFEST.synthetic-v1.json` lists `observations.json` (`grep -c 'observations.json' internal/sources/fred/fixtures/MANIFEST.synthetic-v1.json` prints 1); mapping returns 3 records, 2 with a value, and the expected records are literals in `mapping_test.go`, not computed by the mapping code. Offline substitute: `grep -n 'func TestFixtureMapping' internal/sources/fred/mapping_test.go` finds the function (and `grep -c 't.Parallel()' internal/sources/fred/mapping_test.go` is at least 1).
  - [CI] Pacing: a fake clock proves at most N requests per window (N a package constant [guess: 1 request per second]); no real sleep. Offline substitute: `grep -n 'func TestPacing' internal/sources/fred/pacing_test.go` finds the function (and `grep -c 't.Parallel()' internal/sources/fred/pacing_test.go` is at least 1).
  - [CI] HTTP 429 and 5xx return a `TypedError` of kind Retryable, 400 returns kind Permanent; a cancelled context returns promptly (test with a cancelled context and a handler that blocks until the test ends). Offline substitute: `grep -n 'func TestErrorKinds' internal/sources/fred/client_test.go` finds the function (and `grep -c 't.Parallel()' internal/sources/fred/client_test.go` is at least 1).
  - [CI for `go list`; offline: `grep -rhE '"[^"]*/internal/' internal/sources/fred --include=*.go | grep -vE 'internal/(contracts|clock|sources)"'` prints nothing] `go list -deps ./internal/sources/fred` shows no `internal/` package other than contracts, clock and sources (arch check of issue 2 passes); every test calls `t.Parallel()`.
  - `docs/contracts/sources.md` has the eight headings of the contract-document rule in `## Interface` (`grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/sources.md` prints 8).
  Owner's word: given 2026-11-05 (network and FRED_API_KEY allowed). This issue makes no live call; the owner decides when the first real call is made and puts the key in `$OT_HOME/.env` themselves.
  Depends on: 1 (interface files), 3. Order: after 1, 3.
  Size: S

### runner-window

10. Runner: window materialisation and reconciliation.
  Goal: A minimal runner step that, for each session day after the cursor, materialises the dataset window from the calendar and a window rule and reconciles expected keys against present keys, through the `OpsLedger` interface. It never reads dataset content (keys are opaque).
  Files: internal/runner/reconcile/reconcile.go, internal/runner/reconcile/window.go, internal/runner/reconcile/reconcile_test.go, internal/runner/reconcile/window_test.go, docs/contracts/runner.md. The root package `internal/runner` keeps only `api.go` and `api_test.go`; the sub-package imports it.
  Design: docs/design/week1.md#runner-window
  Acceptance:
  - [CI] The window is built from `clock.Calendar` open and close and a window rule typed in the test (the hookup from the `job.toml` of a job to this rule belongs to the follow-up design, wire-run-once, see Left out); table-driven check on Thanksgiving (no window) and on an early-close day (window closes 13:00 ET), with a fake `Calendar` and a fake `OpsLedger`. Offline substitute: `grep -n 'func TestWindowThanksgivingAndEarlyClose' internal/runner/reconcile/window_test.go` finds the function (and `grep -c 't.Parallel()' internal/runner/reconcile/window_test.go` is at least 1).
  - [CI] Reconciliation with fake clock and fake ledger: a missing key gives one repair run; a run that returns nothing records the expected key as present with status `no_value`, not as missing; a crash (lease expired) is detected and re-run once. Offline substitute: `grep -n 'func TestReconcileRepairNoValueCrash' internal/runner/reconcile/reconcile_test.go` finds the function (and `grep -c 't.Parallel()' internal/runner/reconcile/reconcile_test.go` is at least 1).
  - [CI] Full cycle test plan, late run, recovery under a fake clock, no `time.Sleep`. Offline substitute: `grep -n 'func TestFullCycleLateRun' internal/runner/reconcile/reconcile_test.go` finds the function (and `grep -c 't.Parallel()' internal/runner/reconcile/reconcile_test.go` is at least 1).
  - [CI] `reconcile_cursor` advances only over fully reconciled days; a day with an unresolved gap holds it. Offline substitute: `grep -n 'func TestCursorHoldsOnGap' internal/runner/reconcile/reconcile_test.go` finds the function (and `grep -c 't.Parallel()' internal/runner/reconcile/reconcile_test.go` is at least 1).
  - [CI for `go list`; offline: `grep -rn 'jackc/pgx\|internal/jobs' internal/runner` prints nothing] `go list -deps ./internal/runner` contains no `internal/jobs` and no pgx; every test calls `t.Parallel()`.
  - `docs/contracts/runner.md` has the eight headings of the contract-document rule in `## Interface` (`grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/runner.md` prints 8) and its `## Invariants` names the state machine and the window rule (`grep -ci 'state machine' docs/contracts/runner.md` and `grep -ci 'window rule' docs/contracts/runner.md` are each at least 1).
  - The wiring constructor exists with its declared type: `grep -n 'var _ runner.NewFunc = New' internal/runner/reconcile/reconcile_test.go` prints one line.
  - The root package holds no implementation: `ls internal/runner/*.go` prints only `internal/runner/api.go` and `internal/runner/api_test.go`.
  Depends on: 1, 3. Order: after 1, 3.
  Size: M

### jobs-fred-series

11. Jobs: fred_series job, Plan and run.
  Goal: The first real job: one FRED series through sources and store. `Plan(sessionDay)` returns the expected key set; `Run` fetches through a `Source` and writes through a `SeriesWriter`. It imports no other jobs package. This entry is the judged cut crossing jobs and repo: it reads `job.toml` with github.com/pelletier/go-toml/v2, so it lists go.mod and go.sum.
  Files: go.mod, go.sum, internal/jobs/fredseries/job.toml, internal/jobs/fredseries/plan.go, internal/jobs/fredseries/run.go, internal/jobs/fredseries/plan_test.go, internal/jobs/fredseries/run_test.go, docs/contracts/jobs.md.
  Design: docs/design/week1.md#jobs-fred-series
  Acceptance:
  - [CI] `Plan(2026-11-05)` returns one key `{dataset_id:'obs.series_point', subject_key:'DGS10'}` with a stable `plan_hash`; changing the series list in `job.toml` changes the hash; the same input twice gives the same hash. Offline substitute: `grep -n 'func TestPlanHash' internal/jobs/fredseries/plan_test.go` finds the function (and `grep -c 't.Parallel()' internal/jobs/fredseries/plan_test.go` is at least 1).
  - [CI] `Run` with a fake `Source` and a fake `SeriesWriter` writes the records once; a `no_value` record is written with value nil and its status; a Retryable error is returned unchanged. Offline substitute: `grep -n 'func TestRunWritesOnce' internal/jobs/fredseries/run_test.go` finds the function (and `grep -c 't.Parallel()' internal/jobs/fredseries/run_test.go` is at least 1).
  - [CI] Plan, late run, recovery with a fake clock and fake source: no `time.Sleep`. Offline substitute: `grep -n 'func TestPlanLateRunRecovery' internal/jobs/fredseries/run_test.go` finds the function (and `grep -c 't.Parallel()' internal/jobs/fredseries/run_test.go` is at least 1).
  - [CI for `go list`; offline: `grep -rn 'jackc/pgx\|internal/jobs/' internal/jobs/fredseries --include=*.go` prints nothing] `go list -deps ./internal/jobs/fredseries` shows no other `internal/jobs/` package and no pgx.
  - `job.toml` carries a manifest version and the window rule; `docs/contracts/jobs.md` has the eight headings of the contract-document rule in `## Interface` (`grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/jobs.md` prints 8).
  - The wiring constructor exists with its declared type: `grep -rn 'var _ jobs.NewFredSeriesFunc = New' internal/jobs/fredseries` prints one line.
  - `go.mod` requires go-toml (`grep -c 'pelletier/go-toml/v2 v' go.mod` prints 1) and `internal/jobs/fredseries/plan.go` imports it (`grep -c 'pelletier/go-toml/v2' internal/jobs/fredseries/plan.go` is at least 1).
  Note (not acceptance): the pull request description names go-toml with its licence (`### Libraries` in this design).
  Depends on: 1, 3. Order: after 1, 2, 3, 4, 6, 8.
  Size: M

### pages-macro-rates

12. Pages: macro_rates builder and embed injector.
  Goal: One page builder reads the series Reader and emits the vN JSON, which only formats stored values and computes nothing; the view-model is injected into a static page at serve time. Pages make no request. Its test imports santhosh-tekuri/jsonschema/v6, which the go.mod of issue 1 already holds.
  Files: internal/pages/macro_rates/builder.go, internal/pages/macro_rates/builder_test.go, internal/pages/embed/embed.go, internal/pages/embed/inject.go, internal/pages/embed/inject_test.go, internal/pages/embed/dist/index.html, docs/contracts/pages.md.
  Design: docs/design/week1.md#pages-macro-rates
  Acceptance:
  - [CI] The builder run on `contracts/fixtures/page.macro_rates.v1.input.json` with `generatedAt` 2000-01-01T00:00:00Z (the `generated_at` of the golden file, issue 1) produces JSON equal to `contracts/fixtures/page.macro_rates.v1.golden.json` and validates against `contracts/schemas/page.macro_rates.v1.json` with santhosh-tekuri/jsonschema/v6. Offline substitute: `grep -n 'func TestBuildGolden' internal/pages/macro_rates/builder_test.go` finds the function (and `grep -c 't.Parallel()' internal/pages/macro_rates/builder_test.go` is at least 1).
  - [CI] A gap (nil value) appears in `gaps[]` once and not as 0; a stale series yields a stale gap entry, where stale means that the last point's `observation_date` is more than 4 calendar days before the `Day` given to `Build` (a package constant [guess: a weekend plus one holiday]; date arithmetic on the two `YYYY-MM-DD` values, no calendar, so the builder needs only the reader and the Day). Offline substitute: `grep -n 'func TestGapsAndStale' internal/pages/macro_rates/builder_test.go` finds the function (and `grep -c 't.Parallel()' internal/pages/macro_rates/builder_test.go` is at least 1).
  - [CI for `go list`; offline: `grep -rn 'jackc/pgx\|internal/store/' internal/pages/macro_rates --include=*.go` prints nothing] The builder imports only the `SeriesReader` interface and contracts: `go list -deps ./internal/pages/macro_rates` has no pgx and no store implementation package (arch check passes).
  - The wiring constructors exist with their declared types: `grep -rn 'var _ pages.BuildMacroRatesFunc = Build\|var _ pages.NewHandlerFunc = Handler' internal/pages` prints two lines.
  - [CI] Injector test: Go renders the index with a view-model; the element `<script id="vm" type="application/json">` holds JSON equal to the input; a `</script>` sequence inside a string value is escaped. The element is the one place the page reads the data from: `grep -c 'id="vm"' internal/pages/embed/inject.go` is at least 1 and `grep -c 'id="vm"' internal/pages/embed/dist/index.html` prints 1 (the stub that the test renders). Offline substitute: `grep -n 'func TestInjectEscapes' internal/pages/embed/inject_test.go` finds the function (and `grep -c 't.Parallel()' internal/pages/embed/inject_test.go` is at least 1).
  - [CI] `macro_rates.Build` on in-memory demo data and the same fixed `generatedAt` returns the JSON (a test compares it with the golden file); `embed.Handler` serves the stub `dist/index.html` with the view-model (smoke test with httptest, no browser). The subcommands `ot page` and `ot serve` that call them belong to the follow-up design. Offline substitute: `grep -n 'func TestBuildDemo' internal/pages/macro_rates/builder_test.go` finds the function (and `grep -c 't.Parallel()' internal/pages/macro_rates/builder_test.go` is at least 1).
  - `docs/contracts/pages.md` has the eight headings of the contract-document rule in `## Interface` (`grep -cE '^## (Responsibility|Inputs|Outputs|Invariants|Version history|Contract tests|Jobs and slots|Forbidden)$' docs/contracts/pages.md` prints 8).
  Depends on: 1 (the page schema and its fixtures are landed there), 3. Order: after 1, 3.
  Size: M

### web-migrate

14. Web: migrate the front end from the old repository.
  Goal: Move `web/` over, by copy allow-list. Pages make no request: each reads the view-model from `<script id="vm" type="application/json">`. Stack is ADR 0034 and unchanged. The workspace is `pnpm-workspace.yaml` at the root with one lockfile `pnpm-lock.yaml`; `scripts/check-frontend-deps.mjs` checks `package.json` against `frontend-whitelist.json`. Page components keep the local view-model types of the old repository; they move to the generated types in a later issue.
  Files: web/package.json, web/index.html, web/vite.config.ts, web/vitest.config.ts, web/tsconfig.json, web/eslint.config.js, web/components.json, web/src/main.tsx, web/src/vm.ts, web/src/pages/ (with the split of `HouseholdPage.tsx` below: web/src/pages/household/HouseholdPage.tsx, web/src/pages/household/householdTypes.ts, web/src/pages/household/MoneySections.tsx, web/src/pages/household/AccountsMovers.tsx), web/src/components/, web/src/lib/, package.json, pnpm-workspace.yaml, pnpm-lock.yaml, frontend-whitelist.json, scripts/check-frontend-deps.mjs, web/no-compute-allow.txt, web/deps-allowlist.txt, web/copy-manifest.sha256, docs/adr/0034-frontend-stack.md.
  Design: docs/design/week1.md#web-migrate; own implementation (scripts/check-frontend-deps.mjs), because the check reads the whitelist revision format of the frontend stack and no library knows it
  Libraries (frontend-stack; every version is the pin of rules/frontend-stack.md, whatever the old lockfile had):
  - 按 frontend-stack：用 shadcn 做 UI 组件。
  - 按 frontend-stack：用 radix-ui 做组件原语。
  - 按 frontend-stack：用 lucide-react 做图标。
  - 按 frontend-stack：用 motion 做动画。
  - 按 frontend-stack：用 recharts 做图表。
  - 按 frontend-stack：用 zustand 做客户端状态。
  - 按 frontend-stack：用 react 与 react-dom 做框架。
  - 按 frontend-stack：用 tailwindcss、@tailwindcss/vite 与 tw-animate-css 做样式。
  - 按 frontend-stack：用 clsx、tailwind-merge 与 class-variance-authority 做类名（cn 辅助函数）。
  - 按 frontend-stack：用 vite、@vitejs/plugin-react、typescript、@types/react、@types/react-dom 与 @types/node 做构建。
  - 按 frontend-stack：用 vitest 与 jsdom 做测试。
  - Not a frontend-stack line: charts go through the shadcn chart component, and the echarts charts of the old repository are rewritten to recharts in the migration (owner's choice 2026-11-03). The kinds layout and drag-and-drop have no line because this issue installs neither (`grep -c 'react-resizable-panels\|dnd-kit' web/package.json` prints 0).
  - Tables (@tanstack/react-table) and lint (eslint, typescript-eslint) are entries of whitelist revision 2026-11-05.1, approved by the owner on 2026-11-05.
  Note (not acceptance): the dispatcher queues this issue only once that revision is published by crew's frontend-stack rule.
  Acceptance:
  - Root `package.json` has `"packageManager": "pnpm@10.<minor>.<patch>"` with an exact pnpm 10.x version [guess: the version the old repo pins]; `pnpm-lock.yaml` is committed; no `package-lock.json` or `yarn.lock` exists (`ls` shows neither).
  - [CI for the node run; offline: `grep -c '"revision": "2026-11-05.1"' frontend-whitelist.json` prints 1 and `grep -c 'whitelist' scripts/check-frontend-deps.mjs` is at least 1] `frontend-whitelist.json` has `"revision": "2026-11-05.1"` and, among its `packages` keys, `@tanstack/react-table` 8.21.3, `eslint` 10.13.0 and `typescript-eslint` 8.72.0 (offline: `grep -cE '"(@tanstack/react-table|eslint|typescript-eslint)": \{"version": "(8.21.3|10.13.0|8.72.0)"' frontend-whitelist.json` prints 3) and every name of `web/package.json` dependencies is one of its keys; `node scripts/check-frontend-deps.mjs` exits 0 on the committed `package.json` files and exits 1 on a test copy that adds a dependency not in the whitelist (run on a temporary copy; no network, no install).
  - [CI] `pnpm install --frozen-lockfile`, then `pnpm -C web exec tsc --noEmit`, `pnpm -C web exec eslint .`, `pnpm -C web exec vitest run` and `pnpm -C web build` exit 0 on the head (ci-web). Offline substitute: `pnpm-lock.yaml` exists, `grep -c '^lockfileVersion' pnpm-lock.yaml` prints 1, and the `scripts` of `web/package.json` hold the four commands (`grep -cE '"(typecheck|lint|test|build)"' web/package.json` prints at least 4).
  - No `fetch`, `XMLHttpRequest` or `EventSource` in `web/src` (`grep -rnE 'fetch\(|XMLHttpRequest|EventSource' web/src` prints nothing); each page reads `document.getElementById('vm')`.
  - [CI for the node run; offline: `grep -c 'deps-allowlist' scripts/check-frontend-deps.mjs` is at least 1] `web/deps-allowlist.txt` lists dependency names, one per line (which old dependencies it carries over is a note of the pull request description, not acceptance); every name under `dependencies` and `devDependencies` of `web/package.json` and of the root `package.json` is in it (`node scripts/check-frontend-deps.mjs` also reads this file: it exits 0 on the committed files and 1 on a test copy that adds a name the list lacks; temporary copy, no network).
  - Echarts is gone and the allow-list matches the whitelist: `grep -c echarts web/package.json web/deps-allowlist.txt` prints 0 for each file; every name in `web/deps-allowlist.txt` is a key of `packages` in `frontend-whitelist.json` (offline: `while read n; do grep -qF "\"$n\": {" frontend-whitelist.json || echo "$n"; done < web/deps-allowlist.txt` prints nothing).
  - Old HTML pages are not copied (React only): `find web -name '*.html' -not -path '*/node_modules/*'` prints only `web/index.html`.
  - `web/copy-manifest.sha256` holds one line `<sha256>  docs/adr/0034-frontend-stack.md`, and `sha256sum -c web/copy-manifest.sha256` exits 0 (the committed file matches the recorded hash).
  Note (not acceptance): the recorded hash is that of the ADR in the old clone at b00b4ed, so the ADR is a byte copy; the pull request description states this provenance.
  - [CI] `go run ./scripts/hygiene` and `go run ./scripts/ratchet` (issue 2) exit 0 on the copied set (offline: the hygiene substitute of issue 2, with `<paths>` the copied paths). The size rule of `docs/design/modules.md` applies, and the copied files are new files, so no copied source file ends over 600 lines. Measured at b00b4ed of the old clone, over the source files (`.ts`, `.tsx`, `.js`, `.mjs`, `.mts`, `.css`; the JSON fixtures and the lockfile are data) of `web/**` except `web/src/generated/**` and `node_modules`: exactly one is over 600 lines, `web/src/pages/household/HouseholdPage.tsx` with 617. Its split, in the copy: the type aliases (`Gap`, `Cell`, `MoneyBlock`, `Shown`, `Accounts`, `Scope`, `Tag`, `Movers`, `MoverLine`) go to `householdTypes.ts`; `figure`, `Block`, `MoneyTable`, `IncomeMonths`, `MoneySection` and `AllocationSection` with their helpers go to `MoneySections.tsx`; `ScopeHeading`, `WhoseBadge`, `AccountsSection`, `MoverTable` and `MoversSection` go to `AccountsMovers.tsx`; `HouseholdPage.tsx` keeps `ConcentrationSection`, `HouseholdPage` and the two re-exports, so its importers change nothing. Each of the four ends under 600 lines (`wc -l` on the four files, with `total` aside, prints no number over 600). Offline: `find web -type f \( -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.mjs' \) -not -path '*/node_modules/*' -not -path 'web/src/generated/*' | xargs wc -l` shows no file over 600 lines (the `total` line aside), and `ls web/src/pages/household` lists the three new files beside `HouseholdPage.tsx`.
  - `web/no-compute-allow.txt` lists, one per line, every hit of `grep -rnE 'Math\.(round|floor|ceil|sum)|\.reduce\(' web/src` (the allow-list is empty when the grep prints nothing): `diff <(grep -rnE 'Math\.(round|floor|ceil|sum)|\.reduce\(' web/src | cut -d: -f1,2 | sort) <(cut -d: -f1,2 web/no-compute-allow.txt | sort)` prints nothing. Note (not acceptance): that the front end computes nothing beyond the listed lines is judged by the reviewer.
  Note (not acceptance): the split moves declarations and changes no behaviour and no rendered output; the author runs the tests of the page before and after it.
  Owner's word: given 2026-11-05 — copy `web/` from the old clone (origin/main b00b4ed) by the allow-list of the privacy scan: `web/**` except `web/src/generated/**` (owned by contracts, stubbed by issue 1); the scan (715 files) found no account id, key, token, e-mail, chat id, local path or IP, and every fixture value synthetic.
  Note (not acceptance): before the pull request is opened the author runs the hygiene scan on the copied set (offline substitute for the hygiene item above: the hygiene substitute of issue 2 with `<paths>` the copied paths), and the pull request description names the source commit b00b4ed of the old clone, the copy allow-list (the path globs copied) and the files copied.
  Depends on: 1, 2 (the hygiene and ratchet scripts), 3 (ci-web). Runs in the frontend slot in parallel with 4-12. Order: after 1, 2, 3.
  Size: L

### docs-carryover

17. Docs: carry over the binding decisions and the glossary, de-personalised.
  Goal: The rules optdesk inherits from option-trader are in this repository, with no personal context of the owner.
  Files: docs/GLOSSARY.md, docs/engineering.md, docs/adr/0003-*.md, docs/adr/0004-*.md, docs/adr/0010-*.md, docs/conventions/register.toml, docs/carryover-notes.md, docs/carryover-manifest.txt.
  Design: docs/design/week1.md#docs-carryover
  Acceptance:
  - The six files exist and are not empty (`docs/GLOSSARY.md`, `docs/engineering.md`, ADR 0003, ADR 0004, ADR 0010, `docs/conventions/register.toml`), and the phrase `Python sends` occurs in none of them.
  - `docs/carryover-manifest.txt` has one line per file, `<source path> b00b4ed <sha256 of the source>  <destination path>`: `grep -cE ' b00b4ed [0-9a-f]{64}  docs/' docs/carryover-manifest.txt` prints 6. The sources are `GLOSSARY.md`, `docs/engineering.md`, ADR 0003, ADR 0004, ADR 0010 and `src/option_trader/conventions/register.toml` of the old clone; the destinations are not byte copies (they are de-personalised), so the manifest records the source, it does not equal the destination hash.
  - De-personalised: `grep -nE '\b[DU]U?[0-9]{6,8}\b|[0-9]{9,}' docs/GLOSSARY.md docs/engineering.md docs/adr/0003-*.md docs/adr/0004-*.md docs/adr/0010-*.md docs/conventions/register.toml` prints nothing (account-id shapes and long digit runs); the same files carry no `option-trader#` and no `github.com/<owner>/option-trader` link (next item).
  Note (not acceptance): a private word list of the owner is kept outside the repository and is never named in it; the dispatcher runs it on the copied files before merge and writes only `private word list: 0 hits` into the pull request description.
  - No file holds an issue number or link of the private repository (`grep -nE 'option-trader#|github.com/[^ ]*/option-trader' <files>` prints nothing).
  - `docs/carryover-notes.md` has one `## ` heading per copied file, each naming a path of the Files line (`grep -c '^## ' docs/carryover-notes.md` prints 6) and the phrase `Python sends` occurs in none of it. Note (not acceptance): that the notes list each sentence removed or restated, by section and not quoted, is checked by the owner against the diff; the old source is not in this repository.
  - `go run ./scripts/hygiene` (issue 2) exits 0 on the copied set [CI]; offline: the hygiene substitute of issue 2 with `<paths>` the copied paths.
  Note (not acceptance): whether a copied sentence is personal is judged by the owner in `docs/carryover-notes.md` and the diff; a rule that depended on such a sentence is restated without it. Before the pull request is opened the author runs the hygiene scan on the copied set, and the pull request description names the source commit b00b4ed, the copy allow-list and the files copied.
  Depends on: 2 (the hygiene script), 3. Order: after 2, 3. Size: S

## Left out

- Issues 13, 15 and 16 of the earlier drafts (wire-run-once, page-schemas-28, web-fixtures): cut by a follow-up design whose interface is the landed week-1 code (written once wave 1 merges). 13 wires the modules of 6 to 12 in `cmd/ot` and hooks the window rule of a job manifest (`job.toml`, issue 11) to the window rule of the runner (issue 10) (`ot run-once`, `ot setup pg`, `ot page`, `ot serve`, the adapter from `store.SessionDayReader` to `clock.SessionSource`, the wiring table of `## Interface`); 15 brings the other 28 page schemas with a sha256 manifest of the copies, and the TS generation (json-schema-to-typescript as a devDependency, the TS step of `contracts/generate.sh`, `web/src/generated/index.ts`); 16 brings the page fixtures and their ajv validation tests. Each needs the merged output of several siblings, so no issue of this design can carry it. Their numbers stay unused and the anchors of the other issues do not move.

- One interface issue per module. Not chosen: the files are declarations plus generated records and small, one pull request keeps the names consistent, and the crew check wants one entry marked as the interface; a split costs a review per module for no added safety. The price is that issue 1 crosses modules; it is the only crossing of more than two modules.
- Keeping the advisor's original issues with cross-module Files. Not chosen: crew refuses a Files list that spans two modules, and several issues of the advisor's list did. The cut above follows the module map instead.
- A `doc.go` per package in the skeleton. Not chosen: each package is created by the issue of its module (and its `api.go` by issue 1), so no issue creates a package another issue owns.
- `size.toml` with per-package caps. Not chosen: AGENTS.md puts the ceilings in `docs/design/ratchet.json` and owner-only; issue 2 reads that file.
- Hand-maintained `arch.toml`. Not chosen: it is derived from the depends column of modules.md and checked in `checks.yml`, so two lists cannot drift.
- Hand-written Go records in `api.go` with a drift test against the generated ones. Not chosen: two sources of truth; issue 1 generates the records and `api.go` aliases them.
- Putting `go.mod` in every module that adds a dependency. Not possible: a path belongs to one row; the issues that list go.mod (see `## Modules`) list it, are two-module entries where they leave their module, and do not run at the same time.
- Wiring the web build output into `internal/pages/embed/dist` (embedding the real build). Not chosen for week 1: it needs a build step that writes into the pages module. Issue 12 serves a stub; a later issue of the pages module adds the copy step.
- The calendar generator `pyside/tools/calendar_table.py` (clock) and any population of `master.session_day` outside fixtures: later weeks; week 1 reads the table only from fixtures and tests.
- The pyside layout: AGENTS.md says `src/<pkg>/<module>/api.py`, the declaration's owner_only list names `pyside/ibkr/transport.py`. They disagree; settled with the owner before week 2. The pyside row of modules.md is minimal until then (no interface or test file named).
- The generated TS types for the page components of the migrated front end (they keep local types in issue 14).
- The broker, compute and notify modules and their interface files: later weeks.
- The `Page` and `Builder` names of pages (a page id and version; `Build(ctx, day)` returning the vN JSON bytes): no week-1 issue calls them, since issue 12 uses only the two constructor types of the wiring table; the follow-up design freezes them when it needs them.
- The week-2 issue "Page schemas vN+1: one cleanup pass" (advisor addendum of 2026-11-05 21:1x), scoped by `option-trader-advisor-schema-fitness-2026-11-05.md`: tagged unions instead of `anyOf [X, null]`, shared defs in `common.v1.json`, `home_page` references the five summaries, Python-path def names renamed, register figure id and version in every figure, the five dead versions dropped, unit vocabulary normalised, rounding annotation half-even, money value stays a number, every page carries `stale_after` emitted by Go and the browser stops comparing against `Date.now()`. New vN only; v1 files untouched until the front end has moved. Issue 15 of the follow-up design carries the schemas over unchanged first.
- `buf breaking` and a document-to-contract correspondence check: the plan drops both.
- Owner first items 1-5 and Probe 0 of the advisor list are owner or dispatcher steps, not issues.

## Adversarial

- Nothing wired FRED to store to page: adopted in part. The wiring moved to a follow-up design (see Left out); `## Interface` freezes the wiring table it will use.
- An interface skeleton before any implementation is the biggest risk: accepted. Issue 1 freezes only names that a later issue calls; interface-change issues in week 2 are the accepted cost.
- A simpler cut (fewer modules, defer web): not adopted. ADR 0004 binds clock and session_day, the owner wants web in parallel (issue 14), one module per issue is required.
- Hand-written records beside generated ones: adopted. Issue 1 generates the records and `api.go` aliases them; no drift test.
- The synthetic-v1 marker field against `additionalProperties: false`: adopted. The marker moved to `MANIFEST.synthetic-v1.json` per fixtures directory (issues 1, 5, 9).
- "Independent after issue 1" was false because of go.mod: adopted. Issue 1 holds the shared tools; the issues that list go.mod never run together (see `## Modules`).
- Serial chain with owner merges: adopted for wave 0 (1, 2, 3). Issue 14 waits for the owner-merged workflows (issue 3, wave 0), so its [CI] items have a workflow to be decided by.
- Checks that need the network: adopted. Items marked [CI] carry an offline grep substitute (see `## Issues`); no `npx`, action pins checked by a counting grep (issue 3).
- Pyside layout against the owner_only path: moved to Left out, settled with the owner before week 2.
- Generated TS entry file: issue 1 stubs `web/src/generated/index.ts`; the follow-up design regenerates it under a Design line.
