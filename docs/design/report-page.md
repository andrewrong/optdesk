# Report page: the web front end gets the report_page view

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
- Types: no generated type. `web/src/generated/` has 12 files and `index.ts` has 12 export lines; none is `report_page`. The `selfCheckFiles`
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
   page a payload that says nothing, so the producer waits for the account datasets (`## Left out`). Until then the view is fed by
   fixtures in its tests, as the six views of `web-views-wave.md` were.
2. The interface entry only adds the type. It is a two-module entry (web and contracts) exactly as `pages-self-registering` of
   `web-views-wave.md` was judged: `web/package.json` (web) holds the one `generate:types` script, `web/src/generated/**` and
   `internal/contracts/generated_ts_test.go` belong to contracts (first matching row of `modules.md`). The existing `json-schema-to-typescript`
   is used; there is no new package. No schema file changes, so the pull request carries `Contracts: unchanged`.
3. One view entry, not one per block. The view file imports every block file, so per-block entries would need each other's output. The
   entry is size L and is split by file: `view.tsx` and one file per group of blocks, each under 600 lines.
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

## Interface

Owner: contracts (the generated type), web (`generate:types`). Frozen by `report-types`, read by `report-page-view`.

- `web/src/generated/report_page.v2.ts` exports `interface ReportPage`, made by `json2ts` from `contracts/schemas/page.report_page.v2.json`
  with `--cwd ../contracts/schemas` and the banner of the existing files; `web/src/generated/index.ts` gains
  `export type { ReportPage } from './report_page.v2';`; the `generate:types` script of `web/package.json` gains the call.
- `TestGeneratedSummaryTypes` gains the row `{"report_page.v2.ts", "./report_page.v2", "ReportPage"}`, so the file has 13 rows and `index.ts` 13 export lines.
- If `tsc` fails on a generated type (`json2ts` writes an array with a `maxItems` as a tuple union), the pull request says so and the generated file is not edited by hand.

## Issues

### report-types (interface)

Module: web and contracts (judged two-module entry, Decision 2). Size: S.

Files: `web/package.json`, `web/src/generated/report_page.v2.ts`, `web/src/generated/index.ts`, `internal/contracts/generated_ts_test.go`

Depends on: none.

Design: docs/design/report-page.md#decisions

Goal: the generated type of `report_page` v2 exists and is re-exported, with the script and the Go guard that every other generated type has.

Acceptance:

1. `test -f web/src/generated/report_page.v2.ts`, and `head -1 web/src/generated/report_page.v2.ts` prints `// Code generated from contracts/, DO NOT EDIT.`.
2. `grep -c "^export interface ReportPage " web/src/generated/report_page.v2.ts` prints 1, and `grep -c "export type { ReportPage } from './report_page.v2';" web/src/generated/index.ts` prints 1.
3. `grep -c "page.report_page.v2.json" web/package.json` prints 1, the call writes `src/generated/report_page.v2.ts` with `--cwd ../contracts/schemas` and the same `--bannerComment` as the call for `market_summary.v2`, and `grep -o "json2ts -i" web/package.json | wc -l` prints 15 (14 on main).
4. `internal/contracts/generated_ts_test.go` has the row for `report_page.v2.ts` and `go test ./internal/contracts -run TestGeneratedSummaryTypes -count=1 -race` passes (13 rows, 13 export lines in `index.ts`).
5. Every `properties` key of the top level of `contracts/schemas/page.report_page.v2.json` (`banner`, `blocks`, `chips`, `design_note`, `generated_at`, `market`, `say`, `schema`, `schema_version`, `session_date`, `slot`, `stale`, `status_lines`, `title`) and each of the eleven keys of `Blocks` appears in `web/src/generated/report_page.v2.ts` (`grep -c -E "^\s+<key>\??:" web/src/generated/report_page.v2.ts` prints at least 1 for each).
6. `git diff --name-only origin/main` lists exactly the four paths of Files, and `git diff origin/main -- contracts/schemas` is empty.
7. The result matches the design part Interface of docs/design/report-page.md, judged from the diff.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run generate:types` (no drift in `web/src/generated`), `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint`, `pnpm --dir web run build`.

### report-page-view

Module: web. Size: L.

Files: `web/src/pages/report/view.tsx`, `web/src/pages/report/*.tsx`, `web/src/types/report_page.ts`, `web/test/pages/report/**` (the tests and the fixtures directory `web/test/pages/report/fixtures/` with its own `MANIFEST.synthetic-v1.json`)

Depends on: report-types

Order: after report-types

Design: docs/design/report-page.md#decisions

Goal: `report_page` v2 renders in the shell as a read-only page, for both slots, from fixtures written from the schema alone.

What: `view.tsx` registers the page and draws, in this order: the header (`title`, `say`, `banner.left`, `banner.right`, the `chips`, the `stale` label,
the `design_note`, the `status_lines` as label and text pairs), then the blocks. The five close blocks are drawn in the order `actions`,
`account_moves`, `risks`, `market_watch`, `global_assets`; the six open blocks, when present, are drawn first, in the order `market_line`,
`tiles`, `accounts`, `candidates`, `reminders`, `system_status`. Every block has one frame (title, `sub`, `say`, `note`, `state`, `gaps`,
`gap_text`, `data_times` folded, `link`). `actions` shows its `trades` table and its `candidates` groups; `account_moves` its `accounts` table, its `movers` table and
its `meters`; `risks` its `items` with the `level` (`n`, `w`, `d`); `market_watch` its `indices` and `companies` tables; `global_assets` its `proxies`
as a chart, its `sector` table and its `rotation`; `market_line` its `figures` and `sentence`; `tiles` its `tiles`; `accounts` its `meters`;
`candidates` its `groups`, `covered_call`, `put_spread` and `full_page` link; `reminders` its `rows` and its `rest` table; `system_status` its `lines`, collapsed
when `collapsed` is true. The type file follows `web/src/types/ledger_page.ts` (`Payload` is `ReportPage` of `@/generated/report_page.v2`, `SUPPORTED_VERSIONS = [2]`).
Fixtures are TypeScript modules `ok-close.ts`, `ok-open.ts`, `gap.ts`, `stale.ts`, each `export const <name> = { ... } satisfies ReportPage` (`schema` is
`'report_page'`, `schema_version` is 2), and `bad-version.json` (plain JSON, `schema_version` another number, read with `readFileSync`). The files of `web/src/pages/home/`
(`StaleAlert.tsx`, `CardNotes.tsx`, `DataTimesFold.tsx`) may be imported and are not edited. A needed change to a file outside Files is reported in the pull request description.

按 frontend-stack：用 @tanstack/react-table（through `web/src/components/data-table/data-table.tsx`）做 the tables of `trades`, `accounts`, `movers`, `indices`, `companies`, `sector` and `rest`。

按 frontend-stack：用 recharts via the shadcn chart component (`web/src/charts/Chart.tsx`, `components/ui/chart.tsx`) 做 the `proxies` bars of `global_assets` (the day and five-day `value` of each proxy, the `chart_aria` text as its accessible name; a proxy whose cell is a `Gap` is not a bar and its text is listed under the chart)。

按 frontend-stack：用 shadcn card, badge, alert, accordion and progress (`components/ui`) 做 the header and block frames, the `tone` and `level` marks, the stale and gap lines, the folded `data_times` and `system_status`, and the `meters`。

Acceptance:

1. `test -f web/src/pages/report/view.tsx && test -f web/src/types/report_page.ts`; `grep -c "export const registration" web/src/pages/report/view.tsx` prints 1 and `grep -c "schema: 'report_page'" web/src/pages/report/view.tsx` prints 1.
2. `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json`, `web/src/generated/index.ts`, `web/no-compute-allow.txt`, `web/deps-allowlist.txt`, `web/fixtures/MANIFEST.synthetic-v1.json`, or a file of `web/src/pages/home/`).
3. `wc -l web/src/pages/report/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/report/*.tsx` prints 0 for each; `grep -c -E "toFixed|Intl\.|parseFloat|reduce\(" web/src/pages/report/*.tsx` prints 0 for each (no formatting or arithmetic).
4. `grep -l "useReactTable\|data-table" web/src/pages/report/*.tsx` is not empty and no file of `web/src/pages/report/` imports `recharts` directly (`grep -c "from 'recharts'" web/src/pages/report/*.tsx` prints 0 for each); `grep -c "chart_aria" web/src/pages/report/*.tsx` prints at least 1 in total.
5. Fixtures: the files `ok-close.ts`, `ok-open.ts`, `gap.ts`, `stale.ts`, `bad-version.json` exist under `web/test/pages/report/fixtures/`; `grep -c "satisfies ReportPage" web/test/pages/report/fixtures/{ok-close,ok-open,gap,stale}.ts` prints 1 for each; `MANIFEST.synthetic-v1.json` there has `marker` `synthetic-v1` and lists exactly these five files; `grep -c "@" web/test/pages/report/fixtures/*` prints 0 for each; `grep -c '"schema_version": 2,' web/test/pages/report/fixtures/bad-version.json` prints 0.
6. `ok-close.ts` has `slot: 'close'` and the five close blocks; `ok-open.ts` has `slot: 'open'` and all eleven keys of `blocks` non-null (`grep -c -E "^\s+(actions|account_moves|risks|market_watch|global_assets|market_line|accounts|candidates|reminders|tiles|system_status):" web/test/pages/report/fixtures/ok-open.ts` prints 11); `gap.ts` has a block whose `state` is `'gap'` and a `Gap` in place of a figure; `stale.ts` has a `stale` label.
7. `web/test/pages/report/` holds a vitest file that renders `ok-close` and `ok-open`, and for each block key checks that the block `title` read from the fixture is on the page (11 keys, asserted by name: `grep -c -E "ok-open\.blocks\.<key>" web/test/pages/report/*.test.tsx` prints at least 1 for each of the eleven keys); renders `gap` and checks that the `gap_text` and the `Gap` text are on the page; renders `stale` and checks `stale.text` is on the page; checks that an optional block set to `null` leaves no frame (no element with its `key`); checks the `proxies` chart has the `chart_aria` text as its accessible name; checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
8. `pnpm --dir web run test -- web/test/pages/report` passes, and `git diff origin/main -- web/no-compute-allow.txt` is empty.
9. The result matches the design part Decisions (points 3 to 6) of docs/design/report-page.md, judged from the diff.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest), `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

## Left out

- A Go builder `internal/pages/report_page` and an `ot page --name report_page` name: no account, trade, position or risk dataset and no reader exists
  (`contracts/datasets/` has four datasets); a builder would write gaps only. It follows the datasets and their readers, and then needs its own
  design (module pages, then `ot`).
- One web entry per block: the blocks share `view.tsx`, so the entries would need each other's output; one entry split by file is chosen (Decision 3).
- Generating the type inside the view entry: it would give the view a contracts path and edit three shared files (`generate:types`, `index.ts`, the Go list); a separate
  interface entry keeps them in one place, as in `web-views-wave.md`.
- Dropping `report_page` v1 and a `report_page` v3: a deletion of a schema has no `Contracts:` form (`web-fixtures-vn1.md`); v3 is not written until a producer needs it.
- Links of `link.href` and `candidates.full_page` to other pages: shown as the link text with its `href`; no router change here.
- `macro_rates` and `market_weather` views: other designs.

## Adversarial

Pending.
