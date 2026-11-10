# Pages: the macro_page builder

Author: advisor of optdesk (first draft, revision 2 after the adversarial review)
Source: `## Left out` of `docs/design/week1.md` (builders for other pages, the `Page` and `Builder` names). Identifiers
were read on main: `internal/pages/api.go` holds `BuildMacroRatesFunc` and `NewHandlerFunc`; `internal/pages/macro_rates`
is the one builder; `store.SeriesReader` is `SeriesPoints(ctx, seriesID string, from, to contracts.Day)`.

Goal of the design: one more page the existing front end renders, filled from the series reader, built as a plain
function shaped like `macro_rates.Build`. `page.macro_page.v1` is the only schema that qualifies (see `## Left out`).
The page is `partial` for good: its ease is a figure of `compute` (week 8-9, not landed) and its events need a release
calendar that no dataset holds, so both are named `no_data` gaps. `run_status` is `partial` on every build, and that is
the expected status, not a defect.

## Modules

No change to `docs/design/modules.md`: every source path below belongs to the row `pages` (`internal/pages/**`). The row's
depends cell is `contracts store`; the builder reads only `store.SeriesReader`. The new package is a new component of
the arch files, which `scripts/arch` regenerates from `modules.md` (`checks.yml` runs `go run ./scripts/arch --check`);
the two regenerated files belong to the repo module and are the generated output of the entry (see its Generated output
note).

| issue | module |
|---|---|
| pages-macro-page | pages |

No entry edits `go.mod`, `go.sum`, `contracts/`, `web/` or `docs/design/modules.md`. The jsonschema library of the tests
(`github.com/santhosh-tekuri/jsonschema/v6`) is already in `go.mod`; no new direct dependency.

## Interface

No interface change: `internal/pages/api.go` is not edited and no entry is marked (interface). The `Page`, `Meta` and
`Builder` names are not frozen (see `## Left out`). The builder is a function of the package `macro_page`, like
`macro_rates.Build`; it is not wired to any command here.

Names used as they are: `store.SeriesReader`, `contracts.Day`, `contracts.SeriesPoint` (`ObservationDate`,
`RealtimeStart`, `Value` as a pointer, `Unit`, `PublishedAt`).

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the
diff, with an offline grep where it is about behaviour.

### pages-macro-page

The `page.macro_page.v1` builder: the indicator rows from stored series, the ease and the events as named gaps.
  Goal: `macro_page.Build(ctx, r, c, day, generatedAt)` returns the JSON of `contracts/schemas/page.macro_page.v1.json`, with one indicator row per configured series filled from stored points.
  Files: internal/pages/macro_page/builder.go, internal/pages/macro_page/builder_test.go.
  Design: docs/design/pages-builders.md#pages-macro-page
  Required fields of the schema and where each value comes from (the acceptance items refer to this table):

  | field | value |
  |---|---|
  | `schema`, `schema_version` | the string `macro_page`; the integer 1 |
  | `run_id`, `build_id`, `scope` | `Config.RunID`, `Config.BuildID`, `Config.Scope`, each matching `^[A-Za-z0-9_-]{1,64}$` |
  | `generated_at` | `generatedAt` converted with `Config.Zone` and written in the form the schema pattern requires (the offset the pattern requires; the zone comes from configuration, never a literal in the code) |
  | `data_times` | one `{source, data_time}` per indicator that has a value: `source` is its `SeriesID`, `data_time` the day of its latest valued observation at 00:00:00 in `Config.Zone`; empty when no indicator has a value |
  | `run_status` | `partial`, always |
  | `title`, `sentence`, `ease_title`, `events_title`, `indicators_title`, `notes_title` | fixed Chinese constants of the package (the language of the fixtures), 1 to 200 characters each: `宏观`, `宏观指标来自已存储的序列；松紧度和事件暂无数据。`, `松紧度`, `本周与下周`, `指标`, `说明` |
  | `notes` | the fixed list `日期来自 FRED 发布日历和央行日程，只有日期，没有发布时间。` and `指标取自序列最新存储的一期；没有存储的标为暂缺。` |
  | `ease` | the Gap `{reason: no_data, text: 松紧度暂无数据}` |
  | `events` | an empty array |
  | `indicators` | one row per configured indicator, in configuration order (rules below) |
  | `gaps` | exactly two Gaps, `no_data`: the ease (`松紧度暂无数据`) and the events (`事件日历暂无数据`) |
  | `stale` | rule below |

  Acceptance:
  - `internal/pages/macro_page/builder.go` is package `macro_page` and declares `type Indicator struct` with the string fields `SeriesID`, `Name`, `SourceText` and `DetailHref` (empty means `null`), `type Config struct` with the string fields `RunID`, `BuildID`, `Scope`, the field `Zone *time.Location` and the field `Indicators []Indicator`, and `func Build(ctx context.Context, r store.SeriesReader, c Config, day contracts.Day, generatedAt time.Time) ([]byte, error)`; the shape of the call is that of `macro_rates.Build` plus `c Config`.
  - `Build` returns an error for a `day` that is not `YYYY-MM-DD`, a nil `Zone`, a `RunID`, `BuildID` or `Scope` that does not match `^[A-Za-z0-9_-]{1,64}$`, an empty `Indicators`, more than 20 indicators, and an empty `SeriesID` or `Name`; `builder_test.go` holds `TestBuildErrors` with one case for each.
  - For each indicator `Build` reads `r.SeriesPoints(ctx, SeriesID, day minus lookbackDays, day)` with `lookbackDays = 30`, drops every row whose `RealtimeStart` is after `day`, and keeps for each observation date the remaining row with the greatest `RealtimeStart`; `latest` is the kept point with the latest observation date whose `Value` is not nil.
  - `latest` is a `PercentFigure` with `unit` `percent`, `value` the stored value and `text` the shortest decimal form (`strconv.FormatFloat(v, 'f', -1, 64)`) followed by `%`, when the row `Unit` is `percent`; a row with another unit gives a `missing_input` Gap in `latest` (the FRED client stores every series as `percent` today, so the guard does not fire on stored data); no valued point gives a `no_data` Gap. `status` is `missing` with no valued point, `late` when the observation day of `latest` is more than `freshnessLimitDays = 4` calendar days before `day` (date arithmetic on the two `YYYY-MM-DD` values, as in `macro_rates`), and `good` otherwise; `label` is `缺`, `迟到` or `及时` for them; `name`, `source_text` and `detail_href` are the configured values.
  - `stale`: the oldest shown observation is the earliest of the observation days of the `latest` of the indicators that have a valued point; `data_time` is that day at 00:00:00 in `Config.Zone`, `stale_after` is that day plus 4 calendar days at 09:30:00 in `Config.Zone`, and `text` is `数据日 MM-DD` of that day; when no indicator has a valued point, `data_time` is `day` at 00:00:00, `stale_after` is `day` plus 4 days at 09:30:00, and `text` is `没有数据日`.
  - Every other field takes its value from the table above; `Build` never reads a clock (`grep -c 'time.Now' internal/pages/macro_page/builder.go` prints 0), never builds a zone from a literal (`grep -c 'FixedZone' internal/pages/macro_page/builder.go` prints 0 and `grep -c 'LoadLocation' internal/pages/macro_page/builder.go` prints 0), imports no package of another sub-package of `internal/pages` (`grep -c 'internal/pages/' internal/pages/macro_page/builder.go` prints 0) and no `pgx` package (`grep -c 'jackc/pgx' internal/pages/macro_page/builder.go` prints 0).
  - `builder_test.go` holds `TestBuildGolden`, `TestIndicatorStatus`, `TestVintageBound`, `TestStaleLabel`, `TestFixturesValidate` and `TestBuildErrors` (one `grep -n 'func TestX'` line each). `TestBuildGolden` compiles `contracts/schemas/page.macro_page.v1.json` with the jsonschema library as `internal/pages/macro_rates/builder_test.go` does and validates the output of a fake `store.SeriesReader` against it, and compares it with a golden string written in the test; `TestIndicatorStatus` covers `good`, `late`, `missing` and a non-percent unit in one table; `TestVintageBound` feeds two rows of one date, one with a `RealtimeStart` after `day`, and asserts that the later one is ignored; `TestStaleLabel` covers one indicator, two with different days, and none; `TestFixturesValidate` validates `web/fixtures/macro-page/ok.json`, `web/fixtures/macro-page/gap.json` and `web/fixtures/macro-page/stale.json` against the schema and asserts that `web/fixtures/macro-page/bad-version.json` fails it. No test sleeps or opens a port, and `grep -c 't.Parallel()' internal/pages/macro_page/builder_test.go` is at least `grep -c '^func Test' internal/pages/macro_page/builder_test.go`.
  - The two new source files stay under 600 lines (`wc -l` of each). The diff holds the four listed files and no other; `arch.toml` and `.go-arch-lint.yml` are the only files outside `internal/pages/macro_page` and are exactly the output of `go run ./scripts/arch` (no hand edit).
  - Checked by CI: ci-go (build, vet, tests, lint); `go run ./scripts/arch --check` in `checks.yml` passes.
  Generated output: the new sub-package `internal/pages/macro_page` makes `scripts/arch` add a component to `arch.toml` and `.go-arch-lint.yml`; the entry carries the regenerated files and no hand edit.
  Note (not an acceptance item): the fixtures carry invented events and an ease, so the builder output does not equal them; they are checked against the schema only. No render check of the builder output by the front end is included.
  Depends on: none. Order: any.
  Size: M

## Left out

Each line names the reason or the missing input.

- The `Page`, `Meta` and `Builder` names of `pages`: no registry and no caller of such an interface exists (`cmd/ot` has none), so a freeze would buy nothing and could be wrong when a registry comes; the first design that adds a registry freezes them.
- `page.indicator_detail.v1`: no renderer for it exists in `web/src`; its revision step is also a difference of two stored values (a computation). It follows a renderer and a link from a series to its detail.
- `page.settings_page.v1`: no renderer in `web/src`; its rows need a `store` reader of the option exclusions (a store change) and its form a MAC made with a secret.
- `page.macro_summary.v1`: its card is only the ease (`compute`, week 8-9) and the events (no calendar dataset); a builder would write two gaps and nothing else.
- The ease and the events of `page.macro_page.v1`: named gaps in the entry; the figure waits for `compute`, the events for a calendar dataset and its reader.
- `page.market_weather.v1`, `page.market_summary.v1`: quotes and `compute`; no quote reader landed.
- The position, structure, option, household, company, advice, tracking, ledger, report, full-scan pages and `page.home_page.v1`: account, broker and report datasets and figures of `compute`; none has a reader.
- An adapter making `macro_rates.Build` a builder value, the wiring of this builder into `ot page`, and `detail_href` routes: the row `ot` and `web`, after this design.
- A units check of stored series: the FRED client stores every series as `percent`; a follow-up in `sources` is needed before a series of another unit is configured.

## Adversarial

Source: `pages-builders.adversarial.md`, kept on the dispatcher's machine.

1. Gap against the goal: adopted; only `macro_page` has a renderer, so the cut is that one page, `partial` is stated as permanent, and the missing renderers are in `## Left out`.
2. Unspecified required fields: adopted; the table lists every required field of `page.macro_page.v1.json` with its source and the fixed texts.
3. A simpler way: adopted; one plain function like `macro_rates.Build`, no `Page`, `Meta` or `Builder`, and `indicator_detail` and `settings_page` deferred.
4. Lock-in: adopted; nothing is frozen, so no interface needs to be reversed; the copied constants are those of one package only.
5. Arch files: adopted; the entry lists the two regenerated files, the Generated output note and a no-other-file item, and the `modules.md` claim is limited to the row, not the arch output.
6. Likeliest slip: adopted; exact stale formula and strings, the zone taken from configuration, `released` dropped (the page has no such field), vintage bounded by `RealtimeStart <= day`, `partial` named; not adopted: a front-end render check, which is a `web` change, and the note says none is included.
