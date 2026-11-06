# Schema fixtures: the carried page schemas that have no fixture directory

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1-wiring.md` ("Fixtures for the 17 schemas that have no fixture directory") and
the decision of `docs/design/page-schemas-v2.md` (a minimal instance built from `required` by a test helper; no fixture
file is written). Read on main at c63cbb4.

What was counted on main at c63cbb4:

- `contracts/schemas` holds 30 files named `page.*.json` (and `common.v1.json`, `carryover.sha256`).
- `web/fixtures` holds 13 directories; `fixtureSchema` in `internal/contracts/fixtures_test.go` maps them to 13 schema
  files (`positions-page` maps to `page.positions_page.v3.json`). 30 minus 13 is 17: the number of the earlier design is right.
- One of the 17 is `page.macro_rates.v1.json`, which already has its own fixtures (`contracts/fixtures/page.macro_rates.v1.input.json`
  and `.golden.json`, checked by `TestPageSchema`). The remaining 16 have no fixture of any kind: `home_page.v1`,
  `household_risk.v1`, `indicator_detail.v1`, `ledger_page.v1`, `market_summary.v1`, `option_result_page.v1`,
  `options_summary.v1`, `positions_page.v1`, `positions_page.v2`, `report_page.v1`, `report_page.v2`, `settings_page.v1`,
  `stock_option_page.v1`, `structure_detail.v1`, `structure_detail.v2`, `structure_detail.v3`.
- The helper `minimal` of `internal/contracts/common_test.go` builds a value from `required`. It was tried on all 16 files
  with a table of sample strings for patterns: every one builds and validates. Two limits of the existing `checkPageV2`
  keep it from being used as it is: it sets `schema_version` to 3, which is valid for the v3 files, and it needs an
  override for every pattern (the 30 page files and `common.v1.json` use 10 distinct patterns in all, and two of them occur in nearly every file).
- All 30 carried files have `schema_version` as an integer `const` (1, 2 or 3) and none has `stale_after`.

Goal of the design: each of the 16 schemas has a test that builds the smallest valid instance in memory, validates it, and
checks that a wrong `schema_version` fails with `const`. No fixture file is written, so no personal data, holding or account
value can enter the repository: every string of an instance is `x`, a sample from one table, or the first enum value.

## Modules

No change to `docs/design/modules.md`. Every path below is `internal/contracts/**`, a path of the row `contracts`.

## Interface

No interface change in the sense of `modules.md` (no `api.go`, no schema, no generated file is edited). One test helper is
shared by the four fixture issues and is frozen by `fixture-helper`; the owning module is contracts. It lives in the
package `contracts_test` in `internal/contracts/minimal_test.go`:

- `var minimalSamples map[string]string`: from each `pattern` string that occurs in a `page.*.json` file or in
  `common.v1.json` of `contracts/schemas` to one value that matches it.
- `func checkMinimal(t *testing.T, file string)`: `file` is a name in `contracts/schemas`. It compiles the file as
  `compileCarried` does, builds an instance with `minimal(t, schemasDir, file, raw, minimalSamples)`, and asserts that the
  instance (after a round trip through JSON) is valid, and that the same instance with `schema_version` set to the schema's
  `const` plus 1 fails with a leaf at `/schema_version` whose keyword is `const`.
- `func checkMinimalIn(t *testing.T, dir, file string)`: the same for a file in `dir`; `checkMinimal` calls it with `schemasDir`.

The existing `checkPageV2`, `minimal`, `leaves` and the `v2` tests are used and not changed.

## Issues

`fixture-helper` is the one entry marked (interface); the four others depend on it and on nothing else, and their files are
disjoint, so they run in parallel after it. Every item is read from the diff by `grep -n` or by the named test; the
`go test` runs are under "Checked by CI". Offline, `go vet ./internal/contracts` is the substitute.

### fixture-helper

(interface) The shared sample table and the `checkMinimal` helper, with a test of both.
  Goal: later issues add one table row per schema; the helper reads the version from the schema and needs no override argument.
  Files: internal/contracts/minimal_test.go.
  Depends on: none.
  Acceptance:
  1. `grep -n 'var minimalSamples' internal/contracts/minimal_test.go` finds the table, and it holds exactly the 10 patterns
     that occur in the `page.*.json` files and `common.v1.json` of `contracts/schemas` at c63cbb4 (the two timestamp and date
     patterns, the id pattern, the day, month, path, structure-path, digits, 64-hex and decimal-string patterns).
  2. `grep -n 'func checkMinimal(t \*testing.T, file string)' internal/contracts/minimal_test.go` and
     `grep -n 'func checkMinimalIn(t \*testing.T, dir, file string)' internal/contracts/minimal_test.go` each find one line;
     `checkMinimal` calls `checkMinimalIn` with `schemasDir`.
  3. `checkMinimalIn` reads the `const` of `schema_version` from the raw schema, and fails the test with a message naming the
     file when it is absent or not an integer; it does not hard-code a version number.
  4. `grep -n 'func TestMinimalSamplesCover' internal/contracts/minimal_test.go` finds a test that walks every
     `contracts/schemas/page.*.json` and `common.v1.json`, collects every `pattern` string, fails on one that is not a key of
     `minimalSamples`, and fails on a key that no file uses or whose value does not match its key under `regexp.MustCompile`.
  5. `grep -n 'func TestMinimalHelper' internal/contracts/minimal_test.go` finds a test that writes a small object schema with
     `schema_version` const 3, a pattern property and a `$ref`-free `required` list to `t.TempDir()` and runs `checkMinimalIn`
     on it; no schema of `contracts/schemas` is named in that test.
  6. Every test function of the file calls `t.Parallel()` (`grep -c 't.Parallel()'` is at least the number of `func Test`
     lines), no test calls `time.Sleep`, and the file is under 600 lines.
  7. The diff holds only `internal/contracts/minimal_test.go`; no line of `common_test.go`, `carryover_test.go`,
     `fixtures_test.go`, of `contracts/` or of `web/` is changed. The result matches the `## Interface` part of
     `docs/design/schema-fixtures.md`.
  Checked by CI: `go vet` and `go test ./internal/contracts`.

### fixtures-positions

The positions and structure-detail schemas before the last version.
  Goal: `positions_page.v1`, `positions_page.v2`, `structure_detail.v1` and `structure_detail.v2` have a minimal instance test.
  Files: internal/contracts/minimal_positions_test.go.
  Depends on: fixture-helper.
  Acceptance:
  1. `grep -n 'func TestMinimalPositions' internal/contracts/minimal_positions_test.go` finds one test function.
  2. The file names exactly `page.positions_page.v1.json`, `page.positions_page.v2.json`, `page.structure_detail.v1.json` and
     `page.structure_detail.v2.json`, each once (four string literals), and no other schema.
  3. Each name is run through `t.Run` with `t.Parallel()` in the subtest and in the test, and checked by `checkMinimal(t, file)`.
  4. The file contains no JSON fixture text: `grep -c '"schema_version"' internal/contracts/minimal_positions_test.go` is 0,
     and it writes no file (`grep -c 'os.WriteFile'` is 0).
  5. The file calls no `time.Sleep`, opens no network connection and is under 600 lines.
  6. The diff holds only `internal/contracts/minimal_positions_test.go`. The result matches the `## Issues` part of
     `docs/design/schema-fixtures.md`.
  Checked by CI: `go test ./internal/contracts -run TestMinimalPositions`.

### fixtures-reports

The last structure-detail version, the two report pages and the home page.
  Goal: `structure_detail.v3`, `report_page.v1`, `report_page.v2` and `home_page.v1` have a minimal instance test.
  Files: internal/contracts/minimal_reports_test.go.
  Depends on: fixture-helper.
  Acceptance:
  1. `grep -n 'func TestMinimalReports' internal/contracts/minimal_reports_test.go` finds one test function.
  2. The file names exactly `page.structure_detail.v3.json`, `page.report_page.v1.json`, `page.report_page.v2.json` and
     `page.home_page.v1.json`, each once, and no other schema.
  3. Each name is run through `t.Run` with `t.Parallel()` in the subtest and in the test, and checked by `checkMinimal(t, file)`.
  4. `grep -c '"schema_version"'` and `grep -c 'os.WriteFile'` of the file are both 0.
  5. The file calls no `time.Sleep`, opens no network connection and is under 600 lines.
  6. The diff holds only `internal/contracts/minimal_reports_test.go`. The result matches the `## Issues` part of
     `docs/design/schema-fixtures.md`.
  Checked by CI: `go test ./internal/contracts -run TestMinimalReports`.

### fixtures-household

The household risk, indicator, ledger and market summary pages.
  Goal: `household_risk.v1`, `indicator_detail.v1`, `ledger_page.v1` and `market_summary.v1` have a minimal instance test.
  Files: internal/contracts/minimal_household_test.go.
  Depends on: fixture-helper.
  Acceptance:
  1. `grep -n 'func TestMinimalHousehold' internal/contracts/minimal_household_test.go` finds one test function.
  2. The file names exactly `page.household_risk.v1.json`, `page.indicator_detail.v1.json`, `page.ledger_page.v1.json` and
     `page.market_summary.v1.json`, each once, and no other schema.
  3. Each name is run through `t.Run` with `t.Parallel()` in the subtest and in the test, and checked by `checkMinimal(t, file)`.
  4. `grep -c '"schema_version"'` and `grep -c 'os.WriteFile'` of the file are both 0.
  5. The file calls no `time.Sleep`, opens no network connection and is under 600 lines.
  6. The diff holds only `internal/contracts/minimal_household_test.go`. The result matches the `## Issues` part of
     `docs/design/schema-fixtures.md`.
  Checked by CI: `go test ./internal/contracts -run TestMinimalHousehold`.

### fixtures-options

The option result, options summary, settings and stock-option pages.
  Goal: `option_result_page.v1`, `options_summary.v1`, `settings_page.v1` and `stock_option_page.v1` have a minimal instance test.
  Files: internal/contracts/minimal_options_test.go.
  Depends on: fixture-helper.
  Acceptance:
  1. `grep -n 'func TestMinimalOptions' internal/contracts/minimal_options_test.go` finds one test function.
  2. The file names exactly `page.option_result_page.v1.json`, `page.options_summary.v1.json`, `page.settings_page.v1.json` and
     `page.stock_option_page.v1.json`, each once, and no other schema.
  3. Each name is run through `t.Run` with `t.Parallel()` in the subtest and in the test, and checked by `checkMinimal(t, file)`.
  4. `grep -c '"schema_version"'` and `grep -c 'os.WriteFile'` of the file are both 0.
  5. The file calls no `time.Sleep`, opens no network connection and is under 600 lines.
  6. The diff holds only `internal/contracts/minimal_options_test.go`. The result matches the `## Issues` part of
     `docs/design/schema-fixtures.md`.
  Checked by CI: `go test ./internal/contracts -run TestMinimalOptions`.

Together the four fixture issues name 16 schemas, each once; with the 13 mapped in `fixtureSchema` and `page.macro_rates.v1.json`
this is all 30 `page.*.json` files.

## Left out

- Fixture files for these 16 schemas (`ok`, `gap`, `stale`, `bad-version` as in `web/fixtures`): not chosen. The decision of
  `page-schemas-v2.md` is that a helper builds a minimal instance; a hand-written file needs values, and every value in a
  public repository must be synthetic. Realistic fixtures belong to the builder design of each page, with the page.
- A regular-expression generator library (a new direct dependency) to make samples: not chosen. There are 10 patterns in
  all, one table is checked against them by `TestMinimalSamplesCover`, and a new pattern fails that test with a clear message.
- Changing `checkPageV2` to take a version: not chosen; it is the helper of the v2 line and its test is frozen. `checkMinimal`
  is a second helper that reads the version from the schema. Merging the two follows when the v1/v2/v3 files are deleted.
- `page.macro_rates.v1.json`: has `contracts/fixtures` input and golden files; not repeated.
- The units check of stored series (`pages-builders.md` says the FRED client stores every series as `percent`;
  `internal/sources/fred/mapping.go` sets the unit there): not here, it is the module sources, not contracts. It is a separate
  candidate: a test in `internal/sources/fred` that fails when a configured series would be stored with a unit other than
  its declared one, before any non-percent series is configured.
- Checking an instance of the 13 existing directories against `minimal`: they already have files; the web module owns them.

## Adversarial

pending
