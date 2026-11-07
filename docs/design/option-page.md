# Option data page: `option_data_page` shows the stored option data like `macro_page` shows macro data

Author: the optdesk dispatcher (revision 2, after the adversarial review)
Owner approved: (pending)
Revision: 2

Source: owner request of 2026-11-07 (optdesk-485, raise concurrency): a page for the stored option data (`obs.option_eod`,
`obs.quote_snapshot`), with a JSON page schema, a Go builder in the pages registry, `ot page --name <page>`, generated TS types and a
web view on recorded or synthetic fixtures. Added constraint of the same day: the front end is built in parallel with collection.
Read on main at 0c3d9e6 (the one sha every file:line below is read at).

Goal: one page `option_data_page` (schema `page.option_data_page.v1`) that renders the rows the two stores hold, built by Go, drawn by
the web from fixtures alone. The name is not `option_page`: that name is a carried schema (`contracts/schemas/page.option_page.v1.json`)
and a web registration (`web/src/shell/pages.tsx:69`) for another page. The `ot` wiring is a follow-up cut (`## Left out`).

## What main holds today

- Readers: `OptionEodReader.OptionEod(ctx, underlyingID int64, from, to contracts.Day)` ordered by session_day, instrument_id, capture_id
  (`internal/store/api.go:36-40`) and `QuoteSnapshotReader.QuoteSnapshots(ctx, instrumentID int64, from, to)` ordered by session_day,
  slot, capture_id (`internal/store/api.go:47-51`), implemented over the views `rd.option_eod_r1` and `rd.quote_snapshot_r1`
  (`internal/store/snapshot/optioneod.go:30-37`); the page role `ot_ro_pages` has `SELECT` on both views
  (`migrations/0006_option_eod.sql:99-100`, `migrations/0007_quote_snapshot.sql:69-70`).
- The records are `contracts.OptionEod` and `contracts.QuoteSnapshot` (`internal/contracts/api.go:16-20`); prices, sizes, `iv_*_decimal`,
  `delta`, `gamma`, `theta_per_calendar_day`, `vega_per_vol_point` and `underlying_price` are nullable
  (`contracts/datasets/obs.option_eod.r1.json`). No record holds strike, expiry, right or a symbol; `instrument_id` is a plain bigint
  (`docs/design/store-week2.md:74`) and the master tables are left out of that design (`store-week2.md:355`).
- `pages.Builder` takes a `store.SeriesReader` (`internal/pages/api.go:29`); `pages.Page` is `Meta` plus `Build` (`api.go:32-35`). A page
  closes over its own configuration in its adapter (`docs/design/pages-registry.md`, Left out, the line on a `Builder` with a config
  parameter), as `macro_page.NewPage(c)` does (`internal/pages/macro_page/page.go:15-23`). So this page puts its readers in its
  `Config`; the series reader the caller passes is not used.
- `ot page` opens a series store with `OT_PG_DSN_PAGES` (`cmd/ot/page.go:19-36`); `deps` has no opener of the option stores
  (`cmd/ot/wire.go:29-40`). `internal/store/snapshot` is on the import list of `ot` (`docs/design/modules.md:44`).
- Web: pages are registered in `web/src/shell/pages.tsx:60-66`; `MacroPage.tsx` draws from `DataTable`
  (`web/src/components/data-table/data-table.tsx`) and `ui/card`, `ui/alert`; `ui/chart.tsx` exists. Generated types come from
  `pnpm run generate:types` (`web/package.json:11`) and CI fails on drift of `web/src/generated` (`.github/workflows/ci-web.yml:56-58`).
- Contract tests that every new page file meets: `carriedFiles` treats any `page.*.json` not in `page.macro_rates.v1.json`, `pageV2Files`
  or `pageV3Files` as carried and demands a line in `carryover.sha256` (`internal/contracts/carryover_test.go:76-90`);
  `TestGeneratedSummaryTypes` requires `web/src/generated/index.ts` to hold exactly as many `export` lines as its `files` slice
  (`internal/contracts/generated_ts_test.go:17-60`); the hygiene scan wants every file of `contracts/fixtures/` in
  `MANIFEST.synthetic-v1.json` with its sha256 (`contracts/fixtures/MANIFEST.synthetic-v1.json`, `scripts/hygiene/main.go:22`).
- Size: json2ts turns a `maxItems` into one tuple line per length (`web/src/generated/market_summary.v3.ts:26-50`; that file is 582
  lines), so this schema sets no `maxItems` except `quote_series`.

## Modules

| issue | module |
|---|---|
| option-page-schema (interface) | contracts, with web (one line of `web/package.json`) |
| option-page-web | web |
| option-page-builder | pages |

`option-page-schema` is a licensed cross-module entry (contracts and web): the generated file belongs to contracts and the command that
makes it to web (`web/package.json:11`, a path of the web row), so the one script line travels with the file it produces and the CI drift
step is true from the first pull request. The two entries after it each edit one module.

Text change of `docs/design/modules.md` in this design's own diff: the implementation list of the row `ot` (`modules.md:44-45`) gains
`internal/pages/option_data_page`, after `internal/pages/macro_page`, so the builder package is a component of the arch files and the
follow-up `ot` cut needs no text change. Generated in the same diff, never by hand: `arch.toml` and `.go-arch-lint.yml`
(`go run ./scripts/arch`; `go run ./scripts/arch --check` exits 0). No row, path, entry, tests or depends cell changes. No new Go or web
dependency.

Fixtures are in `contracts/fixtures/` (contracts), not `web/fixtures`, so the fixture map of `internal/contracts/fixtures_test.go:16-31`
is untouched; the web tests read them from there.

## Interface

Owner: contracts. New file `contracts/schemas/page.option_data_page.v1.json`, `$id`
`https://optdesk.invalid/schemas/page.option_data_page.v1.json`, `$ref`s to `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `Gap`,
`SourceTime`, `StaleLabel` (no local def of those names), `additionalProperties: false`, `schema_version` `{"const": 1}`, at most 400
lines. Contracts line: `page.option_data_page v1 (new)`. Top level, all required unless marked:

| field | type |
|---|---|
| `schema` | const `option_data_page` |
| `schema_version` | const 1 |
| `run_id`, `build_id`, `scope` | `Id` |
| `generated_at` | `Timestamp` |
| `data_times` | list of `SourceTime` (sources `option_eod`, `quote_snapshot`) |
| `run_status` | `RunStatus` |
| `stale` | `StaleLabel` or null |
| `title`, `sentence`, `chain_title`, `series_title`, `notes_title` | text, 1 to 200 characters |
| `notes`, `gaps` | list of text, list of `Gap` |
| `underlying` | `Underlying` |
| `chain` | `Chain` |
| `quote_series` | list of `QuoteSeries`, `maxItems` 3 |
| `stale_after` | optional, `Timestamp` |

Local defs. `Figure` is `{kind: const "value", value: number, text: text, unit: text}` and `Cell` is `Figure` or `Gap` (the shared
convention: `Gap` already has `kind: "gap"`). `Underlying` is `{id: integer, symbol: text or null, label: text, session_day: date or
null, price: Cell, currency}`; `price` is the `underlying_price` of the chosen row of the day (a `Gap` when there is none or it is
null). `Chain` is `{session_day: date or null, rows: list of ChainRow}`. `ChainRow` is `{instrument_id: integer, expiry: date or null,
right: "call" or "put" or null, strike: Cell, status: {code: 1 to 5, text}, bid, ask, last, volume, open_interest, iv, delta, gamma,
theta, vega}` (cells); `iv` is the stored `iv_decimal`, `theta` the stored `theta_per_calendar_day`, `vega` the stored
`vega_per_vol_point`. `QuoteSeries` is `{instrument_id, caption: text, points: list of QuotePoint}`; `QuotePoint` is `{session_day, slot: text,
bid, ask, last}` (cells). `symbol`, `expiry`, `right` and `strike` are in v1 so
that the collection design's contract-terms reader fills them without a new version; until it exists the builder writes null for the
three and a `Gap` (`no_data`) for `strike`.

Sizes are the builder's, not the schema's: at most 500 rows, 200 points per series, 20 gaps and 20 notes, each enforced by the builder
and its tests.

Formatting rule (one text for the fixtures, the builder and the web; the web never formats): `Figure.value` is the stored number.
`unit` and `text` by cell: prices (`bid`, `ask`, `last`, `strike`, `underlying.price`) unit `price`, text with 2 decimals; `bid_size`
and `ask_size` are not shown; `volume`, `open_interest` unit `count`, 0 decimals; `iv` unit `iv_decimal`, text the value times 100 with
1 decimal and a percent sign (display scaling only); `delta`, `gamma`, `theta`, `vega` units of the same names, 4 decimals. Status text
by code: 1 `ok`, 2 `no quote`, 3 `not listed`, 4 `stale`, 5 `error`. `slot` is the stored slot string. The five fixed texts of the page
are the literals of `ok.json`, which the builder copies: `title` `Option data`, `chain_title` `Option chain, last stored day`,
`series_title` `Quotes by slot`, `notes_title` `Notes`, and `sentence` `<n> contracts on <session_day>.` with n the row count as a
number in text.

Sample inputs: `contracts/fixtures/page.option_data_page.v1.input.json` holds the day, `run_id`, `build_id`, `scope`, `generated_at`,
the underlying id, the eod records (with two captures of one instrument) and the quote records of four instruments; `ok.json` is the page
the rule above makes from it (3 series, strike gaps, symbol null). The builder golden decodes both JSON texts and compares the decoded
values (key order and indentation do not matter).

## Issues

Which entries run when. The day this design merges: `option-page-schema` only. As soon as it merges, `option-page-web` and
`option-page-builder` start together (width 2, disjoint files, each buildable from the interface alone). The web view is built and tested
on the fixtures and never waits for the builder or for collection. The builder reads only the two readers that exist today (cited above),
so it does not wait for collection; what waits for collection is real rows in the stores and the terms, not any entry here.

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff, with an
offline grep or a named test.

### option-page-schema (interface)

The page schema, its generated TS type, sample input, fixtures and the contract tests
  Goal: `page.option_data_page.v1` exists with its generated type and fixtures, and every existing contract test stays green.
  Files: contracts/schemas/page.option_data_page.v1.json, web/src/generated/option_data_page.v1.ts, web/src/generated/index.ts, web/package.json, contracts/fixtures/page.option_data_page.v1.input.json, contracts/fixtures/page.option_data_page.v1.ok.json, contracts/fixtures/page.option_data_page.v1.terms.json, contracts/fixtures/page.option_data_page.v1.gap.json, contracts/fixtures/page.option_data_page.v1.stale.json, contracts/fixtures/page.option_data_page.v1.bad-version.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/optiondatapage_test.go, internal/contracts/carryover_test.go, internal/contracts/generated_ts_test.go, docs/contracts/page-schemas.md
  Design: docs/design/option-page.md#interface
  Contracts: page.option_data_page v1 (new)
  Acceptance:
  1. The schema has the fields and defs of `## Interface`, `schema_version` const 1, `additionalProperties: false` on every object, the six shared defs by `$ref` (offline: `grep -c 'common.v2.json#/\$defs/' contracts/schemas/page.option_data_page.v1.json` prints at least 6), no `maxItems` except on `quote_series` (`grep -c maxItems` prints 1), and `wc -l` of the schema is at most 400.
  2. `web/package.json` `generate:types` gains one `json2ts` call for the schema with the banner of the other calls; `git diff web/package.json` shows that script line only and `pnpm-lock.yaml` is unchanged.
  3. `web/src/generated/option_data_page.v1.ts` is the output of `pnpm --dir web run generate:types` (banner `// Code generated from contracts/, DO NOT EDIT.`); `wc -l` of it prints at most 400; `index.ts` exports `OptionDataPage` from it.
  4. `generated_ts_test.go` lists the third file, so `TestGeneratedSummaryTypes` expects three export lines and passes; `carryover_test.go` skips the new schema in `carriedFiles` by a named list (`pageNewFiles`), so `TestCarryoverManifest` passes and `contracts/schemas/carryover.sha256` is unchanged.
  5. `ok.json` holds 6 or more rows with unique `instrument_id`, one row of status 2 with `Gap` cells, one row with a null `delta` as a `Gap`, and 3 series; `terms.json` is the same page with `symbol`, `expiry`, `right` and `strike` filled; `gap.json` has empty `rows`, empty `quote_series` and a `Gap` price; `stale.json` has a non-null `stale`; `bad-version.json` has `schema_version` 2. Instrument ids are 9000 and above, no symbol of a real company, no account id.
  6. `TestOptionDataPageFixtures` (`t.Parallel()`, reads only the schema and the files, uses the compile helper of `common2_test.go`) validates `ok`, `terms`, `gap` and `stale`, and expects the one failure of `bad-version` at `/schema_version` with keyword `const`; `MANIFEST.synthetic-v1.json` lists the six new files with the sha256 of each (the hygiene scan passes).
  7. `docs/contracts/page-schemas.md` lists the schema and the page name `option_data_page` and says it is not `option_page`.
  8. No file outside the fifteen listed is in the diff; `git diff --stat` shows `internal/pages`, `cmd/`, `docs/design/` and `web/src` other than the two generated files absent.
  Checked by CI: ci-go (`go test ./internal/contracts/...`, hygiene scan), ci-web `tsc`, `eslint` and the type drift step; the PR body carries `Contracts: page.option_data_page v1 (new)` from the opening; the design verdict (an interface change).
  Depends on: none (merged work only).
  Size: L

### option-page-web

The web view of `option_data_page`, built and tested on the fixtures
  Goal: the shell shows the page for `schema: option_data_page` from the fixtures, with no store reader and no computation.
  Files: web/src/pages/optiondata/OptionDataPage.tsx, web/src/pages/optiondata/ChainTable.tsx, web/src/pages/optiondata/QuoteChart.tsx, web/src/types/option_data_page.ts, web/src/shell/pages.tsx, web/test/pages/optiondata/OptionDataPage.test.tsx, web/test/shell/pages.test.tsx
  Design: docs/design/option-page.md#interface
  UI parts (frontend-stack, ADR 0034):
  - 按 frontend-stack：用 shadcn `Card`, `CardHeader`, `CardTitle` 做 the page frame and the `underlying` block (symbol or label, price cell, session day).
  - 按 frontend-stack：用 the shadcn data-table pattern (`web/src/components/data-table/data-table.tsx`, @tanstack/react-table) 做 the chain table, one column per `ChainRow` cell, sorting on demand only.
  - 按 frontend-stack：用 recharts through shadcn chart (`web/src/components/ui/chart.tsx`, `ChartContainer`, `LineChart`) 做 the bid, ask and last lines of a `quote_series` by `slot`.
  - 按 frontend-stack：用 shadcn `Alert` 做 the stale label and the page gaps, and `Badge` 做 the row status text.
  - 按 frontend-stack：用 lucide-react 做 the icons of gap and stale states, and shadcn `Tabs` 做 the switch between quote series.
  - 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand is not used (no shared state).
  Acceptance:
  1. `OptionDataPage` shows only strings from the payload (`title`, `sentence`, `chain_title`, each cell `text`, `notes`); offline: `grep -n -E 'toFixed|toLocale|Intl\.|Math\.|\.length|\.sort\(' web/src/pages/optiondata/*.tsx` prints nothing, and `web/no-compute-allow.txt` is unchanged.
  2. `web/src/types/option_data_page.ts` re-exports `OptionDataPage` from `@/generated/option_data_page.v1` as `Payload` and exports `validate`, which accepts `schema_version` 1 only and returns the gap result of `web/src/types/placeholder.ts` otherwise; `web/src/shell/pages.tsx` registers `option_data_page` in `explicit`.
  3. `OptionDataPage.test.tsx` reads the fixtures from `contracts/fixtures/` and checks: `ok` shows 6 or more rows and 3 series; the status-2 row and the null-delta cell show the gap text, never `0`; `terms` shows the strike, expiry and right text; `gap` shows the empty-table message and no chart; `stale` shows the stale text; `bad-version` is rejected by `validate`; `pages['option_data_page']` is this component.
  4. No test uses network or clock or sleeps (`grep -c 'setTimeout\|sleep' web/test/pages/optiondata/OptionDataPage.test.tsx` prints 0); each new file stays under 600 lines; no dependency is added (`git diff` shows no `web/package.json` or `pnpm-lock.yaml`).
  5. No file outside the seven listed is in the diff.
  Checked by CI: ci-web `tsc`, `eslint`, `vitest`, the build, and the type drift step.
  Depends on: option-page-schema (merged).
  Size: L

### option-page-builder

The Go builder and the page value, over the two readers of today
  Goal: `option_data_page.NewPage(Config)` returns a `pages.Page` whose JSON validates against the schema, built from `store.OptionEodReader` and `store.QuoteSnapshotReader` only.
  Files: internal/pages/option_data_page/builder.go, internal/pages/option_data_page/format.go, internal/pages/option_data_page/page.go, internal/pages/option_data_page/builder_test.go, internal/pages/option_data_page/page_test.go, docs/contracts/pages.md
  Design: docs/design/option-page.md#interface
  Behaviour (selection and formatting by the rule of `## Interface`, no arithmetic on prices): `Config` is `RunID`, `BuildID`, `Scope` (each `^[A-Za-z0-9_-]{1,64}$`), `Zone *time.Location` (nil is an error), `UnderlyingID int64`, `Label string`, `Eod store.OptionEodReader`, `Quotes store.QuoteSnapshotReader`. `Build(ctx, r, day, generatedAt)` ignores `r`; reads `Eod.OptionEod(UnderlyingID, day-7, day)`; takes the latest `session_day` with rows; per instrument keeps the row with the greatest `received_at` (a tie keeps the later in reader order); `underlying.price` is the `underlying_price` of the row with the greatest `received_at` of that day; orders rows by `open_interest` descending (null last), then `instrument_id`, and keeps at most 500; for the first three instruments reads `Quotes.QuoteSnapshots(id, day-7, day)` and keeps at most 200 points in reader order. `symbol`, `expiry` and `right` are null and `strike` is a `Gap` `no_data`, until a terms reader exists. A null stored value is a `Gap` cell with reason `no_data`. An empty eod read gives empty `rows`, a page gap and `run_status` `partial`; a reader error is returned, never a half page.
  Acceptance:
  1. `NewPage(c)` copies `c`, returns `Meta{Name: "option_data_page", Schema: "page.option_data_page.v1"}`; among `internal/...` imports only `internal/pages`, `internal/contracts` and `internal/store` appear; no zone literal and no `pgx` under the directory (grep).
  2. `TestBuildValidates` (fake readers, fixed `generatedAt`, `time.UTC`): the output validates against the schema with `common.v2.json` loaded as a resource.
  3. `TestBuildGolden`: the records of `contracts/fixtures/page.option_data_page.v1.input.json`, given through fake readers, build a page whose decoded JSON equals the decoded `ok.json`; the test never writes a file.
  4. `TestBuildSelection`: two captures of one instrument keep the later `received_at`; rows order by open interest, nulls last; three series for four instruments; the fake sees `day-7` and `day` and nothing else; 600 eod rows give 500 rows, 300 quote rows give 200 points, 30 gaps give 20.
  5. `TestBuildGaps`: a null bid, a null delta and status 2 give `Gap` cells and no zero; an empty eod read gives `partial`, a non-empty `gaps` and zero quote reads; a reader error returns no bytes.
  6. Every test calls `t.Parallel()`, with no network, store, clock or sleep (`grep -c 'time.Sleep\|time.Now' internal/pages/option_data_page/*.go` prints 0); each new file stays under 600 lines; `docs/contracts/pages.md` names the package, its Config and the tests (Version history line).
  7. No file outside the six listed is in the diff; `go run ./scripts/arch --check` exits 0.
  Checked by CI: ci-go (`go build ./...`, `go test ./internal/pages/...`, golangci-lint, go-arch-lint).
  Depends on: option-page-schema (merged); this design merged (arch files).
  Size: L

## Left out

- The `ot` wiring (a follow-up cut that starts after `option-page-builder` merges, since it imports that package): `deps` openers of the option stores with `snapshot.OpenOptionEod` and `snapshot.OpenQuoteSnapshot` opened with the `OT_PG_DSN_PAGES` line only (the role has the grants, cited above), the name in `pageNamed` (`macro-page-wiring.md`, issue `ot-macro-page`, which it also needs), an `--underlying <id>` flag, a demo reader for the demo profile and its golden, and `docs/contracts/ot.md`. The text change of `modules.md` is already in this design's diff.
- A reader of the contract terms (strike, expiry, right, symbol) by `instrument_id` and `underlying_id`: none exists (`store-week2.md:355`). Need from the parallel option-collection design: that reader, with an interface issue of its own. When it lands, the builder fills the nullable fields of v1 (a builder change, no schema version).
- A reader that lists the underlyings with rows, and one for the latest `session_day` of a dataset (the builder reads a 7-day window; a longer gap shows an empty page with a gap): the same need.
- Anything about collection jobs, the broker or the capture cadence: the parallel design. No entry here waits for it.
- Aggregates (rows per day, curves, mid prices, spreads): figures of `compute` (weeks 8-9); the page computes nothing.
- A `Builder` that takes the readers as a parameter (an edit of `internal/pages/api.go:29`): rejected; the page closes over its readers.
- A shell link or home card for the page, selection of an underlying in the web, and retiring the carried `option_page`.

## Adversarial

1. Gap against the goal: adopted in part. `ot` leaves `## Issues` (a follow-up, `## Left out`), so web and builder are buildable from the interface alone; strike, expiry, right and symbol are nullable in v1 so the schema is not a throwaway; the blind golden gets a formatting rule, a sample input file and a decoded-JSON comparison; `underlying.price` has a named source; `Figure` gets `kind`.
2. Biggest unnamed risk (generated TS size from `maxItems`): adopted. Only `quote_series` has `maxItems`; sizes are bounded in the builder and its tests; acceptance 1 and 3 of the interface measure the schema and the generated file with `wc -l`.
3. A simpler way: adopted in part. The script line moves into the interface entry (a licensed contracts and web entry) and `ot` is left out; the schema is not cut to eight cells, because the owner asked for the stored fields and the greeks.
4. Lock-in (a v1 replaced by a v2): adopted by making the terms fields nullable in v1; the similar names `option_page` and `option_data_page` stay, retiring the old one is left out.
5. False claim (carryover unchanged, three red tests): adopted. `carryover_test.go`, `generated_ts_test.go` and the fixture manifest are in `Files`, and the item no longer expects `carryover.sha256` to change by accident of the test.
6. Likeliest slip (the interface entry red or refused): adopted. Every list and test it must touch is in its `Files`; the generated file comes from the command CI runs.

Full answer: scratchpad adversarial-option-page.md of the dispatcher window (2026-11-07).
