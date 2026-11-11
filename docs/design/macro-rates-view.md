# Macro rates view: a v2 schema with the texts, the builder that fills it, and the page view

Author: the optdesk designer (draft)
Source: `## Left out` of `docs/design/web-views-wave-2.md` ("`macro_rates`: its payload has no `schema` field") and of
`docs/design/option-page-serve.md` ("a web view for it is its own design"). Read on main at fefd76de1aa2776bd091c12c8c338b183af70649
(every fact below, including the file names and the symbols).

## What main holds today

- `contracts/schemas/page.macro_rates.v1.json` has `schema_version` as the string "1", no `schema` field, and numbers only: a
  series is `{id, unit, points[{date, value|null}]}` and a gap is `{series_id, date, reason}`. It has no title, no text for a value and
  no text for a gap. Its golden is `contracts/fixtures/page.macro_rates.v1.golden.json`, built from `page.macro_rates.v1.input.json`.
- `internal/pages/macro_rates` has `Build(ctx, r store.SeriesReader, day, generatedAt)`, `Page()` (Meta `macro_rates`,
  `page.macro_rates.v1`) and the series list `seriesIDs = ["DGS10"]`; it reads 30 days before `day` and sets a stale gap when the last
  point is more than 4 days before `day`. `cmd/ot` lists it in `pageList` and `ot serve` and `ot page` default to `--name macro_rates`.
- The web shell picks a view by `payload.schema` (`web/src/shell/App.tsx`) and finds registrations by one glob of
  `web/src/pages/*/*.tsx` (`discoverCards` in `web/src/shell/pages.tsx`). No file of `web/` names `macro_rates`. The placeholder
  validator `placeholderValidate` accepts only a numeric `schema_version`. `web/src/charts/Chart.tsx` draws a `ChartSpec` with recharts
  through the shadcn chart component, and `web/src/components/data-table/data-table.tsx` is the tanstack table.
- The web renders data and computes nothing, so a view cannot show v1: it would have to format "4.2" as "4.20%", name the series and
  write the gap sentences itself.

## Decisions

1. A new version, `page.macro_rates.v2`, not an amendment of v1. v1 has a consumer in the repository (its golden, the builder tests and
   `internal/contracts/page_schema_test.go`); changing it in place would break those until the builder lands, and the interface entry
   and the builder entry must be able to run apart. v1 stays on disk, tested, and is retired by never being read (the choice of
   `web-v2-types.md`, Decision 7). The v2 version freezes at its first consumer, the view of this design.
2. v2 follows the envelope of the pages that already have a view: `schema` is the constant `macro_rates`, `schema_version` is the
   integer 2, `Gap` is `common.v2.json#/$defs/Gap`. It leaves out `run_id`, `build_id`, `scope`, `data_times`, `run_status` and `stale`:
   they need a `Config` the builder does not have (`Build` has the fixed `pages.Builder` signature, and `pages.Page` is wired in
   `cmd/ot/wire.go`, not editable here). Staleness is what v1 already says: a `stale` entry of `gaps`, with its text. v2 freezes the English texts, the `title` and the `unit` enum: a second series with its own title or a translation of the texts needs v3. It has no `stale_after`, and it stays outside `pageV2Files` of `internal/contracts` on purpose (that list's convention test requires `stale_after`, which this envelope does not carry); moving it into that line later is a v3.
3. Every visible string is a field. The builder writes English text in the form of this design; the view shows the text and compares
   nothing. The numbers stay for the chart.
4. The view is one page `web/src/pages/macro_rates/view.tsx` that registers itself as `macro_rates`; the shell is not edited.
5. Not in the top bar and not on a site route: `ot serve --name macro_rates` already serves it at the root, and the shell then shows
   the view. A nav item is left out (`## Left out`).
6. No new dependency. recharts and shadcn chart are already in `web/`.

## Modules

No row of `docs/design/modules.md` is added or changed, and no entry edits `arch.toml` or `.go-arch-lint.yml`. Every path of an entry
belongs to one row, by the first glob that matches: the interface entry is `contracts` (`contracts/**`, `internal/contracts/**`,
`web/src/generated/**`, `docs/contracts/page-schemas.md`), the builder entry is `pages` (`internal/pages/**`, `docs/contracts/pages.md`),
the view entry is `web` (`web/**`). The interface entry also edits `web/package.json` (row `web`), so it is a two-module entry, judged here as `pages-types-wave-2` was.

## Interface

Owner: contracts. The schema file `contracts/schemas/page.macro_rates.v2.json`, its generated type and its fixtures. Contracts line:
`page.macro_rates v2 (new)`.

Top level, all required, `additionalProperties: false`:

| field | type |
|---|---|
| `schema` | constant `macro_rates` |
| `schema_version` | constant 2 (integer) |
| `generated_at` | string, date-time |
| `session_day` | string, date |
| `title` | text, 1 to 200 characters |
| `sentence` | text, 1 to 200 characters |
| `series` | list of `Series` |
| `gaps` | list of `GapLine` |

`Series`: `{id: string, title: text, unit: enum percent | ratio | ccy_amount | count, latest: LatestValue or Gap, points: list of Point}`.
`Point`: `{date, value: number or null, text}`. `LatestValue`: `{kind: "value", date, value: number, text}`. `Gap` is the one of
`common.v2.json`. `GapLine`: `{series_id, date, reason: enum no_value | stale, text}`. The generated type is
named `MacroRates` (the schema `title`).

The text the builder writes (the fixtures are the reference, the builder entry makes them true):

- `title` is `Macro rates`; `sentence` is `<n> series on <session_day>.`
- series `title` is `10-year Treasury yield` for the id `DGS10` and the id itself for any other.
- point `text`: unit percent gives two decimals and `%` (`4.10%`), ratio four decimals, ccy_amount two decimals, count none; a null value
  gives `no value`. `latest.text` is the text of that point.
- `latest` is the last point as a `LatestValue` when the series is not stale and that value is not null; a series with no point gives a
  `Gap` with reason `no_data` and text `no points`; a stale series gives reason `stale` and text `latest point <date> is old`; a last
  point with a null value gives reason `no_data` and text `no value on <date>`.
- `GapLine` text: `no value on <date>`; for stale, `no point within 4 days of <date>`.

Fixtures in `contracts/fixtures`, listed in `MANIFEST.synthetic-v1.json`, pinned completely here so the builder's `reflect.DeepEqual`
tests match them without editing `contracts/`. All four have `schema` `macro_rates`, `generated_at` `2000-01-01T00:00:00Z`, `session_day`
`2026-11-04`, `title` `Macro rates`, `sentence` `1 series on 2026-11-04.` and one series `{id: DGS10, title: 10-year Treasury yield,
unit: percent}`. Each is produced by the builder from one reader input (observations of `DGS10`, day 2026-11-04):

- `page.macro_rates.v2.golden.json`: input = `page.macro_rates.v1.input.json` (2026-11-02 4.1, 2026-11-03 null, 2026-11-04 4.2). `points`:
  `{2026-11-02, 4.1, "4.10%"}`, `{2026-11-03, null, "no value"}`, `{2026-11-04, 4.2, "4.20%"}`. `latest`:
  `{kind: value, date: 2026-11-04, value: 4.2, text: "4.20%"}`. `gaps`: `[{series_id: DGS10, date: 2026-11-03, reason: no_value,
  text: "no value on 2026-11-03"}]`.
- `page.macro_rates.v2.gap.json`: input = 2026-11-02 4.1, 2026-11-03 4.2, 2026-11-04 null. `points`: `{2026-11-02, 4.1, "4.10%"}`,
  `{2026-11-03, 4.2, "4.20%"}`, `{2026-11-04, null, "no value"}`. `latest`: `{kind: gap, reason: no_data, text: "no value on 2026-11-04"}`.
  `gaps`: `[{series_id: DGS10, date: 2026-11-04, reason: no_value, text: "no value on 2026-11-04"}]`.
- `page.macro_rates.v2.stale.json`: input = 2026-10-27 4.1, 2026-10-28 4.2. `points`: `{2026-10-27, 4.1, "4.10%"}`,
  `{2026-10-28, 4.2, "4.20%"}`. `latest`: `{kind: gap, reason: stale, text: "latest point 2026-10-28 is old"}`. `gaps`:
  `[{series_id: DGS10, date: 2026-11-04, reason: stale, text: "no point within 4 days of 2026-11-04"}]` (the stale gap sits at the
  session day, as in v1).
- `page.macro_rates.v2.bad-version.json`: the golden with `schema_version` 1.

Invented data only.

## Issues

Which entries run when. The day this design merges: `macro-rates-v2-schema` only. When it merges, `macro-rates-builder` and
`macro-rates-view` start together (disjoint modules; the builder test reads the golden, the view test reads the fixtures, neither reads
the output of the other). One series (`DGS10`) is the scope. End to end in a browser (`ot serve --name macro_rates` and the shell) needs both merged and is the owner's check, not an item.

### macro-rates-v2-schema (interface)

The v2 schema, the `generate:types` line for it, its generated type, the fixtures and the tests that check them. A judged two-module entry (contracts and `web/package.json`), the precedent being `pages-types-wave-2` of `docs/design/web-views-wave-2.md` (Decision 3).
Files: contracts/schemas/page.macro_rates.v2.json, web/package.json, web/src/generated/macro_rates.v2.ts, web/src/generated/index.ts, contracts/fixtures/page.macro_rates.v2.golden.json, contracts/fixtures/page.macro_rates.v2.gap.json, contracts/fixtures/page.macro_rates.v2.stale.json, contracts/fixtures/page.macro_rates.v2.bad-version.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/macrorates_v2_test.go, internal/contracts/generated_ts_test.go, internal/contracts/carryover_test.go, docs/contracts/page-schemas.md
Design: docs/design/macro-rates-view.md#interface
Contracts: page.macro_rates v2 (new)
Depends on: none (merged work only).
Goal: the schema, the generated type and the four fixtures exist and agree; v1 and its fixtures are unchanged.
Acceptance:
1. `contracts/schemas/page.macro_rates.v2.json` has `"$id": "https://optdesk.invalid/schemas/page.macro_rates.v2.json"` (the test loader resolves by it), the eight required top-level fields of `## Interface` in `docs/design/macro-rates-view.md`, `"const": "macro_rates"` for `schema`, `"const": 2` for `schema_version`, `additionalProperties: false` on every object, and refers to `common.v2.json#/$defs/Gap` (`grep -c 'common.v2.json#/\$defs/Gap' contracts/schemas/page.macro_rates.v2.json` prints at least 1); `wc -l` of it is at most 200.
2. `web/src/generated/macro_rates.v2.ts` has the banner `// Code generated from contracts/, DO NOT EDIT.` on line 1 and `export interface MacroRates `; `web/src/generated/index.ts` gains exactly the line `export type { MacroRates } from './macro_rates.v2';`; `internal/contracts/generated_ts_test.go` lists the file and the name in `TestGeneratedSummaryTypes`. `web/package.json` gains the one schema in the script `generate:types`, with the same flags as the other entries (`--cwd ../contracts/schemas` and the two-line banner), and nothing else changes in the file (`git diff` shows added characters in that script line only). Offline proxy for the file: line 1 is the banner, it has `export interface MacroRates `, and its property names are the eight top-level fields of `## Interface`; the byte-for-byte match is under `Checked by CI`.
3. The four fixtures exist; `git diff --stat` does not list `contracts/schemas/page.macro_rates.v1.json`, `contracts/fixtures/page.macro_rates.v1.golden.json` or `contracts/fixtures/page.macro_rates.v1.input.json`; `MANIFEST.synthetic-v1.json` gains the four paths with their sha256 and loses none.
4. `TestMacroRatesV2Fixtures` in `internal/contracts/macrorates_v2_test.go` validates `golden`, `gap` and `stale` against the schema and shows that `bad-version` fails at `schema_version`; it also shows by inline mutation that an unknown reason of a `GapLine`, a missing `text` of a `Point` and an extra top-level field each fail. `golden.json` has the three points and the one `no_value` gap line of `## Interface`.
5. `internal/contracts/carryover_test.go` lists `page.macro_rates.v2.json` in `pageNewFiles` and no other line of it changes; `docs/contracts/page-schemas.md` has a table row for `page.macro_rates.v2` with `2` (an integer), and its count sentence of the schemas is raised by one (stated as one more than before); the count of copies stays 29, since a new v2 schema is not a copy and `contracts/schemas/carryover.sha256` is unchanged. After editing a fixture, the sha256 of `MANIFEST.synthetic-v1.json` is recomputed with `shasum -a 256`. The pull request body carries `Contracts: page.macro_rates v2 (new)`.
6. Every new test calls `t.Parallel()`, opens no network connection and never sleeps; each new file stays under 600 lines; `grep -c "@"` of each fixture prints 0.
7. No file outside the thirteen listed is in the diff, and the result matches the `## Interface` part of `docs/design/macro-rates-view.md`.
Checked by CI: ci-go (`go build ./...`, `go test ./internal/contracts/...`, hygiene scan); ci-web `tsc` and `eslint`; the byte-for-byte match of `web/src/generated/macro_rates.v2.ts` with the output of `generate:types` (the `type drift` step of `ci-web.yml` runs the script and fails on any diff of `web/src/generated`; it runs on this entry's pull request, where the script line and the generated file land together, and again on every later run; json2ts cannot run offline); the pull request body carries `Contracts: page.macro_rates v2 (new)` from the opening; the design verdict (an interface change).
Size: M

### macro-rates-builder

`macro_rates.Build` writes the v2 page
Files: internal/pages/macro_rates/builder.go, internal/pages/macro_rates/text.go, internal/pages/macro_rates/page.go, internal/pages/macro_rates/builder_test.go, internal/pages/macro_rates/page_test.go, docs/contracts/pages.md
Design: docs/design/macro-rates-view.md#interface
Order: after macro-rates-v2-schema
Depends on: macro-rates-v2-schema (merged: the schema and `page.macro_rates.v2.golden.json`).
Goal: with the reader of `page.macro_rates.v1.input.json`, `Build` returns bytes equal to the v2 golden as JSON; the builder keeps its signature.
Acceptance:
1. `Build` keeps the signature `func Build(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error)`; `grep -n 'func Build(' internal/pages/macro_rates/builder.go` shows it, and `git diff --stat` does not list `internal/pages/api.go`.
2. `page.go` returns `Meta{Name: "macro_rates", Schema: "page.macro_rates.v2"}`; `TestPage` in `page_test.go` checks it and that the bytes of `Page().Build` equal those of `Build`.
3. `TestBuildGolden` decodes the output and the file `contracts/fixtures/page.macro_rates.v2.golden.json` and compares them with `reflect.DeepEqual`, and validates the output against `contracts/schemas/page.macro_rates.v2.json` with the jsonschema library already in `go.mod`.
4. `text.go` holds the text of the page (title, sentence, series title, point text per unit, latest and gap text) as in `## Interface`; `TestText` has one table case per unit (percent `4.10%`, ratio four decimals, ccy_amount two decimals, count none) and for each of the gap and latest texts.
5. `TestGapsAndStale` keeps its v1 cases and also checks the v2 `latest` of a stale series, of a series with no point, and of a last point with a null value, each against the text of `## Interface`; `TestBuildFixtures` builds the exact inputs named for `gap.json` and `stale.json` in `## Interface` (day 2026-11-04, `generated_at` 2000-01-01T00:00:00Z) and compares each, by `reflect.DeepEqual`, with `page.macro_rates.v2.stale.json` and `page.macro_rates.v2.gap.json`, so each fixture has one producer; `TestBuildDemo` (it reads the v1 shape) is rewritten to the v2 golden and kept; the series list is still `DGS10` only, and the freshness limit and the 30 day window are unchanged (the two constants of `builder.go` keep their values).
6. `TestBuildErrors` still passes: a bad day and a reader error return an error and no bytes. Every test calls `t.Parallel()` and none sleeps; each new file stays under 600 lines.
7. `docs/contracts/pages.md` names `page.macro_rates.v2` as the output of `macro_rates.Build`; no file outside the six listed is in the diff, and the result matches the `## Interface` part of `docs/design/macro-rates-view.md`.
Checked by CI: ci-go (`go build ./...`, `go test ./internal/pages/...`, golangci-lint, go-arch-lint).
Size: M

### macro-rates-view

The page view of the rates series
Files: web/src/pages/macro_rates/view.tsx, web/src/types/macro_rates.ts, web/test/pages/macro_rates/view.test.tsx
Design: docs/design/macro-rates-view.md#decisions
Order: after macro-rates-v2-schema
Depends on: macro-rates-v2-schema (merged: the generated type `MacroRates`, `index.ts`, and the four fixtures).
Goal: a payload of `page.macro_rates.v2` shows, per series, its title, latest figure or gap text, a line chart of the points and no table under it, and the gap lines, with every string read from the payload.
UI parts (frontend-stack, ADR 0034):
- 按 frontend-stack：用 shadcn `Card`, `CardHeader`, `CardTitle` 做 the page frame and one card per series.
- 按 frontend-stack：用 recharts through shadcn chart (`web/src/charts/Chart.tsx` over `web/src/components/ui/chart.tsx`, `LineChart`) 做 the line of the points of a series by `date`; a null value is not drawn as a zero (the point keeps `value: null`; the line joins its neighbours, as `Chart.tsx` sets `connectNulls` today).
- No table of the points: the series card shows the chart only, no date/value table under it (owner 2026-11-11; landed in #1496). The `text` of a point is the tooltip text of the chart.
- 按 frontend-stack：用 shadcn `Alert` 做 the gap lines and the gap `latest`, and `Badge` 做 the reason of a gap line.
- 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand is not used (no shared state).
Acceptance:
1. `web/src/types/macro_rates.ts` has `SUPPORTED_VERSIONS = [2]`, `export type Payload = MacroRates` imported from `@/generated/macro_rates.v2`, and `validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)`; `grep -c "macro_rates.v1" web/src/types/macro_rates.ts` prints 0.
2. `web/src/pages/macro_rates/view.tsx` exports `registration = { schema: 'macro_rates', page: page<Payload>(validate, MacroRatesView) }`; `git diff --stat` lists no file of `web/src/shell/`.
3. The view shows `payload.title`, `payload.sentence`, for each series its `title`, `latest.text` (a `LatestValue` or a `Gap` alike) and the chart, with no table of the points (the `text` of a point is its tooltip), and for each of `payload.gaps` its `text` and `reason`; it formats no number (`pnpm --dir web run lint` includes the no-compute check, and `grep -n "toFixed\|Intl\.\|toLocale" web/src/pages/macro_rates/view.tsx` prints nothing).
4. The chart spec is plain data: categories are the point `date`s, values are the point `value`s and the tooltip text is the point `text`; a test reads the spec of `golden` and checks that the null point has value `null`.
5. `web/package.json` is not in the diff (the `generate:types` line landed with `macro-rates-v2-schema`).
6. `view.test.tsx` reads `page.macro_rates.v2.golden.json`, `gap.json`, `stale.json` and `bad-version.json` of `contracts/fixtures` with `readFileSync`, renders the first three and checks that every expected string is read from the fixture (title, sentence, series title, latest text, a point text, a gap text), and checks that `validate` refuses `bad-version` with reason `unsupported_version`. It stubs `ResizeObserver`, opens no port and sleeps never.
7. Each new file stays under 600 lines; no file outside the three listed is in the diff; the result matches the `## Decisions` part of `docs/design/macro-rates-view.md` (items 3 to 6).
Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck`, `lint`, `test` and `build`; hygiene.
Size: M

## Left out

- Amending v1 in place ("amended before first consumer"): not chosen, Decision 1. It would put the schema, the golden and the builder in one pull request or break the v1 tests between two; the cost of v2 is a second schema file on disk.
- A `Config` for the builder (`run_id`, `build_id`, `scope`, `data_times`, `run_status`, the stale label of `common.v2`): needs `Page()` to take arguments and `cmd/ot/wire.go` to change (an `ot` entry). The page says its staleness in `gaps`, as v1 did.
- More series than `DGS10` (the 2s10s spread, DGS2, a daily change): `seriesIDs` is one entry because the collection stores that one today; a change or spread is a compute figure, and the view and the builder compute nothing. Adding an id is one string and a title in `text.go`.
- A nav item and a site route for the page: `ot serve --site` has a fixed route table (`option-page-serve.md`) and the top bar belongs to the shell; one route and one item are a later cut.
- Deleting `page.macro_rates.v1.json`, its fixtures and `BuildMacroRatesFunc`'s v1 comment in `internal/pages/api.go`: `api.go` is the interface file of pages, and its doc line still says v1 after this lands; fixing it is an interface change of its own, reported rather than edited here.
- Localised text (Chinese): the fixtures of the pages built in Go are English; a language switch is not designed anywhere yet.

## Adversarial

1. Gap against the goal: not changed. One series is the scope; the browser check is the owner's (said in `## Issues`).
2. Biggest unnamed risk (generated type not checkable offline, fixtures with no producer): adopted. The byte-for-byte check is under `Checked by CI` (the existing `type drift` step of `ci-web.yml`); `TestBuildFixtures` produces the `.gap.json` and `.stale.json` fixtures.
3. A simpler way: adopted in part. The `missing` and `source_error` reasons of `GapLine` are cut (the builder never emits them); `latest` stays as it is.
4. Lock-in: adopted. Decision 2 says v2 freezes the texts, title and unit enum, drops `stale_after` and sits outside `pageV2Files`.
5. Claim most likely false: adopted. The null-value wording follows `connectNulls`; the schema `$id` is in interface acceptance 1; the count in `page-schemas.md` is relative.
6. Likeliest slip: adopted. `TestBuildDemo` is named in the builder acceptance (moves to the v2 golden); the sha256 step is stated.

Source: `macro-rates-view.adversarial.md` in the dispatcher's scratchpad.
7. The design session's points (the fixtures pinned completely, the `generate:types` line moved into the interface entry as a two-module entry, the json2ts item under `Checked by CI` with an offline proxy) were applied.