# Generated types: no shared list

Author: designer of optdesk (first draft)
Source: owner, 2026-11-07: interface entries of different designs must be able to run in parallel. Read on main at df7deae.

Goal of the design: after it, adding a generated TypeScript type touches only new files (the schema and its generated
`.ts`), never a list that other designs also edit. Four shared places exist today, all read on main:

- `web/src/generated/index.ts`: hand-written, 12 `export type { Name } from './file'` lines, one appended by each design.
  Nothing imports it: every consumer under `web/src` imports a file directly (`@/generated/structure_detail.v3`).
- the table `files` of `TestGeneratedSummaryTypes` and the table `pairs` of `TestGeneratedV3Types` in
  `internal/contracts/generated_ts_test.go`.
- the `generate:types` script of `web/package.json`: 14 `json2ts` calls chained with `&&`, one per schema, on one line.
- numbers in `docs/contracts/page-schemas.md` ("Thirty schemas", "13 directories", "15 `bad-version.json` files").

Not every schema has a generated type (14 of the `page.*.json` files do), so the script cannot simply generate the whole
directory: that would add dead types for the superseded versions and collide names (`HeldCompaniesSummary` is the title of
v1 and v2). The set of generated types is the set of `.ts` files in `web/src/generated`: a new type is registered by
committing its file, a new file nobody else edits.

## Modules

`docs/design/modules.md` changes in one cell: the entry cell of the row `contracts` names `web/src/generated/index.ts`; it
becomes `web/src/generated/*.ts`. The paths of the rows do not change (first glob wins: `web/src/generated/**` and
`internal/contracts/**` and `docs/contracts/page-schemas.md` are `contracts`; `web/scripts/**` and `web/package.json` are
`web`; `docs/design/modules.md` is `docs`). `scripts/arch` reads the entry cell only for `internal/` packages, so
`arch.toml` and `.go-arch-lint.yml` do not change.

| issue | module |
|---|---|
| generated-index-contracts | contracts |
| generated-index-script | web |
| generated-index-modules | docs |

No new dependency: the script uses `node:` built-ins and the `json-schema-to-typescript` already in `web/package.json`
(the codegen scheme of the frontend whitelist).

## Interface

Owner: contracts. The contract of the generated TypeScript types, replacing the file `web/src/generated/index.ts`:

- One file per type: `web/src/generated/<name>.ts`, generated from `contracts/schemas/page.<name>.json` (for example
  `macro_rates.v1.ts` from `page.macro_rates.v1.json`), line 1 `// Code generated from contracts/, DO NOT EDIT.`, exporting
  `interface <title>` where `<title>` is the `title` of the schema.
- There is no index file. A consumer imports the file it needs.
- The set of types is the set of `.ts` files of the directory. `pnpm --dir web run generate:types` regenerates every file
  of the set; `pnpm --dir web run generate:types page.<name>` also creates the file of a new schema. A file with no
  schema, or a schema title that differs from the exported name, fails the contract test.
- (interface) entry: the deletion of `web/src/generated/index.ts` and the test that states the above.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff.

### generated-index-contracts (interface)

The contract of the interface part: delete the hand-written index and make the contract test read the directory.
  Goal: `web/src/generated/index.ts` is gone, `TestGeneratedTypes` checks every generated `.ts` against its schema without a table, `TestGeneratedV3Types` takes its files from the directory, and `docs/contracts/page-schemas.md` states the rule without a count.
  Files: web/src/generated/index.ts, internal/contracts/generated_ts_test.go, docs/contracts/page-schemas.md.
  Design: docs/design/generated-index.md#interface
  Acceptance:
  1. `test ! -e web/src/generated/index.ts` succeeds, and `grep -rn "generated'" web/src web/test` finds no import of the directory itself (every import names a file).
  2. `internal/contracts/generated_ts_test.go` declares `func TestGeneratedTypes` (the name replaces `TestGeneratedSummaryTypes`; `grep -c 'TestGeneratedSummaryTypes'` prints 0) which calls `t.Parallel()`, lists `web/src/generated` with `os.ReadDir`, and for each `*.ts` file runs a parallel subtest asserting: line 1 is the banner; `contracts/schemas/page.<name>.json` exists; the file has a line matching `^export interface <title> ` with `<title>` the `title` read from that schema. It also fails when `index.ts` exists and when the directory holds no `.ts` file.
  3. `grep -c -E 'household_summary|market_summary\.v2|macro_summary' internal/contracts/generated_ts_test.go` prints 0 (no file name of a type is written in the test) and `grep -c 'files := \[\]struct\|pairs := \[\]struct' internal/contracts/generated_ts_test.go` prints 0.
  4. `TestGeneratedV3Types` keeps its body (banner, exported name, no `null`, every property name, const and enum value of the schema and of the `common.v2.json` defs it reaches) and runs it once for every `*.v3.ts` file found in the directory, the schema being `page.<name>.json` and the exported name its title; a subtest per file with `t.Parallel()`.
  5. The test file has no `time.Sleep`, no network call and no fixed path other than the constants `generatedDir` and `schemasDir`; `wc -l` of it is under 600.
  6. `docs/contracts/page-schemas.md`: `grep -c -E 'Thirty|13 directories|15 .bad-version' docs/contracts/page-schemas.md` prints 0 (the sentences say "the schemas listed below", "the directories of `web/fixtures`", "every `bad-version.json` file"); the figure "29 copies" and the table of schema ids are unchanged; a sentence states the contract of `## Interface` of this design (one file per type, no index, `generate:types page.<name>` adds one) and `grep -c 'generated-index.md' docs/contracts/page-schemas.md` prints 1.
  7. The diff holds the three listed files and no other, and the result matches the Interface part of `docs/design/generated-index.md`.
  Checked by CI: ci-go runs `go test ./internal/contracts/...` with the existing 14 files; ci-web (tsc, eslint, vitest, build) still passes without the index.
  Depends on: none. Order: any.
  Size: S

### generated-index-script

`generate:types` takes its targets from the directory instead of a chain of 14 calls.
  Goal: one node script regenerates every file of `web/src/generated` byte for byte as today and creates the file of a named schema; `web/package.json` calls it.
  Files: web/scripts/generate-types.mjs, web/package.json.
  Design: docs/design/generated-index.md#interface
  Order: after generated-index-contracts.
  Acceptance:
  1. `web/scripts/generate-types.mjs` is new, under 150 lines, and imports only `node:` modules (`grep -c "from '" web/scripts/generate-types.mjs` equals `grep -c "from 'node:" web/scripts/generate-types.mjs`).
  2. In `web/package.json` the value of `generate:types` is exactly `node scripts/generate-types.mjs`; no other line of the file changes (`git diff` shows one removed and one added line); `grep -c json2ts web/package.json` prints 0 and `json-schema-to-typescript` stays in `devDependencies`.
  3. The script's targets are the base names of every `*.ts` file of `src/generated` (ignoring any file that is not `<name>.ts` with `<name>` matching `^[a-z0-9_]+\.v[0-9]+$`) together with the command line arguments (`page.<name>` or `<name>`, a form that does not match exits non-zero); each target runs `json2ts` once with `-i ../contracts/schemas/page.<name>.json`, `-o src/generated/<name>.ts`, `--cwd ../contracts/schemas` and the `--bannerComment` of today, written once as a constant (`grep -c 'Code generated from contracts/, DO NOT EDIT' web/scripts/generate-types.mjs` prints 1; the constant also holds `/* eslint-disable */`).
  4. A target whose schema file does not exist ends the script with a non-zero status and a message naming the schema; the script writes no file other than the targets (`grep -c 'index' web/scripts/generate-types.mjs` prints 0).
  5. The script runs `json2ts` through `execFileSync` with an argument array (no shell string), from the directory `web`.
  6. The diff holds the two listed files and no other, and the result matches the Interface part of `docs/design/generated-index.md`.
  Checked by CI: the `type drift` step of `ci-web.yml` (`pnpm --dir web run generate:types`, then `git diff --exit-code web/src/generated` and no untracked file) passes with the 14 committed files unchanged, so the script reproduces them byte for byte.
  Depends on: generated-index-contracts.
  Size: S

### generated-index-modules

The entry cell of the row `contracts` follows the interface.
  Goal: `docs/design/modules.md` no longer names the deleted index file as an interface file of the row `contracts`.
  Files: docs/design/modules.md.
  Design: docs/design/generated-index.md#modules
  Order: after generated-index-contracts.
  Acceptance:
  1. `grep -c 'web/src/generated/index.ts' docs/design/modules.md` prints 0 and `grep -c 'web/src/generated/\*.ts' docs/design/modules.md` prints 1, on the line of the row `contracts` (`grep -n '^| contracts |'`).
  2. Every other cell of every row is unchanged: `git diff` shows one removed and one added line, the row `contracts`.
  3. The diff holds the one listed file, and the result matches the Modules part of `docs/design/generated-index.md`.
  Checked by CI: `go run ./scripts/arch --check` in `checks.yml` agrees (`arch.toml` and `.go-arch-lint.yml` unchanged).
  Depends on: generated-index-contracts.
  Size: XS

## Left out

- Generate every `page.*.json`: no list at all, but 16 dead types of superseded versions and a title collision for an index; the directory as the set is the smaller change. Considered and not chosen.
- A generated `index.ts` (same script): two designs adding a type would both rewrite the same file and conflict in git, which is the problem; and nothing imports it.
- A marker key in each schema (`x-ts`): the 29 carried copies are pinned by `carryover.sha256` and cannot take one.
- A manifest file listing the types: a shared list again.
- The table and the "Also `page.<name>`" paragraphs of `docs/contracts/page-schemas.md` stay: each new schema adds a row at its own place in the alphabet and a paragraph; two designs rarely touch adjacent rows. If it conflicts, a rebase decides, it does not order the entries.
- Row `pairs` for versions other than v3 in `TestGeneratedV3Types`: that body is specific to the v3 conventions; a v2 or v1 type needs no row.
- How designs 285 (macro-rates-view), 286 (report-page) and 287 (home-page) drop their Order chain (the dispatcher edits them): their interface entries each add only `web/src/generated/<name>.ts` (new, produced by `pnpm --dir web run generate:types page.<name>`), and no line to an index, no row in a test, no call in `package.json`. So the lines `Order: after <entry of another of the three designs>` are removed, and each of the three interface entries gets `Depends on: generated-index-script` (merged, and it implies generated-index-contracts) instead; until it merges they must wait, because without the script the only way to generate a type is the shared `package.json` line. The Files line of those entries drops `web/src/generated/index.ts`, `internal/contracts/generated_ts_test.go` and `web/package.json`, and a count in their acceptance of exports in the index or of rows in the test is deleted.
- Landing order: this design's three entries are small; the interface entry and the script entry are judged first, and the three designs' interface entries are cut after the script entry has merged.

## Adversarial

pending
