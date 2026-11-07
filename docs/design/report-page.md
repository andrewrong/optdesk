# Report page: the web front end gets the report_page view (fixtures only)

Author: the optdesk dispatcher
Owner approved: (pending)
Revision: 1

Source: the dispatcher's brief for topic report-page, `docs/design/web-views-wave.md` (self-registering page views),
`docs/design/web-views-wave-2.md`, `docs/design/page-schemas-v2.md`, `docs/design/pages-builders.md`.
Read on main at df7deae (`contracts/schemas/page.report_page.v2.json`, `internal/contracts/generated_ts_test.go`,
`internal/contracts/common_test.go`, `internal/contracts/minimal_test.go`, `web/package.json`, `web/src/shell/pages.tsx`,
`web/src/generated/index.ts`, `web/src/components/data-table/data-table.tsx`, `web/src/charts/Chart.tsx`, `internal/pages/`,
`contracts/datasets/`, `cmd/ot/page.go`).

## What main holds today

What `report_page` is: the report of one market session, written twice a day (`slot` is `open` or `close`). It is a single page
(not a composition of other pages) of 3875 lines of schema.

- Schema: `contracts/schemas/page.report_page.v2.json` (`schema` const `report_page`, `schema_version` const 2, top-level title `ReportPage`).
  `report_page` v1 is a dead version. Top level: `market` (US or JP), `slot`, `session_date`, `generated_at`, `title`, `say`, `banner`
  (`left`, `right`), `status_lines`, `blocks`, and the optional `chips`, `design_note` and `stale`. `blocks` holds five blocks that are always
  there (`actions`, `account_moves`, `risks`, `market_watch`, `global_assets`) and six optional ones that the open slot adds (`market_line`,
  `accounts`, `candidates`, `reminders`, `tiles`, `system_status`). Each block carries `key`, `title`, `sub`, `say`, `note`, `state`
  (`ok`, `partial`, `gap`), `gaps`, `gap_text`, `data_times` and an optional `link`; the figures are `Cell` and `MoneyCell` values with a text,
  a tone and an optional numeric value, and a `Gap` stands where a figure is missing.
- Types: no generated type. `web/src/generated/` holds 15 entries (14 type files and `index.ts`; `market_summary.v3.ts` and `options_summary.v3.ts` are in neither the index nor the test) and `index.ts` has 12 export lines; none is `report_page`. The `selfCheckFiles`
  list of `internal/contracts/minimal_test.go` holds `page.report_page.v2.json` (a self check, no fixtures); the file is not in `pageV2Files`.
- Web: `pages.tsx` has one explicit page (`option_data_page`) and one glob `../pages/*/*.tsx` that registers every file exporting `registration`.
  No file of `web/src` names `report_page`. `web/src/pages/` has no `report` directory. The pieces a view reads exist: `DataTable`
  (`@tanstack/react-table`), `Chart` through `components/ui/chart.tsx` (recharts), `components/ui/{card,badge,alert,accordion,progress}.tsx`,
  and `StaleAlert`, `CardNotes`, `DataTimesFold` under `web/src/pages/home/`.
- Go: no `report_page` package under `internal/pages/` (three exist: `macro_page`, `macro_rates`, `option_data_page`), and `ot page --name`
  has no report name. The `store` readers that exist read option, quote, series and instrument data. `contracts/datasets/` holds four
  datasets (`master.session_day`, `obs.option_eod`, `obs.quote_snapshot`, `obs.series_point`). The report blocks need accounts, trades,
  positions, risks and `compute` figures; none has a dataset or a reader (`pages-builders.md` says so in `## Left out`).
- `web-views-wave.md` left `report_page` out for its size; `web-views-wave-2.md` did not reopen it. This design reopens it.

What is missing: a generated type, a view, and (not cut here) a producer.

## Decisions

1. The cut. One interface entry for the generated type, then one web entry for the view. No Go builder entry: the schema is not
   changed, and a builder would have no reader to call (`## What main holds today`, Go). A builder that wrote only gaps would give the
   page a payload that says nothing, so the producer waits for the account datasets (`## Left out`). Until then the page has no producer: the view is fixtures only, fed by
   fixtures in its tests (no payload exists in production until the account datasets and a builder do), as the six views of `web-views-wave.md` were.
2. The interface entry adds the type and the script call that writes it, so it is a two-module set (contracts and web), listed in the cut:
   `web/src/generated/**` and `internal/contracts/generated_ts_test.go` belong to contracts (first matching row of `modules.md`), and the one-line edit of
   `generate:types` in `web/package.json` belongs to web. It is one entry, as `pages-types-wave-2` of `docs/design/web-views-wave-2.md` is (judged two-module entry): the script, `index.ts`
   and the Go list are shared files, and a generator call without its output (or the reverse) would leave `generate:types` drifting in CI. The existing `json-schema-to-typescript`
   is used; there is no new package. The entry is serialised with `macro-rates-v2-schema` of that design: both append a line to `web/src/generated/index.ts`
   and a row to `internal/contracts/generated_ts_test.go`, so `report-types` goes after it and every count below is relative to main at cut time. No schema file changes, so the pull request carries `Contracts: unchanged`.
3. One view entry, not one per block. The view file imports every block file, so per-block entries would need each other's output. The
   entry is size L and is split by file: `view.tsx` and and the files named in the view entry's What, each under 600 lines.
4. The view renders and computes nothing. Every string, figure, tone and state is a field of the payload; a numeric `value` is only
   handed to a chart as the height of a bar. A `Gap` in place of a figure shows its `text`. A block whose `state` is `gap` or `partial`
   shows its `gaps` and its `gap_text`. An optional block that is `null` is not drawn.
5. Registration is by `export const registration = { schema: 'report_page', page: page(validate, View) }` in `web/src/pages/report/view.tsx`; the
   type file `web/src/types/report_page.ts` follows `web/src/types/ledger_page.ts` with `SUPPORTED_VERSIONS = [2]`. No edit of `pages.tsx`.
6. Fixtures follow `web-views-wave.md` Decision 4: `web/test/pages/report/fixtures/` with its own manifest. Two valid fixtures, because the
   two slots differ in their blocks: `ok-close` (five blocks) and `ok-open` (eleven blocks), plus `gap`, `stale` and `bad-version`.

## Modules

None. No row of `docs/design/modules.md` is added or changed, so `arch.toml` and `.go-arch-lint.yml` do not change (no new Go package).
Paths used: `web/**` and `web/test/**` belong to web; `web/src/generated/**` and `internal/contracts/**` belong to contracts.

| issue | module | paths |
|---|---|---|
| report-types (interface) | contracts, plus web (judged two-module entry, Decision 2) | `web/src/generated/report_page.v2.ts`, `web/src/generated/index.ts`, `internal/contracts/generated_ts_test.go`, `web/package.json` |
| report-page-view | web | `web/src/pages/report/**`, `web/src/types/report_page.ts`, `web/test/pages/report/**` |

## Interface

Owner: contracts (the generated type); the entry also edits one line of `web/package.json` (web). Frozen by `report-types`, read by `report-page-view`.

- `web/src/generated/report_page.v2.ts` exports `interface ReportPage`, made by `json2ts` from `contracts/schemas/page.report_page.v2.json`
  with `--cwd ../contracts/schemas` and the banner of the existing files; `web/src/generated/index.ts` gains
  `export type { ReportPage } from './report_page.v2';`. The `generate:types` call is added by this same entry (Decision 2).
- `TestGeneratedSummaryTypes` gains the row `{"report_page.v2.ts", "./report_page.v2", "ReportPage"}`, so its list has one more row than main at cut time and `index.ts` one more export line than before.
- If `tsc` fails on a generated type (`json2ts` writes an array with a `maxItems` as a tuple union), the pull request says so and the generated file is not edited by hand. An array with `minItems` 1 becomes `[T, ...T[]]`, so a fixture that is `satisfies ReportPage` never writes `[]` for such an array (tsc fails on it).
- `web/package.json` gains one `json2ts` call in `generate:types` for `page.report_page.v2.json` (writing `src/generated/report_page.v2.ts`, `--cwd ../contracts/schemas`, the same `--bannerComment` as `market_summary.v2`) and no other change.
- Size: the new file `web/src/generated/report_page.v2.ts` is exempt from the 600-line cap of a new source file. The function `isSource` of `scripts/ratchet/main.go` does not count a file with a generated mark (`code generated`, `do not edit`, `@generated`, `generated by`) in its first five lines, and every file of `web/src/generated/` starts with `// Code generated from contracts/, DO NOT EDIT.` (checked on origin/main). The CI size check, the step `size ratchet` of `.github/workflows/checks.yml`, runs `go run ./scripts/ratchet`, so it uses that tool. The output needs no ratchet entry and is not split.

## Issues

### report-types (interface)

Module: contracts, plus web (judged two-module entry, Decision 2). Size: S.

Files: `web/src/generated/report_page.v2.ts`, `web/src/generated/index.ts`, `internal/contracts/generated_ts_test.go`, `web/package.json` (only the `generate:types` call for `page.report_page.v2.json`)

Depends on: macro-rates-v2-schema

Order: after macro-rates-v2-schema (docs/design/macro-rates-view.md)

Design: docs/design/report-page.md#decisions

Goal: the generated type of `report_page` v2 exists and is re-exported, with the Go guard that every other generated type has and the `generate:types` call that writes it.

Acceptance:

1. `test -f web/src/generated/report_page.v2.ts`, and `head -1 web/src/generated/report_page.v2.ts` prints `// Code generated from contracts/, DO NOT EDIT.`.
2. `grep -c "^export interface ReportPage " web/src/generated/report_page.v2.ts` prints 1, and `grep -c "export type { ReportPage } from './report_page.v2';" web/src/generated/index.ts` prints 1.
3. `grep -c "page.report_page.v2.json" web/package.json` prints 1, and `git diff origin/main -- web/package.json` changes only the `generate:types` line, which gains the one `json2ts` call with `--cwd ../contracts/schemas` and the same `--bannerComment` as the call for `market_summary.v2`.
4. `internal/contracts/generated_ts_test.go` has the row for `report_page.v2.ts` and `go test ./internal/contracts -run TestGeneratedSummaryTypes -count=1 -race` passes (one more row than main, one more export line in `index.ts` than before: compare with `git show origin/main:<file>`).
5. Every `properties` key of the top level of `contracts/schemas/page.report_page.v2.json` (`banner`, `blocks`, `chips`, `design_note`, `generated_at`, `market`, `say`, `schema`, `schema_version`, `session_date`, `slot`, `stale`, `status_lines`, `title`) and each of the eleven keys of `Blocks` appears in `web/src/generated/report_page.v2.ts` (`grep -c -E "^\s+<key>\??:" web/src/generated/report_page.v2.ts` prints at least 1 for each).
6. `git diff --name-only origin/main` lists exactly the four paths of Files, and `git diff origin/main -- contracts/schemas` is empty.
7. The result matches the design part Interface of docs/design/report-page.md, judged from the diff.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run generate:types` (the type drift step: `web/src/generated/report_page.v2.ts` is the output of `json2ts` on the schema, no drift in `web/src/generated`; an offline sandbox has no `node_modules`, so this cannot be decided there and the acceptance does not claim it), `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint`, `pnpm --dir web run build`.

### report-page-view

Module: web. Size: L.

Files: `web/src/pages/report/view.tsx`, `web/src/pages/report/*.tsx`, `web/src/types/report_page.ts`, `web/test/pages/report/**` (the tests and the fixtures directory `web/test/pages/report/fixtures/` with its own `MANIFEST.synthetic-v1.json`)

Depends on: report-types

Order: after report-types (and so after macro-rates-v2-schema)

Design: docs/design/report-page.md#decisions

Goal: `report_page` v2 renders in the shell as a read-only page, for both slots, from fixtures written from the schema alone (fixtures only: no producer exists).

What: the generated type and the `generate:types` call come from `report-types`; this entry does not edit `web/package.json`. The files, each under 600 lines: `view.tsx` (registration, `Payload`, the order of the blocks), `frame.tsx` (header and the block frame: `state`, `gaps`, `gap_text`, `data_times`, `link`; each frame carries the attribute `data-block-key` with the block key), `cells.tsx` (`Cell`, `MoneyCell`, `Gap`, tones, `meters`), `close-blocks.tsx` (`actions`, `account_moves`, `risks`, `market_watch`), `global-assets.tsx` (`global_assets` with the chart) and `open-blocks.tsx` (`market_line`, `tiles`, `accounts`, `candidates`, `reminders`, `system_status`). `view.tsx` registers the page and draws, in this order: the header (`title`, `say`, `banner.left`, `banner.right`, the `chips`, the `stale` label,
the `design_note`, the `status_lines` as label and text pairs), then the blocks. The five close blocks are drawn in the order `actions`,
`account_moves`, `risks`, `market_watch`, `global_assets`; the six open blocks, when present, are drawn first, in the order `market_line`,
`tiles`, `accounts`, `candidates`, `reminders`, `system_status`. Every block has one frame (title, `sub`, `say`, `note`, `state`, `gaps`,
`gap_text`, `data_times` folded, `link`). `actions` shows its `trades` table and its `candidates` groups; `account_moves` its `accounts` table, its `movers` table and
its `meters`; `risks` its `items` with the `level` (`n`, `w`, `d`); `market_watch` its `indices` and `companies` tables; `global_assets` its `proxies`
as a chart, its `sector` table and its `rotation`; `market_line` its `figures` and `sentence`; `tiles` its `tiles`; `accounts` its `meters`;
`candidates` its `groups`, `covered_call`, `put_spread` and `full_page` link; `reminders` its `rows` and its `rest` table; `system_status` its `lines`, collapsed
when `collapsed` is true. The type file follows `web/src/types/ledger_page.ts` (`Payload` is `ReportPage` of `@/generated/report_page.v2`, `SUPPORTED_VERSIONS = [2]`).
Fixtures are TypeScript modules `ok-close.ts`, `ok-open.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies ReportPage` with the exports `okClose`, `okOpen`, `gap` and `stale` (the file names keep the hyphen; the tests and acceptances use the export names). Each fixture imports `ReportPage` by the relative path `../../../../../src/generated/report_page.v2`, not by `@/generated/...` (as `web-views-wave-2.md` Decision 4), so no fixture holds an `@` (`schema` is
`'report_page'`, `schema_version` is 2), and `bad-version.json` (plain JSON, `schema_version` another number, read with `readFileSync`). Every new file under `web/test/pages/report/` (the fixture modules and the vitest file or files) stays under 600 lines, as every new source file does (only a file with a generated mark in its first lines is exempt, `isSource` in `scripts/ratchet/main.go`); `ok-open.ts` holds all eleven blocks, so its tables carry few rows and its text is short. If it does not fit, the pull request description says so and the file is not split (a part module would break the manifest of acceptance 5). The files of `web/src/pages/home/`
(`StaleAlert.tsx`, `CardNotes.tsx`, `DataTimesFold.tsx`) may be imported and are not edited. A needed change to a file outside Files is reported in the pull request description.

按 frontend-stack：用 @tanstack/react-table（through `web/src/components/data-table/data-table.tsx`）做 the tables of `trades`, `accounts`, `movers`, `indices`, `companies`, `sector` and `rest`。

按 frontend-stack：用 recharts via the shadcn chart component (`web/src/charts/Chart.tsx`, `components/ui/chart.tsx`) 做 the `proxies` bars of `global_assets` (the day and five-day `value` of each proxy, the `chart_aria` text as its accessible name; a proxy whose cell is a `Gap` is not a bar and its text is listed under the chart; when the whole `proxies` field is a `Gap` (the schema allows an array or a `Gap`) there is no chart, no `chart_aria` name, and the `Gap` text is shown in its place)。

按 frontend-stack：用 shadcn card, badge, alert, accordion and progress (`components/ui`) 做 the header and block frames, the `tone` and `level` marks, the stale and gap lines, the folded `data_times` and `system_status`, and the `meters`。

Acceptance:

1. `test -f web/src/pages/report/view.tsx && test -f web/src/types/report_page.ts`; `grep -c "export const registration" web/src/pages/report/view.tsx` prints 1 and `grep -c "schema: 'report_page'" web/src/pages/report/view.tsx` prints 1.
2. `git diff --name-only origin/main` lists no path outside the Files line, and `git diff origin/main -- web/package.json` is empty (in particular not `web/src/shell/pages.tsx`, `web/src/generated/index.ts`, `web/no-compute-allow.txt`, `web/deps-allowlist.txt`, `web/fixtures/MANIFEST.synthetic-v1.json`, or a file of `web/src/pages/home/`).
3. `wc -l web/src/pages/report/*.tsx web/test/pages/report/fixtures/*.ts web/test/pages/report/*.test.tsx` shows every file under 600 lines (each fixture module, `ok-close.ts`, `ok-open.ts`, `gap.ts` and `stale.ts`, and each vitest file); `grep -c "fetch(" web/src/pages/report/*.tsx` prints 0 for each; `grep -c -E "toFixed|Intl\.|parseFloat|reduce\(" web/src/pages/report/*.tsx` prints 0 for each (no formatting or arithmetic).
4. `grep -l "useReactTable\|data-table" web/src/pages/report/*.tsx` is not empty and no file of `web/src/pages/report/` imports `recharts` directly (`grep -c "from 'recharts'" web/src/pages/report/*.tsx` prints 0 for each); `grep -c "chart_aria" web/src/pages/report/*.tsx` prints at least 1 in total.
5. Fixtures: the files `ok-close.ts`, `ok-open.ts`, `gap.ts`, `stale.ts`, `bad-version.json` exist under `web/test/pages/report/fixtures/`; `grep -c "satisfies ReportPage" web/test/pages/report/fixtures/{ok-close,ok-open,gap,stale}.ts` prints 1 for each; `MANIFEST.synthetic-v1.json` there has `marker` `synthetic-v1` and lists exactly these five files; `grep -c "@" web/test/pages/report/fixtures/*` prints 0 for each; `grep -c '"schema_version": 2,' web/test/pages/report/fixtures/bad-version.json` prints 0.
6. `okClose` (file `ok-close.ts`) has `slot: 'close'` and the five close blocks; `okOpen` (file `ok-open.ts`) has `slot: 'open'` and all eleven keys of `blocks` non-null (asserted in the vitest file as `Object.keys(okOpen.blocks)` holding the eleven keys, each value not `null`; no grep count, because `accounts` and `candidates` also occur as nested keys); `gap.ts` (export `gap`) has a block whose `state` is `'gap'` and a `Gap` in place of a figure, and its `blocks.global_assets.proxies` is a whole `Gap` (not an array; the case of acceptance 7), so no sixth fixture file and no payload built in the test; `stale.ts` has a `stale` label.
7. `web/test/pages/report/` holds a vitest file that renders `okClose` and `okOpen`, and for each block key checks that the block `title` read from the fixture is on the page (`getAllByText`, since a title can repeat in `sub` or `say`; 11 keys, asserted by name: `grep -c -E "okOpen\.blocks\.<key>" web/test/pages/report/*.test.tsx` prints at least 1 for each of the eleven keys); renders `gap` (the export) and checks that the `gap_text` and the `Gap` text are on the page, that its `global_assets` frame (`data-block-key`) draws no chart and has no `chart_aria` name, and that the `Gap` text of `gap.blocks.global_assets.proxies` is shown in its place; renders `stale` (the export) and checks `stale.text` is on the page; checks that an optional block set to `null` leaves no frame (no element with `data-block-key` equal to its key); checks that the `proxies` chart of `okClose` has the `chart_aria` text as its accessible name; checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
8. `pnpm --dir web run test -- web/test/pages/report` passes, and `git diff origin/main -- web/no-compute-allow.txt` is empty.
9. The result matches the design part Decisions (points 3 to 6) of docs/design/report-page.md, judged from the diff.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest), `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

## Left out

- A Go builder `internal/pages/report_page` and an `ot page --name report_page` name: no account, trade, position or risk dataset and no reader exists
  (`contracts/datasets/` has four datasets); a builder would write gaps only. It follows the datasets and their readers, and then needs its own
  design (module pages, then `ot`).
- One web entry per block: the blocks share `view.tsx`, so the entries would need each other's output; one entry split by file is chosen (Decision 3).
- Generating the type inside the view entry: it would give the view a contracts path and edit two more shared files (`index.ts`, the Go list); the separate
  interface entry keeps them in one place and also carries the one `generate:types` line (a two-module set, as `pages-types-wave-2` of `docs/design/web-views-wave-2.md`), so the view entry has no `web/package.json`.
- A smaller view cut (the five close blocks first, the six open blocks as a second issue): both slots share the frame, and the file split above already keeps each file small.
- Dropping `report_page` v1 and a `report_page` v3: a deletion of a schema has no `Contracts:` form (`web-fixtures-vn1.md`); v3 is not written until a producer needs it.
- Links of `link.href` and `candidates.full_page` to other pages: shown as the link text with its `href`; no router change here.
- `macro_rates` and `market_weather` views: other designs.

## Adversarial

Answer in `report-page.adversarial.md` in the dispatcher's scratchpad.

The design session's points on the size of the generated file, the fixture export names and the relative import were applied. The later design-session points were applied too: `wc -l` bounds for the new fixture and test files, the json2ts drift check moved to `## Checked by CI`, the fixture `gap.ts` named for the whole-`Gap` `proxies`, and `web/package.json` moved into the interface entry as a two-module set.

1. Gap against the goal: adopted. Title, Goal and Decision 1 say "fixtures only"; the view entry names its files and block groups.
2. Overlap with design PR 285 and the whole-`Gap` `proxies`: adopted. `report-types` is `Order: after macro-rates-v2-schema` and depends on it by name; the whole-`Gap` case is specified and tested.
3. Simpler way (relative counts, smaller view cut): counts adopted (relative to main at cut time); the smaller view cut is not adopted, because the file split keeps each file small.
5. False claim: adopted. The `generated/` entries are 15 (14 type files and `index.ts`); the grep count of acceptance 6 is replaced by a vitest assertion on `Object.keys(okOpen.blocks)`.
6. Likeliest slip: adopted. The attribute is `data-block-key`; titles are read with `getAllByText`.
(a) Two-module set: adopted. `report-types` is a judged two-module entry (contracts and web) and carries the `generate:types` edit; the view entry has no `web/package.json`.
(c) `maxItems` and tuples: adopted. The sentence on `[T, ...T[]]` and empty arrays in fixtures is added to Interface.
