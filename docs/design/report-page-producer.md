# Report page producer: a pages builder that serves `report_page` v2 per request, with the risks block of risk-alerts

Author: the optdesk designer (first draft)
Owner approved: (pending)
Revision: 2

Source: the dispatcher's question "no code produces `report_page`, so the `risks` block planned by pull request 492 is not visible".
Read on main at 5f0edc8: `AGENTS.md`, `docs/design/modules.md`, `docs/design/report-page.md` (Decision 1 and `## Left out`),
`docs/design/pages-builders.md`, `docs/design/pages-registry.md`, `docs/design/option-page-serve.md`, `docs/design/scheduler.md`,
`docs/design/compute-figures.md`, `docs/contracts/pages.md`, `docs/contracts/jobs.md`, `docs/contracts/store.md`,
`contracts/schemas/page.report_page.v2.json`, `internal/pages/api.go`, `internal/pages/option_data_page/page.go`,
`internal/pages/embed/embed.go`, `internal/store/api.go` (`ReadingReader`, `GapReader`), `internal/store/reading/reader.go`,
`migrations/0013_readings.sql`, `cmd/ot/serve.go`, `cmd/ot/pagelist.go`, `cmd/ot/wire.go`, `arch.toml`.
Read on the branch `design/risk-alerts` (pull request 492, revision 2): `docs/design/risk-alerts.md` (Decisions 3 and 13,
`alerts-risks-block`, `## Left out`).

## What main holds today

- Schema: `contracts/schemas/page.report_page.v2.json` has five always-present blocks (`actions`, `account_moves`, `risks`, `market_watch`,
  `global_assets`) and six optional ones. In the four non-risk blocks every figure field is `anyOf` the typed value or `Gap`
  (`trades`, `accounts`, `movers`, `indices`, `companies`, `proxies`, `sector`; `chart_aria` is a plain string), and `gaps` is an array of at most 20.
  So a block that says "no data" is valid with a `Gap` in each field. `RisksBlock.items` is a list of `RiskItem` or a `Gap`. `generated_at` must
  match a pattern that ends in `+08:00` (`x-timezone: owner`), so a builder writes it in the owner's zone, as `macro_page.Build` does with `Config.Zone`.
- Producer: none. `internal/pages/` holds `macro_rates`, `macro_page`, `option_data_page`, `home_page` (compose only), `gaps` and `embed`; no `report_page`.
  `report-page.md` (Decision 1 and `## Left out`) kept the Go builder out because no account, trade or position dataset exists and "a builder would write gaps only".
- Pages are built per request, never stored. `pages.Builder` is a function of a day and a time; `option_data_page.NewPage(c)` closes over
  readers it was given and ignores the series reader; `embed.Handler(vm)` writes the JSON into `<script id="vm">`; `ot serve` builds per request and
  `ot page` prints the JSON (`cmd/ot/serve.go`, `page.go`). The `pages` row of `modules.md` forbids computing and writing the database.
- Readings: `compute_figures` (designed, `docs/design/compute-figures.md`) writes `readings.reading`; `store.ReadingReader.LatestReadings` reads the base
  tables, which only `ot_w_readings` can read. The role `ot_ro_pages` has `SELECT` on the view `rd.reading_r1` and nothing on the tables (`docs/contracts/store.md`,
  `migrations/0013_readings.sql`). So no code of the pages role can read a reading today. `store.GapReader` is the precedent: it reads views as `ot_ro_pages`.
- Scheduler: a job answers `Schedule()` with an anchor (`session_open`, `session_close` or a wall time); the runner runs jobs that write datasets and checks
  windows. Nothing in `scheduler.md` or `jobs.md` runs a page.
- Pull request 492 (open, not on main) plans `risks_block.Build(ctx, Input) ([]byte, error)` returning the JSON of one `RisksBlock`, with
  `Input{Rules compute.AlertRules; Evaluate compute.EvaluateAlertsFunc; Read compute.ReadingFunc; Today contracts.Day; GeneratedAt time.Time}`, and
  names the producer as its next step (`## Left out`), with "the stores opened under `ot_ro_pages`". `compute.ReadingFunc` has the shape of the
  method value `LatestReadings`. That method cannot run under `ot_ro_pages` (above): the notify pass of 492 uses the writer role, the page cannot.

## Decisions

1. Which module. `pages`, a new package `internal/pages/report_page`, shaped like `option_data_page`: `Build(ctx, c Config, day, generatedAt) ([]byte, error)`
   and `NewPage(c Config) pages.Page`. No new module row, no change of `internal/pages/api.go` (`Page`, `Meta`, `Builder` exist). The package assembles; it
   computes no figure: every number and text of the `risks` block comes from the 492 rules and `risks_block`, and the other four blocks hold no number.
2. What it assembles in stage 1. The envelope (`schema` `report_page`, `schema_version` 2, `market`, `slot`, `session_date`, `generated_at`, `title`, `say`,
   `banner`, `status_lines` empty, `stale` null) and the five always-present blocks. `risks` is the bytes of the `RiskBlockFunc` of the `Config`, embedded as raw JSON (the
   package does not import `risks_block`: a builder package imports no sibling builder package, so `ot` passes the function in). Each of the other four is a block
   with `key` its own key, `title`, `sub` and `say` (fixed constants, 1 to 200 characters), `state` `gap`, `gaps` one `Gap` of reason `not_built` (the truthful value of the schema enum: no producer exists), `data_times` the string `暂无数据` (the schema makes it a string of 1 to 200 characters, so it is never empty), and each figure field a `Gap` of reason `not_built` with the text `<title>暂未生成`, except `chart_aria` of `global_assets`, which is a plain string constant (`暂无图表数据`). The six optional blocks are absent for both slots. All
   fixed texts are Chinese constants of the package (`strings.go`, the language of the pages; `pages-builders.md` line 58); wording is advisor-set, not an owner decision.
   This reverses the "gaps only" objection of `report-page.md`: the page now has one block with content, which is the goal of owner decision 2 of 492; the four gap blocks say plainly what has no producer, with the reason `not_built`.
3. When it runs. On request, not on a schedule and not as a job. Reasons: the `pages` module does not write the database (`modules.md`), a stored page would need a new dataset, role
   and migration plus a job, and the inputs are already stored with their own times. Freshness reaches the page through the `risks` block's `data_times` and its
   `stale_data` gaps, and through the day the request names. The scheduler design is not edited: no `Schedule()`, no window, no slot. The readings are written by `compute_figures` at its own slots; a request before that shows `partial` or `gap` in `risks`, which is the correct answer.
4. Where it is stored. Nowhere. A reader of the page gets the JSON built from the readings at the time of the request. A page that must be kept (a
   history of reports) is a later design (`## Left out`).
5. How the web reads it. As every page does: `ot serve --name report_page --market us --slot close` builds the JSON per request and `embed.Handler` writes it into `<script id="vm">`; the shell picks the
   view by the `schema` field. The view, its generated type and its fixtures are the other design, `docs/design/report-page.md` (`report-types`, `report-page-view`); this design adds no
   file under `web/` and no schema change (`Contracts: unchanged`). The front end computes nothing; every string in the payload is written here or by `risks_block`.
   Until `report-page-view` lands, the JSON is visible with `ot page` (stdout) and the owner sees the block only through the view; the two designs do not wait for each other.
6. One page name, two flags. `Meta.Name` is `report_page`, `Meta.Schema` is `page.report_page.v2`; the schema carries `market` and `slot` in its envelope, so one schema serves four reports. The market and slot are fields of `Config`
   (checked by `Build`: a value outside `US|JP` and `open|close` is an error). The command takes them as flags: `ot page --name report_page --market us --slot open` (both required for this name; `us|jp`, `open|close`, lower case, turned
   into the envelope values by `ot`), and `ot serve --name report_page` serves one route that takes the same two query parameters (`?market=us&slot=open`; a missing or invalid value answers 400 with a fixed text, no error text). The day is `--day`
   or today as the commands already compute it (`dayOrDefault`). JP calendar days are not handled here (`## Left out`). The advisor chose flags over four names.
7. Reading readings as `ot_ro_pages`, least privilege kept: the page process holds no writer DSN. A new store reader over views: `store.ReadingViewReader.WinnerReadings(ctx, name, subjectKind, subject, from, to)` with the
   signature of `LatestReadings` (a `compute.ReadingFunc` as a method value; 492 needs no change), opened from `store.DSNEnvPages`. Two facts decide its shape.
   (a) 492 Decision 7 evaluates a rule on the reading parameter `data_day` (no such parameter gives `no_data_day`, so `no_data`), and `rd.reading_r1` has no parameter columns
   (`migrations/0013_readings.sql:81-91`). So a migration (store module) adds the view `rd.reading_parameter_r1` (every column of `readings.reading_parameter`, owner `ot_owner`, `SELECT` to `ot_ro_pages`; the base table is not granted),
   and `WinnerReadings` reads the winner rows of `rd.reading_r1`, then the parameters of those exact keys from the new view, and fills `ReadingRecord.Parameters` (`param`, `value_num`, `value_tag`), the same row type `LatestReadings` returns.
   Inputs stay empty (no rule reads them). The migration number is the next free one at cut time (after `alerts-store-migration` of 492 if that has merged).
   (b) The view `rd.reading_r1` is partitioned by `(name, version, subject_kind, subject, day)`, so after a figure version bump it holds two rows for one day. The reader takes no version (the shape of `ReadingFunc` has none) and
   keeps one row per day: the highest `version` of the day, then the winner rule inside that version (highest `rev` of state `ok`, else the highest `rev`), as `LatestReadings` takes the higher version. A test bumps the version and sees one row per day.
   The winner rule is the rule of the page, and PR 492's notify pass must read through the same rule (`WinnerReadings`, as `Read` of its `AlertDeps`), so the page and the Telegram alert never disagree on a row. That is a one-line change of 492 Decision 7
   (the reader behind `ReadingFunc`), made in PR 492, which the dispatcher handles; this design does not edit 492. The notify pass opens its reader under the role it already has: `WinnerReadings` also reads `rd.reading_r1` and the new view, which `ot_w_readings` can read once the grant is given (see the next sentence), so 492's `ot` wiring is the one place that changes.
   The grants follow the existing pattern for every `rd` view (`migrations/0002:58`, `0006:100`, `0007:70`, `0009:31`, `0010:10`, `0012:77-78`, `0013:104`): `SELECT` on the view to the role that reads it, none on the base table. The migration grants `rd.reading_parameter_r1` to `ot_ro_pages` and `ot_w_readings`. `docs/contracts/store.md` line 75 (the grants of `readings`) names both views.
   This is an interface change of `internal/store/api.go`, so it is its own first entry.
   error: it is a `partial` or `gap` state inside the `risks` block (492 Decision 13).
8a. Lifecycle of the reader (the `ot` entry). The reader is opened once per process: once per `ot page` command and once at the start of `ot serve --name report_page`, as `serve.go` opens `seriesReader` once before it listens (`defer closeFn()`), and closed on exit.
   `pageNamed(a, name)` has no context and returns no closer, so `report_page` is not built by it: `page.go` and `serve.go` each test the name `report_page` before calling `pageNamed` and call `report.go` (`reportPage(ctx, a, market, slot) (pages.Page, func(), error)` for the command, a handler over one opened reader for the server).
   `pageNamed` is unchanged except that its known-pages list gains `report_page` after `macro_page`, so the two tests that assert that text change by that one word and are in the entry's Files. The `.env` line is `OT_PG_DSN_PAGES` only.
9. Libraries. None new. `encoding/json` and `santhosh-tekuri/jsonschema/v6` (already in `go.mod`, used by the 492 block test) check the output against `contracts/schemas/page.report_page.v2.json`. No own
   implementation of anything a mainstream library gives.
10. Stage 2 (described, not cut). Each non-risk block gains its own `BlockFunc` in `Config`, filled when its data exists: `market_watch` and `global_assets` from market readings and series,
    `actions` and `account_moves` from the account datasets (none exists), the open-slot blocks later. A new block never changes `Build`'s signature: it replaces the gap constant by the bytes of its function, as `risks` does now.

## Modules

No row of `docs/design/modules.md` is added. The list of implementation packages `cmd/ot` may import gains `internal/pages/report_page` (one sentence edited by this pull request, with
`arch.toml` regenerated by `go run ./scripts/arch`, as 492 does for its three packages; the two pull requests edit the same sentence, so the second to merge rebases it).
`internal/store/reading` is already in that list. The `pages` row keeps its depends cell (`contracts store`, plus `compute` once 492 merges: `report_page` imports neither).

| issue | module | paths |
|---|---|---|
| report-reading-view-interface (interface) | store | `internal/store/api.go`, `internal/store/api_test.go` |
| report-reading-view-impl | store | `migrations/NNNN_reading_parameter_view.sql`, `internal/store/reading/view.go`, `internal/store/reading/view_test.go`, `internal/store/reading/pg_view_test.go`, `docs/contracts/store.md` |
| report-page-builder | pages, plus repo (judged two-module entry: `.go-arch-lint.yml`, as `alerts-risks-block` of 492) | `internal/pages/report_page/**`, `docs/contracts/pages.md`, `.go-arch-lint.yml` |
| report-page-ot | ot | `cmd/ot/report.go`, `cmd/ot/report_test.go`, `cmd/ot/page.go`, `cmd/ot/serve.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/pagelist.go`, `cmd/ot/pagelist_test.go`, `cmd/ot/serve_test.go`, `docs/contracts/ot.md` |

## Interface

Owner `store`, file `internal/store/api.go`; additions only.

```go
// ReadingViewReader reads the winner rows of rd.reading_r1 as the role ot_ro_pages, which has no grant on a readings table.
// Rows carry the parameters (data_day among them) read from the view rd.reading_parameter_r1, and no inputs. One row per day: the highest version of the day, then in that version
// its highest-rev ok row, else its highest-rev row.
type ReadingViewReader interface {
	// WinnerReadings returns one row per day with day between from and to, inclusive, ascending by day, for one name, subject kind and subject; no rows gives an empty slice.
	WinnerReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)
	Close() error
}

// OpenReadingViewFunc opens the view reader from a DSN of the role ot_ro_pages (internal/store/reading.OpenView).
type OpenReadingViewFunc func(ctx context.Context, dsn string) (ReadingViewReader, error)
```

Owner `pages`, package `internal/pages/report_page` (a plain package like `option_data_page`; `internal/pages/api.go` is not edited):

```go
// RiskBlockFunc returns the JSON of the RisksBlock of report_page v2 for a day (ot closes it over risks_block.Build of risk-alerts).
type RiskBlockFunc func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Config is one report: a market (US or JP), a slot (open or close), the owner's zone (generated_at is written in it) and the block source.
type Config struct {
	Market, Slot string
	Zone         *time.Location
	Risks        RiskBlockFunc
}

func Build(ctx context.Context, c Config, day contracts.Day, generatedAt time.Time) ([]byte, error)
func NewPage(c Config) pages.Page // Meta{Name: "report_page", Schema: "page.report_page.v2"}; Build ignores the series reader
```

Compatibility with 492: `compute.ReadingFunc` is `func(ctx, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)`, which is
the method value `ReadingViewReader.WinnerReadings`; `ot` passes it as `risks_block.Input.Read`. The `risks` block is one field of `Config`, filled by the 492 rules and not otherwise known to `report_page`.

## Issues

The cut: the store interface first, then three entries that can run in parallel (the store implementation, the builder and, once both and 492's block exist, the `ot` entry). Items are facts read from the diff with an offline
command; items decided by workflows are under `Checked by CI`. Every test calls `t.Parallel()`, uses `t.TempDir()` for any file, a fake clock where time matters, no sleep, no fixed port, no live database
(a test that needs PostgreSQL carries the tag `pg`).

### report-reading-view-interface (interface)

Module: store. Size: S.
Files: `internal/store/api.go`, `internal/store/api_test.go`
Order: first. No other design edits `internal/store/api.go` at cut time except 492's `alerts-store-interface` (`SentMark`, `SentStore`): if it has not merged, this entry goes after it (same file).
Design: docs/design/report-page-producer.md#interface
Goal: `ReadingViewReader` and `OpenReadingViewFunc` are declared, with a test of the contract from both sides.
Acceptance:
1. `grep -c "type ReadingViewReader interface\|type OpenReadingViewFunc func" internal/store/api.go` prints 2, and the method line is `WinnerReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)`.
2. `grep -n "func TestReadingViewReaderContract" internal/store/api_test.go` prints one line; `go test ./internal/store -run TestReadingViewReaderContract -race -count=1` passes: a fake that implements the interface (one side) and a caller that reads two days through it and sees ascending days and an empty slice for no rows (the other).
3. `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep '^-' | grep -vc '^---'` prints 0 (additions only), and `grep -c "database/sql\|pgx" internal/store/api.go` is unchanged from main.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists only the two files of the Files line.
Checked by CI: `go build ./...`, `go test ./internal/store/...`, golangci-lint, go-arch-lint.

### report-reading-view-impl

Module: store. Size: M.
Files: `migrations/NNNN_reading_parameter_view.sql` (NNNN the next free number at cut time), `internal/store/reading/view.go`, `internal/store/reading/view_test.go`, `internal/store/reading/pg_view_test.go` (tag `pg`, new), `docs/contracts/store.md`
Order: after report-reading-view-interface, and after `alerts-store-migration` of 492 (`docs/design/risk-alerts.md`) if both are unmerged: the migration number is the next free one.
Design: docs/design/report-page-producer.md#decisions (Decision 7)
Goal: `reading.OpenView(ctx, dsn)` returns a `store.ReadingViewReader` that reads `rd.reading_r1` and `rd.reading_parameter_r1` only, one row per day, with the parameters (`data_day`) filled, so a connection of `ot_ro_pages` feeds the 492 rules.
Acceptance:
1. `test -f migrations/NNNN_reading_parameter_view.sql` and `grep -c "CREATE VIEW rd.reading_parameter_r1\|GRANT SELECT ON rd.reading_parameter_r1 TO ot_ro_pages, ot_w_readings\|OWNER TO ot_owner" migrations/NNNN_reading_parameter_view.sql` prints 3 or more; `grep -c "GRANT.*readings\.reading_parameter\b.*ot_ro_pages" migrations/NNNN_reading_parameter_view.sql` prints 0 (no grant on a table); a `Down` section drops the view.
2. `grep -c "rd.reading_r1\|rd.reading_parameter_r1" internal/store/reading/view.go` is at least 2 and `grep -c "readings\.reading\b\|readings\.reading_input\|readings\.reading_parameter\b" internal/store/reading/view.go` prints 0 (views only, no table).
3. `go test ./internal/store/reading -run 'TestViewQueryShape|TestViewRowMapping|TestViewKeepsDataDay|TestViewOneRowPerDayAfterVersionBump|TestViewEmptyIsEmptySlice|TestViewRejectsReversedRange' -race -count=1` passes without a database. `TestViewKeepsDataDay` feeds a fake row source with a parameter row `data_day` and fails if `ReadingRecord.Parameters` of the result lacks it (so a mapping that drops the parameter fails offline); `TestViewOneRowPerDayAfterVersionBump` feeds rows of version 1 and 2 for one day and sees the version 2 row only; `TestViewQueryShape` checks the SQL text as a string (the `WHERE` on name, kind, subject and day range, `ORDER BY day`).
4. `grep -c "^func TestViewAgainstRoPages\|^func TestViewParameterDataDay" internal/store/reading/pg_view_test.go` prints 2 and the file's first line is `//go:build pg`; the tests connect as `ot_ro_pages`, insert through the writer a reading with the parameter `data_day`, and read it back through `WinnerReadings` (the parameter present, an `ok` row of rev 1 beating a `missing_input` row of rev 2, one row per day after a version bump). They run only in GitHub CI.
5. `docs/contracts/store.md` (module store) names `WinnerReadings`, `OpenView`, both views, the winner and version rule, `OT_PG_DSN_PAGES`, and its grants line (line 75) names both views for `ot_ro_pages` and `ot_w_readings`, and adds a Version history line; `grep -c "DSNEnv\|os.Getenv" internal/store/reading/view.go` prints 0.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths of the Files line; each new file is under 600 lines.
Checked by CI: `go test ./internal/store/...` with the tag `pg` in the PostgreSQL job (the migration applies and the shape tests of `internal/store/setup` pass), golangci-lint, go-arch-lint.

### report-page-builder

Module: pages, plus repo (judged two-module entry: `.go-arch-lint.yml`, as `alerts-risks-block` of 492). Size: M.
Files: `internal/pages/report_page/builder.go`, `internal/pages/report_page/blocks.go`, `internal/pages/report_page/strings.go`, `internal/pages/report_page/page.go`, `internal/pages/report_page/builder_test.go`, `internal/pages/report_page/testdata/` (new), `docs/contracts/pages.md`, `.go-arch-lint.yml` (regenerated by `go run ./scripts/arch`: the component of the new directory and the `ot` list)
Order: first among the builders; needs none of 492's code (its test passes a fake `RiskBlockFunc`). It is the entry that makes 492's block visible once `report-page-ot` wires the two.
Design: docs/design/report-page-producer.md#decisions (Decisions 1, 2, 6, 8)
Goal: `Build` and `NewPage` of `## Interface` return the JSON of `contracts/schemas/page.report_page.v2.json`: the envelope, the `risks` block as the bytes the `Config` function gives, four gap blocks (reason `not_built`), no optional block.
Acceptance:
1. `grep -rhE "^func (TestBuildCloseUS|TestBuildOpenJP|TestBuildRisksBytesEmbedded|TestBuildGapBlocks|TestBuildGapBlockRequiredKeys|TestBuildNoOptionalBlocks|TestBuildGeneratedAtInZone|TestBuildMatchesSchema|TestBuildBadMarketOrSlot|TestBuildRisksErrorIsError|TestNewPageMeta)\(" internal/pages/report_page | wc -l` prints 11, and `go test ./internal/pages/report_page -race -count=1` passes.
2. `TestBuildMatchesSchema` validates every output (both slots, both markets, a `risks` block of state `ok`, `partial` and `gap`) against `contracts/schemas/page.report_page.v2.json` with `santhosh-tekuri/jsonschema/v6`; `grep -c "jsonschema" go.mod` is unchanged from main.
3. `TestBuildRisksBytesEmbedded`: the `blocks.risks` of the output equals, as JSON, the fake function's bytes; `TestBuildGapBlocks`: `actions`, `account_moves`, `market_watch` and `global_assets` each have their own `key`, a non-empty `title`, `sub` and `say`, `state` `gap`, a non-empty string `data_times`, one `Gap` of reason `not_built` in `gaps`, and a `Gap` of reason `not_built` in every figure field that the schema allows as a `Gap` (the test lists the required fields of each block from the schema file, so a new required field fails it); `chart_aria` of `global_assets` is a non-empty string; `TestBuildGapBlockRequiredKeys` checks `key`, `sub`, `say`, `data_times` and `chart_aria` by name.
4. `TestBuildGeneratedAtInZone` uses `time.FixedZone("test", 8*3600)` and checks the `+08:00` form; `grep -rc "LoadLocation\|time.Now()\|time.Sleep\|float64" internal/pages/report_page/*.go | grep -v '_test.go' | grep -v ':0$'` prints nothing.
5. `grep -c "t.Parallel()" internal/pages/report_page/builder_test.go` is at least `grep -c "^func Test" internal/pages/report_page/builder_test.go`.
6. `grep -rc "internal/pages/risks_block\|internal/pages/\(macro\|option\|home\)\|internal/store/reading\|internal/compute" internal/pages/report_page | grep -v ':0$'` prints nothing, and `go run ./scripts/arch --check` prints `arch: ok`.
7. `docs/contracts/pages.md` names `report_page.Build`, `NewPage`, `Config`, `RiskBlockFunc`, the page name `report_page` and the gap-only blocks, with a Version history line.
8. `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths of the Files line (`testdata/` counts as one path); `git diff $(git merge-base HEAD origin/main) -- contracts web internal/pages/api.go | grep -c .` prints 0; each new file is under 600 lines. The pull request carries `Contracts: unchanged`.
Checked by CI: `go build ./...`, `go test ./internal/pages/...`, golangci-lint, go-arch-lint, the size rule.

### report-page-ot

Module: ot. Size: M.
Files: `cmd/ot/report.go`, `cmd/ot/report_test.go`, `cmd/ot/page.go`, `cmd/ot/serve.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/pagelist.go`, `cmd/ot/pagelist_test.go`, `cmd/ot/serve_test.go`, `docs/contracts/ot.md`
Order: after report-reading-view-impl, report-page-builder (which regenerates `.go-arch-lint.yml`, so this entry does not edit it) and, from pull request 492 (`docs/design/risk-alerts.md`), alerts-compute-interface, alerts-rules (`alertrules.Evaluate`, `Load`, `DefaultRules`) and alerts-risks-block (`risks_block.Build`). If 492 is not merged this entry waits. `cmd/ot/wire.go` is also edited by other open designs (their `deps` fields): the second to merge rebases.
Design: docs/design/report-page-producer.md#decisions (Decisions 3 to 8a)
Goal: `ot page --name report_page --market us --slot open` prints the page and `ot serve --name report_page` serves it (`?market=us&slot=open`), built per request from the stored readings read as `ot_ro_pages` through one reader opened once per process, with the `risks` block of 492.
Behaviour:
- `page.go` and `serve.go` test the name `report_page` before `pageNamed`: `page.go` reads `--market` and `--slot` (both required for this name, else an error that names them), opens the reader once, builds the page with `report.go`, prints it and closes the reader; `serve.go` opens the reader once before it listens (as it does `seriesReader`), serves one handler from `report.go` that reads `market` and `slot` from the query for each request (missing or invalid: 400 with a fixed text), builds per request, and closes the reader on exit. Other names behave as before.
- `pagelist.go`: `pageNamed` keeps its signature; its known-pages list gains `report_page` after `macro_page` and nothing else changes.
- `report.go` holds `reportPage(ctx, a, market, slot)` (the page, a closer, an error), the handler, and the reader open: the `.env` line `store.DSNEnvPages` only (a missing line is an error that names the variable and no value) and `OpenReadingViewFunc`. `Risks` is a closure over `risks_block.Build` with `Rules` from `alertrules.DefaultRules`, `Evaluate` `alertrules.Evaluate`, `Read` the method value `WinnerReadings`, and `Today`, `GeneratedAt` from the arguments. `Zone` is the zone of the profile (a nil zone is the same error as `macro_page`).
- With the demo profile no store is opened: a demo `ReadingViewReader` in `report.go` returns fixed synthetic readings, with the parameter `data_day`, for the rule figures (no real quote, no account).
- `wire.go` gains the `deps` fields `openReadingView` (`store.OpenReadingViewFunc`, real value `reading.OpenView`), `reportPage`, `alertRules`, `evaluateAlerts` and `riskBlock`; only `wire.go` imports the implementation packages.
Acceptance:
1. `TestPageReportFlags` (fake `deps`): `--name report_page --market us --slot open` builds; a missing `--market` or `--slot`, `--market xx` and `--slot noon` are errors naming the flag; `--name macro_rates` and `--name macro_page` give the bytes they gave before.
2. `TestUnknownPageListsReports`: the error of an unknown name is `unknown page "nosuch"; known pages: <the pageList names>, macro_page, report_page`; the expectations of `cmd/ot/pagelist_test.go:41` and `cmd/ot/serve_test.go:115` change by that one word, and `git diff` of each shows only those lines.
3. `TestReportOpensPagesRoleOnly`: the fake opener sees the DSN of the line `OT_PG_DSN_PAGES` and no other DSN name, exactly once for `ot page` and exactly once for a `serve` that answers three requests; the fake reader is closed once, on exit, in both; a missing line fails before any read with the variable name in the error and no DSN text; `grep -c "OT_PG_DSN\|DSNEnv" cmd/ot/report.go` prints at most 1 and that use is `store.DSNEnvPages`.
4. `TestReportRisksAreFilled` (fake reader returning one row with the parameter `data_day` equal to the day, and a real `alertrules.Evaluate` with a one-rule set): the JSON of `ot page --name report_page --market us --slot close --day <fixed>` has `blocks.risks.state` `ok` and one item; the same row without `data_day` gives state `gap` (the 492 reason `no_data_day`), so the test fails if the reader mapping or the builder drops the parameter; the output validates against `contracts/schemas/page.report_page.v2.json`.
5. `TestReportServeBuildsPerRequest`: two requests to the report handler with a fake reader whose rows change between them return two different `risks` blocks; `?market=us` alone, `?market=xx&slot=open` and a reader error answer 400, 400 and 500 with fixed texts and no error text in the response.
6. `TestReportDemoOpensNoStore`: with the demo profile and an opener that fails the test when called, the page builds and its `risks` block has state `ok` or `partial`.
7. `git diff cmd/ot/wire.go` adds no import other than `reading` (if not yet imported), `report_page`, `risks_block` and `alertrules`; `docs/contracts/ot.md` names `report_page`, `--market`, `--slot`, the query parameters and `OT_PG_DSN_PAGES` for the report.
8. `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths of the Files line; each new file is under 600 lines; `go run ./scripts/arch --check` prints `arch: ok`.
Checked by CI: `go build ./...`, `go test ./cmd/ot/...`, golangci-lint, go-arch-lint.

## Order

| # | issue | module | size | order |
|---|---|---|---|---|
| 1 | report-reading-view-interface (interface) | store | S | first (after alerts-store-interface of 492 if that has not merged: same file) |
| 2 | report-reading-view-impl | store | M | after 1 (and after alerts-store-migration of 492, for the migration number) |
| 3 | report-page-builder | pages, plus repo (judged) | M | first; parallel with 1 and 2 |
| 4 | report-page-ot | ot | M | after 2, 3 and, of 492, alerts-compute-interface, alerts-rules, alerts-risks-block |

Entries 1 and 3 start the day the design merges. What the owner sees end to end in a browser also needs `report-types` and `report-page-view` of `docs/design/report-page.md`: no entry here waits for them and they do not wait for this design.

## Left out

- The four non-risk blocks with data (`actions`, `account_moves`, `market_watch`, `global_assets`) and the six optional open-slot blocks: each needs a dataset or a figure reader that does not exist
  (`report-page.md`, `## What main holds today`). Decision 10 is the seam; each is a later design that fills one `BlockFunc`.
- A stored report (a table of pages, a history by slot, a push of the page): not needed for the goal; it would add a dataset, a role, a migration and a job, and the scheduler would need a `Schedule()`. Revisit if the owner wants to read yesterday's report as it was.
- `status_lines` from `internal/pages/gaps` (`Freshness`, `Lines` over `store.GapReader`): the stage-1 line is empty; a later entry adds a `GapLines` function to the `Config`. Not cut: `GapReader` has no opener wired for pages.
- The JP market calendar and the default day of a JP report: `--day` or the existing default (`marketZone`) is used; the slot's session day comes from `clock` in a later design.
- Changing 492: its `ReadingFunc` shape fits the new reader. 492's notify pass must read through the same winner rule (Decision 7); that one-line change of 492 Decision 7 is made in PR 492, not here.
- Removing `report_page` v1, a v3 schema, the web view and its type: `report-page.md`.

## Open questions for the owner

None. The advisor settled the grants (the existing `rd` view pattern), the winner rule, the Chinese gap texts (constants of `strings.go`, not of the schema), flags instead of four names, and one reader opened once per process.

## Adversarial

Source: `adversarial-report-page.md`, kept in the dispatcher's scratchpad (review of the first draft). The dispatcher decided (a) to (e) below before the second revision; the advisor settled the rest before the third.

1. Gap against the goal (the risks block would be `gap` on every request: `rd.reading_r1` has no `data_day`): adopted. Decision 7 (a) adds the view `rd.reading_parameter_r1` by a migration and the reader fills `Parameters`; `report-reading-view-impl` has the offline test `TestViewKeepsDataDay` and the `pg` tests, and `report-page-ot` item 4 fails if `data_day` is dropped. The false sentence "the rules read value, unit, state, reason_code and day only" is removed.
2. Biggest unnamed risk (version partition, two rows per day after a bump): adopted. Decision 7 (b): the reader takes the highest version of the day, then the winner rule; the signature stays that of `ReadingFunc`; `TestViewOneRowPerDayAfterVersionBump` and the interface comment say it.
3. Simpler way (use `OT_PG_DSN_READINGS` and `LatestReadings`): not adopted, by the dispatcher's decision (a): the page process keeps least privilege and holds no writer DSN. The cost is two store entries, now one interface and one implementation with a migration.
4. Lock-in: (a) winner versus newest is settled in Decision 7 (the page and 492's notify pass both read the winner rule); (b) raw bytes from `RiskBlockFunc` is kept on purpose (the 492 block owns its freshness; a page-level `data_times` rule is a later design with Decision 10); (c) one name `report_page` with `--market` and `--slot`, so no name per report is fixed; (d) fixed by point 5.
5. Gap blocks wrong against the schema: adopted. `data_times` is a non-empty string, `chart_aria` a string constant, the reason `not_built`, and `key`, `title`, `sub`, `say` are named and tested against the schema (`TestBuildGapBlockRequiredKeys`).
6. Likeliest slip (entry 4 lifecycle, known-pages tests, `.go-arch-lint.yml`): adopted, then changed by the advisor: the reader is opened once per process and `page.go` and `serve.go` are in Files (Decision 8a); `pagelist_test.go` and `serve_test.go` are in Files with the exact new text (acceptance 2); `.go-arch-lint.yml` is in the Files of `report-page-builder` (it regenerates the component and the `ot` list; checked on this branch: `go run ./scripts/arch --check` prints `arch: ok`), which makes that entry a judged two-module entry like 492's `alerts-risks-block`; `report-page-ot` stays one module.

Must-fix list: 1 adopted (point 1), 2 adopted (point 2), 3 adopted (point 5), 4 adopted with the files listed (point 6), 5 adopted for the builder entry only (the `ot` entry needs no edit of the file once the builder entry regenerates it).
Also noted, not a blocker: `cmd/ot/wire.go` is edited by other open designs; the order line says the second to merge rebases.
