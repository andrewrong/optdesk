# Page schema skeleton v2: common defs and the summary pages

Author: advisor of optdesk (second draft, narrowed after the design review)
Source: `## Left out` of `docs/design/week1.md` (the week-2 issue "Page schemas vN+1: one cleanup pass"),
`docs/design/week1-wiring.md` (schemas-28, which carried the 29 schemas over unchanged), `docs/contracts/page-schemas.md`
and `docs/contracts/00-protocol.md` (version rule), cut against origin/main.

Goal of the design: `contracts/schemas` holds a shared `common.v1.json` and, for the five summary pages, NEW files
`page.<name>.v2.json` that use it. Every existing `page.*.json` stays byte-identical, so `contracts/schemas/carryover.sha256`
still holds. The front end keeps reading the v1/v2/v3 files.

This is the structural skeleton of the cleanup pass, not the whole pass: shared defs, one `schema_version` convention, the
optional `stale_after` field, renamed Python-path defs and the rounding annotation. The semantic items (tagged unions, register
figure id and version, unit vocabulary), the other page groups and the front end are in `## Left out`. Under the version rule a
version with no consumer may still be amended in place, so the skeleton is cheap to correct once a Go builder consumes it.

## Modules

No change to `docs/design/modules.md`: every path below belongs to the row `contracts` (`contracts/**`, `internal/contracts/**`,
`docs/contracts/page-schemas.md`).

| issue | module | order |
|---|---|---|
| common-v1 | contracts (interface) | first |
| schemas-summaries-a | contracts | after common-v1, parallel |
| schemas-summaries-b | contracts | after common-v1, parallel |

`common-v1` is the one entry marked (interface): AGENTS.md calls the JSON schema of a page the TypeScript interface, and
`common.v1.json` is referenced by every page file after it. The two other issues need `common-v1` and nothing from each other;
their files are disjoint. No issue touches `web/`, `internal/pages/`, `go.mod`, `go.sum` or `contracts/generate.sh`; the Go tests use
the santhosh-tekuri/jsonschema/v6 library that is already a direct dependency.

## Interface

Frozen by `common-v1`, read by the two issues after it.

- `contracts/schemas/common.v1.json`, `$id` `https://optdesk.invalid/schemas/common.v1.json`, holds only `$defs`. A page file
  refers to a def as `common.v1.json#/$defs/<Name>`.
- A new page file has `$id` `https://optdesk.invalid/schemas/<file name>` and is named `page.<name>.v2.json`.

`$defs` of `common.v1.json`:

| def | meaning | difference from the carried copies |
|---|---|---|
| `Id` | string of 1 to 64 characters of `A-Za-z0-9_-` (the pattern of `run_id`, `build_id`, `scope`, `SourceTime.source`) | none |
| `Timestamp` | string with `format` `date-time` (RFC 3339, any offset), `x-timezone` `owner` | the carried copies use a pattern with one fixed offset; the new def does not carry it |
| `RunStatus` | enum `ok`, `partial`, `failed` | none |
| `SourceTime` | `{source: Id, data_time: Timestamp}` | `$ref` to `Id` and `Timestamp` |
| `StaleLabel` | `{data_time, stale_after, text}` | `$ref` to `Timestamp` |
| `Gap` | `{reason, text}`, the 13-reason enum | none; 24 of the 25 carried pages that define `Gap` use it. `options_summary` has a 3-reason subset (`missing_quote`, `no_data`, `unsupported_currency`); its v2 keeps that subset as a local `Gap`, so no page accepts a reason it did not accept before |
| `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure` | the four figure defs that every carried page defining them defines identically except the rounding annotation | `x-rounding` is `ROUND_HALF_EVEN`; the carried defs say `ROUND_HALF_UP` (14 of the 30 carried files hold it). This is a change of the annotation, which no validator reads |

Conventions of every new page file (checked by the generic test of `common-v1`, which skips a file that does not exist yet):

1. `schema_version` is `{"const": 2}` with `"type": "integer"`, the number being that of the file name.
2. `stale_after` is an OPTIONAL top-level property, `$ref` `Timestamp`; it is declared and not in `required`. `week1.md` asks every page to
   carry `stale_after` emitted by Go, and 9 of the 30 carried pages have no stale block today, among them the five summaries; making it
   required waits for the Go builder that emits it. The existing `stale` block property stays as it is.
3. The page uses the shared defs by `$ref` and has no local def of the same name, except the local `Gap` of `options_summary`.
4. No def name contains `__`. A def named `option_trader__viewmodels__<module>__<Name>` is renamed `<ModulePascal><Name>`
   (`household_summary__Tiles` gives `HouseholdSummaryTiles`); a def that has a counterpart in `common.v1.json` becomes the `$ref`.
5. `ROUND_HALF_UP` does not occur; every `x-rounding` value is `ROUND_HALF_EVEN`. A money value stays a JSON number.
6. Every `x-timezone` value is `owner`.

Go names in `internal/contracts` test files: `pageV2Files` (a `var` with the six file names of the summaries and `home_page`, see
`## Left out`), `compilePageV2(t, name)` (compiles `contracts/schemas/<name>.json` with format assertion and a loader that resolves
`https://optdesk.invalid/schemas/<file>` to the file of `contracts/schemas`) and `checkPageV2(t, name, overrides)`. `checkPageV2`
builds a minimal instance in memory from the `required` lists of the schema (const, first enum value, `date-time` and plain
strings, numbers at their minimum, `$ref` followed); a string with a `pattern` takes its value from `overrides`, a map typed in the
calling test. It asserts that the instance is valid, that the same instance with `schema_version` changed fails at `/schema_version`
with keyword `const`, and that it still validates with `stale_after` absent. No fixture file is written.

## Issues

`common-v1` has no dependency; the two others depend on `common-v1` and on nothing else. Items under "Checked by CI" are decided
by the workflows of the head commit; the other items are read from the diff.

### common-v1

(interface) The shared schema, the generic convention test and the helpers of `## Interface`.
  Goal: a page file can `$ref` the shared defs, and the planned file names are known to the carry-over test, so that adding one does not break it.
  Files: contracts/schemas/common.v1.json, internal/contracts/common_test.go, internal/contracts/carryover_test.go, docs/contracts/page-schemas.md.
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): the pull request body carries `Contracts: common.v1 (new)`, as `.github/workflows/checks.yml` requires for a change under `contracts/schemas`.
  Acceptance:
  - `contracts/schemas/common.v1.json` is valid JSON with `$id` `https://optdesk.invalid/schemas/common.v1.json` and `$defs` with exactly the ten names `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `StaleLabel`, `Gap`, `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure`; `grep -c 'ROUND_HALF_UP' contracts/schemas/common.v1.json` prints 0 and `grep -c 'pattern' contracts/schemas/common.v1.json` prints a number that is at most the count of `Id` and the string patterns of the carried figure defs, never the date-time pattern: `grep -c 'd{4}' contracts/schemas/common.v1.json` prints 0.
  - `grep -n 'func TestCommonDefs' internal/contracts/common_test.go` finds the function: for each of the ten defs a table row holds one valid and one invalid instance, typed in the test, and the test asserts both outcomes; the `Timestamp` rows hold an instance with a zero offset and one with a non-zero offset, both valid.
  - `grep -n 'func TestPageV2Conventions' internal/contracts/common_test.go` finds the function: for each name of `pageV2Files` whose file exists it asserts `$id`, the integer `const` 2 of `schema_version`, `stale_after` in `properties` and not in `required`, no def name with `__`, no `ROUND_HALF_UP`, and every `x-timezone` value `owner`; a name whose file does not exist is skipped, not failed.
  - `grep -n 'func TestCheckPageV2Helper' internal/contracts/common_test.go` finds the function: it runs `checkPageV2` on a small page schema written to `t.TempDir()` that has a `$ref` to `common.v1.json`, and asserts the three outcomes of `## Interface` (valid, wrong version fails with `const`, valid without `stale_after`).
  - `internal/contracts/common_test.go` declares `pageV2Files`, `compilePageV2` and `checkPageV2` as in `## Interface`.
  - `internal/contracts/carryover_test.go` skips the names of `pageV2Files` in `carriedFiles` and changes nothing else: `TestCarryoverManifest` still expects 29 files; `contracts/schemas/carryover.sha256` and every existing `contracts/schemas/page.*.json` are unchanged.
  - `docs/contracts/page-schemas.md` keeps its five headings, lists `common.v1` under `## Outputs`, lists the six conventions under `## Invariants`, and adds a `v2 line` entry under `## Version history` that says the new files are added by later issues and the existing files are unchanged.
  - `grep -c 't.Parallel()' internal/contracts/common_test.go` is at least 3; no test writes outside `t.TempDir()`, sleeps or connects to a network; `internal/contracts/common_test.go` is under 600 lines.
  - Checked by CI: ci-go (the tests above, `TestCarryoverManifest` still green); checks (`Contracts: common.v1 (new)`, generated code unchanged because nothing is generated from these files); hygiene.
  Depends on: none.
  Size: M

### schemas-summaries-a

  Goal: `held_companies_summary`, `household_summary` and `macro_summary` have v2 files that use the common defs.
  Files: contracts/schemas/page.held_companies_summary.v2.json, contracts/schemas/page.household_summary.v2.json, contracts/schemas/page.macro_summary.v2.json, internal/contracts/pagev2_summaries_a_test.go.
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): the pull request body carries `Contracts: page.held_companies_summary.v2 (new), page.household_summary.v2 (new), page.macro_summary.v2 (new)`.
  Acceptance:
  - The three schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; no existing `contracts/schemas/page.*.json` is changed.
  - `grep -n 'func TestPageV2SummariesA' internal/contracts/pagev2_summaries_a_test.go` finds the function; it calls `checkPageV2` for the three names in subtests, and `grep -c 't.Parallel()' internal/contracts/pagev2_summaries_a_test.go` is at least 2.
  - In each of the three files `grep -c 'option_trader__' <file>` prints 0 and `grep -c 'ROUND_HALF_UP' <file>` prints 0.
  - Each file refers to `common.v1.json` for `StaleLabel`, `SourceTime`, `Gap` and the figure defs it uses: `grep -c 'common.v1.json#' <file>` is at least 3, and the file has no local def named `StaleLabel`, `SourceTime`, `Gap`, `UsdFigure`, `PercentFigure`, `SharePercentFigure` or `UsdSignedFigure`.
  - `grep -c 'd{4}' <file>` prints 0 for each of the three files (no date-time pattern with a fixed offset).
  - `TestPageV2Conventions` of `internal/contracts/common_test.go` passes with the three files present.
  - The test file is under 600 lines; no file of `web/`, `internal/pages/`, `contracts/fixtures/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go; checks (the `Contracts:` line above); hygiene.
  Depends on: common-v1.
  Size: M

### schemas-summaries-b

  Goal: `market_summary` and `options_summary` have v2 files that use the common defs; `options_summary` keeps its 3-reason `Gap`.
  Files: contracts/schemas/page.market_summary.v2.json, contracts/schemas/page.options_summary.v2.json, internal/contracts/pagev2_summaries_b_test.go.
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): the pull request body carries `Contracts: page.market_summary.v2 (new), page.options_summary.v2 (new)`.
  Acceptance:
  - The two schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; no existing `contracts/schemas/page.*.json` is changed.
  - `grep -n 'func TestPageV2SummariesB' internal/contracts/pagev2_summaries_b_test.go` finds the function; it calls `checkPageV2` for the two names in subtests, and `grep -c 't.Parallel()' internal/contracts/pagev2_summaries_b_test.go` is at least 2.
  - `page.options_summary.v2.json` has a local `Gap` whose `reason` enum is exactly `missing_quote`, `no_data`, `unsupported_currency`; the test asserts that an instance with the reason `stale` in a gap fails against it, using an instance built by the test.
  - In each of the two files `grep -c 'option_trader__' <file>` prints 0 and `grep -c 'ROUND_HALF_UP' <file>` prints 0; `grep -c 'd{4}' <file>` prints 0.
  - `page.market_summary.v2.json` refers to `common.v1.json` for `StaleLabel`, `SourceTime`, `Gap` and the figure defs it uses (`grep -c 'common.v1.json#' contracts/schemas/page.market_summary.v2.json` is at least 3) and has no local def of those names; `page.options_summary.v2.json` does the same except for `Gap`.
  - `TestPageV2Conventions` of `internal/contracts/common_test.go` passes with the two files present.
  - The test file is under 600 lines; no file of `web/`, `internal/pages/`, `contracts/fixtures/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go; checks (the `Contracts:` line above); hygiene.
  Depends on: common-v1.
  Size: M

## Left out

- `home_page` v2 (a `$ref` to the five summary v2 files): it needs all five summary files, so it follows once `schemas-summaries-a` and `-b` land. It is in `pageV2Files` so that the carry-over test already skips its name.
- The other 19 pages (`advice_page`, `companies_page`, `full_scan`, `household_page`, `household_risk`, `indicator_detail`, `ledger_page`, `macro_page`, `macro_rates` with its integer `schema_version`, `market_weather`, `option_page`, `option_result_page`, `positions_page` v4, `report_page` v3, `settings_page`, `stock_option_page`, `structure_detail` v4, `structures_list`, `tracking`): they follow once a Go builder consumes a v2 page, or once tagged unions are specified, so that they are not written twice. Each such design adds its names to `pageV2Files` (an edit of `common_test.go`, an interface file of this design).
- Tagged unions instead of `anyOf [X, null]`: the docs name the change but give no tag name, no list of the fields and no encoding of an absent value.
- Register figure id and version in every figure: the field names are not specified, and the compute module owns the register.
- Unit vocabulary normalised: the target vocabulary is not specified (the carried files mix upper and lower case spellings).
- Making `stale_after` required, the rule that computes it, and the Go builders that emit it: the `pages` module, after a builder exists.
- Moving the front end to the new files (`web/src/types/*`, `web/fixtures`, the browser stopping its comparison with the local clock): the web module. Deleting the v1/v2/v3 files, `carryover.sha256` and renaming the `$id` host of the carried copies follow that move.
- Generated TypeScript from the schemas: the generator is not on `frontend-whitelist.json`; the owner has not decided.
- The five dead versions (`positions_page` v1 and v2, `structure_detail` v1 and v2, `report_page` v1): no next version is written for them; their files are not deleted here.
- Fixture files for the v2 pages: the helper builds a minimal instance, so none are written; realistic fixtures belong to the builder design.

## Adversarial

Source: page-schemas-v2.adversarial.md (kept on the dispatcher's machine).

1. Gap against the goal: adopted. Renamed to "skeleton" and the semantic items are named as open in the opening and in `## Left out`.
2. Biggest unnamed risk (a second rewrite, duplicate `stale_after`): adopted in part. The version rule allows amending before a consumer, stated in the opening; `stale_after` is optional; the unions stay out and the other groups wait for a builder.
3. Simpler way: adopted. The cut is `common-v1` plus the summaries; the four other groups moved to `## Left out`. `home_page` waits because it needs all five summaries.
4. Lock-in: adopted. `stale_after` optional, `Gap` per-page subset kept, no fixed offset in `Timestamp`; the skip list holds six names, not 25.
5. Claim most likely false: adopted. The `Gap` and rounding differences are stated in the table with the page affected; the home-page dependency is removed by moving it out.
6. Likeliest slip: adopted. Issues have at most 3 schemas; fixtures are built by a helper from `required`, no hand-written fixture files.
