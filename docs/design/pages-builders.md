# Pages: the Page and Builder names, and the builders that need no figure

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1.md` ("The `Page` and `Builder` names of pages ... the follow-up design freezes
them when it needs them") and the sentence of `docs/design/week1-wiring.md` that repeats it. Identifiers were read on
main at 164d783: `internal/pages/api.go` holds `BuildMacroRatesFunc` and `NewHandlerFunc` only; `internal/pages/macro_rates`
is the one builder; `internal/store/api.go` has `SeriesReader`, `SessionDayReader` and `OpsLedger` as readers.

Goal of the design: every further page builder is a value of one frozen shape (`pages.Builder`), and the page schemas
that the landed readers can fill without a figure of `compute` (week 8-9, not landed) get a builder each. A page
whose central figure would be a gap forever is not built here (see `## Left out`).

## Modules

No change to `docs/design/modules.md`: every path below belongs to the row `pages` (`internal/pages/**`,
`docs/contracts/pages.md`). The row's depends cell is `contracts store`; the builders read only `store.SeriesReader`.

| issue | module |
|---|---|
| pages-interface | pages |
| pages-indicator-detail | pages |
| pages-macro-page | pages |
| pages-settings-page | pages |

Every entry lies in one module. No entry edits `go.mod`, `go.sum`, `contracts/`, `web/` or `docs/design/modules.md`.
The jsonschema library of the tests (`github.com/santhosh-tekuri/jsonschema/v6`) is already in `go.mod`; no new direct
dependency.

## Interface

One entry is marked (interface): `pages-interface`. It edits `internal/pages/api.go`, which is an interface change; the
three builder entries depend on it and on nothing else.

Names added to `internal/pages/api.go` (declarations only):

| name | shape | meaning |
|---|---|---|
| `pages.Page` | `struct{ Schema string; Version int }` | the page schema name without prefix and version, and its version: `{"indicator_detail", 1}` is `page.indicator_detail.v1` |
| `pages.Meta` | `struct{ RunID, BuildID, Scope string }` | the three fixed strings every page of the schemas carries; each must match `^[A-Za-z0-9_-]{1,64}$`; the caller (the composition root) supplies them |
| `pages.Builder` | `interface{ Page() Page; Build(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error) }` | `Build` returns the vN JSON bytes; it reads no clock, computes nothing, and a missing value is a named gap, never 0 |

Names used as they are: `store.SeriesReader` (`SeriesPoints(ctx, seriesID string, from, to contracts.Day)`),
`contracts.Day`, `contracts.SeriesPoint` (fields `ObservationDate`, `RealtimeStart`, `Value` as a pointer, `Status`,
`Unit`, `PublishedAt`, `Source`), and the two existing function types of `internal/pages/api.go`, unchanged.

Not frozen (see `## Left out`): constructor function types for the builders, an adapter that makes
`macro_rates.Build` a `Builder`, and the registry of pages in `cmd/ot`.

Conventions shared by the three builders, repeated in each entry so that none needs a sibling:

- `generated_at` of these schemas is a string in the owner zone; the schema pattern fixes the offset. A builder writes
  `generatedAt` converted with a fixed-offset zone made by `time.FixedZone` (no tzdata file) in the format of the
  schema pattern.
- `schema_version` is the integer 1 and `schema` the page name, as the schemas say (`macro_rates` writes the string
  "1"; that schema says so and is not touched).
- The `stale` label: `data_time` is the day of the oldest observation shown at 00:00:00 in the owner zone;
  `stale_after` is that day plus 4 calendar days at 09:30:00 in the owner zone (the rule of the fixtures of
  `web/fixtures/macro-page`); it is date arithmetic on `YYYY-MM-DD` values, as in `macro_rates`. The constants are
  `freshnessLimitDays = 4` and `lookbackDays` in each package.
- The shared helpers are copied into each package (about 20 lines); a shared package would make the builders depend on
  one another. Accepted cost.

## Issues

`pages-interface` waits for nothing. The three builders wait for it only and run in parallel. Items under `Checked by CI`
are decided by the workflows of the head commit; every other item is a fact read from the diff, with an offline grep
where it is about behaviour. Each new source file stays under 600 lines.

### pages-interface

(interface) Freeze `Page`, `Meta` and `Builder` in `internal/pages/api.go`.
  Goal: later builders share one shape, and the names are frozen before the first of them is written.
  Files: internal/pages/api.go, internal/pages/api_test.go, docs/contracts/pages.md.
  Design: docs/design/pages-builders.md#interface
  Acceptance:
  - `internal/pages/api.go` declares `type Page struct`, `type Meta struct` and `type Builder interface`, each exactly once (`grep -c '^type Page struct' internal/pages/api.go`, `grep -c '^type Meta struct' internal/pages/api.go` and `grep -c '^type Builder interface' internal/pages/api.go` each print 1), with the fields and methods of the table of `## Interface`.
  - `internal/pages/api.go` still declares `BuildMacroRatesFunc` and `NewHandlerFunc` with the signatures they have on main, has no function body (`grep -c '^func ' internal/pages/api.go` prints 0) and imports only the standard library, `internal/contracts` and `internal/store`.
  - `internal/pages/api_test.go` holds `TestBuilderContract` next to the unchanged `TestPagesContract` (`grep -n 'func TestBuilderContract' internal/pages/api_test.go` and `grep -n 'func TestPagesContract' internal/pages/api_test.go` each find one line): one side a fake `Builder` that returns its `Page()` and a JSON body containing the `generatedAt` it was given, the other a caller that uses only `pages.Builder`; `grep -c 't.Parallel()' internal/pages/api_test.go` prints 2.
  - `docs/contracts/pages.md` states in `## Outputs` the three names and the rule that `Build` reads no clock and computes nothing, adds a line to `## Version history`, and in `## Contract tests` lists one line per package `internal/pages/indicator_detail`, `internal/pages/macro_page` and `internal/pages/settings_page`, each naming `TestBuilderIdentity` (`grep -c 'TestBuilderIdentity' docs/contracts/pages.md` prints 3); no heading is added or renamed.
  - No file outside the three listed is in the diff.
  - Checked by CI: ci-go (build, vet, tests, `golangci-lint`, the arch check); the interface change needs no generated output.
  Depends on: none. Order: first.
  Size: S

### pages-indicator-detail

The `page.indicator_detail.v1` builder for one series, read from stored points.
  Goal: `indicator_detail.New(r, meta, cfg)` returns a `pages.Builder` whose `Build` yields the detail page of one series: the latest value, the points of the last year, a gap for each missing piece.
  Files: internal/pages/indicator_detail/builder.go, internal/pages/indicator_detail/builder_test.go.
  Design: docs/design/pages-builders.md#pages-indicator-detail
  Acceptance:
  - `internal/pages/indicator_detail/builder.go` is package `indicator_detail` and declares `type Config struct` with the string fields `Slug` (the series id and the page slug), `Title`, `SeriesTitle`, `ExplanationTitle` and the field `Explanation []string`, and `func New(r store.SeriesReader, m pages.Meta, c Config) (pages.Builder, error)`; `New` returns an error for a `Meta` field or `Slug` that does not match `^[A-Za-z0-9_-]{1,64}$`, for an empty `Title`, `SeriesTitle` or `ExplanationTitle`, and for more than 20 `Explanation` entries; the file holds `var _ pages.Builder = (*builder)(nil)` (`grep -c 'var _ pages.Builder' internal/pages/indicator_detail/builder.go` prints 1) and `Page()` returns `pages.Page{Schema: "indicator_detail", Version: 1}`.
  - `Build` reads `r.SeriesPoints(ctx, Slug, day minus lookbackDays, day)` with `lookbackDays = 365` and keeps, for each observation date, the row with the greatest `RealtimeStart`; a row whose `Value` is nil is left out of `points` and gives one `no_data` gap that names its date in `text`; `points` are in date order, each with `observed_on`, `vintage_on` (the `RealtimeStart` day), `value`, `text` (the shortest decimal form of the value, `strconv.FormatFloat(v, 'f', -1, 64)`) and `released` (the day of `PublishedAt` in UTC, as a `DateText`).
  - `latest` is the last kept point (`value` with `unit` `series_value`, `observed`, `released`), or a `no_data` gap when there is none; `revision` and `reference_line` are `null`, and one `not_built` gap says that the revision is not built (the step of a revision is a difference, which a builder does not compute); `run_status` is `partial` whenever `gaps` is not empty and `ok` otherwise.
  - `builder.go` never reads a clock (`grep -c 'time.Now' internal/pages/indicator_detail/builder.go` prints 0), imports no package of another sub-package of `internal/pages` (`grep -c 'internal/pages/' internal/pages/indicator_detail/builder.go` prints 0; the root import ends in `pages"`) and imports no `pgx` package (`grep -c 'jackc/pgx' internal/pages/indicator_detail/builder.go` prints 0); `generated_at` follows the convention of `## Interface`, the `stale` label follows the same convention, and with no point the day itself is its `data_time`.
  - `internal/pages/indicator_detail/builder_test.go` holds `TestBuildGolden`, `TestBuildVintages`, `TestBuildGapsAndStale`, `TestBuildErrors` and `TestBuilderIdentity` (`grep -n 'func TestBuildGolden' internal/pages/indicator_detail/builder_test.go` and the four others each find one line); `TestBuildGolden` compiles `contracts/schemas/page.indicator_detail.v1.json` with the jsonschema library as `internal/pages/macro_rates/builder_test.go` does and validates the output of a fake `store.SeriesReader` against it, and `TestBuildGapsAndStale` validates the output of an empty reader and of a nil-value row; no test in the file sleeps or opens a port, and `grep -c 't.Parallel()' internal/pages/indicator_detail/builder_test.go` is at least `grep -c '^func Test' internal/pages/indicator_detail/builder_test.go`.
  - Both new files stay under 600 lines (`wc -l` of each); no file outside the two listed is in the diff.
  - Checked by CI: ci-go. Generated output: the new sub-package `internal/pages/indicator_detail` adds a component to the arch files (`arch.toml`, `.go-arch-lint.yml`), derived from `docs/design/modules.md` by the repo module's script; the regenerated arch files are expected in the pull request and are not part of this entry's Files.
  Depends on: pages-interface. Order: after pages-interface; parallel with the other two builders.
  Size: M

### pages-macro-page

The `page.macro_page.v1` builder: the indicator list from stored series, the ease and the events as named gaps.
  Goal: `macro_page.New(r, meta, cfg)` returns a `pages.Builder` whose `Build` yields the macro page with one indicator row per configured series, filled from stored points, and with the two parts the landed readers cannot fill (the ease figure, a `compute` result; the release calendar of the events, no dataset yet) written as named `no_data` gaps.
  Files: internal/pages/macro_page/builder.go, internal/pages/macro_page/builder_test.go.
  Design: docs/design/pages-builders.md#pages-macro-page
  Acceptance:
  - `internal/pages/macro_page/builder.go` is package `macro_page` and declares `type Indicator struct` with the string fields `SeriesID`, `Name`, `SourceText` and `DetailHref` (empty means `null`), `type Config struct` with the field `Indicators []Indicator`, and `func New(r store.SeriesReader, m pages.Meta, c Config) (pages.Builder, error)`; `New` returns an error for a bad `Meta` field, for an empty `Indicators`, for more than 20 indicators and for an empty `SeriesID` or `Name`; the file holds `var _ pages.Builder = (*builder)(nil)` (`grep -c 'var _ pages.Builder' internal/pages/macro_page/builder.go` prints 1) and `Page()` returns `pages.Page{Schema: "macro_page", Version: 1}`.
  - For each indicator `Build` reads `r.SeriesPoints(ctx, SeriesID, day minus lookbackDays, day)` with `lookbackDays = 30`; `latest` is the last point that has a value, as a `PercentFigure` with `unit` `percent`, `value` the stored value and `text` that value in shortest decimal form followed by `%`, when the series `Unit` is `percent`; a series with another unit gives a `missing_input` gap in `latest`; no point with a value gives a `no_data` gap.
  - `status` is `missing` (label for missing) with no point with a value, `late` when the last such point is more than `freshnessLimitDays = 4` calendar days before `day`, and `good` otherwise, with one fixed label per status; `source_text` is the configured text; `ease` is a `no_data` Gap, `events` is an empty array, `gaps` holds one `no_data` gap for the ease and one for the events, `run_status` is `partial`, and `stale` and `generated_at` follow the conventions of `## Interface`.
  - `builder.go` never reads a clock (`grep -c 'time.Now' internal/pages/macro_page/builder.go` prints 0), imports no package of another sub-package of `internal/pages` (`grep -c 'internal/pages/' internal/pages/macro_page/builder.go` prints 0) and imports no `pgx` package (`grep -c 'jackc/pgx' internal/pages/macro_page/builder.go` prints 0).
  - `internal/pages/macro_page/builder_test.go` holds `TestBuildGolden`, `TestIndicatorStatus`, `TestFixturesValidate`, `TestBuildErrors` and `TestBuilderIdentity` (one `grep -n 'func TestX'` line each); `TestBuildGolden` compiles `contracts/schemas/page.macro_page.v1.json` with the jsonschema library as `internal/pages/macro_rates/builder_test.go` does and validates the output of a fake reader against it; `TestFixturesValidate` validates `web/fixtures/macro-page/ok.json`, `web/fixtures/macro-page/gap.json` and `web/fixtures/macro-page/stale.json` against the same schema and asserts that `web/fixtures/macro-page/bad-version.json` fails it; `TestIndicatorStatus` covers `good`, `late`, `missing` and a non-percent unit in one table; no test sleeps or opens a port, and `grep -c 't.Parallel()' internal/pages/macro_page/builder_test.go` is at least `grep -c '^func Test' internal/pages/macro_page/builder_test.go`.
  - Both new files stay under 600 lines; no file outside the two listed is in the diff.
  - Checked by CI: ci-go. Generated output: the new sub-package `internal/pages/macro_page` adds a component to the arch files (`arch.toml`, `.go-arch-lint.yml`); the regenerated arch files are expected in the pull request and are not part of this entry's Files.
  Note (not an acceptance item): the fixtures carry invented events and an ease, so the builder output does not equal them; the fixtures are checked against the schema only.
  Depends on: pages-interface. Order: after pages-interface; parallel with the other two builders.
  Size: M

### pages-settings-page

The `page.settings_page.v1` builder from fixed configuration: a page with no rows and one named gap.
  Goal: `settings_page.New(meta)` returns a `pages.Builder` whose `Build` yields the settings page that the schema allows when nothing can be listed yet: no row, no form, a `no_reader` gap.
  Files: internal/pages/settings_page/builder.go, internal/pages/settings_page/builder_test.go.
  Design: docs/design/pages-builders.md#pages-settings-page
  Acceptance:
  - `internal/pages/settings_page/builder.go` is package `settings_page` and declares `func New(m pages.Meta) (pages.Builder, error)`, which returns an error for a `Meta` field that does not match `^[A-Za-z0-9_-]{1,64}$`; the file holds `var _ pages.Builder = (*builder)(nil)` (`grep -c 'var _ pages.Builder' internal/pages/settings_page/builder.go` prints 1) and `Page()` returns `pages.Page{Schema: "settings_page", Version: 1}`.
  - `Build(ctx, day, generatedAt)` writes `rows` as an empty array, `form` as `null`, `gap` as a Gap with reason `no_reader`, `data_times` as an empty array, `as_of` as the `day` string, `run_status` `partial`, a fixed `title`, and `generated_at` by the convention of `## Interface`; it returns an error for a `day` that is not `YYYY-MM-DD`; it takes no reader, because nothing landed stores the exclusions, and it never reads a clock (`grep -c 'time.Now' internal/pages/settings_page/builder.go` prints 0).
  - `builder.go` imports no package of another sub-package of `internal/pages` (`grep -c 'internal/pages/' internal/pages/settings_page/builder.go` prints 0) and no `pgx` package (`grep -c 'jackc/pgx' internal/pages/settings_page/builder.go` prints 0).
  - `internal/pages/settings_page/builder_test.go` holds `TestBuildGolden`, `TestBuildErrors` and `TestBuilderIdentity` (one `grep -n 'func TestX'` line each); `TestBuildGolden` compiles `contracts/schemas/page.settings_page.v1.json` with the jsonschema library as `internal/pages/macro_rates/builder_test.go` does, validates the output against it and compares the output with a golden string written in the test; no test sleeps, and `grep -c 't.Parallel()' internal/pages/settings_page/builder_test.go` is at least `grep -c '^func Test' internal/pages/settings_page/builder_test.go`.
  - Both new files stay under 600 lines; no file outside the two listed is in the diff.
  - Checked by CI: ci-go. Generated output: the new sub-package `internal/pages/settings_page` adds a component to the arch files (`arch.toml`, `.go-arch-lint.yml`); the regenerated arch files are expected in the pull request and are not part of this entry's Files.
  Note (not an acceptance item): there is no fixture directory for this page in `web/fixtures`, so the schema is the only external check.
  Depends on: pages-interface. Order: after pages-interface; parallel with the other two builders.
  Size: S

## Left out

Each line names the input that is missing.

- `page.macro_summary.v1`: its card holds only the ease (a figure of `compute`, week 8-9) and the events (a release calendar, no dataset lands it); a builder would write two gaps and nothing else, and the fixtures `web/fixtures/macro-summary` could not be matched. It follows when the ease and the calendar exist.
- The ease and the events of `page.macro_page.v1`: written as gaps by `pages-macro-page`; the figure waits for `compute`, the events for a calendar dataset and a reader in `store`.
- The rows and the form of `page.settings_page.v1`: the exclusion flags need a reader in `store` (a store change) and the form carries a MAC made with a secret; both are outside this cut.
- The revision and the reference line of `page.indicator_detail.v1`: the step of a revision is a difference of two stored values (a computation); the reference line needs a configured value nobody has fixed. Written as `null` and one `not_built` gap.
- `page.market_weather.v1`, `page.market_summary.v1`: quotes and `compute`; no reader of quotes landed.
- `page.positions_page.v1` to `v3`, `page.structures_list.v1`, `page.structure_detail.v1` to `v3`, `page.option_page.v1`, `page.option_result_page.v1`, `page.stock_option_page.v1`, `page.options_summary.v1`: positions, orders and `compute`; the broker datasets and their readers are not landed.
- `page.household_page.v1`, `page.household_summary.v1`, `page.household_risk.v1`, `page.companies_page.v1`, `page.held_companies_summary.v1`: account and holdings datasets and figures of `compute`.
- `page.advice_page.v1`, `page.tracking.v1`, `page.ledger_page.v1`, `page.report_page.v1`, `page.report_page.v2`, `page.full_scan.v1`: closed trades, advice rows and reports from datasets that have no reader.
- `page.home_page.v1`: it embeds the summaries of the pages above; it waits for them.
- A `Builder` adapter for `macro_rates.Build`, constructor function types for the new builders, the registry of pages and the `ot page` subcommand wiring: the adapter would edit `internal/pages/macro_rates`, which no entry here touches, and the wiring is the row `ot`, a follow-up after both this design and the `ot` designs land.
- Builders that read `store.OpsLedger` or `store.SessionDayReader`: no schema of `contracts/schemas` shows run status or calendar data, so no page needs them yet.

## Adversarial

