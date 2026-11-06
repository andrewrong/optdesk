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

## Invariants

- The 29 copies are unchanged except the `x-timezone` substitution (every value is `owner`, checked by the test); the hash of each edited copy is in the manifest, and `internal/contracts/carryover_test.go` fails on a file without a line, a line without a file, or a hash that differs.
- The keywords `x-decimals`, `x-timezone` and `x-rounding` are unknown to the compiler and ignored.
- Every fixture of `web/fixtures` validates against the schema that `internal/contracts/fixtures_test.go` maps its directory to (`positions-page` to `page.positions_page.v3.json`), except the 13 `bad-version.json` files, which fail at `/schema_version` with the keyword `const`.
- No schema and no fixture is edited to make a fixture pass.
- The `const` of `schema_version` is an integer in the copies and a string in `page.macro_rates.v1`; unifying it is for week 2.

## Version history

- v1: the 29 carried schemas (`page.macro_rates.v1` was added before them), the manifest and the fixture validation test.
