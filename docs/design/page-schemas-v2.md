# Page schemas vN+1: the cleanup pass

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1.md` (the week-2 issue "Page schemas vN+1: one cleanup pass"),
`docs/design/week1-wiring.md` (schemas-28, which carried the 29 schemas over unchanged), `docs/contracts/page-schemas.md`
(its invariants and version history) and `docs/contracts/00-protocol.md` (version rule), cut against origin/main at 164d783.

Goal of the design: `contracts/schemas` holds a shared `common.v1.json` and, for each of the 25 pages that stay in use, one NEW
file `page.<name>.v<N+1>.json` that follows the conventions of `## Interface`. Every existing `page.*.json` stays byte-identical, so
`contracts/schemas/carryover.sha256` still holds. The front end keeps reading the v1/v2/v3 files; moving it is left out.

## Modules

No change to `docs/design/modules.md`: every path below belongs to the row `contracts` (`contracts/**`, `internal/contracts/**`,
`docs/contracts/page-schemas.md`).

| issue | module | order |
|---|---|---|
| common-v1 | contracts (interface) | first |
| schemas-summaries-home | contracts | after common-v1, parallel |
| schemas-household-positions | contracts | after common-v1, parallel |
| schemas-report-ledger | contracts | after common-v1, parallel |
| schemas-macro-market | contracts | after common-v1, parallel |
| schemas-options-misc | contracts | after common-v1, parallel |

`common-v1` is the one entry marked (interface): AGENTS.md calls the JSON schema of a page the TypeScript interface, and
`common.v1.json` is referenced by every page file after it. The five other issues need `common-v1` and nothing from each other:
they edit disjoint files, and the one cross-page reference (`home_page` to the five summaries) sits in the same issue as the five
summaries. No issue touches `web/`, `internal/pages/`, `go.mod`, `go.sum` or `contracts/generate.sh`; the Go tests use the
santhosh-tekuri/jsonschema/v6 library that is already a direct dependency.

## Interface

Frozen by `common-v1`, read by the five issues after it.

Files and names:

- `contracts/schemas/common.v1.json`, `$id` `https://optdesk.invalid/schemas/common.v1.json`, holds only `$defs`. A page file
  refers to a def as `common.v1.json#/$defs/<Name>`; the compiler resolves it by the `$id` base of the page.
- Every new page file has `$id` `https://optdesk.invalid/schemas/<file name>` and is named `page.<name>.v<N>.json` with these
  numbers (the next number after the highest file of the page; the five dead versions are not carried, see `## Left out`):
  `advice_page` 2, `companies_page` 2, `full_scan` 2, `held_companies_summary` 2, `home_page` 2, `household_page` 2,
  `household_risk` 2, `household_summary` 2, `indicator_detail` 2, `ledger_page` 2, `macro_page` 2, `macro_rates` 2,
  `macro_summary` 2, `market_summary` 2, `market_weather` 2, `option_page` 2, `option_result_page` 2, `options_summary` 2,
  `positions_page` 4, `report_page` 3, `settings_page` 2, `stock_option_page` 2, `structure_detail` 4, `structures_list` 2,
  `tracking` 2.
- Fixtures of a new page: `contracts/fixtures/pages-v2/page.<name>.v<N>.valid.json` and `.bad-version.json` (not `web/fixtures`,
  which belongs to the web module).

`$defs` of `common.v1.json`. Each body is the body the carried copies share, copied without change, except where the third column says so:

| def | meaning | change from the carried copies |
|---|---|---|
| `Id` | string of 1 to 64 characters of `A-Za-z0-9_-` (the pattern of `run_id`, `build_id`, `scope`, `SourceTime.source`) | none |
| `Timestamp` | date-time string with the fixed-offset pattern of the carried copies, `x-timezone` `owner` | none |
| `RunStatus` | enum `ok`, `partial`, `failed` | none |
| `SourceTime` | `{source: Id, data_time: Timestamp}` | `$ref` to `Id` and `Timestamp` instead of inline patterns |
| `StaleLabel` | `{data_time, stale_after, text}` | `$ref` to `Timestamp` |
| `Gap` | `{reason, text}`, the 13-reason enum of the 24 pages that share it | none |
| `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure` | the four figure defs that every carried page defining them defines identically | `x-rounding` is `ROUND_HALF_EVEN` |

Conventions of every new page file (checked by the generic test of `common-v1`, which skips a file that does not exist yet):

1. `schema_version` is `{"const": N}` with `"type": "integer"`, N the number in the file name. `macro_rates` moves from the string
   `"1"` to the integer.
2. `stale_after` is a required top-level property, `$ref` `Timestamp`. Go emits it; the contract fixes the field and its type, not the
   rule that computes its value (see `## Left out`). The existing `stale` block property of a page stays as it is.
3. The page uses the shared defs by `$ref` and has no local def of the same name.
4. No def name contains `__`. A def named `option_trader__viewmodels__<module>__<Name>` is renamed `<ModulePascal><Name>`, the module
   name as written in snake case turned to Pascal case (`report_open__Accounts` gives `ReportOpenAccounts`,
   `household_summary__Tiles` gives `HouseholdSummaryTiles`). A def that has a `common.v1.json` counterpart is replaced by the `$ref`.
5. `ROUND_HALF_UP` does not occur; every `x-rounding` value is `ROUND_HALF_EVEN`. A money value stays a JSON number.
6. Every `x-timezone` value is `owner`.

Go names in `internal/contracts` test files, used by the five issues: `pageV2Files` (the 25 file names above, in a `var`),
`compilePageV2(t, name)` (compiles `contracts/schemas/<name>.json` with a loader that resolves `https://optdesk.invalid/schemas/<file>`
to the file of `contracts/schemas`) and `checkPageV2(t, name)` (the three checks of the issue entries).

Not frozen: tagged unions, figure register id and version, unit vocabulary (all in `## Left out`).

## Issues

`common-v1` has no dependency. The five others depend on `common-v1` and on nothing else. Items under "Checked by CI" are decided
by the workflows of the head commit; the other items are read from the diff.

### common-v1

(interface) The shared schema, the generic convention test and the helpers of `## Interface`.
  Goal: a page file can `$ref` the shared defs, and the 25 planned file names are known to the carry-over test, so that adding one of them does not break it.
  Files: contracts/schemas/common.v1.json, internal/contracts/common_test.go, internal/contracts/carryover_test.go, docs/contracts/page-schemas.md.
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): the pull request body carries `Contracts: common.v1 (new)`, as `.github/workflows/checks.yml` requires for a change under `contracts/schemas`.
  Acceptance:
  - `contracts/schemas/common.v1.json` is valid JSON with `$id` `https://optdesk.invalid/schemas/common.v1.json` and `$defs` with exactly the ten names `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `StaleLabel`, `Gap`, `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure`; `grep -c 'ROUND_HALF_UP' contracts/schemas/common.v1.json` prints 0.
  - `grep -n 'func TestCommonDefs' internal/contracts/common_test.go` finds the function: for each of the ten defs a table row holds one valid and one invalid instance, typed in the test, and the test asserts both outcomes.
  - `grep -n 'func TestPageV2Conventions' internal/contracts/common_test.go` finds the function: for each name of `pageV2Files` whose file exists it asserts `$id`, the integer `const` of `schema_version` equal to the number of the file name, `stale_after` in `required` and in `properties`, no def name with `__`, no `ROUND_HALF_UP`, and every `x-timezone` value `owner`; a name whose file does not exist is skipped, not failed.
  - `grep -n 'func TestCheckPageV2Helper' internal/contracts/common_test.go` finds the function: it runs `checkPageV2` on a page schema and fixtures written to `t.TempDir()` and asserts that `checkPageV2` reports (through a recording `testing.TB` fake or a sub-test that is expected to fail) a missing `stale_after`, a wrong `schema_version` and a valid instance.
  - `internal/contracts/common_test.go` declares `pageV2Files` with the 25 names of `## Interface`, `compilePageV2` and `checkPageV2`; `checkPageV2` validates `contracts/fixtures/pages-v2/<name>.valid.json` (must pass), the same instance with `stale_after` removed in memory (must fail at the instance root with keyword `required`) and `<name>.bad-version.json` (must fail at `/schema_version` with keyword `const`).
  - `internal/contracts/carryover_test.go` skips the names of `pageV2Files` in `carriedFiles` and changes nothing else: `TestCarryoverManifest` still expects 29 files and the manifest `contracts/schemas/carryover.sha256` is not in the diff; no existing `contracts/schemas/page.*.json` is in the diff.
  - `docs/contracts/page-schemas.md` keeps its five headings, lists `common.v1` and the 25 new ids with their `schema_version` const under `## Outputs`, lists the six conventions under `## Invariants`, and gets a `v2 line` entry under `## Version history` that says the new files are added by later issues and the existing files are unchanged.
  - `grep -c 't.Parallel()' internal/contracts/common_test.go` is at least 3; no test writes outside `t.TempDir()`, sleeps or connects to a network.
  - `internal/contracts/common_test.go` is under 600 lines.
  - Checked by CI: ci-go (the tests above, including `TestCarryoverManifest` still green); checks (`Contracts: common.v1 (new)`, generated code unchanged because nothing is generated from these files); hygiene.
  Depends on: none.
  Size: M

### schemas-summaries-home

  Goal: the five summary pages and the home page that embeds them have vN+1 files that use the common defs, and `home_page` refers to the five summaries instead of repeating them.
  Files: contracts/schemas/page.held_companies_summary.v2.json, contracts/schemas/page.household_summary.v2.json, contracts/schemas/page.macro_summary.v2.json, contracts/schemas/page.market_summary.v2.json, contracts/schemas/page.options_summary.v2.json, contracts/schemas/page.home_page.v2.json, internal/contracts/pagev2_summaries_test.go, and for each of these six pages the files `contracts/fixtures/pages-v2/page.<name>.v2.valid.json` and `contracts/fixtures/pages-v2/page.<name>.v2.bad-version.json` (12 fixture files).
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): the pull request body carries `Contracts: ` followed by the six ids `page.<name>.v2 (new)`, comma separated; the six new schema files make it required.
  Acceptance:
  - The six schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; the six existing v1 files are not in the diff.
  - `grep -n 'func TestPageV2Summaries' internal/contracts/pagev2_summaries_test.go` finds the function; it calls `checkPageV2` for each of the six names in subtests, and `grep -c 't.Parallel()' internal/contracts/pagev2_summaries_test.go` is at least 2.
  - The 12 fixture files exist under `contracts/fixtures/pages-v2/` with the names above and contain no account id, path, e-mail or token.
  - In `page.home_page.v2.json`, the properties `companies`, `household`, `macro`, `market` and `options` are `$ref`s to the file names `page.held_companies_summary.v2.json`, `page.household_summary.v2.json`, `page.macro_summary.v2.json`, `page.market_summary.v2.json` and `page.options_summary.v2.json`; `grep -o 'page\.[a-z_]*_summary\.v2\.json' contracts/schemas/page.home_page.v2.json | sort -u | wc -l` prints 5.
  - In each of the six files `grep -c 'option_trader__' <file>` prints 0 and `grep -c 'ROUND_HALF_UP' <file>` prints 0; `StaleLabel`, `SourceTime`, `Gap` and the figure defs that the file uses are `$ref`s to `common.v1.json`, not local defs.
  - `TestPageV2Conventions` of `internal/contracts/common_test.go` passes with the six files present (integer `schema_version` 2, required `stale_after`, `owner` time zone).
  - The test file is under 600 lines; no file of `web/`, `internal/pages/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go (the tests); checks (the `Contracts:` line above, generated code unchanged); hygiene.
  Depends on: common-v1.
  Size: L

### schemas-household-positions

  Goal: `household_page`, `household_risk`, `positions_page` (v4) and `structure_detail` (v4) have files that follow the conventions.
  Files: contracts/schemas/page.household_page.v2.json, contracts/schemas/page.household_risk.v2.json, contracts/schemas/page.positions_page.v4.json, contracts/schemas/page.structure_detail.v4.json, internal/contracts/pagev2_household_test.go, and for each of the four pages the files `contracts/fixtures/pages-v2/page.<name>.v<N>.valid.json` and `contracts/fixtures/pages-v2/page.<name>.v<N>.bad-version.json` (8 fixture files, N as in `## Interface`).
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): `Contracts: ` followed by `page.household_page.v2 (new), page.household_risk.v2 (new), page.positions_page.v4 (new), page.structure_detail.v4 (new)`.
  Acceptance:
  - The four schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; no existing `page.household_page.*`, `page.household_risk.*`, `page.positions_page.*` or `page.structure_detail.*` file is in the diff.
  - `grep -n 'func TestPageV2Household' internal/contracts/pagev2_household_test.go` finds the function; it calls `checkPageV2` for the four names, and `grep -c 't.Parallel()' internal/contracts/pagev2_household_test.go` is at least 2.
  - The 8 fixture files exist with the names above and contain no account id, path, e-mail or token.
  - `grep -c 'ROUND_HALF_UP' <file>` prints 0 for each of the four files; each uses `$ref` to `common.v1.json` for `StaleLabel`, `SourceTime`, `Gap` and the four figure defs wherever it uses them, with no local def of those names.
  - `TestPageV2Conventions` passes with the four files present.
  - The test file is under 600 lines; no file of `web/`, `internal/pages/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go; checks (the `Contracts:` line above); hygiene.
  Depends on: common-v1.
  Size: L

### schemas-report-ledger

  Goal: `report_page` (v3), `ledger_page` and `stock_option_page` have files that follow the conventions, including the renamed Python-path defs of the report.
  Files: contracts/schemas/page.report_page.v3.json, contracts/schemas/page.ledger_page.v2.json, contracts/schemas/page.stock_option_page.v2.json, internal/contracts/pagev2_report_test.go, contracts/fixtures/pages-v2/page.report_page.v3.valid.json, contracts/fixtures/pages-v2/page.report_page.v3.bad-version.json, contracts/fixtures/pages-v2/page.ledger_page.v2.valid.json, contracts/fixtures/pages-v2/page.ledger_page.v2.bad-version.json, contracts/fixtures/pages-v2/page.stock_option_page.v2.valid.json, contracts/fixtures/pages-v2/page.stock_option_page.v2.bad-version.json.
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): `Contracts: ` followed by `page.report_page.v3 (new), page.ledger_page.v2 (new), page.stock_option_page.v2 (new)`.
  Acceptance:
  - The three schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; no existing `page.report_page.*`, `page.ledger_page.*` or `page.stock_option_page.*` file is in the diff.
  - `grep -n 'func TestPageV2Report' internal/contracts/pagev2_report_test.go` finds the function; it calls `checkPageV2` for the three names, and `grep -c 't.Parallel()' internal/contracts/pagev2_report_test.go` is at least 2.
  - The six fixture files exist with the names in the Files line and contain no account id, path, e-mail or token.
  - `grep -c 'option_trader__' contracts/schemas/page.report_page.v3.json` prints 0, and the six defs of `page.report_page.v2.json` that carried the prefix appear in the new file as `ReportAccountMovesAccounts`, `ReportAccountMovesMeter`, `ReportActionsCandidates`, `ReportOpenAccounts`, `ReportOpenCandidates`, `ReportOpenMeter`.
  - `grep -c 'ROUND_HALF_UP' <file>` prints 0 for each of the three files; shared defs are `$ref`s to `common.v1.json` with no local def of the same name.
  - `TestPageV2Conventions` passes with the three files present.
  - The test file is under 600 lines; no file of `web/`, `internal/pages/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go; checks (the `Contracts:` line above); hygiene.
  Depends on: common-v1.
  Size: L

### schemas-macro-market

  Goal: `macro_page`, `macro_rates`, `market_weather`, `indicator_detail` and `companies_page` have files that follow the conventions; `macro_rates` moves to the integer `schema_version`.
  Files: contracts/schemas/page.macro_page.v2.json, contracts/schemas/page.macro_rates.v2.json, contracts/schemas/page.market_weather.v2.json, contracts/schemas/page.indicator_detail.v2.json, contracts/schemas/page.companies_page.v2.json, internal/contracts/pagev2_macro_test.go, and for each of the five pages the files `contracts/fixtures/pages-v2/page.<name>.v2.valid.json` and `contracts/fixtures/pages-v2/page.<name>.v2.bad-version.json` (10 fixture files).
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): `Contracts: ` followed by the five ids `page.<name>.v2 (new)`.
  Acceptance:
  - The five schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; no existing `page.macro_page.*`, `page.macro_rates.*`, `page.market_weather.*`, `page.indicator_detail.*` or `page.companies_page.*` file is in the diff.
  - `grep -n 'func TestPageV2Macro' internal/contracts/pagev2_macro_test.go` finds the function; it calls `checkPageV2` for the five names, and `grep -c 't.Parallel()' internal/contracts/pagev2_macro_test.go` is at least 2.
  - The 10 fixture files exist with the names above and contain no account id, path, e-mail or token.
  - `grep -n '"schema_version"' contracts/schemas/page.macro_rates.v2.json` shows a `const` that is the integer 2, not a string; `page.macro_rates.v1.json`, `contracts/fixtures/page.macro_rates.v1.golden.json` and `contracts/fixtures/page.macro_rates.v1.input.json` are not in the diff.
  - `grep -c 'ROUND_HALF_UP' <file>` prints 0 for each of the five files; shared defs are `$ref`s to `common.v1.json` with no local def of the same name.
  - `TestPageV2Conventions` passes with the five files present.
  - The test file is under 600 lines; no file of `web/`, `internal/pages/`, `internal/jobs/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go; checks (the `Contracts:` line above); hygiene.
  Depends on: common-v1.
  Size: M

### schemas-options-misc

  Goal: `full_scan`, `option_page`, `option_result_page`, `structures_list`, `tracking`, `advice_page` and `settings_page` have files that follow the conventions; `advice_page` and `settings_page`, which have no stale block today, gain the required `stale_after`.
  Files: contracts/schemas/page.full_scan.v2.json, contracts/schemas/page.option_page.v2.json, contracts/schemas/page.option_result_page.v2.json, contracts/schemas/page.structures_list.v2.json, contracts/schemas/page.tracking.v2.json, contracts/schemas/page.advice_page.v2.json, contracts/schemas/page.settings_page.v2.json, internal/contracts/pagev2_options_test.go, and for each of the seven pages the files `contracts/fixtures/pages-v2/page.<name>.v2.valid.json` and `contracts/fixtures/pages-v2/page.<name>.v2.bad-version.json` (14 fixture files).
  Design: docs/design/page-schemas-v2.md#interface. Note (not acceptance): `Contracts: ` followed by the seven ids `page.<name>.v2 (new)`.
  Acceptance:
  - The seven schema files exist, are valid JSON, each with `$id` `https://optdesk.invalid/schemas/<its file name>`; no existing file of those seven pages is in the diff.
  - `grep -n 'func TestPageV2Options' internal/contracts/pagev2_options_test.go` finds the function; it calls `checkPageV2` for the seven names, and `grep -c 't.Parallel()' internal/contracts/pagev2_options_test.go` is at least 2.
  - The 14 fixture files exist with the names above and contain no account id, path, e-mail or token.
  - `grep -c '"stale_after"' contracts/schemas/page.advice_page.v2.json` and the same for `page.settings_page.v2.json` are at least 2 (the `required` entry and the property).
  - `grep -c 'ROUND_HALF_UP' <file>` prints 0 for each of the seven files; shared defs are `$ref`s to `common.v1.json` with no local def of the same name.
  - `TestPageV2Conventions` passes with the seven files present.
  - The test file is under 600 lines; no file of `web/`, `internal/pages/` or `contracts/generate.sh` is in the diff.
  - Checked by CI: ci-go; checks (the `Contracts:` line above); hygiene.
  Depends on: common-v1.
  Size: L

## Left out

- Tagged unions instead of `anyOf [X, null]`: the docs name the change but give no tag name, no list of the fields that become unions and no encoding for an absent value. Decided with the owner before a design cuts it; it would be one more vN+1 for every page that has such a field.
- Register figure id and version in every figure: the field names are not specified, and the compute module (week 8-9) owns the register. Later design.
- Unit vocabulary normalised: the target vocabulary is not specified (the carried files mix `USD`, `usd`, `jpy`, `JPY`, `percent` and others, and `macro_rates` has its own enum). Later design, with the compute contract.
- The rule that computes the top-level `stale_after`, and the Go builders that emit it and the integer `schema_version`: the `pages` module, a design after the builders exist (`internal/pages`). Here only the field and its type are frozen.
- Moving the front end to the new files: `web/src/types/*`, `web/fixtures`, and the browser stopping its comparison with `Date.now()` are the web module. The v1/v2/v3 files stay until it has moved; deleting them, deleting `carryover.sha256` and renaming the `$id` host of the carried copies follow that move.
- Generated TypeScript from the schemas: the generator is not on `frontend-whitelist.json`; the owner has not decided. No `web/src/generated` file is added.
- The five dead versions (`positions_page` v1 and v2, `structure_detail` v1 and v2, `report_page` v1): no vN+1 is written for them; the new line starts from the latest version of each page. Their files are not deleted here.
- Fixtures in `web/fixtures` for the new files, and fixtures with realistic content: the new fixtures here are minimal and validate the schema only.
- `docs/contracts/page-schemas.md` is edited by `common-v1` only; the five issues after it do not touch it.

## Adversarial
