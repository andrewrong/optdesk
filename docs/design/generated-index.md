# Generated types: no shared list

Author: designer of optdesk (first draft)
Source: owner, 2026-11-07 (revised after the adversarial answer and the dispatcher's decisions): interface entries of different designs must be able to run in parallel. Read on main at df7deae.

Goal of the design: after it, adding a generated TypeScript type touches only new files (the schema and its generated
`.ts`), never a list that other designs also edit. Five shared places exist today, all read on main:

- `web/src/generated/index.ts`: hand-written, 12 `export type { Name } from './file'` lines, one appended by each design.
  Nothing imports it: every consumer under `web/src` imports a file directly (`@/generated/structure_detail.v3`).
- the table `files` of `TestGeneratedSummaryTypes` and the table `pairs` of `TestGeneratedV3Types` in
  `internal/contracts/generated_ts_test.go`.
- the `generate:types` script of `web/package.json`: one `json2ts` call per schema, chained with `&&` on one line.
- `pageNewFiles` of `internal/contracts/carryover_test.go`: one literal list of the schemas written in this repository, which each
  new schema extends.
- `docs/contracts/page-schemas.md` carries counts ("Thirty schemas", "13 directories", "15 `bad-version.json` files"); they go.

Not every schema has a generated type (only some of the `page.*.json` files do), so the script cannot simply generate the whole
directory: that would add dead types for the superseded versions and collide names (`HeldCompaniesSummary` is the title of
v1 and v2). The set of generated types is the set of `.ts` files in `web/src/generated`: a new type is registered by
committing its file, a new file nobody else edits.

## Modules

This design PR itself edits `docs/design/modules.md` (not an entry), in two cells of the rows it already has: the entry cell of the row
`contracts` names `web/src/generated/index.ts`, it becomes `web/src/generated/*.ts`; and `web/tests/**` (two places, the prose under the
table and the row `web`) becomes `web/test/**`, the directory main has. The paths of the rows do not change (first glob wins:
`web/src/generated/**`, `internal/contracts/**` and `docs/contracts/page-schemas.md` are `contracts`; `web/scripts/**` and `web/package.json`
are `web`). `scripts/arch` reads the entry cell only for `internal/` packages, so `arch.toml` and `.go-arch-lint.yml` do not change.

| issue | module |
|---|---|
| generated-index (interface) | contracts and web (the judged two-module entry; precedent `pages-types-wave-2` in `docs/design/web-views-wave-2.md`) |

One entry, because the contract test, the deleted index and the script that creates a type are one interface: split, the script would
generate a file nothing checks, or the tests would read a directory the script does not fill. The cut lists the two modules:
`contracts/**` is not touched (the schemas stay), the contracts module paths are `web/src/generated/**`, `internal/contracts/**`,
`docs/contracts/page-schemas.md`, and the web module paths are `web/package.json` and `web/scripts/generate-types.mjs`.

No new dependency: the script uses `node:` built-ins and the `json-schema-to-typescript` already in `web/package.json`.

## Interface

Owner: contracts. The contract of the generated TypeScript types, replacing the file `web/src/generated/index.ts`:

- One file per type: `web/src/generated/<name>.ts`, generated from `contracts/schemas/page.<name>.json` (for example
  `macro_rates.v1.ts` from `page.macro_rates.v1.json`), line 1 `// Code generated from contracts/, DO NOT EDIT.`, exporting
  `interface <title>` where `<title>` is the `title` of the schema.
- There is no index file. A consumer imports the file it needs.
- The set of types is the set of `.ts` files of the directory. `pnpm --dir web run generate:types` regenerates every file of the set;
  `pnpm --dir web run generate:types page.<name>` also creates the file of a new schema. A file with no schema, or a schema title that
  differs from the exported name, fails the contract test.
- The set of schemas written in this repository (`pageNewFiles` of `carryover_test.go`) is derived, not listed: every
  `page.*.json` that is not in `carryover.sha256`, not in `pageV2Files`, not in `pageV3Files` and not `page.macro_rates.v1.json`
  is new. A new schema touches no shared list in a Go test. Derivation alone would make every file fall into some set, so a new schema has a
  positive test that does not read the manifest: its `$id` is `https://optdesk.invalid/schemas/page.<name>.json` (the carried copies keep
  `https://option-trader.invalid/`). A carried copy whose manifest line was deleted therefore fails on its `$id`, and the count of 29
  carried files and the hash of every manifest line stay as checks.
- The v3 body checks (no `null`, every property, const and enum value) run for every `*.v3.ts` file whose schema is not in `carryover.sha256`:
  today `market_summary.v3` and `options_summary.v3`, the two named in the v3 line of `docs/contracts/page-schemas.md`. The carried
  `structure_detail.v3.ts` is not checked by that body (it is not in the v3 line, and nothing here asserts it would pass); the choice is recorded in `## Left out`.
- An offline sandbox has no `web/node_modules`, so a worker there cannot run the command and may write the generated file by hand,
  to match what `json2ts` prints (line 1 the banner, line 2 `/* eslint-disable */`). Whether the bytes match is decided by CI only:
  the `type drift` step of `ci-web.yml`, listed under `## Checked by CI` below.
- Residual shared file, named: `docs/contracts/page-schemas.md` (the table row of a new schema and its "Also" paragraph). Two designs
  adding a schema edit adjacent lines; that is a rebase-level conflict, which a rebase resolves and which does not order the entries.
- (interface) entry: the deletion of `web/src/generated/index.ts` and the tests that state the above.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff.

### generated-index (interface)

Delete the hand-written index, make the contract tests read the directory, and make `generate:types` one script.
  Goal: `web/src/generated/index.ts` is gone; `TestGeneratedTypes`, `TestGeneratedV3Types` and the set `pageNewFiles` take their files from the
  directories, not from tables; `generate:types` regenerates every file of `web/src/generated` and creates the file of a named schema;
  `docs/contracts/page-schemas.md` states the rule without a count.
  Files: web/src/generated/index.ts, internal/contracts/generated_ts_test.go, internal/contracts/carryover_test.go, docs/contracts/page-schemas.md, web/scripts/generate-types.mjs, web/package.json.
  Design: docs/design/generated-index.md#interface
  Modules: contracts and web (two-module entry, precedent `pages-types-wave-2`).
  Acceptance:
  1. `test ! -e web/src/generated/index.ts` succeeds, and `grep -rn "generated'" web/src web/test` finds no import of the directory itself (every import names a file).
  2. `internal/contracts/generated_ts_test.go` declares `func TestGeneratedTypes` (`grep -c 'TestGeneratedSummaryTypes'` prints 0) which calls
     `t.Parallel()`, lists `web/src/generated` with `os.ReadDir`, and for each `*.ts` file runs a parallel subtest asserting: line 1 is the banner;
     `contracts/schemas/page.<name>.json` exists; the file has a line matching `^export interface <title> ` with `<title>` the `title` read from that
     schema. It also fails when `index.ts` exists and when the directory holds no `.ts` file.
  3. `grep -c -E 'household_summary|market_summary\.v2|macro_summary' internal/contracts/generated_ts_test.go` prints 0 and
     `grep -c 'files := \[\]struct\|pairs := \[\]struct' internal/contracts/generated_ts_test.go` prints 0.
  4. `TestGeneratedV3Types` keeps its body (banner, exported name, no `null`, every property name, const and enum value of the schema and of the
     `common.v2.json` defs it reaches) and runs it once for every `*.v3.ts` file of the directory whose `page.<name>.json` is not in `carryover.sha256`
     (a subtest per file with `t.Parallel()`); the subtests are exactly `market_summary.v3.ts` and `options_summary.v3.ts` at the head, and
     `structure_detail.v3.ts` has none (it is a carried copy).
  5. In `internal/contracts/carryover_test.go` the variable `pageNewFiles` is gone as a literal list: the schemas it named are derived from the
     directory by the rule of the Interface part (`grep -c 'page.option_data_page.v1.json' internal/contracts/carryover_test.go` prints 0), and the
     test fails for a `page.*.json` file that is neither carried (in `carryover.sha256`) nor new: a derived-new file whose `$id` is not
     `https://optdesk.invalid/schemas/page.<name>.json` fails (test: a copy of a carried schema with its manifest line removed is rejected), and
     `TestCarryoverManifest` keeps the count of 29 carried files and the sha256 of each manifest line.
  6. The test files have no `time.Sleep`, no network call and no fixed path other than the constants `generatedDir` and `schemasDir`; `wc -l` of each is under 600.
  7. `docs/contracts/page-schemas.md`: `grep -c -E 'Thirty|13 directories|15 .bad-version' docs/contracts/page-schemas.md` prints 0; the figure "29 copies"
     and the table of schema ids are unchanged; a sentence states the contract of `## Interface` of this design and
     `grep -c 'generated-index.md' docs/contracts/page-schemas.md` prints 1.
  8. `web/scripts/generate-types.mjs` is new, under 150 lines, and imports only `node:` modules (`grep -c "from '" web/scripts/generate-types.mjs`
     equals `grep -c "from 'node:" web/scripts/generate-types.mjs`).
  9. In `web/package.json` the value of `generate:types` is exactly `node scripts/generate-types.mjs`; `git diff` of the file shows one removed and one
     added line; `grep -c json2ts web/package.json` prints 0 and `json-schema-to-typescript` stays in `devDependencies`.
  10. The script's targets are the base names of every `*.ts` file of `src/generated` other than `index.ts` (a file whose base name does not match
     `^[a-z0-9_]+\.v[0-9]+$` is ignored), together with the command line arguments (`page.<name>` or `<name>`; a form that does not match exits non-zero);
     each target runs `json2ts` once with `-i ../contracts/schemas/page.<name>.json`, `-o src/generated/<name>.ts`, `--cwd ../contracts/schemas` and the
     `--bannerComment` of today, written once as a constant (`grep -c 'Code generated from contracts/, DO NOT EDIT' web/scripts/generate-types.mjs`
     prints 1; the constant also holds `/* eslint-disable */`).
  11. A target whose schema file does not exist ends the script with a non-zero status and a message naming the schema; the script writes no file other
     than the targets (`grep -c 'index.ts' web/scripts/generate-types.mjs` prints 0). It runs `json2ts` through `execFileSync` with an argument array (no
     shell string), from the directory `web`.
  12. The diff holds the six listed files and no other, and the result matches the Interface part of `docs/design/generated-index.md`.
  ## Checked by CI
  - ci-go runs `go test ./internal/contracts/...` with the generated files at the head (the figure is not stated: the three designs ordered before this entry add some) and passes: the v3 body runs on `market_summary.v3.ts` and `options_summary.v3.ts` only, the same two as today.
  - ci-web (tsc, eslint, vitest, build) passes without the index; its `type drift` step (`pnpm --dir web run generate:types`, then
    `git diff --exit-code web/src/generated` and no untracked file) passes, so the script reproduces the committed files byte for byte, and any
    generated file written by hand matches `json2ts`.
  Depends on: home-page-v2-schema (by name, `docs/design/home-page.md`; the dispatcher rewrites it to the issue number at cut time). Order: after home-page-v2-schema.
  Size: M

## Decisions

- Net time: this design does not delay the three open designs (285 macro-rates-view, 286 report-page, 287 home-page). They keep their own
  Order chain (`macro-rates-v2-schema`, then `report-types`, then `home-page-v2-schema`) and their own Files lines. This design serves the
  designs that come after, so its one entry is ordered after `home-page-v2-schema` and never collides with them.
- One entry, not three: the contract test, the deleted index and the script are one interface (see Modules).
- Lock-in, accepted: the name rule `^[a-z0-9_]+\.v[0-9]+$` and the v3 gate of `TestGeneratedV3Types` (a `*.v3.ts` file gets the v3 checks) become
  conventions of the directory; a naming change later is a change of this interface.

## Left out

- Generate every `page.*.json`: no list at all, but 16 dead types of superseded versions and a title collision for an index; the directory as the set is the smaller change. Considered and not chosen.
- A generated `index.ts` (same script): two designs adding a type would both rewrite the same file and conflict in git; and nothing imports it.
- A marker key in each schema (`x-ts`): the 29 carried copies are pinned by `carryover.sha256` and cannot take one.
- A manifest file listing the types: a shared list again.
- Edits to the designs 285, 286 and 287: none. They keep their Order chain, their Files lines (including `index.ts`, `generated_ts_test.go`,
  `carryover_test.go`, `web/package.json`) and their counts; the entries that land first edit the shared lists as today, and this entry,
  ordered after them, deletes the lists and takes what the three added into the directory rule. If one of them changes the lists after this
  entry's cut, a rebase decides.
- The table and the "Also `page.<name>`" paragraphs of `docs/contracts/page-schemas.md` stay, named as the residual rebase-level conflict (Interface).
- The v3 body check on the carried `structure_detail.v3.ts`: not run, because the v3 line of `docs/contracts/page-schemas.md` does not name it and passing is not shown; the gate is the manifest, not the file name. Considered and not chosen.
- Row `pairs` for versions other than v3 in `TestGeneratedV3Types`: that body is specific to the v3 conventions; a v2 or v1 type needs no row.
- Splitting the entry into contracts, script and modules entries: rejected for the reason in Modules.

## Adversarial

Answer: `generated-index.adversarial.md`, in the dispatcher's scratchpad.

1. Gap against the goal (carryover_test.go `pageNewFiles`, `page-schemas.md`): adopted. The entry derives `pageNewFiles` from the directory; `page-schemas.md` is named as the residual rebase-level conflict.
2. Generated files written by hand offline: adopted. Stated in Interface; the byte match is `## Checked by CI` (`type drift`).
3. Three entries versus one two-module interface entry: adopted. One entry, precedent `pages-types-wave-2`; the `modules.md` cell edit moves into this PR.
4. Lock-in of the name regex and the v3 gate: noted in Decisions, not changed.
5. Script acceptance grepped `index` and a false premise about counts kept by other designs: adopted. The grep is `index.ts`; the premise is dropped.
6. Design-session points (earlier finding 1, `structure_detail.v3.ts` in the v3 gate, new-schema `$id` test against the dead 'neither set' check, the figure 14): applied.
Net time (the dispatcher's point e): adopted; the three open designs are not made to depend on this one (Decisions, Left out).
