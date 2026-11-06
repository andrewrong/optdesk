# page schemas contract

## Responsibility

Hold the JSON schemas of the pages in `contracts/schemas/page.*.json`. Thirty schemas are listed: `page.macro_rates.v1` is written for this repository, and the 29 others are copies of `docs/schemas/<name>.v<N>.json` of the old option-trader repository at commit b00b4ed, the commit the web migration used. The copies are unchanged except that every `x-timezone` value is the literal string `owner` (a privacy rule of this public repository), and the week-2 pass writes new versions only.

## Inputs

- `contracts/schemas/carryover.sha256`: one line `<sha256>  contracts/schemas/<file>` for each of the 29 copies, in file name order. `page.macro_rates.v1.json` is not in it.
- `web/fixtures/<dir>/*.json`: the page fixtures that the migrated front end follows, 13 directories.

## Outputs

The schema ids with the `const` of their `schema_version`:

| schema id | schema_version |
|---|---|
| page.advice_page.v1 | 1 |
| page.companies_page.v1 | 1 |
| page.full_scan.v1 | 1 |
| page.held_companies_summary.v1 | 1 |
| page.home_page.v1 | 1 |
| page.household_page.v1 | 1 |
| page.household_risk.v1 | 1 |
| page.household_summary.v1 | 1 |
| page.indicator_detail.v1 | 1 |
| page.ledger_page.v1 | 1 |
| page.macro_page.v1 | 1 |
| page.macro_rates.v1 | "1" (a string) |
| page.macro_summary.v1 | 1 |
| page.market_summary.v1 | 1 |
| page.market_weather.v1 | 1 |
| page.option_page.v1 | 1 |
| page.option_result_page.v1 | 1 |
| page.options_summary.v1 | 1 |
| page.positions_page.v1 | 1 |
| page.positions_page.v2 | 2 |
| page.positions_page.v3 | 3 |
| page.report_page.v1 | 1 |
| page.report_page.v2 | 2 |
| page.settings_page.v1 | 1 |
| page.stock_option_page.v1 | 1 |
| page.structure_detail.v1 | 1 |
| page.structure_detail.v2 | 2 |
| page.structure_detail.v3 | 3 |
| page.structures_list.v1 | 1 |
| page.tracking.v1 | 1 |

Also `common.v1` (`contracts/schemas/common.v1.json`, `$id` `https://optdesk.invalid/schemas/common.v1.json`): only `$defs` `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `StaleLabel`, `Gap`, `UsdFigure`, `PercentFigure`, `SharePercentFigure`, `UsdSignedFigure`, used as `common.v1.json#/$defs/<Name>`.

## Invariants

Conventions of every file of the v2 line (`page.<name>.v2.json`, checked by `TestPageV2Conventions`):

1. `schema_version` is `{"const": 2}` with `"type": "integer"`, the number being that of the file name.
2. `stale_after` is an optional top-level property, `$ref` `Timestamp`: declared, not in `required`.
3. The page uses the shared defs of `common.v1` by `$ref` and has no local def of the same name, except the local `Gap` of `options_summary`.
4. No def name contains `__`; a def named `option_trader__viewmodels__<module>__<Name>` is renamed `<ModulePascal><Name>`, and one with a counterpart in `common.v1` becomes the `$ref`.
5. `ROUND_HALF_UP` does not occur; every `x-rounding` value is `ROUND_HALF_EVEN`. A money value stays a JSON number.
6. Every `x-timezone` value is `owner`.

Invariants of the carried files:

- The 29 copies are unchanged except the `x-timezone` substitution (every value is `owner`, checked by the test); the hash of each edited copy is in the manifest, and `internal/contracts/carryover_test.go` fails on a file without a line, a line without a file, or a hash that differs.
- The keywords `x-decimals`, `x-timezone` and `x-rounding` are unknown to the compiler and ignored.
- Every fixture of `web/fixtures` validates against the schema that `internal/contracts/fixtures_test.go` maps its directory to (`positions-page` to `page.positions_page.v3.json`), except the 15 `bad-version.json` files, which fail at `/schema_version` with the keyword `const`.
- No schema and no fixture is edited to make a fixture pass.
- The `const` of `schema_version` is an integer in the copies and a string in `page.macro_rates.v1`; unifying it is for week 2.

## Version history

- v1: the 29 carried schemas (`page.macro_rates.v1` was added before them), the manifest and the fixture validation test.
- v2 line: `common.v1` is added; the `page.<name>.v2.json` files of the five summaries and `home_page` are added by later issues, and the existing files are unchanged.
- v2 TypeScript types: the types of `page.market_summary.v2` and `page.options_summary.v2` are generated into `web/src/generated/` by the web script `generate:types` (`json-schema-to-typescript` 16.0.0), not by `contracts/generate.sh`; fixtures `web/fixtures/market-summary` and `web/fixtures/options-summary` are checked against these files.
